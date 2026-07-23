package themes

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// ErrInvalidCSS 包裹所有 CSS 层面的校验失败。错误文案带上触发的选择器或
// 属性，便于主题作者定位。
var ErrInvalidCSS = errors.New("invalid theme css")

const (
	// MaxCSSBytes 是单个主题 theme.css 的体积上限。
	MaxCSSBytes = 262144
	// MaxDataURIBytes 是单条 data: URI 的长度上限。
	MaxDataURIBytes = 8192
	// MaxZIndex 是主题可用的最大 z-index。这是纵深防御，不是隔离边界——
	// 边界是宿主 wrapper 上的 contain: paint（见 docs/theme-api.md §1）。
	MaxZIndex = 50
	// AssetURLScheme 是包内资产的唯一引用语法：url("asset:<path>")。
	AssetURLScheme = "asset:"
)

// allowedAtRules 是 at-rule 白名单。@layer 刻意不在其中：层序是全局的，
// 无法靠改名隔离。
var allowedAtRules = map[string]bool{
	"media":     true,
	"supports":  true,
	"keyframes": true,
	"font-face": true,
}

// allowedFunctions 是值中允许出现的函数白名单。这是正向白名单而非黑名单：
// CSS 里能触发资源加载的函数不止 url()（src()、image()、image-set()、
// cross-fade()、element() 都可以，且新函数会持续出现），逐个枚举拒绝永远
// 追不上标准。
var allowedFunctions = map[string]bool{
	"var": true, "calc": true, "min": true, "max": true, "clamp": true,
	"rgb": true, "rgba": true, "hsl": true, "hsla": true,
	"oklch": true, "oklab": true, "lch": true, "lab": true, "color-mix": true,
	"linear-gradient": true, "radial-gradient": true, "conic-gradient": true,
	"repeating-linear-gradient": true, "repeating-radial-gradient": true,
	"repeating-conic-gradient": true,
	"cubic-bezier":             true, "steps": true, "url": true, "format": true, "local": true,
	"translate": true, "translatex": true, "translatey": true, "translatez": true,
	"translate3d": true, "scale": true, "scalex": true, "scaley": true, "scalez": true,
	"scale3d": true, "rotate": true, "rotatex": true, "rotatey": true, "rotatez": true,
	"rotate3d": true, "skew": true, "skewx": true, "skewy": true,
	"matrix": true, "matrix3d": true, "perspective": true,
	"blur": true, "brightness": true, "contrast": true, "drop-shadow": true,
	"grayscale": true, "hue-rotate": true, "invert": true, "opacity": true,
	"saturate": true, "sepia": true, "env": true,
	// 选择器位置的函数伪类，单独在选择器校验里处理，这里一并放行避免误伤。
	"is": true, "where": true, "not": true, "has": true,
	"nth-child": true, "nth-last-child": true, "nth-of-type": true,
	"nth-last-of-type": true, "lang": true, "dir": true,
}

// bannedProperties 是老式脚本注入面。
var bannedProperties = map[string]bool{
	"behavior":     true,
	"-moz-binding": true,
}

func invalidCSS(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidCSS, fmt.Sprintf(format, args...))
}

