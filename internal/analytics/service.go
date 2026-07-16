package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

var (
	ErrDisabled = errors.New("analytics is disabled")
	ErrNotFound = errors.New("published page not found")
	ErrInvalid  = errors.New("invalid analytics event")
)

type Event struct {
	Type          string
	PageID        string
	SnapshotID    string
	SiteID        string
	ClientEventID string
	ClientAddress string
	UserAgent     string
	Referrer      string
}

type Overview struct {
	TotalPV         int64   `json:"totalPV"`
	TotalUV         int64   `json:"totalUV"`
	TodayPV         int64   `json:"todayPV"`
	TodayUV         int64   `json:"todayUV"`
	PVChange        float64 `json:"pvChange"`
	UVChange        float64 `json:"uvChange"`
	AveragePages    float64 `json:"averagePages"`
	AvgSessionPages float64 `json:"avgSessionPages"`
	BounceRate      float64 `json:"bounceRate"`
	TodayVisitors   int64   `json:"todayVisitors"`
	VisitorsChange  float64 `json:"visitorsChange"`
}

type DailyStat struct {
	Date string `json:"date"`
	PV   int64  `json:"pv"`
	UV   int64  `json:"uv"`
}

type Bucket struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value int64  `json:"value"`
}

type RecentVisit struct {
	AnonymousID    string    `json:"anonymousId"`
	Device         string    `json:"device"`
	ReferrerDomain string    `json:"referrerDomain"`
	VisitedAt      time.Time `json:"visitedAt"`
}

type Breakdown struct {
	TopSites     []Bucket      `json:"topSites"`
	Categories   []Bucket      `json:"categories"`
	Devices      []Bucket      `json:"devices"`
	Referrers    []Bucket      `json:"referrers"`
	RecentVisits []RecentVisit `json:"recentVisits"`
}

type Service struct {
	db  *sql.DB
	key []byte
	now func() time.Time
}

func NewService(db *sql.DB, privacyKey []byte) (*Service, error) {
	if db == nil || len(privacyKey) < 16 {
		return nil, errors.New("analytics database and privacy key are required")
	}
	return &Service{db: db, key: append([]byte(nil), privacyKey...), now: time.Now}, nil
}

func (s *Service) Record(ctx context.Context, event Event) error {
	if event.Type != "page_view" && event.Type != "site_click" || event.PageID == "" {
		return ErrInvalid
	}
	var currentSnapshot string
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT pp.current_snapshot_id, ss.analytics_enabled
		FROM page_publications pp CROSS JOIN system_settings ss
		JOIN navigation_pages p ON p.id = pp.page_id
		LEFT JOIN users u ON u.id = p.owner_id
		WHERE pp.page_id = ? AND pp.current_snapshot_id IS NOT NULL AND pp.visibility != 'private'
		  AND (p.kind = 'system' OR u.status = 'active')`, event.PageID,
	).Scan(&currentSnapshot, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !enabled {
		return ErrDisabled
	}
	if event.SnapshotID != "" && event.SnapshotID != currentSnapshot {
		return ErrNotFound
	}
	var persistedSiteID any
	if event.Type == "site_click" {
		if event.SiteID == "" {
			return ErrInvalid
		}
		var payload []byte
		if err := s.db.QueryRowContext(ctx, "SELECT payload_json FROM published_snapshots WHERE id = ?", currentSnapshot).Scan(&payload); err != nil {
			return err
		}
		var snapshot struct {
			Categories []struct {
				Sites []struct {
					ID string `json:"id"`
				} `json:"sites"`
			} `json:"categories"`
		}
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return err
		}
		found := false
		for _, category := range snapshot.Categories {
			for _, site := range category.Sites {
				if site.ID == event.SiteID {
					found = true
					break
				}
			}
		}
		if !found {
			return ErrInvalid
		}
		// A draft can delete a site while the previous snapshot is still public. In
		// that case we keep the click event but omit the foreign-keyed live site ID.
		var live bool
		if err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sites WHERE id = ? AND page_id = ?)", event.SiteID, event.PageID).Scan(&live); err != nil {
			return err
		}
		if live {
			persistedSiteID = event.SiteID
		}
	}
	now := s.now().UTC()
	date := now.Format("2006-01-02")
	visitorHash := s.visitorHash(date, event.ClientAddress, event.UserAgent)
	referrerDomain := cleanReferrer(event.Referrer)
	device := deviceType(event.UserAgent)
	var clientEventID any
	if event.ClientEventID != "" {
		clientEventID = event.ClientEventID
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO analytics_events(page_id, snapshot_id, site_id, event_type, client_event_id, visitor_hash,
		                            device, referrer_domain, occurred_date, occurred_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.PageID, currentSnapshot, persistedSiteID, event.Type, clientEventID, visitorHash,
		device, referrerDomain, date, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		if event.ClientEventID != "" && strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
			return nil
		}
		return err
	}
	if event.Type == "page_view" {
		_, err = s.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO analytics_daily_visitors(page_id, occurred_date, visitor_hash) VALUES (?, ?, ?)`,
			event.PageID, date, visitorHash,
		)
	}
	return err
}

