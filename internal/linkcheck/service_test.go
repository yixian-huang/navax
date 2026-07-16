package linkcheck

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	navaxdb "github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/navigation"
)

const (
	linkUserID = "user_linkcheck_owner"
	linkPageID = "page_linkcheck_personal"
	linkCatID  = "category_linkcheck_default"
)

var linkTestNow = time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)

type fakeResolver struct {
	mu        sync.Mutex
	addresses map[string][]netip.Addr
	calls     map[string]int
}

func (r *fakeResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.calls == nil {
		r.calls = make(map[string]int)
	}
	r.calls[host]++
	addresses, ok := r.addresses[host]
	if !ok {
		return nil, errors.New("host not found")
	}
	return append([]netip.Addr(nil), addresses...), nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestCheckRunsConcurrentlyInInputOrderAndPersists(t *testing.T) {
	db := openLinkDB(t)
	siteIDs := seedLinkPage(t, db, 9)
	resolver := publicResolver("public.test")
	var active atomic.Int32
	var maximum atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		current := active.Add(1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		defer active.Add(-1)
		time.Sleep(15 * time.Millisecond)
		return response(request, http.StatusNoContent, nil), nil
	})
	service := NewServiceWithOptions(db, Options{
		Resolver: resolver, Transport: transport, Concurrency: 3, MaxActiveBatches: 1,
		RequestTimeout: time.Second, BatchTimeout: 3 * time.Second, Now: func() time.Time { return linkTestNow },
	})
	results, err := service.Check(context.Background(), linkActor(), linkPageID, siteIDs)
	if err != nil {
		t.Fatal(err)
	}
	if maximum.Load() < 2 || maximum.Load() > 3 {
		t.Fatalf("maximum concurrent requests = %d, want 2..3", maximum.Load())
	}
	for index, result := range results {
		if result.SiteID != siteIDs[index] || result.Status != StatusReachable || result.HTTPStatus == nil || *result.HTTPStatus != http.StatusNoContent {
			t.Fatalf("result[%d] = %+v", index, result)
		}
		if result.CheckedAt != linkTestNow {
			t.Fatalf("checkedAt = %s", result.CheckedAt)
		}
	}
	assertLinkInt(t, db, len(siteIDs), "SELECT COUNT(*) FROM link_check_results")
	assertLinkInt(t, db, len(siteIDs), "SELECT COUNT(*) FROM link_check_results WHERE status = 'reachable' AND http_status = 204")
}

func TestHeadFallsBackToBoundedGet(t *testing.T) {
	db := openLinkDB(t)
	siteIDs := seedLinkPage(t, db, 1)
	var methods []string
	var mu sync.Mutex
	body := &countingReader{reader: bytes.NewReader(bytes.Repeat([]byte("x"), 1024))}
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		mu.Lock()
		methods = append(methods, request.Method)
		mu.Unlock()
		if request.Method == http.MethodHead {
			return response(request, http.StatusMethodNotAllowed, nil), nil
		}
		if request.Header.Get("Range") != "bytes=0-15" {
			t.Errorf("Range = %q", request.Header.Get("Range"))
		}
		return response(request, http.StatusOK, io.NopCloser(body)), nil
	})
	service := NewServiceWithOptions(db, Options{
		Resolver: publicResolver("public.test"), Transport: transport, Concurrency: 1,
		RequestTimeout: time.Second, BatchTimeout: time.Second, MaxResponseBytes: 16,
	})
	results, err := service.Check(context.Background(), linkActor(), linkPageID, siteIDs)
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || methods[0] != http.MethodHead || methods[1] != http.MethodGet {
		t.Fatalf("methods = %v", methods)
	}
	if body.read > 16 {
		t.Fatalf("response body bytes read = %d, want <= 16", body.read)
	}
	if results[0].Status != StatusReachable || results[0].HTTPStatus == nil || *results[0].HTTPStatus != http.StatusOK {
		t.Fatalf("unexpected result: %+v", results[0])
	}
}

