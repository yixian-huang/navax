// Package themes 承载主题包的解析、校验、编译与版本化存储。
// 它是主题内容进入实例的唯一信任边界：浏览器只拿编译产物。
package themes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ErrInvalidManifest 包裹所有 manifest 层面的校验失败。
var ErrInvalidManifest = errors.New("invalid theme manifest")

// SpecVersion 是本实现支持的规范版本。
const SpecVersion = 1

// MaxTier 是当前宿主接受的最高能力级别。tier 2（声明式布局）随子项目 C
// 发布，tier 3（JS）语义未定，两者一律拒绝。
const MaxTier = 1

const maxManifestBytes = 64 * 1024

var (
	slugPattern    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)
	semverPattern  = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	oklchPattern   = regexp.MustCompile(`^(\d+(?:\.\d+)?) (\d+(?:\.\d+)?) (\d+(?:\.\d+)?)$`)
	hexPattern     = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	fontStackChars = regexp.MustCompile(`^[A-Za-z0-9 ,'"\-_]+$`)
	lengthChars    = regexp.MustCompile(`^[0-9a-zA-Z.%\-+ ()/,]+$`)
)

var (
	requiredFonts  = []string{"heading", "body", "label", "mono"}
	requiredColors = []string{"background", "foreground", "primary", "accent"}
	allowedModes   = map[string]bool{"light": true, "dark": true, "both": true}
	allowedVibes   = map[string]bool{"serious": true, "cute": true}
)

// Tokens 是主题的设计令牌。color 与 font 必填，radius/elevation 缺失时回落基线值。
type Tokens struct {
	Font      map[string]string            `json:"font"`
	Radius    map[string]string            `json:"radius,omitempty"`
	Elevation map[string]string            `json:"elevation,omitempty"`
	Color     map[string]map[string]string `json:"color"`
}

// Manifest 是 theme.json 的内存表示，字段与 api/openapi.yaml 的
// ThemeManifestV1 一一对应。
type Manifest struct {
	SpecVersion int       `json:"specVersion"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Subtitle    string    `json:"subtitle,omitempty"`
	Description string    `json:"description,omitempty"`
	Version     string    `json:"version"`
	Author      string    `json:"author"`
	License     string    `json:"license,omitempty"`
	Homepage    string    `json:"homepage,omitempty"`
	Mode        string    `json:"mode"`
	Vibe        string    `json:"vibe"`
	Swatches    [3]string `json:"swatches"`
	Tier        int       `json:"tier"`
	Tokens      Tokens    `json:"tokens"`
}

// FontFamilies 返回令牌中引用的全部字体族名，供 @font-face 交叉检查。
// 族名按 CSS 字体栈拆分并去引号，顺序稳定。
func (m Manifest) FontFamilies() []string {
	seen := map[string]bool{}
	for _, stack := range m.Tokens.Font {
		for _, family := range strings.Split(stack, ",") {
			name := strings.Trim(strings.TrimSpace(family), `'"`)
			if name != "" {
				seen[name] = true
			}
		}
	}
	families := make([]string, 0, len(seen))
	for name := range seen {
		families = append(families, name)
	}
	sort.Strings(families)
	return families
}

func invalidManifest(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidManifest, fmt.Sprintf(format, args...))
}

