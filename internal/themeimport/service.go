package themeimport

import (
	"context"
	"errors"
	"time"

	"github.com/yixian-huang/navax/internal/themes"
)

// Service 编排主题导入:解包 → 组包 → (店内)编译并落库。
type Service struct {
	store  *themes.Store
	github *GitHubClient
	quota  int
	now    func() time.Time
}

// NewService 构造导入编排服务。quota 是每用户私有主题配额上限,转发给
// themes.Store.InstallPrivate。
func NewService(store *themes.Store, github *GitHubClient, quota int) *Service {
	return &Service{store: store, github: github, quota: quota, now: time.Now}
}

// ValidationIssue 是 dry-run 校验的结构化错误。当前粒度为首个错误。
type ValidationIssue struct {
	Stage   string `json:"stage"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// classifyIssue 把管线错误映射到阶段,导入失败与 dry-run 共用同一份映射。
func classifyIssue(err error) ValidationIssue {
	issue := ValidationIssue{Message: err.Error()}
	switch {
	case errors.Is(err, themes.ErrInvalidArchive):
		issue.Stage = "archive"
	case errors.Is(err, themes.ErrInvalidManifest):
		issue.Stage = "manifest"
	case errors.Is(err, themes.ErrInvalidCSS):
		issue.Stage = "css"
		issue.Path = "theme.css"
	case errors.Is(err, themes.ErrInvalidAsset):
		issue.Stage = "asset"
	default:
		issue.Stage = "archive"
	}
	if issue.Stage == "manifest" {
		issue.Path = "theme.json"
	}
	return issue
}

// packageFromZip 解 zip 并组装 Package,zip 导入与 dry-run 共用这条路径。
func packageFromZip(zipData []byte) (themes.Package, error) {
	files, err := themes.ExtractZip(zipData)
	if err != nil {
		return themes.Package{}, err
	}
	return themes.PackageFromFiles(files)
}

// install 是 ImportZip/ImportGitHub 共用的落库步骤:compile 回调只做纯 CPU
// 的 themes.Compile,真正的网络拉取(GitHub tarball)必须在调用 install 之前
// 完成,不能挪进事务内的 compile 回调。
func (s *Service) install(ctx context.Context, ownerID string, pkg themes.Package, sourceType, sourceURL, sourceRef string) (themes.InstalledTheme, error) {
	return s.store.InstallPrivate(ctx, ownerID, pkg.Manifest.ID, sourceType, sourceURL, sourceRef, s.quota,
		func(themeID string) (themes.Compiled, error) { return themes.Compile(pkg, themeID) }, s.now().UTC())
}

// ImportZip 安装 zip 上传的主题包。source_ref 用上传内容的摘要标识来源。
func (s *Service) ImportZip(ctx context.Context, ownerID string, zipData []byte) (themes.InstalledTheme, error) {
	pkg, err := packageFromZip(zipData)
	if err != nil {
		return themes.InstalledTheme{}, err
	}
	return s.install(ctx, ownerID, pkg, "upload", "", themes.ContentDigest(zipData))
}

// ImportGitHub 拉取仓库 tarball 并安装;source_ref 锁定 commit sha。
// FetchTarball 在事务外发生(网络 I/O 不应占用数据库事务),随后的编译只是
// 纯 CPU 计算。
func (s *Service) ImportGitHub(ctx context.Context, ownerID, repoURL, ref string) (themes.InstalledTheme, error) {
	fetched, err := s.github.FetchTarball(ctx, repoURL, ref)
	if err != nil {
		return themes.InstalledTheme{}, err
	}
	files, err := themes.ExtractTarGz(fetched.Data)
	if err != nil {
		return themes.InstalledTheme{}, err
	}
	pkg, err := themes.PackageFromFiles(files)
	if err != nil {
		return themes.InstalledTheme{}, err
	}
	return s.install(ctx, ownerID, pkg, "github", fetched.CanonicalURL, fetched.SHA)
}

// UpdateStatus 是更新检查结果。upload 来源无 upstream,HasUpdate 恒 false。
type UpdateStatus struct {
	SourceType string `json:"sourceType"`
	HasUpdate  bool   `json:"hasUpdate"`
	CurrentSha string `json:"currentSha"`
	LatestSha  string `json:"latestSha"`
}

// CheckUpdate 检查某 owner 的私有主题有无 upstream 新版(只查不升)。
// 仅 github 来源做网络解析;upload 来源直接返回无更新。github 来源里,
// 通过 extraHosts 白名单(如自建 Gitea)导入的主题同样落 source_type =
// "github"(见 ImportGitHub),但那些主机没有 commits API、无法解析 HEAD——
// ResolveHeadSHA 用 ErrUpdateCheckUnsupported 表达这一点,这里接住并优雅
// 退化为「无法确认、视为无更新」,而不是把这种主机能力缺口报成错误。
func (s *Service) CheckUpdate(ctx context.Context, ownerID, themeID string) (UpdateStatus, error) {
	sourceType, sourceURL, currentRef, err := s.store.PrivateThemeSource(ctx, ownerID, themeID)
	if err != nil {
		return UpdateStatus{}, err
	}
	if sourceType != "github" {
		return UpdateStatus{SourceType: sourceType}, nil
	}
	latest, err := s.github.ResolveHeadSHA(ctx, sourceURL, "")
	if err != nil {
		if errors.Is(err, ErrUpdateCheckUnsupported) {
			return UpdateStatus{SourceType: "github", HasUpdate: false, CurrentSha: currentRef}, nil
		}
		return UpdateStatus{}, err
	}
	return UpdateStatus{SourceType: "github", HasUpdate: latest != currentRef, CurrentSha: currentRef, LatestSha: latest}, nil
}

// Uninstall 转发到 store(removed=false 表示墓碑)。
func (s *Service) Uninstall(ctx context.Context, ownerID, themeID string) (bool, error) {
	return s.store.UninstallPrivate(ctx, ownerID, themeID, s.now().UTC())
}

// ValidatePackage 对 zip 做 dry-run:走完整管线但不落库。空切片 = 合法。
func (s *Service) ValidatePackage(zipData []byte) []ValidationIssue {
	pkg, err := packageFromZip(zipData)
	if err != nil {
		return []ValidationIssue{classifyIssue(err)}
	}
	if _, err := themes.Compile(pkg, "validate-preview"); err != nil {
		return []ValidationIssue{classifyIssue(err)}
	}
	return nil
}
