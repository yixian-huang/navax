// Package themeimport 负责第三方主题的获取与导入编排。信任边界仍在
// internal/themes 的校验/编译管线;本包只做「拿到字节」与「串起流程」。
package themeimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/netguard"
	"github.com/yixian-huang/navax/internal/themes"
)

var (
	// ErrHostNotAllowed:URL 不在导入白名单或形状非法——用户输入问题(422)。
	ErrHostNotAllowed = errors.New("theme import host not allowed")
	// ErrUpstream:上游仓库不可达/不存在——上游问题(502)。
	ErrUpstream = errors.New("theme import upstream failed")
	// ErrUpdateCheckUnsupported:主机在白名单内(已通过 parseRepoURL 校验,
	// 不是 SSRF 拒绝),但没有等价于 GitHub commits API 的接口,无法在不
	// 下载的前提下解析 HEAD。调用方(Service.CheckUpdate)应把它当作
	// 「无法确认、视为无更新」优雅退化,而不是报错。
	ErrUpdateCheckUnsupported = errors.New("theme update check not supported for this host")
)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// githubTokenHosts 是允许附加 Authorization 头的主机集合。token 只在
// 请求这些官方 GitHub 主机时下发,绝不外泄给 extraHosts 白名单里的
// 自建 Gitea 等第三方主机。
var githubTokenHosts = map[string]bool{
	"github.com":          true,
	"api.github.com":      true,
	"codeload.github.com": true,
}

// Fetched 是一次拉取的产物。SHA 对 GitHub 是解析后的 commit sha;
// 对 Gitea 兼容主机是调用方显式给出的 ref(这些主机不做 API 解析)。
type Fetched struct {
	Data         []byte
	SHA          string
	CanonicalURL string
}

type GitHubClient struct {
	client     *http.Client
	extraHosts map[string]bool
	token      string
}

// maxGitHubRedirects 与既有 netguard.GuardedClient 一致的 3 跳上限。
const maxGitHubRedirects = 3

// NewGitHubClient 构造拉取器。resolver/transport 供测试注入;生产传 nil,
// 使用严格 netguard 校验器(公网单播 only)与 30s 超时、3 跳重定向的守护 client。
//
// 生产与测试注入两条路径都不直接用 netguard.GuardedClient,而是自己组装
// http.Client:Transport 仍是 netguard.Transport 包裹(保留每次 RoundTrip 的
// IP 复核,生产路径下还经 netguard.Dialer 把拨号钉死在已校验的解析结果上,
// 防 DNS rebinding),但 CheckRedirect 额外叠加了主机白名单——否则一次 3xx
// 就能把拉取器带去白名单外的任意公网主机(IP 复核本身并不限制"是哪个域名"，
// 只限制"是不是内网/元数据地址")。
func NewGitHubClient(resolver netguard.Resolver, transport http.RoundTripper, extraHosts []string, token string) *GitHubClient {
	const timeout = 30 * time.Second
	validator := netguard.NewValidator(resolver)
	extras := make(map[string]bool, len(extraHosts))
	for _, host := range extraHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			extras[host] = true
		}
	}

	base := transport
	if base == nil {
		dialer := netguard.Dialer{Validator: validator, Dialer: net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}}
		base = &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
			ExpectContinueTimeout: time.Second,
			DisableCompression:    true,
		}
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: netguard.Transport{Validator: validator, Base: base},
	}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > maxGitHubRedirects {
			return errors.New("too many redirects")
		}
		if _, err := validator.Validate(request.Context(), request.URL); err != nil {
			return err
		}
		host := strings.ToLower(request.URL.Hostname())
		if !githubTokenHosts[host] && !extras[host] {
			return fmt.Errorf("%w: 重定向目标主机 %s 不在白名单", ErrHostNotAllowed, host)
		}
		return nil
	}
	return &GitHubClient{client: client, extraHosts: extras, token: token}
}

