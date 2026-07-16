package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Transport wraps a RoundTripper and re-validates the request URL before every
// round trip, including on redirects when installed as the client Transport.
type Transport struct {
	Validator Validator
	Base      http.RoundTripper
}

func (t Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	if _, err := t.Validator.Validate(request.Context(), request.URL); err != nil {
		return nil, err
	}
	return t.Base.RoundTrip(request)
}

// Dialer validates the dial address and connects only to a vetted resolved IP,
// so a hostname that passed validation cannot be re-resolved to an internal
// address at connection time.
type Dialer struct {
	Validator Validator
	Dialer    net.Dialer
}

func (d Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, blocked("invalid dial address")
	}
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(host, port)}
	addresses, err := d.Validator.Validate(ctx, target)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, resolved := range addresses {
		connection, err := d.Dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
		if err == nil {
			return connection, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	if lastErr == nil {
		lastErr = errors.New("target host has no dialable addresses")
	}
	return nil, lastErr
}

// GuardedClient builds an SSRF-safe *http.Client for one-off connectivity
// checks. Redirects are re-validated and capped at maxRedirects.
func GuardedClient(validator Validator, timeout time.Duration, maxRedirects int) *http.Client {
	dialer := Dialer{Validator: validator, Dialer: net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}}
	base := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: time.Second,
		DisableCompression:    true,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: Transport{Validator: validator, Base: base},
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return errors.New("too many redirects")
			}
			_, err := validator.Validate(request.Context(), request.URL)
			return err
		},
	}
}
