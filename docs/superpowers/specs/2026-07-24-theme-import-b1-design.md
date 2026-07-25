# 主题导入与私有安装(子项目 B1)设计

日期:2026-07-24
状态:方向已与用户确认(配额默认 10、升级纳入 B1),spec 待审阅
范围:第三方主题的 zip 上传与 GitHub 一键导入、私有安装/卸载/升级、`preview.png`、`POST /api/v1/themes/validate`、导入 UI。B2(目录审核、版本级 kill switch、更新检查、starter 仓库)不在本文。
设计依据:`docs/superpowers/specs/2026-07-23-theme-spec-v1-design.md`(§3 决策表、§7、§10.B)

## 1. 目标与既有地基

子项目 A 交付了完整的校验/编译/版本化管线,但**全系统没有任何创建 `themes` 行的代码路径**——第三方主题当前完全装不进来。B1 补上导入面,原则是**管线零改动**:两条来源(zip、GitHub)解包成统一的内存文件集后,走与内置主题完全相同的 `Compile` → 幂等 `UpsertVersion` 路径。信任边界仍全部在服务端。

## 2. 解包层(新增,唯一的新安全面)

新文件 `internal/themes/archive.go`:把 zip(`archive/zip`)与 gzip tarball(`archive/tar`,GitHub codeload 产物)解为 `map[string][]byte`,交给既有 `CompilePackage`。两种格式共用同一套防护:

| 防护 | 值(导出常量) | 理由 |
|---|---|---|
| 解压总量上限 | 16 MiB | 解压炸弹;是包体上限(4 MiB)的 4 倍,给非包文件(README、LICENSE、.git 残留)留余量 |
| 文件数上限 | 200 | 遍历成本与 inode 滥用 |
| 路径校验 | 规范化后拒绝绝对路径与 `..` 逃逸(zip-slip);拒绝 tar 符号链接/硬链接/设备条目 | 经典解包攻击面 |
| 顶层目录剥离 | GitHub tarball 的 `{repo}-{sha}/` 前缀自动剥一层;zip 若全部条目共享单一顶层目录同样剥离 | 作者打包习惯差异 |

解包只做「取出 `theme.json`、`theme.css`、`assets/**`、`preview.png`」——白名单外的文件(README、LICENSE、`.github/` 等)直接忽略,不计入包体 4 MiB,但计入解压总量 16 MiB。单文件的体积约束沿用编译层既有常量(CSS ≤ 256 KiB、单资产 ≤ 512 KiB),解包层不重复。

## 3. GitHub 导入

- 输入:仓库 URL(`https://github.com/{owner}/{repo}`)+ 可选 `ref`(分支/标签/40 位 sha,缺省默认分支)。
- 解析:`ref` 是 40 位十六进制则直接使用;否则经 `https://api.github.com/repos/{owner}/{repo}/commits/{ref}` 解析出 commit sha。**锁定 sha** 后从 `https://codeload.github.com/{owner}/{repo}/tar.gz/{sha}` 拉取 tarball——`source_ref` 永远存 sha,upstream 变更不影响已装主题(v1 §3 版本策略)。
- 网络防护:所有出网请求走既有 `internal/netguard`(每次 DNS 解析与重定向都复核回环/私网/link-local/元数据地址);主机白名单默认 `api.github.com` + `codeload.github.com`,自建实例经 `NAVAX_THEME_IMPORT_HOSTS` 追加(GitLab 等,B1 只保证 GitHub 布局可用);响应体读取有硬上限(压缩体 ≤ 16 MiB)。
- 匿名 GitHub API 限额为 60 次/小时/IP:可选 `NAVAX_GITHUB_TOKEN` 提升限额(只加 `Authorization` 头,永不回显);文档写明该限制。zip 上传是无网络依赖的后备路径。

## 4. 数据、归属与生命周期