func (s *Service) Overview(ctx context.Context, userID string, period int) (Overview, error) {
	pageID, err := s.personalPageID(ctx, userID)
	if err != nil {
		return Overview{}, err
	}
	period = normalizePeriod(period)
	now := s.now().UTC()
	start := now.AddDate(0, 0, -(period - 1)).Format("2006-01-02")
	previousStart := now.AddDate(0, 0, -(period*2 - 1)).Format("2006-01-02")
	previousEnd := now.AddDate(0, 0, -period).Format("2006-01-02")
	today := now.Format("2006-01-02")
	var result Overview
	var previousPV int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE event_type = 'page_view' AND occurred_date >= ?),
		  COUNT(*) FILTER (WHERE event_type = 'page_view' AND occurred_date = ?),
		  COUNT(*) FILTER (WHERE event_type = 'page_view' AND occurred_date BETWEEN ? AND ?)
		FROM analytics_events WHERE page_id = ?`, start, today, previousStart, previousEnd, pageID,
	).Scan(&result.TotalPV, &result.TodayPV, &previousPV); err != nil {
		return Overview{}, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FILTER (WHERE occurred_date >= ?), COUNT(*) FILTER (WHERE occurred_date = ?)
		FROM analytics_daily_visitors WHERE page_id = ?`, start, today, pageID,
	).Scan(&result.TotalUV, &result.TodayUV); err != nil {
		return Overview{}, err
	}
	var previousUV int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM analytics_daily_visitors WHERE page_id = ? AND occurred_date BETWEEN ? AND ?", pageID, previousStart, previousEnd).Scan(&previousUV); err != nil {
		return Overview{}, err
	}
	result.PVChange, result.UVChange = percentChange(result.TotalPV, previousPV), percentChange(result.TotalUV, previousUV)
	if result.TotalUV > 0 {
		result.AveragePages = float64(result.TotalPV) / float64(result.TotalUV)
		result.AvgSessionPages = result.AveragePages
	}
	var bounced, visitors int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FILTER (WHERE views = 1), COUNT(*) FROM (
		  SELECT occurred_date, visitor_hash, COUNT(*) AS views FROM analytics_events
		  WHERE page_id = ? AND event_type = 'page_view' AND occurred_date >= ? GROUP BY occurred_date, visitor_hash
		)`, pageID, start).Scan(&bounced, &visitors); err != nil {
		return Overview{}, err
	}
	if visitors > 0 {
		result.BounceRate = float64(bounced) / float64(visitors)
	}
	result.TodayVisitors = result.TodayUV
	result.VisitorsChange = result.UVChange
	return result, nil
}

func (s *Service) Trends(ctx context.Context, userID string, period int) ([]DailyStat, error) {
	pageID, err := s.personalPageID(ctx, userID)
	if err != nil {
		return nil, err
	}
	period = normalizePeriod(period)
	startDate := s.now().UTC().AddDate(0, 0, -(period - 1))
	rows, err := s.db.QueryContext(ctx, `
		SELECT occurred_date,
		       COUNT(*) FILTER (WHERE event_type = 'page_view') AS pv,
		       COUNT(DISTINCT visitor_hash) FILTER (WHERE event_type = 'page_view') AS uv
		FROM analytics_events WHERE page_id = ? AND occurred_date >= ? GROUP BY occurred_date`,
		pageID, startDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byDate := make(map[string]DailyStat)
	for rows.Next() {
		var stat DailyStat
		if err := rows.Scan(&stat.Date, &stat.PV, &stat.UV); err != nil {
			return nil, err
		}
		byDate[stat.Date] = stat
	}
	items := make([]DailyStat, 0, period)
	for offset := 0; offset < period; offset++ {
		date := startDate.AddDate(0, 0, offset).Format("2006-01-02")
		stat := byDate[date]
		stat.Date = date
		items = append(items, stat)
	}
	return items, rows.Err()
}

func (s *Service) Breakdown(ctx context.Context, userID string, period int) (Breakdown, error) {
	pageID, err := s.personalPageID(ctx, userID)
	if err != nil {
		return Breakdown{}, err
	}
	period = normalizePeriod(period)
	start := s.now().UTC().AddDate(0, 0, -(period - 1)).Format("2006-01-02")
	result := Breakdown{}
	result.TopSites, err = s.buckets(ctx, `
		SELECT s.id, s.title, COUNT(*) FROM analytics_events e JOIN sites s ON s.id = e.site_id
		WHERE e.page_id = ? AND e.event_type = 'site_click' AND e.occurred_date >= ?
		GROUP BY s.id, s.title ORDER BY COUNT(*) DESC, s.title LIMIT 20`, pageID, start)
	if err != nil {
		return Breakdown{}, err
	}
	result.Categories, err = s.buckets(ctx, `
		SELECT c.id, c.name, COUNT(*) FROM analytics_events e JOIN sites s ON s.id = e.site_id JOIN categories c ON c.id = s.category_id
		WHERE e.page_id = ? AND e.event_type = 'site_click' AND e.occurred_date >= ?
		GROUP BY c.id, c.name ORDER BY COUNT(*) DESC, c.name`, pageID, start)
	if err != nil {
		return Breakdown{}, err
	}
	result.Devices, err = s.buckets(ctx, `
		SELECT device, device, COUNT(*) FROM analytics_events
		WHERE page_id = ? AND event_type = 'page_view' AND occurred_date >= ? GROUP BY device ORDER BY COUNT(*) DESC`, pageID, start)
	if err != nil {
		return Breakdown{}, err
	}
	result.Referrers, err = s.buckets(ctx, `
		SELECT referrer_domain, CASE WHEN referrer_domain = '' THEN '直接访问' ELSE referrer_domain END, COUNT(*) FROM analytics_events
		WHERE page_id = ? AND event_type = 'page_view' AND occurred_date >= ? GROUP BY referrer_domain ORDER BY COUNT(*) DESC LIMIT 20`, pageID, start)
	if err != nil {
		return Breakdown{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT hex(visitor_hash), device, referrer_domain, occurred_at FROM analytics_events
		WHERE page_id = ? AND event_type = 'page_view' AND occurred_date >= ? ORDER BY occurred_at DESC LIMIT 20`, pageID, start)
	if err != nil {
		return Breakdown{}, err
	}
	defer rows.Close()
	result.RecentVisits = make([]RecentVisit, 0)
	for rows.Next() {
		var visit RecentVisit
		var hash, occurredAt string
		if err := rows.Scan(&hash, &visit.Device, &visit.ReferrerDomain, &occurredAt); err != nil {
			return Breakdown{}, err
		}
		if len(hash) > 12 {
			hash = hash[:12]
		}
		visit.AnonymousID = strings.ToLower(hash)
		visit.VisitedAt, err = time.Parse(time.RFC3339Nano, occurredAt)
		if err != nil {
			return Breakdown{}, err
		}
		result.RecentVisits = append(result.RecentVisits, visit)
	}
	return result, rows.Err()
}

