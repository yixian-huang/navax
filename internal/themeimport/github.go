// Package themeimport 负责第三方主题的获取与导入编排。信任边界仍在
// internal/themes 的校验/编译管线;本包只做「拿到字节」与「串起流程」。
package themeimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

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

// NewGitHubClient 构造拉取器。resolver/transport 供测试注入;生产传 nil,
// 使用严格 netguard 校验器(公网单播 only)与 30s 超时、3 跳重定向的守护 client。
func NewGitHubClient(resolver netguard.Resolver, transport http.RoundTripper, extraHosts []string, token string) *GitHubClient {
	validator := netguard.NewValidator(resolver)
	var client *http.Client
	if transport != nil {
		client = &http.Client{Timeout: 30 * time.Second, Transport: netguard.Transport{Validator: validator, Base: transport}}
	} else {
		client = netguard.GuardedClient(validator, 30*time.Second, 3)
	}
	extras := make(map[string]bool, len(extraHosts))
	for _, host := range extraHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			extras[host] = true
		}
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
		data, err := c.download(ctx, "https://"+host+"/"+owner+"/"+repo+"/archive/"+url.PathEscape(ref)+".tar.gz")
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
	data, err := c.download(ctx, "https://codeload.github.com/"+owner+"/"+repo+"/tar.gz/"+sha)
	if err != nil {
		return Fetched{}, err
	}
	return Fetched{Data: data, SHA: sha, CanonicalURL: canonical}, nil
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

func (c *GitHubClient) download(ctx context.Context, target string) ([]byte, error) {
	return c.get(ctx, target)
}

func (c *GitHubClient) get(ctx context.Context, target string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.client.Do(request)
	if err != nil {
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
