package linkcheck

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/yixian-huang/navax/internal/netguard"
)

// blocked wraps ErrBlocked for link-check-specific rejections raised outside
// netguard (e.g. a malformed site URL before validation runs).
func blocked(reason string) error { return fmt.Errorf("%w: %s", ErrBlocked, reason) }

// newHTTPClient builds the SSRF-safe client used to probe navigation sites. It
// reuses netguard's validator, safe dialer, and validating transport while
// keeping the connection-pool tuning specific to batch link checking.
func newHTTPClient(options Options, validator netguard.Validator) *http.Client {
	base := options.Transport
	if base == nil {
		dialer := netguard.Dialer{Validator: validator, Dialer: net.Dialer{Timeout: options.RequestTimeout, KeepAlive: 30 * time.Second}}
		base = &http.Transport{
			Proxy:                  nil,
			DialContext:            dialer.DialContext,
			ForceAttemptHTTP2:      true,
			MaxIdleConns:           options.Concurrency * options.MaxActiveBatches,
			MaxIdleConnsPerHost:    options.Concurrency,
			MaxConnsPerHost:        options.Concurrency,
			IdleConnTimeout:        30 * time.Second,
			TLSHandshakeTimeout:    options.RequestTimeout,
			ResponseHeaderTimeout:  options.RequestTimeout,
			ExpectContinueTimeout:  time.Second,
			MaxResponseHeaderBytes: 64 << 10,
			DisableCompression:     true,
		}
	}
	return &http.Client{
		Transport: netguard.Transport{Validator: validator, Base: base},
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > options.MaxRedirects {
				return errors.New("too many redirects")
			}
			_, err := validator.Validate(request.Context(), request.URL)
			return err
		},
	}
}
