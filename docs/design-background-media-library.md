# 设计：导航页背景媒体库

状态：草案（产品 + 技术）  
相关：主题设置 `appearance.background`、资产上传 `internal/assets`、发布快照

## 1. 目标

在**不依赖 S3** 的前提下（本地 `data/assets` 默认可用），为导航页背景提供：

1. **站长预设库**：管理员预上传最多 N 张（默认 N=12）媒体，所有用户可在选择器中选用。  
2. **用户私有库**：每个用户最多 **3** 个自有背景，可选用、删除、替换。  
3. **当前选用**：页面设置仍只保存「当前生效的一条」`appearance.background`（兼容现有发布模型）。  
4. **动态媒体**：支持静态图、动图（GIF/动 WebP）、短视频（WebM/MP4）作背景。  
5. **上传压缩**：在**不主动降低分辨率**的前提下减小体积（质量/码率/编码优化；可选「最长边上限」仅作防护）。

## 2. 非目标（首期）

- 不为每个用户做独立 CDN 转码集群；转码在**上传请求内同步/短时**完成（超时则拒绝或仅存原图）。  
- 不支持直播流、外链任意视频（防 SSRF / 版权 / 体积失控）。  
- 不改变「草稿 → 发布」模型：选中的背景仍进草稿，发布后访客才看到。

## 3. 角色与权限

| 角色 | 预设库 | 用户库 | 选用到自己的导航页 |
|------|--------|--------|-------------------|
| 游客 | 只读公开页上的**已发布**背景 | 无 | 无 |
| 登录用户 | 可浏览、选用站长预设 | 上传 ≤3、删除自己的、选用 | 写个人/有权限的页草稿 |
| 管理员 | 上传/排序/启用禁用/删除预设 | 同上 + 可管理他人策略（可选二期） | 主站（system）草稿 |

**选用规则**：  
- 选用预设：草稿 `background.value` 存**预设媒体的公开 URL**（或稳定 id，见 §5）。  
- 选用自有：存用户资产 URL。  
- 删除：若当前页草稿/已发布仍引用该媒体，删除时 **拒绝** 或 **自动回退** 为 `type:none`（推荐：删除时若引用则要求先换掉，避免线上断图）。

## 4. 用户流程

### 4.1 主题设置页 — 背景选择器

```
[ 当前预览 ]
[ 不透明度 ────○── ]

来源：  (•) 站长精选   ( ) 我的上传   ( ) 外链 URL（保留现有能力，可折叠）

┌ 网格缩略图 ┐  ┌ ─┐  ┌ ─┐
│ 预设1 预设2 │  │+│  │…│
└─────────────┘  └─┘  └─┘
选中 → 写入草稿 background
「发布更新」→ 公开页生效
```

- **站长精选**：只读网格；角标区分「图 / 动图 / 视频」。  
- **我的上传**：展示 0–3 槽位；空槽「上传」；满则禁用上传并提示。  
- 管理员额外入口：「管理预设库」→ 管理端或同页折叠管理区。

### 4.2 公开导航页

- `type: image`：`<img>` 全屏 cover（现有）。  
- `type: video`：`<video autoplay muted loop playsinline>` 全屏 cover；无声音。  
- 动图仍走 `image`（浏览器当作图解码）。  
- 对比度采样：对 **视频首帧 / 静图** 做亮度采样（现有 `wallpaperTone` 扩展）。

## 5. 数据模型

### 5.1 新表 `background_media`（建议）

| 字段 | 说明 |
|------|------|
| id | `bgm_…` |
| scope | `instance`（站长预设）\| `user`（用户私有） |
| owner_user_id | 预设为 null；用户库为用户 id |
| asset_id | 指向 `assets.id` 或内联 object_key |
| media_kind | `image` \| `video` |
| mime_type | image/png, image/jpeg, image/webp, image/gif, video/webm, video/mp4 |
| url | 公开路径 `/api/v1/assets/...` 或 `/api/v1/media/...` |
| poster_url | 视频封面（可选，上传时截帧） |
| width, height | 像素 |
| duration_ms | 视频时长；图为 null |
| size_bytes | 压缩后体积 |
| sort_order | 预设排序 |
| enabled | 预设可下架 |
| created_at | |

