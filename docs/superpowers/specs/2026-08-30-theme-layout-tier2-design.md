# 子项目 C：tier 2 声明式布局

日期：2026-08-30
状态：实现中
依据：`docs/superpowers/specs/2026-07-23-theme-spec-v1-design.md` §3 布局优先级、§5.2 `tier`、§10.C

## 1. 目标

主题可以声明页面结构旋钮（模板、密度、列数、分类样式）的默认值、允许范围与锁定；可选声明 `full` 模板下的区块顺序。用户设置越界时一次性夹取到合法值。公开页仍由宿主 React 渲染，主题不执行代码。

## 2. 非目标

- 不为 search-focus / browse-first / sidebar 做通用 slot 重写（它们自带结构）。
- 不新增独立「卡片形态」字段（沿用 `density`）。
- 不把 6 个内置主题升到 tier 2。
- 不做可视化布局编辑器、主题市场、tier 3。

## 3. 契约

`theme.json` 在 `tier === 2` 时**必须**有 `layout`；`tier === 1` 时**禁止**出现 `layout`。宿主 `MaxTier = 2`。

```json
"layout": {
  "template": { "default": "full", "allowed": ["full", "search-focus"], "locked": false },
  "density": { "default": "comfortable", "allowed": ["comfortable", "compact"], "locked": false },
  "columns": { "default": 4, "min": 2, "max": 6, "locked": false },
  "categoryStyle": { "default": "tabs", "allowed": ["tabs", "folders"], "locked": true },
  "sections": ["greeting", "search", "sites"]
}
```

| 旋钮 | 值域 | 规则 |
|---|---|---|
| template | `full` `search-focus` `browse-first` `sidebar` | `default ∈ allowed`，`allowed` 非空去重 |
| density | `list` `compact` `comfortable` | 同上 |
| categoryStyle | `tabs` `sidebar` `grid` `folders` | 同上 |
| columns | 1..8 | `min ≤ default ≤ max` |
| sections | `greeting` `search` `sites` | 可选；出现则去重且必须含 `search` 与 `sites`；只作用于 `template=full` |

`locked: true`：用户不能改，夹取后恒为 `default`。

## 4. 夹取

`themes.ApplyLayout(spec, current) → next`：

- 无 spec（tier 1）：原样返回。
- locked：该旋钮 = default。
- 否则：枚举不在 allowed 则改 default；columns 夹到 `[min,max]`。

时机：

1. `ReplaceSettings`：按即将写入的 `appearance.themeId` 夹取后落草稿。
2. Preview / Publish：写入快照前再夹一次，公开页不依赖「用户是否已保存」。

前端切换主题或改布局时按 `GET /themes` 带回的 `layout` 限制选项并禁用锁定旋钮。

## 5. 区块顺序

仅 `layout.template` 解析结果为 `full` 时读取 `sections`（缺省 `["greeting","search","sites"]`）。`FooterActions` 始终在最后，不进契约。其它模板忽略 `sections`。

## 6. API

- `ThemeManifestV1.layout` 可选对象（语义由 tier 约束）。
- `Theme.layout` 可选，来自当前版本 manifest；tier 1 省略。
- 不新增端点。

## 7. 测试

- 清单：tier2 缺 layout / tier1 带 layout / 合法 layout / 非法 allowed。
- 夹取表：越界、锁定、tier1 透传。
- 契约：导入 tier2 zip → 改设置越界 → 回读被夹取。
- E2E：导入 tier2 主题后密度开关只剩 allowed 项。
