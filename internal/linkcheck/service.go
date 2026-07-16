package linkcheck

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/navigation"
	"github.com/yixian-huang/navax/internal/netguard"
)

const (
	defaultConcurrency      = 8
	defaultMaxActiveBatches = 2
	defaultRequestTimeout   = 5 * time.Second
	defaultBatchTimeout     = 25 * time.Second
	defaultMaxResponseBytes = 64 << 10
	defaultMaxRedirects     = 5
)

func NewService(db *sql.DB) *Service {
	return NewServiceWithOptions(db, Options{})
}

func NewServiceWithOptions(db *sql.DB, options Options) *Service {
	if options.Resolver == nil {
		options.Resolver = net.DefaultResolver
	}
	if options.Concurrency <= 0 {
		options.Concurrency = defaultConcurrency
	}
	if options.MaxActiveBatches <= 0 {
		options.MaxActiveBatches = defaultMaxActiveBatches
	}
	if options.RequestTimeout <= 0 {
		options.RequestTimeout = defaultRequestTimeout
	}
	if options.BatchTimeout <= 0 {
		options.BatchTimeout = defaultBatchTimeout
	}
	if options.MaxResponseBytes <= 0 {
		options.MaxResponseBytes = defaultMaxResponseBytes
	}
	if options.MaxRedirects <= 0 {
		options.MaxRedirects = defaultMaxRedirects
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	validator := netguard.NewValidator(options.Resolver)
	return &Service{
		db: db, client: newHTTPClient(options, validator), now: options.Now,
		concurrency: options.Concurrency, requestTimeout: options.RequestTimeout,
		batchTimeout: options.BatchTimeout, maxResponseBytes: options.MaxResponseBytes,
		batchSlots: make(chan struct{}, options.MaxActiveBatches),
	}
}

func (s *Service) Check(ctx context.Context, actor navigation.Actor, pageID string, siteIDs []string) ([]Result, error) {
	if err := validateSiteIDs(siteIDs); err != nil {
		return nil, err
	}
	if err := authorizePage(ctx, s.db, actor, pageID); err != nil {
		return nil, err
	}
	targets, err := loadTargets(ctx, s.db, pageID, siteIDs)
	if err != nil {
		return nil, err
	}
	select {
	case s.batchSlots <- struct{}{}:
		defer func() { <-s.batchSlots }()
	default:
		return nil, ErrBusy
	}

	batchContext, cancel := context.WithTimeout(ctx, s.batchTimeout)
	defer cancel()
	results := make([]Result, len(targets))
	type job struct {
		index  int
		target siteTarget
	}
	jobs := make(chan job)
	workers := min(s.concurrency, len(targets))
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			for item := range jobs {
				results[item.index] = s.probe(batchContext, item.target)
			}
		}()
	}
	for index, target := range targets {
		jobs <- job{index: index, target: target}
	}
	close(jobs)
	wait.Wait()

	if err := persistResults(ctx, s.db, results); err != nil {
		return nil, err
	}
	return results, nil
}

func validateSiteIDs(siteIDs []string) error {
	if len(siteIDs) < 1 || len(siteIDs) > 50 {
		return fmt.Errorf("%w: siteIds must contain between 1 and 50 IDs", ErrInvalid)
	}
	seen := make(map[string]struct{}, len(siteIDs))
	for _, siteID := range siteIDs {
		if strings.TrimSpace(siteID) != siteID || len(siteID) < 8 || len(siteID) > 64 {
			return fmt.Errorf("%w: every siteId must be an ID of 8 to 64 characters", ErrInvalid)
		}
		if _, exists := seen[siteID]; exists {
			return fmt.Errorf("%w: siteIds must be unique", ErrInvalid)
		}
		seen[siteID] = struct{}{}
	}
	return nil
}

