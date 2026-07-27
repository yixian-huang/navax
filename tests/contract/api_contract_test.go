package contract

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// TestAPIContract 按“系统 → 引导 → 管理员 → 邀请注册 → 用户 → 公开读取”的顺序
// 跑通关键流程；每一步的请求与响应都会与 api/openapi.yaml 校验。
// 步骤间有状态依赖，必须顺序执行。
func TestAPIContract(t *testing.T) {
	testingShort(t)

	guest := newAPIClient(t)
	admin := newAPIClient(t)
	user := newAPIClient(t)

	const adminEmail = "admin@example.com"
	const adminPassword = "Contract-Pass-2026!"
	const userEmail = "member@example.com"
	const userPassword = "Member-Pass-2026!"

	var (
		adminPageID          string
		systemPageID         string
		publicThemeVersionID string
		userPageID           string
		userSlug             string
		inviteToken          string
		categoryID           string
		userID               string
	)

	t.Run("系统端点", func(t *testing.T) {
		mustStatus(t, guest.call(t, http.MethodGet, "/healthz", nil), http.StatusOK, "healthz")
		mustStatus(t, guest.call(t, http.MethodGet, "/readyz", nil), http.StatusOK, "readyz")
		mustStatus(t, guest.call(t, http.MethodGet, "/api/v1/version", nil), http.StatusOK, "version")

		status := guest.call(t, http.MethodGet, "/api/v1/bootstrap/status", nil)
		mustStatus(t, status, http.StatusOK, "bootstrap status")
		if initialized, _ := status.data()["initialized"].(bool); initialized {
			t.Fatal("实例不应处于已初始化状态")
		}
	})

	t.Run("引导初始化", func(t *testing.T) {
		body := map[string]any{
			"adminUsername": "admin",
			"adminEmail":    adminEmail,
			"adminPassword": adminPassword,
			"instanceName":  "nav.ax",
			"publicBaseUrl": baseURL,
		}

		rejected := admin.call(t, http.MethodPost, "/api/v1/bootstrap", body,
			withHeader("X-Setup-Token", "wrong-token-wrong-token-wrong-token-000"))
		mustStatus(t, rejected, http.StatusUnauthorized, "错误 setup token 应被拒绝")

		created := admin.call(t, http.MethodPost, "/api/v1/bootstrap", body,
			withHeader("X-Setup-Token", setupToken))
		mustStatus(t, created, http.StatusCreated, "bootstrap")

		status := guest.call(t, http.MethodGet, "/api/v1/bootstrap/status", nil)
		if initialized, _ := status.data()["initialized"].(bool); !initialized {
			t.Fatal("bootstrap 后实例应为已初始化")
		}
	})

	t.Run("认证会话", func(t *testing.T) {
		session := admin.call(t, http.MethodGet, "/api/v1/auth/session", nil)
		mustStatus(t, session, http.StatusOK, "bootstrap 后的会话")

		mustStatus(t, admin.call(t, http.MethodPost, "/api/v1/auth/logout", nil), http.StatusOK, "登出")

		badLogin := admin.call(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email": adminEmail, "password": "wrong-password-!",
		})
		mustStatus(t, badLogin, http.StatusUnauthorized, "错误密码登录")

		login := admin.call(t, http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email": adminEmail, "password": adminPassword,
		})
		mustStatus(t, login, http.StatusOK, "管理员登录")
	})

	t.Run("未认证访问受保护端点", func(t *testing.T) {
		denied := guest.call(t, http.MethodGet, "/api/v1/pages/current", nil, withoutRequestValidation())
		mustStatus(t, denied, http.StatusUnauthorized, "游客访问 pages/current")
	})

	t.Run("页面草稿编辑", func(t *testing.T) {
		page := admin.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil)
		mustStatus(t, page, http.StatusOK, "获取个人页面")
		adminPageID = stringField(t, page.data(), "id", "个人页面")

		category := admin.call(t, http.MethodPost,
			fmt.Sprintf("/api/v1/pages/%s/categories", adminPageID),
			map[string]any{"name": "工具", "icon": "ri-tools-line"})
		mustStatus(t, category, http.StatusCreated, "创建分类")
		categoryID = stringField(t, category.data(), "id", "分类")

		site := admin.call(t, http.MethodPost,
			fmt.Sprintf("/api/v1/pages/%s/sites", adminPageID),
			map[string]any{"categoryId": categoryID, "title": "Example", "url": "https://example.com"})
		mustStatus(t, site, http.StatusCreated, "创建站点")
		stringField(t, site.data(), "id", "站点")

		refreshed := admin.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil)
		revision := numberField(t, refreshed.data(), "draftRevision", "页面修订号")

		// 排序请求必须覆盖页面上全部分类与站点。
		order := admin.call(t, http.MethodPut,
			fmt.Sprintf("/api/v1/pages/%s/content-order", adminPageID),
			map[string]any{
				"expectedRevision": revision,
				"categories":       contentOrderFromPage(t, refreshed.data()),
			})
		mustStatus(t, order, http.StatusOK, "内容排序")
	})

	t.Run("页面设置", func(t *testing.T) {
		page := admin.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil)
		settings, ok := page.data()["settings"].(map[string]any)
		if !ok {
			t.Fatalf("页面缺少 settings: %v", page.data())
		}
		appearance := settings["appearance"].(map[string]any)
		appearance["themeId"] = "slate-dark"
		settings["expectedRevision"] = numberField(t, page.data(), "draftRevision", "修订号")

		updated := admin.call(t, http.MethodPut,
			fmt.Sprintf("/api/v1/pages/%s/settings", adminPageID), settings)
		mustStatus(t, updated, http.StatusOK, "更新设置")

		conflict := admin.call(t, http.MethodPut,
			fmt.Sprintf("/api/v1/pages/%s/settings", adminPageID), settings)
		mustStatus(t, conflict, http.StatusPreconditionFailed, "过期修订号应返回 412")
	})

	t.Run("发布与公开读取", func(t *testing.T) {
		publication := admin.call(t, http.MethodGet,
			fmt.Sprintf("/api/v1/pages/%s/publication", adminPageID), nil)
		mustStatus(t, publication, http.StatusOK, "获取发布状态")
		slug := stringField(t, publication.data(), "slug", "发布 slug")

		page := admin.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil)
		revision := numberField(t, page.data(), "draftRevision", "修订号")

		visibility := admin.call(t, http.MethodPut,
			fmt.Sprintf("/api/v1/pages/%s/publication", adminPageID),
			map[string]any{"visibility": "public", "slug": slug, "showAuthor": true,
				"seoTitle": "", "seoDescription": ""})
		mustStatus(t, visibility, http.StatusOK, "设置可见性")

		published := admin.call(t, http.MethodPost,
			fmt.Sprintf("/api/v1/pages/%s/publish", adminPageID),
			map[string]any{"expectedRevision": revision},
			withHeader("Idempotency-Key", "contract-publish-personal-0001"))
		mustStatus(t, published, http.StatusOK, "发布个人页面")

		publicPage := guest.call(t, http.MethodGet, "/api/v1/public/pages/"+slug, nil)
		mustStatus(t, publicPage, http.StatusOK, "公开读取个人页面")

		// 发布必然锁定主题版本;该字段自本次起登记进 openapi。
		publicThemeVersionID = stringField(t, publicPage.data(), "themeVersionId", "公开页主题版本")
		if !regexp.MustCompile(`^v[0-9a-f]{32}$`).MatchString(publicThemeVersionID) {
			t.Fatalf("themeVersionId 形状不合法: %q", publicThemeVersionID)
		}

		previewResult := admin.call(t, http.MethodGet,
			fmt.Sprintf("/api/v1/pages/%s/preview", adminPageID), nil)
		mustStatus(t, previewResult, http.StatusOK, "草稿预览")
		// 预览与发布解析出同一个版本(同一份草稿、同一个谓词)。
		if got := stringField(t, previewResult.data(), "themeVersionId", "预览主题版本"); got != publicThemeVersionID {
			t.Fatalf("预览主题版本 %q != 发布主题版本 %q", got, publicThemeVersionID)
		}

		missing := guest.call(t, http.MethodGet, "/api/v1/public/pages/does-not-exist", nil)
		mustStatus(t, missing, http.StatusNotFound, "读取不存在的公开页面")
	})

	t.Run("主题目录与公开样式表", func(t *testing.T) {
		list := guest.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, list, http.StatusOK, "主题目录")

		cssPath := "/api/v1/public/themes/" + publicThemeVersionID + ".css"
		css := guest.call(t, http.MethodGet, cssPath, nil)
		mustStatus(t, css, http.StatusOK, "主题样式表")
		if cacheControl := css.header.Get("Cache-Control"); !strings.Contains(cacheControl, "immutable") {
			t.Fatalf("样式表缺少 immutable 缓存头: %q", cacheControl)
		}
		etag := css.header.Get("ETag")
		if etag == "" {
			t.Fatal("样式表缺少 ETag")
		}
		cached := guest.call(t, http.MethodGet, cssPath, nil, withHeader("If-None-Match", etag))
		mustStatus(t, cached, http.StatusNotModified, "样式表 304")

		missing := guest.call(t, http.MethodGet,
			"/api/v1/public/themes/v00000000000000000000000000000000.css", nil)
		mustStatus(t, missing, http.StatusNotFound, "未知版本 404")

		missingAsset := guest.call(t, http.MethodGet,
			"/api/v1/public/themes/"+publicThemeVersionID+"/assets/nope.png", nil)
		mustStatus(t, missingAsset, http.StatusNotFound, "未知资产 404")
	})

	t.Run("系统页发布与公开首页", func(t *testing.T) {
		system := admin.call(t, http.MethodGet, "/api/v1/pages/current?scope=system", nil)
		mustStatus(t, system, http.StatusOK, "获取系统页面")
		systemPageID = stringField(t, system.data(), "id", "系统页面")
		revision := numberField(t, system.data(), "draftRevision", "系统页修订号")

		publication := admin.call(t, http.MethodGet,
			fmt.Sprintf("/api/v1/pages/%s/publication", systemPageID), nil)
		mustStatus(t, publication, http.StatusOK, "系统页发布状态")
		slug := stringField(t, publication.data(), "slug", "系统页 slug")

		visibility := admin.call(t, http.MethodPut,
			fmt.Sprintf("/api/v1/pages/%s/publication", systemPageID),
			map[string]any{"visibility": "public", "slug": slug, "showAuthor": false,
				"seoTitle": "", "seoDescription": ""})
		mustStatus(t, visibility, http.StatusOK, "系统页设置可见性")

		published := admin.call(t, http.MethodPost,
			fmt.Sprintf("/api/v1/pages/%s/publish", systemPageID),
			map[string]any{"expectedRevision": revision},
			withHeader("Idempotency-Key", "contract-publish-system-0001"))
		mustStatus(t, published, http.StatusOK, "发布系统页面")

		home := guest.call(t, http.MethodGet, "/api/v1/public/home", nil)
		mustStatus(t, home, http.StatusOK, "公开首页")
	})

	t.Run("公开事件与个人统计", func(t *testing.T) {
		event := guest.call(t, http.MethodPost, "/api/v1/public/events",
			map[string]any{"type": "page_view", "pageId": adminPageID, "clientEventId": "contract-evt-0001"})
		mustStatus(t, event, http.StatusAccepted, "上报公开事件")

		overview := admin.call(t, http.MethodGet, "/api/v1/me/analytics/overview?period=7", nil)
		mustStatus(t, overview, http.StatusOK, "统计概览")
		trends := admin.call(t, http.MethodGet, "/api/v1/me/analytics/trends?period=30", nil)
		mustStatus(t, trends, http.StatusOK, "统计趋势")
		breakdown := admin.call(t, http.MethodGet, "/api/v1/me/analytics/breakdown", nil)
		mustStatus(t, breakdown, http.StatusOK, "统计明细")
	})

	t.Run("导入导出", func(t *testing.T) {
		exported := admin.call(t, http.MethodGet,
			fmt.Sprintf("/api/v1/pages/%s/export?format=navax-json", adminPageID), nil)
		mustStatus(t, exported, http.StatusOK, "导出 JSON")

		bookmarks := admin.call(t, http.MethodGet,
			fmt.Sprintf("/api/v1/pages/%s/export?format=bookmarks-html", adminPageID), nil)
		mustStatus(t, bookmarks, http.StatusOK, "导出书签")
	})

	t.Run("公开配置与背景图上传", func(t *testing.T) {
		config := guest.call(t, http.MethodGet, "/api/v1/public/config", nil)
		mustStatus(t, config, http.StatusOK, "公开配置")
		limits, _ := config.data()["limits"].(map[string]any)
		if numberField(t, limits, "maxUploadBytes", "上传上限") <= 0 {
			t.Fatal("maxUploadBytes 应为正数")
		}

		uploaded := admin.uploadPNG(t, "background", tinyPNG(t))
		mustStatus(t, uploaded, http.StatusCreated, "上传背景图")
		assetURL := stringField(t, uploaded.data(), "url", "资源")

		fetched := guest.call(t, http.MethodGet, assetURL, nil)
		mustStatus(t, fetched, http.StatusOK, "读取上传的图片")

		rejected := admin.uploadPNG(t, "background", []byte("not a real image"))
		mustStatus(t, rejected, http.StatusUnsupportedMediaType, "非图片内容应被拒绝")
	})

	t.Run("邀请注册新用户", func(t *testing.T) {
		invitation := admin.call(t, http.MethodPost, "/api/v1/admin/invitations",
			map[string]any{"maxUses": 1, "expiresInDays": 7})
		mustStatus(t, invitation, http.StatusCreated, "创建邀请")
		inviteToken = stringField(t, invitation.data(), "token", "邀请")

		validated := guest.call(t, http.MethodGet, "/api/v1/auth/invitations/"+inviteToken, nil)
		mustStatus(t, validated, http.StatusOK, "校验邀请")

		registered := user.call(t, http.MethodPost,
			"/api/v1/auth/invitations/"+inviteToken+"/register",
			map[string]any{"username": "member01", "email": userEmail, "password": userPassword})
		mustStatus(t, registered, http.StatusCreated, "邀请注册")
		account, _ := registered.data()["user"].(map[string]any)
		userID = stringField(t, account, "id", "注册用户")
	})

	t.Run("用户编辑与发布", func(t *testing.T) {
		page := user.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil)
		mustStatus(t, page, http.StatusOK, "用户页面")
		userPageID = stringField(t, page.data(), "id", "用户页面")

		category := user.call(t, http.MethodPost,
			fmt.Sprintf("/api/v1/pages/%s/categories", userPageID),
			map[string]any{"name": "阅读"})
		mustStatus(t, category, http.StatusCreated, "用户创建分类")
		userCategoryID := stringField(t, category.data(), "id", "用户分类")

		site := user.call(t, http.MethodPost,
			fmt.Sprintf("/api/v1/pages/%s/sites", userPageID),
			map[string]any{"categoryId": userCategoryID, "title": "IETF", "url": "https://www.ietf.org"})
		mustStatus(t, site, http.StatusCreated, "用户创建站点")

		publication := user.call(t, http.MethodGet,
			fmt.Sprintf("/api/v1/pages/%s/publication", userPageID), nil)
		userSlug = stringField(t, publication.data(), "slug", "用户 slug")

		visibility := user.call(t, http.MethodPut,
			fmt.Sprintf("/api/v1/pages/%s/publication", userPageID),
			map[string]any{"visibility": "public", "slug": userSlug, "showAuthor": true,
				"seoTitle": "", "seoDescription": ""})
		mustStatus(t, visibility, http.StatusOK, "用户设置可见性")

		refreshed := user.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil)
		revision := numberField(t, refreshed.data(), "draftRevision", "用户页修订号")
		published := user.call(t, http.MethodPost,
			fmt.Sprintf("/api/v1/pages/%s/publish", userPageID),
			map[string]any{"expectedRevision": revision},
			withHeader("Idempotency-Key", "contract-publish-user-00001"))
		mustStatus(t, published, http.StatusOK, "用户发布")

		publicPage := guest.call(t, http.MethodGet, "/api/v1/public/pages/"+userSlug, nil)
		mustStatus(t, publicPage, http.StatusOK, "公开读取用户页面")
	})

	t.Run("子域名申请", func(t *testing.T) {
		enabled := admin.call(t, http.MethodPatch, "/api/v1/admin/settings",
			map[string]any{"domain": map[string]any{"rootDomain": "contract.test", "subdomainsEnabled": true}})
		mustStatus(t, enabled, http.StatusOK, "启用子域名")

		none := user.call(t, http.MethodGet, "/api/v1/me/subdomain", nil)
		mustStatus(t, none, http.StatusOK, "查询子域名（无申请）")

		applied := user.call(t, http.MethodPost, "/api/v1/me/subdomain",
			map[string]any{"label": "member01"})
		mustStatus(t, applied, http.StatusCreated, "申请子域名")

		current := user.call(t, http.MethodGet, "/api/v1/me/subdomain", nil)
		mustStatus(t, current, http.StatusOK, "查询子域名")
	})

	t.Run("个人资料", func(t *testing.T) {
		profile := user.call(t, http.MethodGet, "/api/v1/me/profile", nil)
		mustStatus(t, profile, http.StatusOK, "个人资料")
		sessions := user.call(t, http.MethodGet, "/api/v1/me/sessions", nil)
		mustStatus(t, sessions, http.StatusOK, "会话列表")
	})

	t.Run("主题导入与私有安装", func(t *testing.T) {
		// zip 导入 → 201 且响应过 Theme schema 校验。
		imported := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "lilac.zip", buildThemeZip(t, "lilac"))
		mustStatus(t, imported, http.StatusCreated, "导入主题")
		importedID := stringField(t, imported.data(), "id", "导入主题 ID")

		// upload 主题检查更新:无 upstream,hasUpdate=false(无网络)。
		checkUpd := user.call(t, http.MethodPost, "/api/v1/me/themes/"+importedID+"/check-update", nil)
		mustStatus(t, checkUpd, http.StatusOK, "检查更新(upload)")
		if hasUpdate, _ := checkUpd.data()["hasUpdate"].(bool); hasUpdate {
			t.Fatal("upload 主题不应有更新")
		}

		// 列表出现(带会话)。
		list := user.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, list, http.StatusOK, "含私有主题的列表")
		if !strings.Contains(string(list.body), importedID) {
			t.Fatalf("列表缺少导入的主题 %s", importedID)
		}

		// 匿名(无会话)看不到别人的私有主题。这是唯一端到端覆盖
		// OptionalSession 真按 Cookie 解析 actorID(而不是恒定放行/恒定拒绝)
		// 的断言:/api/v1/themes 本身对匿名开放(200),但响应体不得包含
		// importedID——否则任何人都能靠猜测/枚举 id 蹭到别人的私有主题。
		anonymous := guest.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, anonymous, http.StatusOK, "匿名读取主题列表")
		if strings.Contains(string(anonymous.body), importedID) {
			t.Fatalf("匿名列表泄露了他人私有主题 %s", importedID)
		}

		// 应用到自己的页面并发布，公开读锁定其版本。PUT /settings 是全量替换
		// （PageSettingsUpdate = PageSettings 全字段 + expectedRevision），
		// 因此要在取回的完整 settings 上原地改 themeId，再补上 expectedRevision。
		page := user.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil)
		mustStatus(t, page, http.StatusOK, "用户页面(应用主题前)")
		settings, ok := page.data()["settings"].(map[string]any)
		if !ok {
			t.Fatalf("用户页面缺少 settings: %v", page.data())
		}
		appearance, ok := settings["appearance"].(map[string]any)
		if !ok {
			t.Fatalf("用户页面缺少 appearance: %v", settings)
		}
		appearance["themeId"] = importedID
		settings["expectedRevision"] = numberField(t, page.data(), "draftRevision", "应用主题前修订号")
		updated := user.call(t, http.MethodPut, fmt.Sprintf("/api/v1/pages/%s/settings", userPageID), settings)
		mustStatus(t, updated, http.StatusOK, "应用私有主题")

		refreshed := user.call(t, http.MethodGet, "/api/v1/pages/current?scope=personal", nil)
		publishRevision := numberField(t, refreshed.data(), "draftRevision", "应用主题后修订号")
		published := user.call(t, http.MethodPost, fmt.Sprintf("/api/v1/pages/%s/publish", userPageID),
			map[string]any{"expectedRevision": publishRevision}, withHeader("Idempotency-Key", "contract-theme-import-0001"))
		mustStatus(t, published, http.StatusOK, "发布私有主题页面")

		// 配额 = 2:第二个成功,第三个 409。
		second := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "second.zip", buildThemeZip(t, "second"))
		mustStatus(t, second, http.StatusCreated, "第二个私有主题")
		secondID := stringField(t, second.data(), "id", "第二主题 ID")
		third := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "third.zip", buildThemeZip(t, "third"))
		mustStatus(t, third, http.StatusConflict, "配额 409")

		// 同 slug 重复导入 = 升级,不占额度。
		again := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "lilac2.zip", buildThemeZip(t, "lilac"))
		mustStatus(t, again, http.StatusCreated, "重复导入即升级")

		// dry-run:坏包 200 + valid=false。
		invalid := user.uploadMultipart(t, "/api/v1/themes/validate", nil, "file", "bad.zip", []byte("not a zip"))
		mustStatus(t, invalid, http.StatusOK, "校验坏包")
		if valid, _ := invalid.data()["valid"].(bool); valid {
			t.Fatal("坏包 valid 应为 false")
		}

		// 卸载未被引用的第二主题 → 204;再删 → 404。
		removed := user.call(t, http.MethodDelete, "/api/v1/me/themes/"+secondID, nil)
		mustStatus(t, removed, http.StatusNoContent, "卸载私有主题")
		missing := user.call(t, http.MethodDelete, "/api/v1/me/themes/"+secondID, nil)
		mustStatus(t, missing, http.StatusNotFound, "重复卸载 404")

		// 坏包导入 → 422。
		bad := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "bad.zip", []byte("not a zip"))
		mustStatus(t, bad, http.StatusUnprocessableEntity, "坏包 422")
	})

	t.Run("主题目录审核", func(t *testing.T) {
		imported := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "catalogcand.zip", buildThemeZip(t, "catalogcand"))
		mustStatus(t, imported, http.StatusCreated, "导入待审核主题")
		themeID := stringField(t, imported.data(), "id", "待审核主题 ID")

		submitted := user.call(t, http.MethodPost, "/api/v1/me/themes/"+themeID+"/catalog-request", nil)
		mustStatus(t, submitted, http.StatusCreated, "提交目录审核")
		requestID := stringField(t, submitted.data(), "id", "审核申请 ID")
		if got := stringField(t, submitted.data(), "status", "审核申请状态"); got != "pending" {
			t.Fatalf("submitted status = %q", got)
		}

		// 审核期间禁止升级。
		locked := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "catalogcand2.zip", buildThemeZip(t, "catalogcand"))
		mustStatus(t, locked, http.StatusConflict, "审核期间升级被拒")

		// 重复提交同一主题 → 409。
		duplicate := user.call(t, http.MethodPost, "/api/v1/me/themes/"+themeID+"/catalog-request", nil)
		mustStatus(t, duplicate, http.StatusConflict, "重复提交 409")

		queue := admin.call(t, http.MethodGet, "/api/v1/admin/theme-catalog-requests?status=pending", nil)
		mustStatus(t, queue, http.StatusOK, "目录审核队列")
		if !strings.Contains(string(queue.body), requestID) {
			t.Fatalf("审核队列缺少申请 %s", requestID)
		}

		approved := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-catalog-requests/"+requestID,
			map[string]any{"decision": "approve"})
		mustStatus(t, approved, http.StatusOK, "批准目录申请")
		if got := stringField(t, approved.data(), "status", "批准后状态"); got != "approved" {
			t.Fatalf("approved status = %q", got)
		}

		publicList := guest.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, publicList, http.StatusOK, "公开主题列表")
		if !strings.Contains(string(publicList.body), themeID) {
			t.Fatalf("公开列表应含已批准主题 %s", themeID)
		}

		// 批准后 slug 释放给同 owner 的新私有导入；拿它验证 slug 冲突路径。
		afterApproval := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "catalogcand3.zip", buildThemeZip(t, "catalogcand"))
		mustStatus(t, afterApproval, http.StatusCreated, "批准后重新导入同 slug 建私有副本")
		conflictThemeID := stringField(t, afterApproval.data(), "id", "冲突主题 ID")
		conflict := user.call(t, http.MethodPost, "/api/v1/me/themes/"+conflictThemeID+"/catalog-request", nil)
		mustStatus(t, conflict, http.StatusUnprocessableEntity, "slug 冲突 422")

		// 卸载 conflictThemeID 腾出配额(测试环境配额=2,此刻 lilac + conflictThemeID
		// 已占满)。它从未拿到过 pending 目录申请(上面的提交在 slug 冲突检查处
		// 就 422 提前返回了),卸载走的是普通的无引用物理删除分支。
		conflictRemoved := user.call(t, http.MethodDelete, "/api/v1/me/themes/"+conflictThemeID, nil)
		mustStatus(t, conflictRemoved, http.StatusNoContent, "卸载 slug 冲突的私有副本腾出配额")

		// 撤回 + 卸载回合：一个独立 slug 的新主题,走「提交 → 撤回(DELETE
		// catalog-request,契约里此前从未被驱动过的 204 路径)→ 卸载」。撤回
		// 把请求转成 revoked 但保留该行(theme_catalog_requests 只在批准时被
		// 主题的 CASCADE 顺带清掉,撤回/拒绝的记录会一直留着),卸载走物理
		// 删除分支(deleteThemeTx 先删 theme_versions 再删 themes)——这正是
		// migrations/0016 补上 version_id ON DELETE CASCADE 之前会以
		// FOREIGN KEY constraint failed 500 的路径,端到端钉住这个修复。
		revocable := user.uploadMultipart(t, "/api/v1/me/themes/import", nil, "file", "catalogcand4.zip", buildThemeZip(t, "catalogcand4"))
		mustStatus(t, revocable, http.StatusCreated, "导入用于撤回+卸载回归的主题")
		revocableThemeID := stringField(t, revocable.data(), "id", "撤回+卸载主题 ID")

		revocableSubmitted := user.call(t, http.MethodPost, "/api/v1/me/themes/"+revocableThemeID+"/catalog-request", nil)
		mustStatus(t, revocableSubmitted, http.StatusCreated, "提交撤回+卸载回归申请")

		cancelled := user.call(t, http.MethodDelete, "/api/v1/me/themes/"+revocableThemeID+"/catalog-request", nil)
		mustStatus(t, cancelled, http.StatusNoContent, "撤回目录申请")

		uninstalledAfterRevoke := user.call(t, http.MethodDelete, "/api/v1/me/themes/"+revocableThemeID, nil)
		mustStatus(t, uninstalledAfterRevoke, http.StatusNoContent, "卸载曾有撤回申请的主题")
	})

	t.Run("公开目录与发现", func(t *testing.T) {
		directory := guest.call(t, http.MethodGet, "/api/v1/public/directory", nil)
		mustStatus(t, directory, http.StatusOK, "公开目录")
		discover := guest.call(t, http.MethodGet, "/api/v1/public/discover", nil)
		mustStatus(t, discover, http.StatusOK, "公开发现")
	})

	t.Run("管理端读取", func(t *testing.T) {
		for _, path := range []string{
			"/api/v1/admin/overview",
			"/api/v1/admin/users?page=1&pageSize=20",
			"/api/v1/admin/invitations?page=1&pageSize=20",
			"/api/v1/admin/settings",
			"/api/v1/admin/audit",
			"/api/v1/admin/themes",
			"/api/v1/admin/subdomains",
			"/api/v1/admin/links",
			"/api/v1/admin/directory/categories",
			"/api/v1/admin/directory/sites",
		} {
			result := admin.call(t, http.MethodGet, path, nil)
			mustStatus(t, result, http.StatusOK, "管理端 "+path)
		}
	})

	t.Run("主题版本视图与版本级 kill switch", func(t *testing.T) {
		// slate(默认)版本列表:一条、当前、active。
		list := admin.call(t, http.MethodGet, "/api/v1/admin/themes/slate/versions", nil)
		mustStatus(t, list, http.StatusOK, "slate 版本列表")

		// sakura 版本列表 → 取 versionId。
		sakuraList := admin.call(t, http.MethodGet, "/api/v1/admin/themes/sakura/versions", nil)
		mustStatus(t, sakuraList, http.StatusOK, "sakura 版本列表")
		items, _ := sakuraList.json["data"].([]any)
		if len(items) == 0 {
			t.Fatal("sakura 应至少一个版本")
		}
		first, _ := items[0].(map[string]any)
		versionID, _ := first["versionId"].(string)
		if versionID == "" {
			t.Fatal("缺 versionId")
		}

		// 停用 sakura 版本 → 200 disabled。
		disabled := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/"+versionID,
			map[string]any{"status": "disabled"})
		mustStatus(t, disabled, http.StatusOK, "停用 sakura 版本")
		if got := stringField(t, disabled.data(), "status", "版本状态"); got != "disabled" {
			t.Fatalf("status = %q", got)
		}

		// 停用 slate(默认)当前版本 → 409。
		slateItems, _ := list.json["data"].([]any)
		slateFirst, _ := slateItems[0].(map[string]any)
		slateVersion, _ := slateFirst["versionId"].(string)
		rejected := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/"+slateVersion,
			map[string]any{"status": "disabled"})
		mustStatus(t, rejected, http.StatusConflict, "停用默认版本被拒")

		// 恢复 sakura。
		restored := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/"+versionID,
			map[string]any{"status": "active"})
		mustStatus(t, restored, http.StatusOK, "恢复 sakura 版本")

		// 非法 status 值 → 422。
		invalidStatus := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/"+versionID,
			map[string]any{"status": "bogus"}, withoutRequestValidation())
		mustStatus(t, invalidStatus, http.StatusUnprocessableEntity, "非法版本状态 422")

		// 未知版本 → 404。
		missing := admin.call(t, http.MethodPatch, "/api/v1/admin/theme-versions/v00000000000000000000000000000000",
			map[string]any{"status": "disabled"})
		mustStatus(t, missing, http.StatusNotFound, "未知版本 404")
	})

	t.Run("版本级 kill switch", func(t *testing.T) {
		// 先确认 sakura 在 user 的 eligible 列表里(非默认、active)。
		before := user.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, before, http.StatusOK, "停用前主题列表")
		if !strings.Contains(string(before.body), "\"sakura\"") {
			t.Fatal("停用前列表应含 sakura")
		}

		// 管理员停用 sakura 当前版本。
		disabled := admin.call(t, http.MethodPatch, "/api/v1/admin/themes/sakura",
			map[string]any{"status": "disabled"})
		mustStatus(t, disabled, http.StatusOK, "停用 sakura 版本")
		if got := stringField(t, disabled.data(), "status", "sakura 状态"); got != "disabled" {
			t.Fatalf("status = %q, want disabled", got)
		}

		// 停用后 sakura 从 user 的 eligible 列表消失(410 分支由 httpapi 单测覆盖,这里断言列表回落)。
		after := user.call(t, http.MethodGet, "/api/v1/themes", nil)
		mustStatus(t, after, http.StatusOK, "停用后主题列表")
		if strings.Contains(string(after.body), "\"sakura\"") {
			t.Fatal("停用后列表不应再含 sakura")
		}

		// 停用默认主题 slate 的当前版本 → 409。
		rejected := admin.call(t, http.MethodPatch, "/api/v1/admin/themes/slate",
			map[string]any{"status": "disabled"})
		mustStatus(t, rejected, http.StatusConflict, "停用默认主题版本被拒")

		// 恢复 sakura,避免污染后续断言。
		restored := admin.call(t, http.MethodPatch, "/api/v1/admin/themes/sakura",
			map[string]any{"status": "active"})
		mustStatus(t, restored, http.StatusOK, "恢复 sakura")
	})

	t.Run("越权访问管理端", func(t *testing.T) {
		denied := user.call(t, http.MethodGet, "/api/v1/admin/overview", nil)
		mustStatus(t, denied, http.StatusForbidden, "普通用户访问管理端")
	})

	// Placed last: a completed reset revokes the member's sessions.
	t.Run("密码找回", func(t *testing.T) {
		// Self-service forgot always returns a generic success (no enumeration).
		forgot := guest.call(t, http.MethodPost, "/api/v1/auth/password/forgot",
			map[string]any{"email": userEmail})
		mustStatus(t, forgot, http.StatusOK, "忘记密码")
		unknown := guest.call(t, http.MethodPost, "/api/v1/auth/password/forgot",
			map[string]any{"email": "nobody@example.com"})
		mustStatus(t, unknown, http.StatusOK, "忘记密码-未知邮箱")

		// Admin generates a reset link, which works without SMTP configured.
		link := admin.call(t, http.MethodPost, "/api/v1/admin/users/"+userID+"/password-reset", nil)
		mustStatus(t, link, http.StatusOK, "生成重置链接")
		resetURL := stringField(t, link.data(), "resetUrl", "重置链接")
		parsed, err := url.Parse(resetURL)
		if err != nil {
			t.Fatalf("解析重置链接: %v", err)
		}
		token := parsed.Query().Get("token")
		if token == "" {
			t.Fatal("重置链接缺少 token")
		}

		reset := guest.call(t, http.MethodPost, "/api/v1/auth/password/reset",
			map[string]any{"token": token, "password": "Recovered-Pass-2026!"})
		mustStatus(t, reset, http.StatusOK, "重置密码")

		// The token is single-use.
		reused := guest.call(t, http.MethodPost, "/api/v1/auth/password/reset",
			map[string]any{"token": token, "password": "Another-Pass-2026!"})
		mustStatus(t, reused, http.StatusBadRequest, "重复使用重置链接")

		// The new password authenticates.
		relogin := guest.call(t, http.MethodPost, "/api/v1/auth/login",
			map[string]any{"email": userEmail, "password": "Recovered-Pass-2026!"})
		mustStatus(t, relogin, http.StatusOK, "新密码登录")
	})
}

