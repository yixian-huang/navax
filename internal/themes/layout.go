package themes

const layoutMaxSections = 3

var (
	allowedTemplates      = []string{"full", "search-focus", "browse-first", "sidebar"}
	allowedDensities      = []string{"list", "compact", "comfortable"}
	allowedCategoryStyles = []string{"tabs", "sidebar", "grid", "folders"}
	allowedSections       = []string{"greeting", "search", "sites"}
)

// EnumKnob 是枚举型布局旋钮：默认值、允许集合、是否锁定。
type EnumKnob struct {
	Default string   `json:"default"`
	Allowed []string `json:"allowed"`
	Locked  bool     `json:"locked"`
}

// RangeKnob 是整数范围旋钮（列数）。
type RangeKnob struct {
	Default int  `json:"default"`
	Min     int  `json:"min"`
	Max     int  `json:"max"`
	Locked  bool `json:"locked"`
}

// Layout 是 tier 2 主题的声明式布局。tier 1 必须省略。
type Layout struct {
	Template      EnumKnob  `json:"template"`
	Density       EnumKnob  `json:"density"`
	Columns       RangeKnob `json:"columns"`
	CategoryStyle EnumKnob  `json:"categoryStyle"`
	Sections      []string  `json:"sections,omitempty"`
}

// PageLayoutValues 是页面当前布局旋钮，供夹取使用。放在 themes 包以免
// navigation 与 themes 循环依赖。
type PageLayoutValues struct {
	Template      string
	Density       string
	Columns       int
	CategoryStyle string
}

func validateLayout(m Manifest) error {
	if m.Tier == 2 {
		if m.Layout == nil {
			return invalidManifest("tier 2 必须提供 layout")
		}
		return m.Layout.validate()
	}
	if m.Layout != nil {
		return invalidManifest("tier 1 不得包含 layout")
	}
	return nil
}

func (l Layout) validate() error {
	if err := validateEnumKnob("template", l.Template, allowedTemplates); err != nil {
		return err
	}
	if err := validateEnumKnob("density", l.Density, allowedDensities); err != nil {
		return err
	}
	if err := validateEnumKnob("categoryStyle", l.CategoryStyle, allowedCategoryStyles); err != nil {
		return err
	}
	if l.Columns.Min < 1 || l.Columns.Max > 8 || l.Columns.Min > l.Columns.Max {
		return invalidManifest("layout.columns 的 min/max 必须满足 1 ≤ min ≤ max ≤ 8")
	}
	if l.Columns.Default < l.Columns.Min || l.Columns.Default > l.Columns.Max {
		return invalidManifest("layout.columns.default 必须落在 min..max")
	}
	if err := validateSections(l.Sections); err != nil {
		return err
	}
	return nil
}

func validateEnumKnob(name string, knob EnumKnob, universe []string) error {
	if len(knob.Allowed) == 0 {
		return invalidManifest("layout.%s.allowed 不能为空", name)
	}
	seen := map[string]bool{}
	for _, value := range knob.Allowed {
		if !oneOfString(value, universe) {
			return invalidManifest("layout.%s.allowed 含非法值 %q", name, value)
		}
		if seen[value] {
			return invalidManifest("layout.%s.allowed 重复 %q", name, value)
		}
		seen[value] = true
	}
	if !seen[knob.Default] {
		return invalidManifest("layout.%s.default 必须属于 allowed", name)
	}
	return nil
}

func validateSections(sections []string) error {
	if len(sections) == 0 {
		return nil
	}
	if len(sections) > layoutMaxSections {
		return invalidManifest("layout.sections 最多 %d 项", layoutMaxSections)
	}
	seen := map[string]bool{}
	for _, name := range sections {
		if !oneOfString(name, allowedSections) {
			return invalidManifest("layout.sections 含未知区块 %q", name)
		}
		if seen[name] {
			return invalidManifest("layout.sections 重复 %q", name)
		}
		seen[name] = true
	}
	if !seen["search"] || !seen["sites"] {
		return invalidManifest("layout.sections 必须包含 search 与 sites")
	}
	return nil
}

// ApplyLayout 按主题声明夹取页面布局。spec 为 nil 时原样返回（tier 1）。
func ApplyLayout(spec *Layout, current PageLayoutValues) PageLayoutValues {
	if spec == nil {
		return current
	}
	current.Template = applyEnum(spec.Template, current.Template)
	current.Density = applyEnum(spec.Density, current.Density)
	current.CategoryStyle = applyEnum(spec.CategoryStyle, current.CategoryStyle)
	current.Columns = applyRange(spec.Columns, current.Columns)
	return current
}

func applyEnum(knob EnumKnob, current string) string {
	if knob.Locked || !oneOfString(current, knob.Allowed) {
		return knob.Default
	}
	return current
}

func applyRange(knob RangeKnob, current int) int {
	if knob.Locked {
		return knob.Default
	}
	if current < knob.Min {
		return knob.Min
	}
	if current > knob.Max {
		return knob.Max
	}
	return current
}

func oneOfString(value string, allowed []string) bool {
	for _, item := range allowed {
		if item == value {
			return true
		}
	}
	return false
}
