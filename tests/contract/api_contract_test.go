package contract

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
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
		adminPageID  string
		systemPageID string
		userPageID   string
		userSlug     string
		inviteToken  string
		categoryID   string
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

		missing := guest.call(t, http.MethodGet, "/api/v1/public/pages/does-not-exist", nil)
		mustStatus(t, missing, http.StatusNotFound, "读取不存在的公开页面")
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

	t.Run("越权访问管理端", func(t *testing.T) {
		denied := user.call(t, http.MethodGet, "/api/v1/admin/overview", nil)
		mustStatus(t, denied, http.StatusForbidden, "普通用户访问管理端")
	})
}

// tinyPNG 生成一张合法的 2x2 PNG，供上传接口校验图片内容。
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
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
