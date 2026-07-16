package netguard

import (
	"context"
	"errors"
	"net/netip"
	"net/url"
	"testing"
)

type fakeResolver struct {
	addresses map[string][]netip.Addr
}

func (f *fakeResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	if addresses, ok := f.addresses[host]; ok {
		return addresses, nil
	}
	return nil, errors.New("no such host")
}

func TestStrictValidatorBlocksNonPublicTargets(t *testing.T) {
	resolver := &fakeResolver{addresses: map[string][]netip.Addr{
		"private.test": {netip.MustParseAddr("10.0.0.1")},
		"public.test":  {netip.MustParseAddr("8.8.8.8")},
	}}
	validator := NewValidator(resolver)
	blocked := []string{
		"http://127.0.0.1/", "http://[::1]/", "http://169.254.169.254/latest/meta-data/",
		"http://100.100.100.200/", "http://private.test/", "http://metadata.google.internal/",
		"http://localhost/", "ftp://public.test/", "http://user:pass@public.test/", "http://192.0.2.10/",
	}
	for _, raw := range blocked {
		target, _ := url.Parse(raw)
		if _, err := validator.Validate(context.Background(), target); !errors.Is(err, ErrBlocked) {
			t.Errorf("Validate(%q) = %v, want blocked", raw, err)
		}
	}
	target, _ := url.Parse("https://public.test/path")
	if addresses, err := validator.Validate(context.Background(), target); err != nil || len(addresses) != 1 {
		t.Fatalf("public target = %v, %v", addresses, err)
	}
}

func TestInternalValidatorPermitsPrivateButBlocksMetadataAndLoopback(t *testing.T) {
	resolver := &fakeResolver{addresses: map[string][]netip.Addr{
		"relay.internal": {netip.MustParseAddr("172.20.0.5")}, // Docker bridge range
		"ula.internal":   {netip.MustParseAddr("fd00::1")},
	}}
	validator := NewInternalValidator(resolver)

	for _, raw := range []string{"http://relay.internal:587/", "http://ula.internal/"} {
		target, _ := url.Parse(raw)
		if _, err := validator.Validate(context.Background(), target); err != nil {
			t.Errorf("Validate(%q) = %v, want allowed for internal relay", raw, err)
		}
	}
	// Metadata and loopback stay blocked even in internal mode.
	for _, raw := range []string{"http://169.254.169.254/", "http://127.0.0.1/", "http://metadata.google.internal/"} {
		target, _ := url.Parse(raw)
		if _, err := validator.Validate(context.Background(), target); !errors.Is(err, ErrBlocked) {
			t.Errorf("Validate(%q) = %v, want blocked in internal mode", raw, err)
		}
	}
}
