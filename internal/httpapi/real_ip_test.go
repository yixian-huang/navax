package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestRealIPResolvesOnlyBehindTrustedProxy(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.1.5/32"),
	}

	cases := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string // expected downstream RemoteAddr host
	}{
		{"untrusted peer keeps remote addr", "203.0.113.9:5000", "1.2.3.4", "203.0.113.9"},
		{"trusted peer takes rightmost untrusted", "10.1.2.3:443", "9.9.9.9, 70.70.70.70", "70.70.70.70"},
		{"skips chained trusted hops", "192.168.1.5:443", "70.70.70.70, 10.9.9.9", "70.70.70.70"},
		{"trusted peer no XFF keeps peer", "10.1.2.3:443", "", "10.1.2.3"},
		{"all forwarded trusted keeps peer", "10.1.2.3:443", "10.8.8.8, 10.9.9.9", "10.1.2.3"},
		{"ignores malformed forwarded entries", "10.1.2.3:443", "garbage, 70.70.70.70", "70.70.70.70"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seen string
			handler := RealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = peerAddress(r.RemoteAddr)
			}))
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			request.RemoteAddr = tc.remoteAddr
			if tc.forwarded != "" {
				request.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			handler.ServeHTTP(httptest.NewRecorder(), request)
			if seen != tc.want {
				t.Fatalf("client IP = %q, want %q", seen, tc.want)
			}
		})
	}
}

func TestRealIPNoTrustedProxiesIsNoop(t *testing.T) {
	var seen string
	handler := RealIP(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = peerAddress(r.RemoteAddr)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.RemoteAddr = "203.0.113.9:5000"
	request.Header.Set("X-Forwarded-For", "1.2.3.4") // must be ignored
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if seen != "203.0.113.9" {
		t.Fatalf("client IP = %q, want 203.0.113.9 (XFF must be ignored without trusted proxies)", seen)
	}
}