func loadTargets(ctx context.Context, db *sql.DB, pageID string, siteIDs []string) ([]siteTarget, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(siteIDs)), ",")
	arguments := make([]any, 0, len(siteIDs)+1)
	arguments = append(arguments, pageID)
	for _, siteID := range siteIDs {
		arguments = append(arguments, siteID)
	}
	rows, err := db.QueryContext(ctx, "SELECT id, url FROM sites WHERE page_id = ? AND id IN ("+placeholders+")", arguments...)
	if err != nil {
		return nil, fmt.Errorf("load link check sites: %w", err)
	}
	defer rows.Close()
	byID := make(map[string]siteTarget, len(siteIDs))
	for rows.Next() {
		var target siteTarget
		if err := rows.Scan(&target.ID, &target.URL); err != nil {
			return nil, fmt.Errorf("scan link check site: %w", err)
		}
		byID[target.ID] = target
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list link check sites: %w", err)
	}
	if len(byID) != len(siteIDs) {
		return nil, fmt.Errorf("%w: every siteId must belong to the requested page", ErrInvalid)
	}
	targets := make([]siteTarget, len(siteIDs))
	for index, siteID := range siteIDs {
		targets[index] = byID[siteID]
	}
	return targets, nil
}

func (s *Service) probe(ctx context.Context, target siteTarget) Result {
	checkedAt := s.now().UTC()
	result := Result{SiteID: target.ID, Status: StatusUnreachable, CheckedAt: checkedAt}
	started := time.Now()
	requestContext, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	status, err := s.request(requestContext, http.MethodHead, target.URL)
	if err == nil && status != nil && (*status == http.StatusMethodNotAllowed || *status == http.StatusNotImplemented) {
		status, err = s.request(requestContext, http.MethodGet, target.URL)
	}
	elapsed := int(time.Since(started).Milliseconds())
	result.LatencyMS = &elapsed
	if err != nil {
		result.HTTPStatus = status
		switch {
		case errors.Is(err, ErrBlocked):
			result.Status = StatusBlocked
			result.HTTPStatus = nil
			result.LatencyMS = nil
			result.Message = "目标地址被安全策略阻止"
		case isTimeout(err) || errors.Is(requestContext.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded):
			result.Status = StatusTimeout
			result.Message = "请求超时"
		default:
			result.Status = StatusUnreachable
			result.Message = "请求失败"
		}
		return result
	}
	result.HTTPStatus = status
	if status != nil && *status >= 200 && *status < 400 {
		result.Status = StatusReachable
		return result
	}
	result.Status = StatusUnreachable
	if status != nil {
		result.Message = fmt.Sprintf("HTTP %d", *status)
	}
	return result
}

func (s *Service) request(ctx context.Context, method, target string) (*int, error) {
	request, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, blocked("target URL is invalid")
	}
	request.Header.Set("User-Agent", "nav.ax-link-checker/1")
	request.Header.Set("Accept", "*/*")
	if method == http.MethodGet {
		request.Header.Set("Range", fmt.Sprintf("bytes=0-%d", s.maxResponseBytes-1))
	}
	response, err := s.client.Do(request)
	var status *int
	if response != nil {
		value := response.StatusCode
		status = &value
		if response.Body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, s.maxResponseBytes))
			_ = response.Body.Close()
		}
	}
	return status, err
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func persistResults(ctx context.Context, db *sql.DB, results []Result) error {
	return database.WithinTx(ctx, db, nil, func(tx *sql.Tx) error {
		for _, result := range results {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO link_check_results(site_id, status, http_status, latency_ms, message, checked_at)
				VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT(site_id) DO UPDATE SET
					status = excluded.status,
					http_status = excluded.http_status,
					latency_ms = excluded.latency_ms,
					message = excluded.message,
					checked_at = excluded.checked_at`,
				result.SiteID, result.Status, nullableInt(result.HTTPStatus), nullableInt(result.LatencyMS),
				truncateUTF8(result.Message, 500), result.CheckedAt.Format(time.RFC3339Nano),
			)
			if err != nil {
				return fmt.Errorf("store link check result: %w", err)
			}
		}
		return nil
	})
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func truncateUTF8(value string, maximumBytes int) string {
	if len(value) <= maximumBytes {
		return value
	}
	for len(value) > maximumBytes {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

func authorizePage(ctx context.Context, db *sql.DB, actor navigation.Actor, pageID string) error {
	var kind navigation.PageKind
	var ownerID sql.NullString
	err := db.QueryRowContext(ctx, "SELECT kind, owner_id FROM navigation_pages WHERE id = ?", pageID).Scan(&kind, &ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return navigation.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("authorize link check page: %w", err)
	}
	if kind == navigation.PageKindSystem {
		if !actor.IsAdmin() {
			return navigation.ErrForbidden
		}
		return nil
	}
	if !ownerID.Valid || actor.UserID == "" || ownerID.String != actor.UserID {
		return navigation.ErrForbidden
	}
	return nil
}