- **行创建**:`themes.id = identity.New("thm")`(不透明,v1 §7.2);`slug = manifest.id`;`scope='private'`、`owner_id=当前用户`(数据库触发器强制成对);`source_type ∈ {github, upload}`;`source_url` 存仓库 URL(上传为空);`enabled=1`。
- **元数据回写(顺带消除既知缺口)**:导入与升级把 manifest 的 `name/description/mode/version` 回写到 `themes` 行——该回写统一放进 upsert 路径,**内置主题的启动同步同样受益**,消除「DB 行元数据与 manifest 漂移」(slate 行 `mode='both'` vs manifest `'light'` 一类)。
- **升级 = 重复导入同 slug**(用户已确认纳入 B1):同 owner + 同 slug 命中既有行 → `UpsertVersion`(content hash 幂等,重复内容不产生新版本行)+ 切 `current_version_id` + 回写元数据。页面设置里的 `themeId` 不变,已发布快照因锁版本不受影响;下次发布自动锁到新版本。
- **配额**(用户已确认):每 owner 的 `themes` 行数(含墓碑)≤ `NAVAX_THEME_PRIVATE_QUOTA`,默认 10。升级不占新额度(复用既有行)。
- **卸载** `DELETE /api/v1/me/themes/{id}`:
  - 无任何 `published_snapshots` 引用其任一版本 → **物理删除**:先置 `current_version_id = NULL`(触发器允许置空),删版本与资产(CASCADE),删行。配额立即释放。
  - 仍被快照引用 → **墓碑**:`enabled = 0`,行与版本保留(公开页继续可用),占配额——这是 v1 §8.1.1 撤销语义的直接推论,UI 对墓碑展示「已卸载·仍被历史发布引用」。
  - 正被自己草稿的 `themeId` 引用不阻止卸载:发布时 eligibility 谓词自然回落默认主题,与目录主题被下架同语义。
- **默认主题守卫**:`PATCH /api/v1/admin/themes/{id}` 把 `default=true` 用于 `scope='private'` 的行 → 409(默认主题必须是目录主题,v1 §7.1 不变量);管理员对私有主题的 `enabled` 开关照常可用(全站下架权,v1 §3 已定)。
- **账号删除前置清理——按现实调整**:核实发现系统目前**没有任何删除用户的代码路径**(admin 仅能改状态/踢会话/重置密码),v1 §7.1 设想的接入点不存在。B1 的处置:不写无调用方的清理函数(YAGNI),改为用集成测试**钉住数据库级不变量**——拥有版本行的用户 `DELETE FROM users` 必须被 `theme_versions.theme_id ON DELETE RESTRICT` 挡下(经 themes 行的 CASCADE 传导)——使未来实现账号删除的人第一时间撞上带说明的测试,而不是生产事故。真正的清理接线随账号删除功能立项。

## 5. preview.png

- 包内可选文件,作为普通资产入库(`theme_assets`,path=`preview.png`,PNG magic + ≤ 512 KiB 由既有资产校验层管)。
- `themes.preview` 列回写为其内容寻址 URL(`/api/v1/public/themes/{versionId}/assets/preview.png`),升级时随版本更新;无 preview.png 的包(含全部内置主题)保持空串,前端沿用现有占位。
- openapi 的 `Theme.preview` 语义自此成真,选择器与管理后台卡片免费获得真实预览图。

## 6. API 契约

| 端点 | 语义 |
|---|---|
| `POST /api/v1/me/themes/import` | 二选一:multipart(`file`=zip)或 JSON `{githubUrl, ref?}`。201 返回 `Theme`;错误返回结构化校验错误(见 validate);409=配额满;422=包不合规 |
| `DELETE /api/v1/me/themes/{themeId}` | 卸载(物理删或墓碑,见 §4);204;404=非本人或不存在(不区分,防枚举) |
| `POST /api/v1/themes/validate` | dry-run:multipart zip,走完整管线但不落库。200 返回 `{valid, errors: [{stage, path, message}]}`(stage ∈ manifest/css/asset/archive);要求登录 |

