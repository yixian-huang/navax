# nav.ax - 个性化导航站

## 1. 项目描述
nav.ax 是一个开源、可自行部署的个性化导航站。用户通过邀请注册，可以收藏站点、创建分类、拖拽编排个人导航主页、设置首页信息、切换主题，并将导航主页公开分享。

- **目标用户**：希望拥有个性化导航主页的个人用户
- **核心价值**：沉静、精确、有温度的个人互联网工作台

## 2. 页面结构

### 公共区域
- `/` - 实例主导航首页
- `/u/:slug` - 用户公开分享的导航主页
- `/login` - 登录页
- `/invite/:token` - 接受邀请并创建账号
- `/404` - 未找到页面

### 用户管理区域
- `/app` - 个人导航编辑总览
- `/app/links` - 站点与分类管理
- `/app/layout` - 主页布局和拖拽编排
- `/app/widgets` - 工具组件管理
- `/app/themes` - 主题、背景和显示密度
- `/app/publish` - 公开设置、预览和发布
- `/app/domain` - 子域名申请与域名管理
- `/app/import-export` - 书签导入和数据导出
- `/app/settings` - 个人资料、安全和偏好设置

### 运营管理区域
- `/admin` - 运营概览
- `/admin/users` - 用户管理
- `/admin/invitations` - 邀请链接管理
- `/admin/directory` - 平台推荐站点库
- `/admin/categories` - 平台公共分类
- `/admin/themes` - 可用主题管理
- `/admin/settings` - 系统配置
- `/admin/audit` - 操作记录和系统状态

## 3. 核心功能
- [ ] 搜索引擎切换
- [ ] 分类导航
- [ ] 站点收藏与展示（图标卡片）
- [ ] 三种密度模式（列表、紧凑网格、舒展网格）
- [ ] 搜索过滤收藏
- [ ] 时钟、日期、便签等轻量组件
- [ ] 拖拽排序站点和分类
- [ ] 桌面/平板/移动端预览
- [ ] 自动保存草稿
- [ ] 发布管理（预览、发布、状态显示）
- [ ] 主题切换（亮色/暗色 + 扩展主题）
- [ ] 邀请注册
- [ ] 公开分享主页
- [ ] 书签导入/数据导出
- [ ] 子域名申请与管理 (xxx.nav.ax)
- [ ] 运营管理后台

## 4. 数据模型设计

### NavigationPage
- id, ownerId, title, slug, description
- visibility: private | unlisted | public
- themeId, layout, categories, widgets
- draftUpdatedAt, publishedAt, hasUnpublishedChanges

### User
- id, username, email, avatarUrl, role, status, createdAt

### Category
- id, pageId, name, icon, sortOrder, sites

### Site
- id, categoryId, title, url, icon, description, sortOrder

### Widget
- id, pageId, type, config, position, enabled

### Invitation
- id, code, createdBy, maxUses, usedCount, expiresAt, isRevoked

### Theme
- id, name, author, mode (light/dark/both), tokens, preview

## 5. 后端集成计划
- **Go REST API**：所有数据通过 `/api/v1` 获取
- **认证**：HttpOnly Cookie 管理 session
- 无 Supabase、Firebase、Stripe、Shopify 集成

## 6. 开发阶段计划

### Phase 1: 基础设施与核心页面 ✅
- 目标：建立设计系统、主题引擎、API 层、Mock 数据，完成所有可访问路由的基础页面
- 交付物：设计令牌、CSS 变量主题系统（default-light, default-dark, forest）、API 类型和客户端、Mock 数据拦截器、所有 20+ 路由可访问的基础页面、Toast/ConfirmDialog/Loading/Empty/Error 等交互组件
- 状态：已完成

### Phase 2: 用户管理区域
- 目标：完成 /app/* 所有页面的交互功能
- 交付物：导航管理、拖拽排序、发布流程、主题切换、导入导出

### Phase 3: 公开分享与组件
- 目标：完善公开分享页、Widget 组件、SEO 元数据
- 交付物：公开页完整渲染、时钟/日期/便签组件、Open Graph 支持

### Phase 4: 运营管理后台
- 目标：完成 /admin/* 所有页面
- 交付物：数据表格、筛选、分页、用户管理、邀请管理

### Phase 5: 打磨与优化
- 目标：动画、无障碍、响应式验证、主题完善
- 交付物：动效、键盘导航、WCAG AA、移动端优化
