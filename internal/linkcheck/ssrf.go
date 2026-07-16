package linkcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:2::/48"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2001:10::/28"),
	netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fec0::/10"),
}

type urlValidator struct {
	resolver Resolver
}

func (v urlValidator) validate(ctx context.Context, target *url.URL) ([]netip.Addr, error) {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") {
		return nil, blocked("only HTTP and HTTPS targets are allowed")
	}
	if target.User != nil {
		return nil, blocked("URL credentials are not allowed")
	}
	host := strings.TrimRight(strings.ToLower(strings.TrimSpace(target.Hostname())), ".")
	if host == "" {
		return nil, blocked("target host is empty")
	}
	if isLocalHostname(host) || isMetadataHostname(host) {
		return nil, blocked("local or metadata host is not allowed")
	}
	if port := target.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return nil, blocked("target port is invalid")
		}
	}

	addresses, err := v.resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, address := range addresses {
		if isBlockedAddress(address) {
			return nil, blocked("target resolves to a non-public address")
		}
	}
	return addresses, nil
}

func (v urlValidator) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if address, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{address.Unmap()}, nil
	}
	addresses, err := v.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve target host: %w", err)
	}
	if len(addresses) == 0 {
		return nil, errors.New("resolve target host: no addresses")
	}
	result := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		if address.IsValid() {
			result = append(result, address.Unmap())
		}
	}
	if len(result) == 0 {
		return nil, errors.New("resolve target host: no valid addresses")
	}
	return result, nil
}

func isLocalHostname(host string) bool {
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "local" || strings.HasSuffix(host, ".local")
}

func isMetadataHostname(host string) bool {
	switch host {
	case "metadata", "metadata.google.internal", "metadata.goog", "metadata.aws.internal",
		"metadata.azure.internal", "metadata.oraclecloud.internal", "instance-data", "instance-data.ec2.internal":
		return true
	default:
		return strings.HasSuffix(host, ".metadata.google.internal") || strings.HasSuffix(host, ".instance-data.ec2.internal")
	}
}

func isBlockedAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return true
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func blocked(reason string) error { return fmt.Errorf("%w: %s", ErrBlocked, reason) }

type validatingTransport struct {
	validator urlValidator
	base      http.RoundTripper
}

func (t validatingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if _, err := t.validator.validate(request.Context(), request.URL); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(request)
}

type safeDialer struct {
	validator urlValidator
	dialer    net.Dialer
}

func (d *safeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, blocked("invalid dial address")
	}
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(host, port)}
	addresses, err := d.validator.validate(ctx, target)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, resolved := range addresses {
		connection, err := d.dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
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

func newHTTPClient(options Options, validator urlValidator) *http.Client {
	base := options.Transport
	if base == nil {
		dialer := &safeDialer{validator: validator, dialer: net.Dialer{Timeout: options.RequestTimeout, KeepAlive: 30 * time.Second}}
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
		Transport: validatingTransport{validator: validator, base: base},
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > options.MaxRedirects {
				return errors.New("too many redirects")
			}
			_, err := validator.validate(request.Context(), request.URL)
			return err
		},
	}
}