约束：

- `scope=user`：同一 `owner_user_id` 最多 **3** 行（应用层 + 可选 partial unique 触发器/计数）。  
- `scope=instance`：总数 ≤ **N**（配置项 `max_instance_backgrounds`，默认 12）。

### 5.2 页面设置（兼容扩展）

现有：

```json
"background": { "type": "none"|"image", "value": "", "opacity": 1 }
```

扩展为：

```json
"background": {
  "type": "none" | "image" | "video",
  "value": "https://... | /api/v1/assets/...",
  "opacity": 0.85,
  "mediaId": "bgm_xxx" | null,
  "poster": "/api/v1/assets/..." | null
}
```

- 旧快照无 `mediaId`/`video` 仍可读。  
- 发布快照继续整包 JSON，公开页不查库也能播。

### 5.3 系统配置（system_settings 或 limits）

| 键 | 默认 | 说明 |
|----|------|------|
| max_instance_backgrounds | 12 | 站长预设上限 N |
| max_user_backgrounds | 3 | 用户库上限 |
| max_background_upload_bytes | 8–15MB | 压缩前/后上限（见 §7） |
| max_background_video_seconds | 15 | 视频最长时长 |
| max_background_edge_px | 2560 | 可选：仅超过时等比缩小（防护，默认不缩小） |

## 6. API 草案

### 6.1 列表

- `GET /api/v1/backgrounds/presets`  
  登录可读；返回 enabled 的 instance 媒体（缩略图 + mediaKind）。  
- `GET /api/v1/backgrounds/mine`  
  当前用户私有库（0–3）。

### 6.2 上传

- `POST /api/v1/backgrounds/presets`（admin）  
  multipart: `file`  
  → 校验 → 压缩/转码 → 写 assets + background_media(scope=instance)  
- `POST /api/v1/backgrounds/mine`（user）  
  同上 scope=user；满 3 条 → 422。

### 6.3 管理

- `DELETE /api/v1/backgrounds/presets/{id}`（admin）  
- `DELETE /api/v1/backgrounds/mine/{id}`（owner）  
- `PUT /api/v1/backgrounds/presets/order`（admin）  
- `PATCH /api/v1/backgrounds/presets/{id}` enabled  

### 6.4 选用（可复用现有）

- 继续 `PUT .../pages/{id}/settings`，body 中 `appearance.background` 设为选中项。  
- 前端选择器：点缩略图 → 调 settings，不必新「apply」接口。

### 6.5 读取

- 图片：现有 `GET /api/v1/assets/{kind}/{object}`  
- 视频：同路由或 `GET /api/v1/media/background/{object}`（Range 支持，便于 `<video>` 拖动；首期可先整文件 + 合理 Cache-Control）

## 7. 上传与压缩策略

### 7.1 允许类型

| 类型 | MIME | 说明 |
|------|------|------|
| 静图 | jpeg, png, webp | 主路径 |
| 动图 | gif, webp（动画） | `media_kind=image`，公开页用 img |
| 视频 | webm, mp4 | `media_kind=video`；仅 muted loop |

拒绝：svg、任意可执行内容、超长视频、音频轨强制剥离（转码时 `-an`）。

### 7.2 「不降分辨率」压缩（实现原则）

**静图（服务端，推荐）**

1. 解码校验宽高（背景最短边 ≥ 64，已有规则可提高）。  
2. **默认不缩小宽高**；仅当 `max(w,h) > max_background_edge_px`（默认 2560）时等比缩小（防护超大手机原图）。  
3. 输出：优先 **WebP quality 80–85**（或 JPEG quality 85），strip 元数据；若原图已更小则保留原编码。  
4. 目标：同分辨率下体积明显下降，不主动「压糊」。