// tinyPNG 生成一张合法的 64x64 PNG，供上传接口校验图片内容（背景图要求宽高至少 64px）。
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	img.Set(0, 0, color.RGBA{R: 74, G: 107, B: 82, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("编码 PNG: %v", err)
	}
	return buffer.Bytes()
}

// contentOrderFromPage 用页面数据构造覆盖全部分类与站点的排序请求体。
func contentOrderFromPage(t *testing.T, page map[string]any) []map[string]any {
	t.Helper()
	categories, _ := page["categories"].([]any)
	sites, _ := page["sites"].([]any)
	order := make([]map[string]any, 0, len(categories))
	for _, rawCategory := range categories {
		category, _ := rawCategory.(map[string]any)
		categoryID := stringField(t, category, "id", "排序分类")
		siteIDs := []string{}
		for _, rawSite := range sites {
			site, _ := rawSite.(map[string]any)
			if site["categoryId"] == categoryID {
				siteIDs = append(siteIDs, stringField(t, site, "id", "排序站点"))
			}
		}
		order = append(order, map[string]any{"id": categoryID, "siteIds": siteIDs})
	}
	return order
}

// buildThemeZip 现场构造一个最小合法主题包：manifest 字面量与
// internal/themeimport/service_test.go 的 sampleManifest 同款（颜色四组 +
// 字体四族 + 三个 swatch + tier 1），仅 id 由参数 slug 决定，css 一行合法
// 规则，两者不一致时以 service_test.go 那份为准。
func buildThemeZip(t *testing.T, slug string) []byte {
	t.Helper()
	manifest := fmt.Sprintf(`{
  "specVersion": 1, "id": %q, "name": "Contract Theme", "version": "1.0.0",
  "author": "e2e", "license": "MIT", "mode": "light", "vibe": "serious",
  "swatches": ["#f5f3ff", "#8b5cf6", "#1e1b4b"], "tier": 1,
  "tokens": {
    "font": { "heading": "system-ui", "body": "system-ui", "label": "system-ui", "mono": "monospace" },
    "color": {
      "background": { "50": "0.985 0.010 300" },
      "foreground": { "900": "0.210 0.040 300" },
      "primary":    { "500": "0.585 0.200 300" },
      "accent":     { "500": "0.700 0.150 160" }
    }
  }
}`, slug)

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entries := map[string][]byte{
		"theme.json": []byte(manifest),
		"theme.css":  []byte(`[data-nx="site-card"] { border-radius: var(--radius-md); }`),
	}
	for name, data := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("创建 zip 条目 %s: %v", name, err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatalf("写入 zip 条目 %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("关闭 zip writer: %v", err)
	}
	return buffer.Bytes()
}