// FetchTarball 解析仓库 URL 并拉取 ref 对应的 tarball。
// github.com:ref(缺省默认分支 HEAD)经 api.github.com 解析为 commit sha,
// tarball 走 codeload。白名单追加主机走 Gitea 兼容布局且必须显式 ref。
func (c *GitHubClient) FetchTarball(ctx context.Context, rawURL, ref string) (Fetched, error) {
	owner, repo, host, err := parseRepoURL(rawURL, c.extraHosts)
	if err != nil {
		return Fetched{}, err
	}
	canonical := "https://" + host + "/" + owner + "/" + repo
	if host != "github.com" {
		if strings.TrimSpace(ref) == "" {
			return Fetched{}, fmt.Errorf("%w: 该主机需要显式 ref", ErrHostNotAllowed)
		}
		data, err := c.get(ctx, "https://"+host+"/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/archive/"+url.PathEscape(ref)+".tar.gz")
		if err != nil {
			return Fetched{}, err
		}
		return Fetched{Data: data, SHA: ref, CanonicalURL: canonical}, nil
	}
	sha := strings.ToLower(strings.TrimSpace(ref))
	if !shaPattern.MatchString(sha) {
		refPath := "HEAD"
		if strings.TrimSpace(ref) != "" {
			refPath = ref
		}
		resolved, err := c.resolveSHA(ctx, owner, repo, refPath)
		if err != nil {
			return Fetched{}, err
		}
		sha = resolved
	}
	data, err := c.get(ctx, "https://codeload.github.com/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/tar.gz/"+sha)
	if err != nil {
		return Fetched{}, err
	}
	return Fetched{Data: data, SHA: sha, CanonicalURL: canonical}, nil
}

// ResolveHeadSHA 解析仓库某 ref 的 commit sha,但不下载 tarball——供更新检查用。
// github.com 走 api.github.com/commits;ref 为空查默认分支 HEAD,非空查该
// 具体 ref——ref 来自 themes.source_git_ref,即 Service.CheckUpdate 里
// 持久化的导入时原始 ref,不再恒为空字符串。追加白名单主机没有等价的
// commits API,无法在不下载的前提下确认"现在最新是什么",一律报
// ErrUpdateCheckUnsupported——不是 ErrHostNotAllowed:主机本身合法,只是没有
// 能力回答这个问题。这里对非空 ref 同样报 ErrUpdateCheckUnsupported,不像
// FetchTarball 那样把追加主机上未经验证的 ref 原样当 SHA 回显:本函数承诺
// 返回的是"解析出的 commit sha",解析不出就该报错,不能把未经验证的输入
// 伪装成解析结果。
func (c *GitHubClient) ResolveHeadSHA(ctx context.Context, rawURL, ref string) (string, error) {
	owner, repo, host, err := parseRepoURL(rawURL, c.extraHosts)
	if err != nil {
		return "", err
	}
	if host != "github.com" {
		return "", ErrUpdateCheckUnsupported
	}
	refPath := "HEAD"
	if strings.TrimSpace(ref) != "" {
		refPath = ref
	}
	return c.resolveSHA(ctx, owner, repo, refPath)
}

func parseRepoURL(rawURL string, extras map[string]bool) (owner, repo, host string, err error) {
	parsed, parseErr := url.Parse(strings.TrimSpace(rawURL))
	if parseErr != nil || parsed.Scheme != "https" || parsed.User != nil {
		return "", "", "", fmt.Errorf("%w: 仓库地址必须是 https 且不含凭据", ErrHostNotAllowed)
	}
	host = strings.ToLower(parsed.Hostname())
	if host != "github.com" && !extras[host] {
		return "", "", "", fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return "", "", "", fmt.Errorf("%w: 地址需为 https://%s/{owner}/{repo}", ErrHostNotAllowed, host)
	}
	repo = strings.TrimSuffix(segments[1], ".git")
	if repo == "" {
		return "", "", "", fmt.Errorf("%w: 地址需为 https://%s/{owner}/{repo}", ErrHostNotAllowed, host)
	}
	return segments[0], repo, host, nil
}

func (c *GitHubClient) resolveSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	target := "https://api.github.com/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/commits/" + url.PathEscape(ref)
	body, err := c.get(ctx, target)
	if err != nil {
		return "", err
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || !shaPattern.MatchString(payload.SHA) {
		return "", fmt.Errorf("%w: 无法解析 commit sha", ErrUpstream)
	}
	return payload.SHA, nil
}

func (c *GitHubClient) get(ctx context.Context, target string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if c.token != "" && githubTokenHosts[strings.ToLower(request.URL.Hostname())] {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.client.Do(request)
	if err != nil {
		// CheckRedirect 的拒绝(IP 复核或主机白名单)经 *url.Error 包装后到达
		// 这里;errors.Is 沿 Unwrap 链能看到内层的 ErrHostNotAllowed。
		if errors.Is(err, ErrHostNotAllowed) {
			return nil, err
		}
		if errors.Is(err, netguard.ErrBlocked) {
			return nil, fmt.Errorf("%w: 目标地址被拒绝", ErrHostNotAllowed)
		}
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: 上游返回 %d", ErrUpstream, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, int64(themes.MaxArchiveBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if len(body) > themes.MaxArchiveBytes {
		return nil, fmt.Errorf("%w: 响应体超过 %d 字节上限", ErrUpstream, themes.MaxArchiveBytes)
	}
	return body, nil
}
