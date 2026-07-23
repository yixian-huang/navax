package analytics

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/navigation"
	"github.com/yixian-huang/navax/internal/themes"
)

func TestBrowserType(t *testing.T) {
	t.Parallel()
	cases := []struct{ ua, want string }{
		{"", ""},
		{"Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Chrome/127.0 Safari/537.36", "Chrome"},
		{"Mozilla/5.0 (Windows NT 10.0) Chrome/127.0 Safari/537.36 Edg/127.0.0", "Edge"},
		{"Mozilla/5.0 (X11; Linux) Chrome/126 Safari/537.36 OPR/112.0", "Opera"},
		{"Mozilla/5.0 (Macintosh) Gecko/20100101 Firefox/128.0", "Firefox"},
		{"Mozilla/5.0 (Macintosh) Version/17.0 Safari/605.1.15", "Safari"},
		{"curl/8.4.0", "Other"},
	}
	for _, tc := range cases {
		if got := browserType(tc.ua); got != tc.want {
			t.Errorf("browserType(%q) = %q, want %q", tc.ua, got, tc.want)
		}
	}
}

func TestCountryCode(t *testing.T) {
	t.Parallel()
	if got := countryCode("8.8.8.8"); len(got) != 2 {
		t.Errorf("public IP country = %q, want a 2-letter code", got)
	}
	for _, addr := range []string{"", "not-an-ip", "   "} {
		if got := countryCode(addr); got != "" {
			t.Errorf("countryCode(%q) = %q, want empty", addr, got)
		}
	}
}

func TestRecordAndReadPrivacyPreservingAnalytics(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	authService := auth.NewService(auth.NewSQLStore(db), "01234567890123456789012345678901", time.Hour)
	session, _, err := authService.Bootstrap(ctx, "01234567890123456789012345678901", auth.BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "strong password",
		InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}
	actor := navigation.Actor{UserID: session.User.ID, Username: session.User.Username, Role: session.User.Role}
	// 这条链路要发布页面，因此需要主题版本解析；没有它发布会明确报错。
	themeStore := themes.NewStore(db)
	if err := themes.SyncBuiltin(context.Background(), themeStore, time.Now().UTC()); err != nil {
		t.Fatalf("SyncBuiltin() error = %v", err)
	}
	navigationStore := navigation.NewSQLStore(db)
	navigationStore.SetThemeVersionResolver(func(ctx context.Context, tx *sql.Tx, themeID, actorID string) (string, error) {
		return themes.ResolveEligibleVersion(ctx, tx, themeID, actorID)
	})
	navigationService := navigation.NewService(navigationStore)
	page, err := navigationService.CurrentPage(ctx, actor, navigation.PageKindPersonal)
	if err != nil {
		t.Fatal(err)
	}
	category, err := navigationService.CreateCategory(ctx, actor, page.ID, navigation.CategoryInput{Name: "开发"})
	if err != nil {
		t.Fatal(err)
	}
	site, err := navigationService.CreateSite(ctx, actor, page.ID, navigation.SiteInput{
		CategoryID: category.ID, Title: "Go", URL: "https://go.dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err = navigationService.PageDraft(ctx, actor, page.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := navigationService.Publish(ctx, actor, page.ID, page.DraftRevision, "https://nav.ax"); err != nil {
		t.Fatal(err)
	}
	published, err := navigationService.PublicBySlug(ctx, session.User.Username)
	if err != nil {
		t.Fatal(err)
	}

	service, err := NewService(db, bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	common := Event{
		PageID: page.ID, SnapshotID: published.SnapshotID,
		ClientAddress: "203.0.113.42", UserAgent: "Mozilla/5.0 (iPhone; Mobile)",
		Referrer: "https://search.example/results?q=nav",
	}
	view := common
	view.Type = "page_view"
	view.ClientEventID = "event-page-view-0001"
	if err := service.Record(ctx, view); err != nil {
		t.Fatal(err)
	}
	if err := service.Record(ctx, view); err != nil {
		t.Fatalf("idempotent record: %v", err)
	}
	click := common
	click.Type = "site_click"
	click.SiteID = site.ID
	click.ClientEventID = "event-site-click-0001"
	if err := service.Record(ctx, click); err != nil {
		t.Fatal(err)
	}

	var count int
	var visitorHash []byte
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*), visitor_hash FROM analytics_events").Scan(&count, &visitorHash); err != nil {
		t.Fatal(err)
	}
	if count != 2 || bytes.Contains(visitorHash, []byte(common.ClientAddress)) {
		t.Fatalf("events=%d visitor hash persisted raw address=%v", count, visitorHash)
	}
	var rawAddressColumns int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('analytics_events') WHERE name IN ('ip', 'address', 'user_agent')`).Scan(&rawAddressColumns); err != nil {
		t.Fatal(err)
	}
	if rawAddressColumns != 0 {
		t.Fatalf("analytics schema contains raw identifying columns: %d", rawAddressColumns)
	}

	overview, err := service.Overview(ctx, session.User.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if overview.TotalPV != 1 || overview.TotalUV != 1 || overview.TodayPV != 1 || overview.TodayUV != 1 {
		t.Fatalf("overview = %+v", overview)
	}
	trends, err := service.Trends(ctx, session.User.ID, 7)
	if err != nil || len(trends) != 7 || trends[6].PV != 1 || trends[6].UV != 1 {
		t.Fatalf("trends = %+v, err = %v", trends, err)
	}
	breakdown, err := service.Breakdown(ctx, session.User.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(breakdown.TopSites) != 1 || breakdown.TopSites[0].Key != site.ID || len(breakdown.RecentVisits) != 1 {
		t.Fatalf("breakdown = %+v", breakdown)
	}
	if breakdown.TopSites[0].CategoryName != "开发" {
		t.Fatalf("top site category = %q, want 开发", breakdown.TopSites[0].CategoryName)
	}
	// UA 无浏览器标记应归类为 Other；文档用途 IP 解析不出国家。
	if visit := breakdown.RecentVisits[0]; visit.Browser == "" || visit.Country != "" {
		t.Fatalf("recent visit browser=%q country=%q", visit.Browser, visit.Country)
	}
	service.now = func() time.Time { return time.Now().UTC().AddDate(0, 0, 91) }
	if err := service.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM analytics_events").Scan(&count); err != nil || count != 0 {
		t.Fatalf("expired analytics count = %d, %v", count, err)
	}
}
