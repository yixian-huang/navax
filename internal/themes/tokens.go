package themes

import (
	"fmt"
	"sort"
	"strings"
)

// 基线令牌取自默认主题 slate（web/src/themes/packages/slate.ts）。
// manifest 未提供 radius/elevation 时用它们补齐，因此一个最小可用主题
// 只需写颜色四组与字体四族。
var baselineRadius = map[string]string{
	"none": "0",
	"sm":   "0",
	"md":   "0",
	"lg":   "0",
	"xl":   "0",
	"2xl":  "0",
	"full": "9999px",
}

var baselineElevation = map[string]string{
	"surface": "0 0 0 1px oklch(0.20 0.015 255 / 0.12)",
	"raised":  "0 0 0 1px oklch(0.20 0.015 255 / 0.14)",
	"float":   "0 0 0 1px oklch(0.20 0.015 255 / 0.55)",
	"overlay": "0 0 0 1px oklch(0.20 0.015 255 / 0.16)",
}

// TokensCSS 把设计令牌渲染成作用在主题根上的 CSS 变量块。
//
// packageFamilies 是包内 @font-face 声明的族名（小写）。令牌里引用它们时
// 必须写命名空间化之后的名字，否则 theme.json 与编译后的 theme.css 对不上
// ——@font-face 改了名，令牌还指着原名，字体就找不到了。
//
// 输出顺序固定（字体 → radius → elevation → 颜色，组内按键名排序），
// 保证同一输入的编译结果逐字节一致。
func TokensCSS(m Manifest, scope string, packageFamilies map[string]bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[data-theme=%q] {\n", scope)
	writeTokenGroup(&b, "--font-", namespaceFontStacks(m.Tokens.Font, scope, packageFamilies), nil)
	writeTokenGroup(&b, "--radius-", m.Tokens.Radius, baselineRadius)
	writeTokenGroup(&b, "--elevation-", m.Tokens.Elevation, baselineElevation)
	for _, group := range sortedKeys(m.Tokens.Color) {
		writeTokenGroup(&b, "--"+group+"-", m.Tokens.Color[group], nil)
	}
	b.WriteString("}\n")
	return b.String()
}

// namespaceFontStacks 把字体栈里指向包内字体的族名替换为命名空间化后的名字。
func namespaceFontStacks(fonts map[string]string, scope string, packageFamilies map[string]bool) map[string]string {
	if len(packageFamilies) == 0 {
		return fonts
	}
	out := make(map[string]string, len(fonts))
	for key, stack := range fonts {
		parts := strings.Split(stack, ",")
		for i, part := range parts {
			trimmed := strings.TrimSpace(part)
			quote := ""
			if len(trimmed) >= 2 && (trimmed[0] == '"' || trimmed[0] == '\'') {
				quote = string(trimmed[0])
			}
			family := strings.Trim(trimmed, `'"`)
			if packageFamilies[strings.ToLower(family)] {
				renamed := namespacedFamily(scope, family)
				if quote == "" {
					quote = `"`
				}
				parts[i] = quote + renamed + quote
			} else {
				parts[i] = trimmed
			}
		}
		out[key] = strings.Join(parts, ", ")
	}
	return out
}

// namespacedFamily 是包内字体族名的命名空间化规则，编译器与令牌生成必须
// 用同一个函数，否则两边会对不上。
func namespacedFamily(scope, family string) string {
	return scope + "-" + family
}

// namespacedKeyframe 是动画名的命名空间化规则。
func namespacedKeyframe(scope, name string) string {
	return scope + "-" + name
}

func writeTokenGroup(b *strings.Builder, prefix string, values, fallback map[string]string) {
	merged := make(map[string]string, len(values)+len(fallback))
	for key, value := range fallback {
		merged[key] = value
	}
	for key, value := range values {
		merged[key] = value
	}
	for _, key := range sortedKeys(merged) {
		fmt.Fprintf(b, "  %s%s: %s;\n", prefix, key, merged[key])
	}
}

var _ = sort.Strings
