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
		"https://github.com/alice/.git",            // 裁剪 .git 后 repo 段为空
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

// TestFetchTarballScopesTokenToGitHubHosts 确认 token 不会外泄给 extraHosts
// 白名单里的第三方主机(如自建 Gitea),仅在请求官方 GitHub 主机时下发。
func TestFetchTarballScopesTokenToGitHubHosts(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "git.example.com":
			if got := r.Header.Get("Authorization"); got != "" {
				t.Fatalf("extraHost authorization header = %q, want empty", got)
			}
			return respond(200, "x"), nil
		case "codeload.github.com":
			if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
				t.Fatalf("github authorization header = %q, want Bearer tok123", got)
			}
			return respond(200, "y"), nil
		}
		t.Fatalf("unexpected host %s", r.URL.Host)
		return nil, nil
	})
	client := NewGitHubClient(publicResolver("git.example.com", "codeload.github.com"), transport, []string{"git.example.com"}, "tok123")

	if _, err := client.FetchTarball(context.Background(), "https://git.example.com/alice/lilac", "v1.2.0"); err != nil {
		t.Fatalf("extraHost FetchTarball() error = %v", err)
	}
	sha := strings.Repeat("c", 40)
	if _, err := client.FetchTarball(context.Background(), "https://github.com/alice/lilac", sha); err != nil {
		t.Fatalf("github FetchTarball() error = %v", err)
	}
}

// TestFetchTarballEscapesRepoPathSegments 确认 owner/repo 拼进下载 URL 前会
// 转义:未转义时,已解码的 "#" 之类字符会在 http.NewRequestWithContext 内部
// 再次 Parse 时被当成 Fragment 分隔符,截断请求路径。
func TestFetchTarballEscapesRepoPathSegments(t *testing.T) {
	sha := strings.Repeat("d", 40)
	wantPath := "/alice/li#lac/tar.gz/" + sha
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "codeload.github.com" {
			t.Fatalf("unexpected host %s", r.URL.Host)
		}
		if r.URL.Path != wantPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, wantPath)
		}
		return respond(200, "z"), nil
	})
	client := NewGitHubClient(publicResolver("codeload.github.com"), transport, nil, "")
	if _, err := client.FetchTarball(context.Background(), "https://github.com/alice/li%23lac", sha); err != nil {
		t.Fatalf("FetchTarball() error = %v", err)
	}
}

// TestFetchTarballRejectsRedirectToDisallowedHost 确认重定向目标主机白名单
// 独立于 netguard 的 IP 复核生效:evil.example.com 解析到一个完全合法的公网
// IP(IP 复核会放行),但主机本身不在 {github.com, api.github.com,
// codeload.github.com} ∪ extraHosts 里,必须被拒绝——否则一次 3xx 就能把
// 拉取器带去任意公网主机。
func TestFetchTarballRejectsRedirectToDisallowedHost(t *testing.T) {
	sha := strings.Repeat("e", 40)
	var evilHit bool
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "codeload.github.com":
			resp := respond(http.StatusFound, "")
			resp.Header.Set("Location", "https://evil.example.com/steal.tar.gz")
			return resp, nil
		case "evil.example.com":
			evilHit = true
			return respond(200, "should never be reached"), nil
		}
		t.Fatalf("unexpected host %s", r.URL.Host)
		return nil, nil
	})
	client := NewGitHubClient(publicResolver("codeload.github.com", "evil.example.com"), transport, nil, "")
	if _, err := client.FetchTarball(context.Background(), "https://github.com/alice/lilac", sha); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("err = %v, want ErrHostNotAllowed", err)
	}
	if evilHit {
		t.Fatal("redirect to disallowed host must not be followed")
	}
}

// TestFetchTarballFollowsRedirectToExtraHost 确认白名单不是"只认 github.com
// 三件套":Gitea 兼容主机的重定向(例如反代把 archive 请求转到同域的下载
// 节点)只要落在 extraHosts 内仍会被跟随。
func TestFetchTarballFollowsRedirectToExtraHost(t *testing.T) {
	var finalHit bool
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Host == "git.example.com" && r.URL.Path == "/alice/lilac/archive/v1.2.0.tar.gz":
			resp := respond(http.StatusFound, "")
			resp.Header.Set("Location", "https://git.example.com/dl/lilac-v1.2.0.tar.gz")
			return resp, nil
		case r.URL.Host == "git.example.com" && r.URL.Path == "/dl/lilac-v1.2.0.tar.gz":
			finalHit = true
			return respond(200, "tarball-bytes"), nil
		}
		t.Fatalf("unexpected request %s", r.URL.String())
		return nil, nil
	})
	client := NewGitHubClient(publicResolver("git.example.com"), transport, []string{"git.example.com"}, "")
	fetched, err := client.FetchTarball(context.Background(), "https://git.example.com/alice/lilac", "v1.2.0")
	if err != nil {
		t.Fatalf("FetchTarball() error = %v", err)
	}
	if !finalHit || string(fetched.Data) != "tarball-bytes" {
		t.Fatalf("fetched = %+v, finalHit = %v", fetched, finalHit)
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
