package themes

import (
	"strings"

	"github.com/tdewolff/parse/v2/css"
)

// 选择器校验。
//
// 编译器把每条选择器改写成 `[data-theme="<pkg>"] <selector>`（后代组合符），
// 因此普通选择器的 subject 天然落在主题根内——即便内部用了 `+`/`~`，兄弟
// 元素也仍是根的后代。
//
// 唯一的逃逸口是根引用本身：`[data-nx="page-root"]` 与 `:root` 会被改写为
// 根元素自身，此时紧随其后的 `+`/`~` 就会命中根的兄弟——那正是受保护页脚
// 所在的位置。所以规则是：根引用之后只允许后代与子代组合符。
//
// 这里在扁平 token 流上做判定：函数伪类（:is()、:nth-child(… of …) 等）的
// 参数就在同一条流里，用括号深度跟踪即可覆盖所有接受 selector-list 的语法
// 位置，无需为每个伪类单独枚举。

// forbiddenElements 是主题不得触达的类型选择器。
var forbiddenElements = map[string]bool{
	"html": true,
	"body": true,
}

func validateSelector(values []css.Token) error {
	// rootSeen 表示当前这条选择器里已经出现过根引用。逗号处重置，
	// 因为逗号开启的是独立的一条选择器。
	rootSeen := false

	for i := 0; i < len(values); i++ {
		token := values[i]
		text := string(token.Data)

		switch token.TokenType {
		case css.CommaToken:
			rootSeen = false

		case css.IdentToken:
			// 上一个非空白 token 是 `.` 时，这是类选择器。
			if prev := prevMeaningful(values, i); prev != nil && prev.TokenType == css.DelimToken && string(prev.Data) == "." {
				return invalidCSS("不允许类选择器 .%s —— 请改用 docs/theme-api.md 登记的 data-nx 钩子", decodeCSSIdent(text))
			}
			// 前一个是 `|` 时属于命名空间选择器的一部分，已在 DelimToken 分支拒绝。
			if name := decodeCSSIdent(text); forbiddenElements[name] {
				return invalidCSS("不允许选择 %s —— 主题不得越出主题根，请改用 [data-nx=\"page-root\"]", name)
			}

		case css.DelimToken:
			switch text {
			case "+", "~":
				if rootSeen {
					return invalidCSS("主题根之后不允许相邻/通用兄弟组合符 %q —— 它会命中根外的兄弟元素（含受保护的页脚）", text)
				}
			case "&":
				return invalidCSS("不允许 CSS 嵌套（nesting）—— v1 不承担其作用域语义")
			case "|":
				return invalidCSS("不允许命名空间选择器 —— v1 不承担其语义")
			}

		case css.ColonToken:
			// `:root` 与根引用等价。
			if next := nextMeaningful(values, i); next != nil && next.TokenType == css.IdentToken &&
				decodeCSSIdent(string(next.Data)) == "root" {
				rootSeen = true
			}

		case css.LeftBracketToken:
			attr, isRoot, err := validateAttributeSelector(values, i)
			if err != nil {
				return err
			}
			if isRoot {
				rootSeen = true
			}
			i = attr // 跳到 `]`
		}
	}
	return nil
}

// validateAttributeSelector 校验从 open 处开始的属性选择器，返回 `]` 的下标
// 以及它是否是主题根引用。
func validateAttributeSelector(values []css.Token, open int) (int, bool, error) {
	end := open
	for end < len(values) && values[end].TokenType != css.RightBracketToken {
		end++
	}
	if end >= len(values) {
		return len(values) - 1, false, invalidCSS("属性选择器缺少 ]")
	}

	inner := meaningfulTokens(values[open+1 : end])
	if len(inner) == 0 {
		return end, false, invalidCSS("属性选择器为空")
	}
	name := decodeCSSIdent(string(inner[0].Data))

	switch name {
	case "class":
		return end, false, invalidCSS("不允许对 class 做属性匹配 —— 它是绕过类名限制的等价写法，请改用 data-nx 钩子")
	case FrameAttr:
		return end, false, invalidCSS("不允许命中宿主 wrapper %s —— 它承载隔离边界", FrameAttr)
	case ProtectedAttr:
		return end, false, invalidCSS("不允许命中 %s —— 该区域受许可证保护且位于主题根之外", ProtectedAttr)
	case "data-nx":
		value := ""
		for _, token := range inner[1:] {
			if token.TokenType == css.StringToken || token.TokenType == css.IdentToken {
				value = unquote(string(token.Data))
			}
		}
		if value == "" {
			return end, false, invalidCSS("[data-nx] 必须指定具体钩子名")
		}
		hook := strings.ToLower(value)
		if !IsAllowedHook(hook) {
			return end, false, invalidCSS("未登记的 data-nx 钩子 %q —— 可用钩子见 docs/theme-api.md", hook)
		}
		return end, hook == ThemeRootHook, nil
	}
	return end, false, nil
}

func prevMeaningful(values []css.Token, index int) *css.Token {
	for i := index - 1; i >= 0; i-- {
		if values[i].TokenType == css.WhitespaceToken || values[i].TokenType == css.CommentToken {
			continue
		}
		return &values[i]
	}
	return nil
}

func nextMeaningful(values []css.Token, index int) *css.Token {
	for i := index + 1; i < len(values); i++ {
		if values[i].TokenType == css.WhitespaceToken || values[i].TokenType == css.CommentToken {
			continue
		}
		return &values[i]
	}
	return nil
}
