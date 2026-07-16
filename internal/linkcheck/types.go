// Package linkcheck performs bounded, SSRF-safe checks of navigation sites.
package linkcheck

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/netip"
	"time"

	"github.com/yixian-huang/navax/internal/netguard"
)

const (
	StatusReachable   = "reachable"
	StatusUnreachable = "unreachable"
	StatusBlocked     = "blocked"
	StatusTimeout     = "timeout"
)

var (
	ErrInvalid = errors.New("invalid link check request")
	ErrBusy    = errors.New("link checker is busy")
	// ErrBlocked aliases netguard's so existing callers and tests keep matching
	// blocked-target errors regardless of which package raised them.
	ErrBlocked = netguard.ErrBlocked
)

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type Options struct {
	Resolver         Resolver
	Transport        http.RoundTripper
	Concurrency      int
	MaxActiveBatches int
	RequestTimeout   time.Duration
	BatchTimeout     time.Duration
	MaxResponseBytes int64
	MaxRedirects     int
	Now              func() time.Time
}

type Result struct {
	SiteID     string    `json:"siteId"`
	Status     string    `json:"status"`
	HTTPStatus *int      `json:"httpStatus"`
	LatencyMS  *int      `json:"latencyMs"`
	CheckedAt  time.Time `json:"checkedAt"`
	Message    string    `json:"message,omitempty"`
}

type siteTarget struct {
	ID  string
	URL string
}

type Service struct {
	db               *sql.DB
	client           *http.Client
	now              func() time.Time
	concurrency      int
	requestTimeout   time.Duration
	batchTimeout     time.Duration
	maxResponseBytes int64
	batchSlots       chan struct{}
}