// decodeCSSIdent 解码 CSS 转义并做 ASCII 小写规范化。
//
// 这一步必须显式做：词法器交回的标识符是原文（`h\74 ml` 就是字面的
// `h\74 ml`），不解码就比较等于把 html/class 等黑名单白送。
func decodeCSSIdent(raw string) string {
	var b strings.Builder
	for i := 0; i < len(raw); {
		if raw[i] != '\\' {
			r, size := utf8.DecodeRuneInString(raw[i:])
			b.WriteRune(unicode.ToLower(r))
			i += size
			continue
		}
		i++ // 跳过反斜杠
		if i >= len(raw) {
			break
		}
		// 十六进制转义：最多 6 位，后跟可选的一个空白作为终结符。
		hexEnd := i
		for hexEnd < len(raw) && hexEnd-i < 6 && isHexDigit(raw[hexEnd]) {
			hexEnd++
		}
		if hexEnd > i {
			code, err := strconv.ParseUint(raw[i:hexEnd], 16, 32)
			if err == nil && code > 0 && code <= unicode.MaxRune {
				b.WriteRune(unicode.ToLower(rune(code)))
			}
			i = hexEnd
			if i < len(raw) && (raw[i] == ' ' || raw[i] == '\t' || raw[i] == '\n' || raw[i] == '\r' || raw[i] == '\f') {
				i++
			}
			continue
		}
		// 字面转义：反斜杠后的单个字符原样保留。
		r, size := utf8.DecodeRuneInString(raw[i:])
		b.WriteRune(unicode.ToLower(r))
		i += size
	}
	return b.String()
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// unquote 去掉字符串 token 的引号。
func unquote(raw string) string {
	if len(raw) >= 2 {
		if (raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'') {
			return raw[1 : len(raw)-1]
		}
	}
	return raw
}

// looksLikeURL 判断字符串字面量是否形如 URL。它堵的是不产生 URL token 的
// 通道，例如 image-set("https://…" 1x) 与 --x: "https://…"。
func looksLikeURL(value string) bool {
	lowered := strings.ToLower(strings.TrimSpace(decodeCSSIdent(value)))
	return strings.Contains(lowered, "://") || strings.HasPrefix(lowered, "//")
}

// validator 承载一次校验的可变状态。
type validator struct {
	fontFamilies map[string]bool
	// 包内 @font-face 声明的族名，供 font 简写检查。
	packageFamilies map[string]bool
	atRuleStack     []string
	rulesetDepth    int
	// 当前 ruleset 内累积的声明，用于跨声明检查。
	declarations map[string]string
	// 当前 @font-face 块内收集到的信息。
	fontFaceFamily string
	fontFaceHasSrc bool
	// 收集到的资产引用路径，供 Compile 做存在性交叉检查。
	assetRefs []string
}

// ValidateCSS 按 docs/theme-api.md §5 的规则校验主题 CSS。
// fontFamilies 是 manifest tokens.font 中引用的族名，用于 @font-face 交叉检查。
func ValidateCSS(src []byte, fontFamilies []string) error {
	_, err := validateCSSCollect(src, fontFamilies)
	return err
}

// validateCSSCollect 在校验的同时收集资产引用路径。
func validateCSSCollect(src []byte, fontFamilies []string) ([]string, error) {
	if len(src) > MaxCSSBytes {
		return nil, invalidCSS("CSS 体积 %d 字节超过 %d 字节上限", len(src), MaxCSSBytes)
	}
	v := &validator{
		fontFamilies:    map[string]bool{},
		packageFamilies: map[string]bool{},
		declarations:    map[string]string{},
	}
	for _, family := range fontFamilies {
		v.fontFamilies[strings.ToLower(family)] = true
	}

	// 先扫一遍收集包内 @font-face 族名，font 简写检查需要它。
	if err := v.collectPackageFamilies(src); err != nil {
		return nil, err
	}

	p := css.NewParser(parse.NewInputString(string(src)), false)
	for {
		gt, _, data := p.Next()
		if gt == css.ErrorGrammar {
			if err := p.Err(); err != nil && err.Error() != "EOF" {
				return nil, invalidCSS("解析失败: %v", err)
			}
			break
		}
		if err := v.step(gt, data, p.Values()); err != nil {
			return nil, err
		}
	}
	return v.assetRefs, nil
}

func (v *validator) collectPackageFamilies(src []byte) error {
	p := css.NewParser(parse.NewInputString(string(src)), false)
	inFontFace := false
	for {
		gt, _, data := p.Next()
		if gt == css.ErrorGrammar {
			return nil
		}
		switch gt {
		case css.BeginAtRuleGrammar:
			inFontFace = decodeCSSIdent(strings.TrimPrefix(string(data), "@")) == "font-face"
		case css.EndAtRuleGrammar:
			inFontFace = false
		case css.DeclarationGrammar:
			if inFontFace && decodeCSSIdent(string(data)) == "font-family" {
				for _, token := range p.Values() {
					if token.TokenType == css.StringToken || token.TokenType == css.IdentToken {
						v.packageFamilies[strings.ToLower(unquote(string(token.Data)))] = true
					}
				}
			}
		}
	}
}

func (v *validator) step(gt css.GrammarType, data []byte, values []css.Token) error {
	switch gt {
	case css.AtRuleGrammar, css.BeginAtRuleGrammar:
		return v.atRule(gt, data, values)
	case css.EndAtRuleGrammar:
		if len(v.atRuleStack) > 0 {
			name := v.atRuleStack[len(v.atRuleStack)-1]
			v.atRuleStack = v.atRuleStack[:len(v.atRuleStack)-1]
			if name == "font-face" {
				return v.endFontFace()
			}
		}
		return nil
	case css.BeginRulesetGrammar, css.QualifiedRuleGrammar:
		return v.beginRuleset(values)
	case css.EndRulesetGrammar:
		if v.rulesetDepth > 0 {
			v.rulesetDepth--
		}
		return v.endRuleset()
	case css.DeclarationGrammar:
		return v.declaration(decodeCSSIdent(string(data)), values)
	case css.CustomPropertyGrammar:
		return v.customProperty(string(data), values)
	}
	return nil
}

func (v *validator) atRule(gt css.GrammarType, data []byte, values []css.Token) error {
	name := decodeCSSIdent(strings.TrimPrefix(string(data), "@"))
	switch name {
	case "import":
		return invalidCSS("不允许 @import —— 它可以加载任意外部样式表")
	case "layer":
		return invalidCSS("不允许 @layer —— 层序是全局的，无法按主题隔离")
	}
	if !allowedAtRules[name] {
		return invalidCSS("不支持的 at-rule @%s", name)
	}
	if gt == css.BeginAtRuleGrammar {
		v.atRuleStack = append(v.atRuleStack, name)
		if name == "font-face" {
			v.fontFaceFamily = ""
			v.fontFaceHasSrc = false
		}
	}
	// @media/@supports 的前奏（prelude）不含选择器，无需校验；@keyframes
	// 的名字在编译期做命名空间化。
	_ = values
	return nil
}

func (v *validator) inAtRule(name string) bool {
	for _, entry := range v.atRuleStack {
		if entry == name {
			return true
		}
	}
	return false
}

func (v *validator) beginRuleset(values []css.Token) error {
	// @keyframes 内部的 from/to/百分比不是选择器，跳过。
	if v.inAtRule("keyframes") {
		v.rulesetDepth++
		return nil
	}
	if v.rulesetDepth > 0 {
		return invalidCSS("不允许 CSS 嵌套（nesting）—— v1 不承担其作用域语义")
	}
	v.rulesetDepth++
	v.declarations = map[string]string{}
	return validateSelector(values)
}

func (v *validator) endRuleset() error {
	// 跨声明检查：固定定位必须同时放弃指针事件。这是纵深防御，真正的
	// 边界是宿主 wrapper 的 contain: paint。
	if v.declarations["position"] == "fixed" && v.declarations["pointer-events"] != "none" {
		return invalidCSS("position: fixed 必须同时声明 pointer-events: none")
	}
	v.declarations = map[string]string{}
	return nil
}

func (v *validator) endFontFace() error {
	if v.fontFaceFamily == "" {
		return invalidCSS("@font-face 缺少 font-family")
	}
	if !v.fontFaceHasSrc {
		return invalidCSS("@font-face 缺少 src")
	}
	if !v.fontFamilies[v.fontFaceFamily] {
		return invalidCSS("@font-face 的 font-family %q 未被 tokens.font 引用", v.fontFaceFamily)
	}
	return nil
}

func (v *validator) declaration(property string, values []css.Token) error {
	if bannedProperties[property] {
		return invalidCSS("不允许属性 %s", property)
	}
	if err := v.validateValueTokens(property, values); err != nil {
		return err
	}
	if v.inAtRule("font-face") {
		switch property {
		case "font-family":
			for _, token := range values {
				if token.TokenType == css.StringToken || token.TokenType == css.IdentToken {
					v.fontFaceFamily = strings.ToLower(unquote(string(token.Data)))
				}
			}
		case "src":
			v.fontFaceHasSrc = true
		}
		return nil
	}

	switch property {
	case "content":
		return validateContent(values)
	case "z-index":
		return validateZIndex(values)
	case "font":
		// v1 不解析 font 简写：包内字体只能用 font-family 引用。
		for _, token := range values {
			if token.TokenType == css.StringToken || token.TokenType == css.IdentToken {
				if v.packageFamilies[strings.ToLower(unquote(string(token.Data)))] {
					return invalidCSS("font 简写不得引用包内字体，请改用 font-family")
				}
			}
		}
	}
	v.declarations[property] = strings.ToLower(strings.TrimSpace(joinTokens(values)))
	return nil
}

// customProperty 校验自定义属性。词法器把值交回为单个不透明 token，
// 因此必须重新 lex 后套用同一套值规则——否则
// `--x: url(https://…)` + `background: var(--x)` 就是等价的外链通道。
func (v *validator) customProperty(name string, values []css.Token) error {
	raw := joinTokens(values)
	lexer := css.NewLexer(parse.NewInputString(raw))
	var relexed []css.Token
	for {
		tt, text := lexer.Next()
		if tt == css.ErrorToken {
			break
		}
		relexed = append(relexed, css.Token{TokenType: tt, Data: text})
	}
	return v.validateValueTokens(decodeCSSIdent(name), relexed)
}

func (v *validator) validateValueTokens(property string, values []css.Token) error {
	for _, token := range values {
		switch token.TokenType {
		case css.FunctionToken:
			name := decodeCSSIdent(strings.TrimSuffix(string(token.Data), "("))
			if !allowedFunctions[name] {
				return invalidCSS("不允许函数 %s()（属性 %s）—— 值中的函数走正向白名单", name, property)
			}
		case css.BadURLToken:
			return invalidCSS("属性 %s 的 url() 无法解析", property)
		case css.URLToken:
			if err := v.validateURL(property, string(token.Data)); err != nil {
				return err
			}
		case css.StringToken:
			if looksLikeURL(string(token.Data)) {
				return invalidCSS("属性 %s 的字符串字面量形如 URL，不允许外部地址", property)
			}
		}
	}
	return nil
}

func (v *validator) validateURL(property, raw string) error {
	inner := strings.TrimSpace(raw)
	if lower := strings.ToLower(inner); strings.HasPrefix(lower, "url(") {
		inner = strings.TrimSuffix(inner[len("url("):], ")")
	}
	inner = strings.TrimSpace(unquote(strings.TrimSpace(inner)))

	switch {
	case strings.HasPrefix(strings.ToLower(inner), AssetURLScheme):
		path := strings.TrimPrefix(inner[len(AssetURLScheme):], "/")
		if path == "" {
			return invalidCSS("属性 %s 的 asset: 引用缺少路径", property)
		}
		v.assetRefs = append(v.assetRefs, path)
		return nil
	case strings.HasPrefix(strings.ToLower(inner), "data:"):
		return validateDataURI(property, inner)
	default:
		display := inner
		if len(display) > 64 {
			display = display[:64] + "…"
		}
		return invalidCSS("属性 %s 不允许外部 url(%q)，请改用 url(\"asset:…\")", property, display)
	}
}

func validateDataURI(property, value string) error {
	if len(value) > MaxDataURIBytes {
		return invalidCSS("属性 %s 的 data: URI 长度 %d 超过 %d 字节上限", property, len(value), MaxDataURIBytes)
	}
	lower := strings.ToLower(value)
	for _, mime := range []string{"data:image/png", "data:image/jpeg", "data:image/webp"} {
		if strings.HasPrefix(lower, mime) {
			return nil
		}
	}
	if strings.HasPrefix(lower, "data:image/svg") {
		return invalidCSS("属性 %s 不允许 svg —— 与资产层拒绝 SVG 保持一致", property)
	}
	return invalidCSS("属性 %s 的 data: URI 只允许 image/png、image/jpeg、image/webp", property)
}

func validateContent(values []css.Token) error {
	meaningful := meaningfulTokens(values)
	if len(meaningful) == 1 {
		token := meaningful[0]
		if token.TokenType == css.StringToken && unquote(string(token.Data)) == "" {
			return nil
		}
		if token.TokenType == css.IdentToken && decodeCSSIdent(string(token.Data)) == "none" {
			return nil
		}
	}
	return invalidCSS(`伪元素 content 只允许 "" 与 none —— 非空 content 是文本注入与钓鱼文案的通道`)
}

func validateZIndex(values []css.Token) error {
	meaningful := meaningfulTokens(values)
	if len(meaningful) != 1 || meaningful[0].TokenType != css.NumberToken {
		return invalidCSS("z-index 必须是整数字面量且不超过 %d", MaxZIndex)
	}
	value, err := strconv.Atoi(string(meaningful[0].Data))
	if err != nil {
		return invalidCSS("z-index 必须是整数字面量且不超过 %d", MaxZIndex)
	}
	if value > MaxZIndex {
		return invalidCSS("z-index %d 超过上限 %d", value, MaxZIndex)
	}
	return nil
}

func meaningfulTokens(values []css.Token) []css.Token {
	out := make([]css.Token, 0, len(values))
	for _, token := range values {
		if token.TokenType == css.WhitespaceToken || token.TokenType == css.CommentToken {
			continue
		}
		out = append(out, token)
	}
	return out
}

func joinTokens(values []css.Token) string {
	var b strings.Builder
	for _, token := range values {
		b.Write(token.Data)
	}
	return b.String()
}