**动图**

1. 校验帧数/时长上限（如 ≤ 8s 或帧数上限）。  
2. 首期可 **原样存储**（避免服务端动图重编码复杂）；或限制体积。  
3. 二期：转码为动 WebP。

**视频（服务端，若本机有 ffmpeg）**

1. 时长 ≤ 15s；分辨率最长边 ≤ 1080 或 1440（**视频**可设更严上限，因体积敏感；与静图策略分开配置）。  
2. 转码：H.264/AAC 去音轨 或 VP9/WebM，CRF 质量档，`faststart`。  
3. 截取首帧作 `poster_url`。  
4. 无 ffmpeg：拒绝视频并提示，或仅允许 gif/webp 动效（部署文档写清依赖）。

**客户端（可选增强）**

- 浏览器侧用 Canvas/`createImageBitmap` 做 WebP 再上传，减少上行；**服务端仍二次校验**。

### 7.3 存储

- 继续默认 **local** `NAVAX_DATA_DIR/assets/...`；S3 完整配置时走对象存储，失败回退本地（已实现）。  
- kind 可扩展：`background` 继续用，或 `background-video` 分目录。

## 8. 前端改动要点

| 区域 | 改动 |
|------|------|
| `app/themes` | 背景选择器（预设 / 我的 / 外链）；槽位 3；视频预览 |
| `PublicShell` | `type===video` 渲染 video；tone 采样用 poster 或 video 首帧 |
| `wallpaperTone` | 支持 video 元素 / poster URL 采样 |
| Admin | 预设库管理（可在运营页或 themes 管理区） |
| i18n | 中文文案为主 |

## 9. 安全

- 上传 MIME + 魔数 + 解码校验（现有图片路径扩展到视频容器探测）。  
- 禁止 SVG。  
- 视频/大文件：大小上限、时长上限、CPU 超时（context deadline）。  
- 用户仅能删自己的 `scope=user`。  
- 公开 URL 不可枚举敏感路径（继续随机 object key）。  
- Origin / 会话 / 权限与现有一致。

## 10. 发布与兼容

1. 迁移：建 `background_media`；可选把历史 `background.value` 不回填库。  
2. OpenAPI：新 endpoints + `background.type` 含 `video`。  
3. 契约测试：上传预设 → 用户选用 → 发布 → public home 含 video/image。  
4. Mock：预设 4 条 + 用户库逻辑。

## 11. 分阶段落地（PR 建议）

| 阶段 | 内容 | 价值 |
|------|------|------|
| **P0** | 预设库 + 用户库（静图 only）+ 选择器 + 本地存储 + 静图压缩 | 覆盖你提的前两条主路径 |
| **P1** | 动图（gif/webp）+ 更严体积 | 动态背景轻量版 |
| **P2** | 视频（ffmpeg）+ poster + PublicShell video + Range | 完整动态背景 |
| **P3** | 客户端预压缩、管理端排序/下架运营体验 | 体验与运维 |

## 12. 开放决策（实现前确认）

1. **N 默认值**：12 是否合适？  
2. **视频是否 P0 必做**，还是 P0 只做图 + 动图？  
3. **删除被引用媒体**：硬拒绝 vs 自动清空 background。  
4. **部署是否保证 ffmpeg**（Docker 镜像是否安装）。  
5. **用户 3 张**是「同时保留 3 个文件」还是「历史累计」——按「同时保留 3」设计。

## 13. 成功标准

- 无 S3 时可完整：管理员传预设、用户选预设、用户自传 ≤3、发布后首页可见。  
- 降低壁纸透明度时字色仍可读（现有 tone × opacity 逻辑保留）。  
- 静图同分辨率体积相对原图有可感知下降（抽样回归）。  
- 游客空状态无「添加站点」；背景选择仅登录后。