// ParseManifest 解析并全量校验 theme.json。
func ParseManifest(data []byte) (Manifest, error) {
	if len(data) > maxManifestBytes {
		return Manifest{}, invalidManifest("theme.json 超过 %d 字节上限", maxManifestBytes)
	}
	var m Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return Manifest{}, invalidManifest("json 解析失败: %v", err)
	}
	if m.SpecVersion != SpecVersion {
		return Manifest{}, invalidManifest("specVersion 必须为 %d", SpecVersion)
	}
	if !slugPattern.MatchString(m.ID) {
		return Manifest{}, invalidManifest("id 不符合 ^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$")
	}
	if m.Name == "" || len(m.Name) > 100 {
		return Manifest{}, invalidManifest("name 长度必须在 1..100")
	}
	if len(m.Subtitle) > 100 {
		return Manifest{}, invalidManifest("subtitle 不得超过 100 字节")
	}
	if len(m.Description) > 500 {
		return Manifest{}, invalidManifest("description 不得超过 500 字节")
	}
	if !semverPattern.MatchString(m.Version) {
		return Manifest{}, invalidManifest("version 必须是语义化版本")
	}
	if m.Author == "" || len(m.Author) > 100 {
		return Manifest{}, invalidManifest("author 长度必须在 1..100")
	}
	if !allowedModes[m.Mode] {
		return Manifest{}, invalidManifest("mode 必须是 light|dark|both")
	}
	if !allowedVibes[m.Vibe] {
		return Manifest{}, invalidManifest("vibe 必须是 serious|cute")
	}
	if m.Tier < 1 || m.Tier > 3 {
		return Manifest{}, invalidManifest("tier 必须在 1..3")
	}
	if m.Tier > MaxTier {
		return Manifest{}, invalidManifest("宿主暂不支持 tier %d，当前只接受 tier %d", m.Tier, MaxTier)
	}
	for _, swatch := range m.Swatches {
		if !hexPattern.MatchString(swatch) {
			return Manifest{}, invalidManifest("swatches 必须是 #rrggbb")
		}
	}
	if err := validateTokens(m.Tokens); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func validateTokens(tokens Tokens) error {
	for _, key := range requiredFonts {
		value := tokens.Font[key]
		if value == "" || !fontStackChars.MatchString(value) {
			return invalidManifest("font.%s 缺失或含非法字符", key)
		}
	}
	for _, group := range requiredColors {
		values := tokens.Color[group]
		if len(values) == 0 {
			return invalidManifest("color.%s 至少需要一个档位", group)
		}
	}
	for _, group := range sortedKeys(tokens.Color) {
		for _, step := range sortedKeys(tokens.Color[group]) {
			if err := validateOKLCH(group, step, tokens.Color[group][step]); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"radius", "elevation"} {
		table := tokens.Radius
		if name == "elevation" {
			table = tokens.Elevation
		}
		for _, step := range sortedKeys(table) {
			if err := validateLengthLike(name, step, table[step]); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateOKLCH 除形状外还校验数值范围：仅靠正则会放行 "9 9 999"
// 这类形状合法但不可用的值。
func validateOKLCH(group, step, value string) error {
	match := oklchPattern.FindStringSubmatch(value)
	if match == nil {
		return invalidManifest("color.%s.%s 必须是 OKLCH 三通道，如 \"0.55 0.12 250\"", group, step)
	}
	bounds := [3]struct {
		name     string
		min, max float64
	}{
		{"亮度 L", 0, 1},
		{"色度 C", 0, 0.5},
		{"色相 H", 0, 360},
	}
	for i, bound := range bounds {
		channel, err := strconv.ParseFloat(match[i+1], 64)
		if err != nil {
			return invalidManifest("color.%s.%s 的%s无法解析", group, step, bound.name)
		}
		if channel < bound.min || channel > bound.max {
			return invalidManifest("color.%s.%s 的%s超出 [%g, %g]", group, step, bound.name, bound.min, bound.max)
		}
		if i == 2 && channel == 360 {
			return invalidManifest("color.%s.%s 的%s必须在 [0, 360)", group, step, bound.name)
		}
	}
	return nil
}

// validateLengthLike 只做字符白名单：radius 与 elevation 的完整语法
// 由 CSS 校验器在令牌块上兜底。
func validateLengthLike(name, step, value string) error {
	if value == "" || !lengthChars.MatchString(value) {
		return invalidManifest("%s.%s 缺失或含非法字符", name, step)
	}
	return nil
}

func sortedKeys[V any](table map[string]V) []string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