- 列表复用既有 `GET /api/v1/themes`(eligibility 谓词已含私有分支,带会话即返回本人私有主题——A 阶段已就绪,零改动)。
- 限流:`AbuseProtection()` 规则表新增两条——import 10 次/小时/IP、validate 20 次/小时/IP(与登录/邀请/事件/链接检查并列的独立限流)。
- `api/openapi.yaml` 同步新增端点与 `ThemeImportError` schema;`tests/contract/` 覆盖:zip 导入 201 → 列表出现 → 应用并发布 → 公开 CSS 可取;配额 409;坏包 422;validate 200(valid=false 结构化错误);卸载 204 与发布回落。mock 同步,`make test-mock` 保持绿。

## 7. 前端

- `/app/themes` 新增「导入主题」入口 → 对话框两个 tab:GitHub(URL + ref 输入)/ 上传 zip;提交后轮询无必要(导入是同步请求,秒级)。
- 私有主题在选择器中独立分组「我的主题」,卡片带卸载按钮(墓碑态显示原因)与升级入口(GitHub 来源显示「重新拉取」,upload 显示「上传新版本」)。
- 校验错误按 `stage/path/message` 结构化展示(作者能定位到 `theme.css` 第几段被哪条规则拒绝的程度以 message 文本为准,不做行号映射)。
- 全部走 `web/src/api/themes.ts`(新增域模块)。浏览器六态冒烟照 CLAUDE.md。

## 8. 测试策略

- **解包表驱动**:zip-slip(`../`、绝对路径、`..\\`)、tar 符号链接/硬链接、解压炸弹(高压缩比)、文件数超限、顶层目录剥离(有/无)、白名单外文件忽略。
- **GitHub 导入**:用 `httptest` 假 codeload/api 服务器(netguard 需放行测试回环——沿用 linkcheck 既有测试模式);ref→sha 解析、sha 直用、白名单外主机拒绝、超限响应体截断。SSRF 拒绝矩阵复用 netguard 既有用例,不重复造。
- **归属与配额**:导入后 A 可见/B 不可见(eligibility 已有,补端到端);配额满 409;升级不占额度;卸载物理删 vs 墓碑两分支;墓碑后公开页仍可取 CSS。
- **元数据回写**:导入与内置同步后行元数据与 manifest 一致(顺带钉死缺口 6 的修复)。
- **用户删除不变量**:见 §4 末条。
- **E2E**:fixture zip(放 `tests/e2e/fixtures/`,一个最小合法主题)→ 导入 → 选择器出现 → 应用 → 发布 → 公开页 link 指向其版本;坏包导入报错可读。
- 门槛照旧:`make check`、`go test -race ./...`、`make build`、`make test-contract`、`make test-mock`、`make e2e`。

## 9. 不在范围(B2)

目录审核(私有 → 提交目录 → 管理员审批)、版本级 kill switch 写路径(`theme_versions.status='disabled'` 的管理端入口;B1 管理员只有整包 `enabled` 开关)、更新检查(upstream 新 commit 提示)、官方 starter 仓库、`NAVAX_THEME_IMPORT_HOSTS` 之外的非 GitHub 布局适配。

## 10. 风险与权衡

| 风险 | 缓解 |
|---|---|
| GitHub 匿名 API 限额 60/h/IP | sha 直填可绕过 API;`NAVAX_GITHUB_TOKEN` 可选;zip 上传兜底 |
| 导入是同步请求,大包+慢网可能长时间占用连接 | 压缩体 16 MiB 硬上限 + http client 超时(30s);不引入队列(与「无队列」边界一致) |
| 墓碑永久占配额 | 展示原因;B2 的清理任务可在快照不再引用后回收;配额可配 |
| 私有主题成为 XSS/钓鱼载体 | 与内置同一校验器(A 阶段的全部拒绝规则 + 隔离边界),无新增执行面;`content` 装饰字符规则已限 ≤2 码位 |
| 解包层是全新攻击面 | 白名单提取 + 四项硬限 + 表驱动对抗用例;不解析除 zip/tar.gz 外的任何格式 |
