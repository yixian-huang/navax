package themeimport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

type fakeResolver struct{ addresses map[string][]netip.Addr }

func (f *fakeResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	if addrs, ok := f.addresses[host]; ok {
		return addrs, nil
	}
	return nil, errors.New("host not found")
}

func publicResolver(hosts ...string) *fakeResolver {
	addresses := make(map[string][]netip.Addr, len(hosts))
	for _, host := range hosts {
		addresses[host] = []netip.Addr{netip.MustParseAddr("8.8.8.8")}
	}
	return &fakeResolver{addresses: addresses}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func TestFetchTarballResolvesRefAndDownloads(t *testing.T) {
	var apiHit, tarHit bool
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "api.github.com":
			apiHit = true
			if r.URL.Path != "/repos/alice/lilac/commits/main" {
				t.Fatalf("api path = %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
				t.Fatalf("auth header = %q", got)
			}
			return respond(200, `{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`), nil
		case "codeload.github.com":
			tarHit = true
			if r.URL.Path != "/alice/lilac/tar.gz/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
				t.Fatalf("codeload path = %s", r.URL.Path)
			}
			return respond(200, "tarball-bytes"), nil
		}
		t.Fatalf("unexpected host %s", r.URL.Host)
		return nil, nil
	})
	client := NewGitHubClient(publicResolver("api.github.com", "codeload.github.com"), transport, nil, "tok123")
	fetched, err := client.FetchTarball(context.Background(), "https://github.com/alice/lilac", "main")
	if err != nil {
		t.Fatalf("FetchTarball() error = %v", err)
	}
	if !apiHit || !tarHit || fetched.SHA != strings.Repeat("a", 40) || string(fetched.Data) != "tarball-bytes" {
		t.Fatalf("fetched = %+v (api=%v tar=%v)", fetched, apiHit, tarHit)
	}
	if fetched.CanonicalURL != "https://github.com/alice/lilac" {
		t.Fatalf("canonical = %q", fetched.CanonicalURL)
	}
}

func TestFetchTarballSkipsAPIForExplicitSHA(t *testing.T) {
	sha := strings.Repeat("b", 40)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "codeload.github.com" {
			t.Fatalf("unexpected host %s", r.URL.Host)
		}
		return respond(200, "x"), nil
	})
	client := NewGitHubClient(publicResolver("codeload.github.com"), transport, nil, "")
	fetched, err := client.FetchTarball(context.Background(), "https://github.com/alice/lilac", sha)
	if err != nil || fetched.SHA != sha {
		t.Fatalf("fetched = %+v, err = %v", fetched, err)
	}
}

func TestFetchTarballRejectsDisallowedHostAndScheme(t *testing.T) {
	client := NewGitHubClient(publicResolver(), roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("must not reach network")
		return nil, nil
	}), nil, "")
	for _, target := range []string{
		"https://evil.example.com/alice/lilac",
		"http://github.com/alice/lilac",            // 非 https
		"https://github.com/alice",                 // 缺 repo 段
		"https://user:pass@github.com/alice/lilac", // 内嵌凭据
	} {
		if _, err := client.FetchTarball(context.Background(), target, "main"); !errors.Is(err, ErrHostNotAllowed) {
			t.Fatalf("%s err = %v, want ErrHostNotAllowed", target, err)
		}
	}
}

func TestFetchTarballGiteaCompatibleHost(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "git.example.com" || r.URL.Path != "/alice/lilac/archive/v1.2.0.tar.gz" {
			t.Fatalf("url = %s", r.URL.String())
		}
		return respond(200, "x"), nil
	})
	client := NewGitHubClient(publicResolver("git.example.com"), transport, []string{"git.example.com"}, "")
	fetched, err := client.FetchTarball(context.Background(), "https://git.example.com/alice/lilac", "v1.2.0")
	if err != nil || fetched.SHA != "v1.2.0" {
		t.Fatalf("fetched = %+v, err = %v", fetched, err)
	}
	// Gitea 兼容主机必须显式 ref。
	if _, err := client.FetchTarball(context.Background(), "https://git.example.com/alice/lilac", ""); err == nil {
		t.Fatal("empty ref on gitea-compatible host must fail")
	}
}

func TestFetchTarballMapsUpstreamFailures(t *testing.T) {
	client := NewGitHubClient(publicResolver("api.github.com"), roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return respond(404, `{"message":"Not Found"}`), nil
	}), nil, "")
	if _, err := client.FetchTarball(context.Background(), "https://github.com/alice/gone", "main"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v, want ErrUpstream", err)
	}
}