func TestRedirectToPrivateAddressIsBlockedBeforeSecondRequest(t *testing.T) {
	db := openLinkDB(t)
	siteIDs := seedLinkPage(t, db, 1)
	resolver := &fakeResolver{addresses: map[string][]netip.Addr{
		"public.test":  {netip.MustParseAddr("8.8.8.8")},
		"private.test": {netip.MustParseAddr("10.0.0.2")},
	}}
	var calls atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		location, _ := url.Parse("http://private.test/metadata")
		response := response(request, http.StatusFound, nil)
		response.Header.Set("Location", location.String())
		return response, nil
	})
	service := NewServiceWithOptions(db, Options{Resolver: resolver, Transport: transport, RequestTimeout: time.Second, BatchTimeout: time.Second})
	results, err := service.Check(context.Background(), linkActor(), linkPageID, siteIDs)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("transport calls = %d, private redirect should not be requested", calls.Load())
	}
	if results[0].Status != StatusBlocked || results[0].HTTPStatus != nil || results[0].LatencyMS != nil {
		t.Fatalf("unexpected blocked result: %+v", results[0])
	}
	assertLinkInt(t, db, 1, "SELECT COUNT(*) FROM link_check_results WHERE status = 'blocked' AND http_status IS NULL")
}

func TestRequestTimeoutIsRecorded(t *testing.T) {
	db := openLinkDB(t)
	siteIDs := seedLinkPage(t, db, 1)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	service := NewServiceWithOptions(db, Options{
		Resolver: publicResolver("public.test"), Transport: transport,
		RequestTimeout: 20 * time.Millisecond, BatchTimeout: time.Second,
	})
	results, err := service.Check(context.Background(), linkActor(), linkPageID, siteIDs)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusTimeout || results[0].LatencyMS == nil {
		t.Fatalf("unexpected timeout result: %+v", results[0])
	}
	assertLinkInt(t, db, 1, "SELECT COUNT(*) FROM link_check_results WHERE status = 'timeout'")
}

func TestBatchLimitReturnsBusy(t *testing.T) {
	db := openLinkDB(t)
	siteIDs := seedLinkPage(t, db, 1)
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		once.Do(func() { close(started) })
		select {
		case <-release:
			return response(request, http.StatusOK, nil), nil
		case <-request.Context().Done():
			return nil, request.Context().Err()
		}
	})
	service := NewServiceWithOptions(db, Options{
		Resolver: publicResolver("public.test"), Transport: transport, MaxActiveBatches: 1,
		RequestTimeout: time.Second, BatchTimeout: 2 * time.Second,
	})
	done := make(chan error, 1)
	go func() {
		_, err := service.Check(context.Background(), linkActor(), linkPageID, siteIDs)
		done <- err
	}()
	<-started
	if _, err := service.Check(context.Background(), linkActor(), linkPageID, siteIDs); !errors.Is(err, ErrBusy) {
		t.Fatalf("second batch error = %v, want busy", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPageAuthorizationAndSiteOwnership(t *testing.T) {
	db := openLinkDB(t)
	siteIDs := seedLinkPage(t, db, 1)
	service := NewServiceWithOptions(db, Options{Resolver: publicResolver("public.test"), Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusOK, nil), nil
	})})
	other := navigation.Actor{UserID: "another_user_id", Role: "user"}
	if _, err := service.Check(context.Background(), other, linkPageID, siteIDs); !errors.Is(err, navigation.ErrForbidden) {
		t.Fatalf("authorization error = %v, want forbidden", err)
	}
	if _, err := service.Check(context.Background(), linkActor(), linkPageID, []string{"site_from_other_page"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("site ownership error = %v, want invalid", err)
	}
}

