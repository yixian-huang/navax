package themes

import (
	"fmt"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// AssetBasePlaceholder 是编译产物中资产 URL 的占位前缀。
//
// 它存在是为了打破一个循环：资产的最终 URL 含版本 ID，而版本 ID 是编译
// 产物的哈希。编译期先写占位符并据此算哈希，得到版本 ID 后再替换成真实
// 路径——哈希因此不依赖自身。
const AssetBasePlaceholder = "__NAVAX_THEME_ASSET_BASE__/"

// CompileCSS 把主题 CSS 编译成可直接下发的产物：
//   - 每条选择器加上 [data-theme="<scope>"] 作用域；根引用改写为根自身
//   - @keyframes 名与包内 @font-face 族名做命名空间化，并同步改写引用
//   - url("asset:p") 改写为 AssetBasePlaceholder + p
//
// 同一输入必须逐字节稳定输出——版本 ID 由此哈希得来。
func CompileCSS(src []byte, scope string) ([]byte, error) {
	families, err := collectFontFaceFamilies(src)
	if err != nil {
		return nil, err
	}
	c := &compiler{scope: scope, packageFamilies: families}
	return c.run(src)
}

func collectFontFaceFamilies(src []byte) (map[string]bool, error) {
	v := &validator{packageFamilies: map[string]bool{}}
	if err := v.collectPackageFamilies(src); err != nil {
		return nil, err
	}
	return v.packageFamilies, nil
}

type compiler struct {
	scope           string
	packageFamilies map[string]bool
	out             strings.Builder
	atRuleStack     []string
	rulesetDepth    int
}

func (c *compiler) run(src []byte) ([]byte, error) {
	p := css.NewParser(parse.NewInputString(string(src)), false)
	for {
		gt, _, data := p.Next()
		if gt == css.ErrorGrammar {
			if err := p.Err(); err != nil && err.Error() != "EOF" {
				return nil, invalidCSS("解析失败: %v", err)
			}
			break
		}
		c.step(gt, data, p.Values())
	}
	return []byte(c.out.String()), nil
}

func (c *compiler) inAtRule(name string) bool {
	for _, entry := range c.atRuleStack {
		if entry == name {
			return true
		}
	}
	return false
}

func (c *compiler) step(gt css.GrammarType, data []byte, values []css.Token) {
	switch gt {
	case css.BeginAtRuleGrammar:
		name := decodeCSSIdent(strings.TrimPrefix(string(data), "@"))
		c.atRuleStack = append(c.atRuleStack, name)
		c.out.WriteString("@" + name)
		c.out.WriteString(c.atRulePrelude(name, values))
		c.out.WriteString("{")
	case css.AtRuleGrammar:
		name := decodeCSSIdent(strings.TrimPrefix(string(data), "@"))
		c.out.WriteString("@" + name + c.atRulePrelude(name, values) + ";")
	case css.EndAtRuleGrammar:
		if len(c.atRuleStack) > 0 {
			c.atRuleStack = c.atRuleStack[:len(c.atRuleStack)-1]
		}
		c.out.WriteString("}")
	case css.BeginRulesetGrammar, css.QualifiedRuleGrammar:
		c.rulesetDepth++
		c.out.WriteString(c.selector(values))
		c.out.WriteString("{")
	case css.EndRulesetGrammar:
		if c.rulesetDepth > 0 {
			c.rulesetDepth--
		}
		c.out.WriteString("}")
	case css.DeclarationGrammar:
		property := decodeCSSIdent(string(data))
		c.out.WriteString(property + ":" + c.declarationValue(property, values) + ";")
	case css.CustomPropertyGrammar:
		c.out.WriteString(string(data) + ":" + c.rewriteRaw(joinTokens(values)) + ";")
	}
}

// atRulePrelude 处理 at-rule 的前奏。@keyframes 的名字要命名空间化，
// 其余原样输出。
func (c *compiler) atRulePrelude(name string, values []css.Token) string {
	raw := joinTokens(values)
	if name != "keyframes" {
		return raw
	}
	trimmed := strings.TrimSpace(raw)
	return " " + namespacedKeyframe(c.scope, trimmed)
}

// selector 给每条选择器加作用域。逗号分隔的多条各自处理。
func (c *compiler) selector(values []css.Token) string {
	// @keyframes 内部的 from/to/百分比不是选择器，不加作用域。
	if c.inAtRule("keyframes") {
		return strings.TrimSpace(joinTokens(values))
	}

	parts := splitSelectorList(values)
	scoped := make([]string, 0, len(parts))
	for _, part := range parts {
		scoped = append(scoped, c.scopeOne(part))
	}
	return strings.Join(scoped, ",")
}

// scopeOne 给单条选择器加作用域。
//
// 以根引用（:root 或 [data-nx="page-root"]）开头的选择器，根引用本身被
// 替换为 [data-theme="<scope>"]；其余选择器前置 [data-theme="<scope>"] 与
// 后代组合符。
func (c *compiler) scopeOne(tokens []css.Token) string {
	prefix := fmt.Sprintf("[data-theme=%q]", c.scope)
	trimmed := trimWhitespaceTokens(tokens)
	if consumed := leadingRootReference(trimmed); consumed > 0 {
		rest := strings.TrimRight(joinTokens(trimmed[consumed:]), " \t\n")
		return prefix + rest
	}
	return prefix + " " + strings.TrimSpace(joinTokens(trimmed))
}

// leadingRootReference 返回开头根引用所占的 token 数，没有则返回 0。
func leadingRootReference(tokens []css.Token) int {
	if len(tokens) >= 2 && tokens[0].TokenType == css.ColonToken &&
		tokens[1].TokenType == css.IdentToken && decodeCSSIdent(string(tokens[1].Data)) == "root" {
		return 2
	}
	if len(tokens) > 0 && tokens[0].TokenType == css.LeftBracketToken {
		end := 0
		for end < len(tokens) && tokens[end].TokenType != css.RightBracketToken {
			end++
		}
		if end >= len(tokens) {
			return 0
		}
		inner := meaningfulTokens(tokens[1:end])
		if len(inner) == 0 || decodeCSSIdent(string(inner[0].Data)) != "data-nx" {
			return 0
		}
		for _, token := range inner[1:] {
			if token.TokenType == css.StringToken || token.TokenType == css.IdentToken {
				if strings.ToLower(unquote(string(token.Data))) == ThemeRootHook {
					return end + 1
				}
			}
		}
	}
	return 0
}

func splitSelectorList(values []css.Token) [][]css.Token {
	parts := [][]css.Token{}
	current := []css.Token{}
	depth := 0
	for _, token := range values {
		switch token.TokenType {
		case css.FunctionToken:
			depth++
		case css.RightParenthesisToken:
			if depth > 0 {
				depth--
			}
		case css.CommaToken:
			if depth == 0 {
				parts = append(parts, current)
				current = []css.Token{}
				continue
			}
		}
		current = append(current, token)
	}
	return append(parts, current)
}

func trimWhitespaceTokens(tokens []css.Token) []css.Token {
	start, end := 0, len(tokens)
	for start < end && tokens[start].TokenType == css.WhitespaceToken {
		start++
	}
	for end > start && tokens[end-1].TokenType == css.WhitespaceToken {
		end--
	}
	return tokens[start:end]
}

// declarationValue 重写声明的值：资产 URL、动画名、包内字体族名。
func (c *compiler) declarationValue(property string, values []css.Token) string {
	var b strings.Builder
	for _, token := range values {
		switch {
		case token.TokenType == css.URLToken:
			b.WriteString(c.rewriteURLToken(string(token.Data)))
		case (token.TokenType == css.StringToken || token.TokenType == css.IdentToken) &&
			isFontFamilyProperty(property, c.inAtRule("font-face")):
			b.WriteString(c.rewriteFamilyToken(string(token.Data)))
		case (token.TokenType == css.IdentToken) && isAnimationProperty(property):
			b.WriteString(c.rewriteAnimationToken(string(token.Data)))
		default:
			b.Write(token.Data)
		}
	}
	return strings.TrimSpace(b.String())
}

func isFontFamilyProperty(property string, inFontFace bool) bool {
	return property == "font-family" || (inFontFace && property == "font-family")
}

func isAnimationProperty(property string) bool {
	return property == "animation" || property == "animation-name"
}

func (c *compiler) rewriteFamilyToken(raw string) string {
	family := unquote(raw)
	if !c.packageFamilies[strings.ToLower(family)] {
		return raw
	}
	return `"` + namespacedFamily(c.scope, family) + `"`
}

// rewriteAnimationToken 只改写确实存在的 @keyframes 名；animation 简写里
// 的关键字（infinite、ease 等）不受影响。
func (c *compiler) rewriteAnimationToken(raw string) string {
	name := decodeCSSIdent(raw)
	if animationKeywords[name] {
		return raw
	}
	return namespacedKeyframe(c.scope, raw)
}

var animationKeywords = map[string]bool{
	"none": true, "infinite": true, "alternate": true, "alternate-reverse": true,
	"normal": true, "reverse": true, "forwards": true, "backwards": true, "both": true,
	"running": true, "paused": true, "linear": true, "ease": true, "ease-in": true,
	"ease-out": true, "ease-in-out": true, "step-start": true, "step-end": true,
	"initial": true, "inherit": true, "unset": true, "revert": true,
}

func (c *compiler) rewriteURLToken(raw string) string {
	inner := strings.TrimSpace(raw)
	if lower := strings.ToLower(inner); strings.HasPrefix(lower, "url(") {
		inner = strings.TrimSuffix(inner[len("url("):], ")")
	}
	inner = strings.TrimSpace(unquote(strings.TrimSpace(inner)))
	if !strings.HasPrefix(strings.ToLower(inner), AssetURLScheme) {
		return raw
	}
	path := strings.TrimPrefix(inner[len(AssetURLScheme):], "/")
	return `url("` + AssetBasePlaceholder + path + `")`
}

// rewriteRaw 处理自定义属性这类不透明值：重新 lex 后套用同一套重写。
func (c *compiler) rewriteRaw(raw string) string {
	lexer := css.NewLexer(parse.NewInputString(raw))
	var b strings.Builder
	for {
		tt, text := lexer.Next()
		if tt == css.ErrorToken {
			break
		}
		if tt == css.URLToken {
			b.WriteString(c.rewriteURLToken(string(text)))
			continue
		}
		b.Write(text)
	}
	return strings.TrimSpace(b.String())
}