func (s *Service) PurgeExpired(ctx context.Context) error {
	var retentionDays int
	if err := s.db.QueryRowContext(ctx, "SELECT analytics_retention_days FROM system_settings WHERE id = 1").Scan(&retentionDays); err != nil {
		return err
	}
	cutoff := s.now().UTC().AddDate(0, 0, -retentionDays)
	if _, err := s.db.ExecContext(ctx, "DELETE FROM analytics_events WHERE occurred_at < ?", cutoff.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM analytics_daily_visitors WHERE occurred_date < ?", cutoff.Format("2006-01-02"))
	return err
}

func (s *Service) RunRetention(ctx context.Context) {
	if s == nil {
		return
	}
	_ = s.PurgeExpired(ctx)
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.PurgeExpired(ctx)
		}
	}
}

func (s *Service) personalPageID(ctx context.Context, userID string) (string, error) {
	var pageID string
	if err := s.db.QueryRowContext(ctx, "SELECT id FROM navigation_pages WHERE kind = 'personal' AND owner_id = ?", userID).Scan(&pageID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return pageID, nil
}

func (s *Service) buckets(ctx context.Context, query string, args ...any) ([]Bucket, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Bucket, 0)
	for rows.Next() {
		var item Bucket
		if err := rows.Scan(&item.Key, &item.Label, &item.Value); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) visitorHash(date, address, userAgent string) []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(date))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(strings.TrimSpace(address)))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(userAgent))
	return mac.Sum(nil)
}

func cleanReferrer(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if len(host) > 253 {
		return ""
	}
	return host
}

func deviceType(userAgent string) string {
	value := strings.ToLower(userAgent)
	switch {
	case strings.Contains(value, "bot"), strings.Contains(value, "spider"), strings.Contains(value, "crawler"):
		return "bot"
	case strings.Contains(value, "ipad"), strings.Contains(value, "tablet"):
		return "tablet"
	case strings.Contains(value, "mobile"), strings.Contains(value, "iphone"), strings.Contains(value, "android"):
		return "mobile"
	default:
		return "desktop"
	}
}

func normalizePeriod(period int) int {
	if period == 7 || period == 90 {
		return period
	}
	return 30
}

func percentChange(current, previous int64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return float64(current-previous) / float64(previous) * 100
}