func TestURLValidatorBlocksPrivateReservedAndMetadataTargets(t *testing.T) {
	resolver := &fakeResolver{addresses: map[string][]netip.Addr{
		"private.test": {netip.MustParseAddr("10.0.0.1")},
		"mixed.test":   {netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("fd00::1")},
		"public.test":  {netip.MustParseAddr("8.8.8.8")},
	}}
	validator := urlValidator{resolver: resolver}
	blockedTargets := []string{
		"http://127.0.0.1/", "http://[::1]/", "http://169.254.169.254/latest/meta-data/",
		"http://100.100.100.200/", "http://private.test/", "http://mixed.test/",
		"http://metadata.google.internal/", "http://localhost/", "ftp://public.test/",
		"http://user:password@public.test/", "http://192.0.2.10/",
	}
	for _, raw := range blockedTargets {
		target, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := validator.validate(context.Background(), target); !errors.Is(err, ErrBlocked) {
			t.Errorf("validate(%q) error = %v, want blocked", raw, err)
		}
	}
	publicTarget, _ := url.Parse("https://public.test/path")
	if addresses, err := validator.validate(context.Background(), publicTarget); err != nil || len(addresses) != 1 {
		t.Fatalf("public target validation = %v, %v", addresses, err)
	}
}

type countingReader struct {
	reader io.Reader
	read   int
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	count, err := r.reader.Read(buffer)
	r.read += count
	return count, err
}

func response(request *http.Request, status int, body io.ReadCloser) *http.Response {
	if body == nil {
		body = http.NoBody
	}
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: body, Request: request}
}

func publicResolver(hosts ...string) *fakeResolver {
	addresses := make(map[string][]netip.Addr, len(hosts))
	for _, host := range hosts {
		addresses[host] = []netip.Addr{netip.MustParseAddr("8.8.8.8")}
	}
	return &fakeResolver{addresses: addresses}
}

func openLinkDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := navaxdb.OpenAndMigrate(context.Background(), navaxdb.Config{Path: t.TempDir() + "/linkcheck.sqlite"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedLinkPage(t *testing.T, db *sql.DB, siteCount int) []string {
	t.Helper()
	now := linkTestNow.Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO users(id, username, email, password_hash, role, created_at, updated_at)
		VALUES (?, 'linkowner', 'linkowner@example.test', 'not-used', 'user', ?, ?)`, linkUserID, now, now); err != nil {
		t.Fatal(err)
	}
	var settings string
	if err := db.QueryRow("SELECT settings_json FROM navigation_pages WHERE id = 'page_system_root'").Scan(&settings); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO navigation_pages(id, kind, owner_id, title, description, settings_json, draft_updated_at, created_at, updated_at)
		VALUES (?, 'personal', ?, '链接检查', '', ?, ?, ?, ?)`, linkPageID, linkUserID, settings, now, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO page_publications(page_id, visibility, slug, show_author, updated_at)
		VALUES (?, 'private', 'link-check-test', 1, ?)`, linkPageID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO categories(id, page_id, name, icon, sort_order, is_uncategorized, created_at, updated_at)
		VALUES (?, ?, '未分类', '', 0, 1, ?, ?)`, linkCatID, linkPageID, now, now); err != nil {
		t.Fatal(err)
	}
	siteIDs := make([]string, siteCount)
	for index := range siteCount {
		siteID := "site_linkcheck_" + strconv.Itoa(index+1000)
		siteIDs[index] = siteID
		if _, err := db.Exec(`
			INSERT INTO sites(id, page_id, category_id, title, url, icon, description, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, '', '', ?, ?, ?)`,
			siteID, linkPageID, linkCatID, "站点 "+strconv.Itoa(index), "https://public.test/"+strconv.Itoa(index), index, now, now,
		); err != nil {
			t.Fatal(err)
		}
	}
	return siteIDs
}

func linkActor() navigation.Actor {
	return navigation.Actor{UserID: linkUserID, Username: "linkowner", Role: "user"}
}

func assertLinkInt(t *testing.T, db *sql.DB, want int, query string, args ...any) {
	t.Helper()
	var got int
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("query %q = %d, want %d", query, got, want)
	}
}
