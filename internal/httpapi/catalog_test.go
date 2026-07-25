package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/catalog"
	"github.com/yixian-huang/navax/internal/database"
)

// TestCatalogThemesCacheHeaders 确认 /themes 只在带会话（响应含调用者私有主题）
// 时才禁止共享缓存；匿名调用者的目录主题响应仍可被缓存。Vary: Cookie 始终
// 设置——同一 URL 会因 Cookie 不同而返回不同内容，中间代理必须按此区分变体，
// 否则 A 用户的私有主题列表可能被回放给 B 用户。
func TestCatalogThemesCacheHeaders(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	authService := auth.NewService(auth.NewSQLStore(db), "01234567890123456789012345678901", 24*time.Hour)
	_, token, err := authService.Bootstrap(ctx, "01234567890123456789012345678901", auth.BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "initial-password",
		InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}

	router := chi.NewRouter()
	router.Route("/api/v1", func(api chi.Router) {
		NewCatalogHandler(catalog.NewService(db), authService).Mount(api)
	})

	anon := performRequest(router, http.MethodGet, "/api/v1/themes", nil, "")
	if anon.Code != http.StatusOK {
		t.Fatalf("匿名 /themes 状态 = %d: %s", anon.Code, anon.Body.String())
	}
	if got := anon.Header().Get("Vary"); got != "Cookie" {
		t.Fatalf("匿名 Vary = %q, want Cookie", got)
	}
	if got := anon.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("匿名 Cache-Control = %q, want 空(可被共享缓存)", got)
	}

	withSession := performRequest(router, http.MethodGet, "/api/v1/themes", nil, token)
	if withSession.Code != http.StatusOK {
		t.Fatalf("带会话 /themes 状态 = %d: %s", withSession.Code, withSession.Body.String())
	}
	if got := withSession.Header().Get("Vary"); got != "Cookie" {
		t.Fatalf("带会话 Vary = %q, want Cookie", got)
	}
	if got := withSession.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("带会话 Cache-Control = %q, want private, no-store", got)
	}
}
