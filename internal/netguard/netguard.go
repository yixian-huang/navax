// Package netguard provides SSRF-safe URL validation and HTTP/dial primitives
// shared by every server-side fetcher (link checking, provider connectivity
// tests). It rejects loopback, private, link-local, reserved, and cloud
// metadata targets on every DNS resolution so that the same protection is
// enforced consistently instead of reimplemented per call site.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// ErrBlocked reports that a target host or address is not permitted.
var ErrBlocked = errors.New("target address is blocked")

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

// Resolver resolves a host to IP addresses. *net.Resolver satisfies it.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Validator checks URLs and resolves hosts to permitted addresses.
type Validator struct {
	resolver     Resolver
	allowPrivate bool
}

// NewValidator returns a strict Validator that permits only public unicast
// targets; a nil resolver falls back to net.DefaultResolver. Use this for
// user-supplied URLs (link checking) and always-public APIs.
func NewValidator(resolver Resolver) Validator {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return Validator{resolver: resolver}
}

// NewInternalValidator is like NewValidator but additionally permits private
// (RFC1918/ULA) targets, for admin-configured self-hosted integrations that may
// legitimately point at an internal relay (e.g. an SMTP or object-storage
// sidecar on a private Docker network). It still blocks loopback, link-local
// and cloud-metadata addresses, multicast, and reserved ranges, so the
// credential-leak and localhost-probe SSRF vectors remain closed.
func NewInternalValidator(resolver Resolver) Validator {
	validator := NewValidator(resolver)
	validator.allowPrivate = true
	return validator
}

// Validate rejects non-HTTP(S) schemes, embedded credentials, local/metadata
// hostnames, and any host that resolves to a non-public address. It returns the
// resolved, vetted addresses so callers can dial them directly (defeating
// DNS-rebinding between the check and the connection).
func (v Validator) Validate(ctx context.Context, target *url.URL) ([]netip.Addr, error) {
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
		if blockedAddress(address, v.allowPrivate) {
			return nil, blocked("target resolves to a non-public address")
		}
	}
	return addresses, nil
}

func (v Validator) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
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

// IsBlockedAddress reports whether an address is outside the public unicast
// range (strict mode: private addresses are blocked).
func IsBlockedAddress(address netip.Addr) bool {
	return blockedAddress(address, false)
}

// blockedAddress always rejects loopback, link-local (including cloud metadata),
// multicast, unspecified, and reserved/documentation ranges. Private RFC1918/ULA
// addresses are rejected unless allowPrivate is set.
func blockedAddress(address netip.Addr, allowPrivate bool) bool {
	address = address.Unmap()
	if !address.IsValid() || address.IsLoopback() || address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return true
	}
	if !allowPrivate && (address.IsPrivate() || !address.IsGlobalUnicast()) {
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
