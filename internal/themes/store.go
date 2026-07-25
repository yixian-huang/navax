package themes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

// 版本状态。端点据此区分 404（从未存在）与 410（曾存在、已撤销），
// 因此这两个值必须由一处定义，不能在存储层和 handler 里各写一份字面量。
const (
	VersionStatusActive   = "active"
	VersionStatusDisabled = "disabled"
)

// ErrNotFound 表示请求的主题版本或资产不存在。
var ErrNotFound = errors.New("theme resource not found")

// ErrDefaultThemeUnavailable 表示回落目标本身不可用（默认主题缺失、被停用，
// 或没有 active 的当前版本）。回落链的终点断了必须明确报错，不能静默产出一个
// 引用空版本的快照——调用方据此返回 503 而不是 404。
var ErrDefaultThemeUnavailable = errors.New("default theme has no active version")

// themeIDAliases 把已下架的一方主题映射到最接近的保留主题。
// 前端旧实现（themeResolve.ts，已随主题规范 v1 删除）曾在浏览器里做同样的
// 映射；现在解析统一收敛到服务端这一处，前端不再自行兜底。
var themeIDAliases = map[string]string{
	"kyoto":      "slate",
	"terracotta": "slate",
	"mono":       "slate",
	"mochi":      "sakura",
	"pastelsky":  "sakura",
	"cyber":      "orbit",
}

var allowedSourceTypes = map[string]bool{"builtin": true, "github": true, "upload": true}

// Store 持久化不可变的主题版本与其资产。
type Store struct{ db *sql.DB }

// NewStore 返回一个基于给定数据库的主题版本存储。
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// UpsertVersion 幂等地写入一个编译产物，并把它设为该主题的当前版本。
//
// 幂等键是 (theme_id, content_hash)：同一份包重复编译写入不会产生新行，因此
// 内置主题可以在每次启动时无条件 upsert。资产只在真正新插入版本时写入——版本
// 内容寻址且不可变，已存在的版本其资产必然已经落库。
func (s *Store) UpsertVersion(ctx context.Context, packageID string, compiled Compiled, sourceType, sourceRef string, now time.Time) (string, error) {
	var versionID string
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		id, err := upsertVersionTx(ctx, tx, packageID, compiled, sourceType, sourceRef, "", now)
		if err != nil {
			return err
		}
		versionID = id
		return nil
	})
	if err != nil {
		return "", err
	}
	return versionID, nil
}

// upsertVersionTx 是 UpsertVersion 的事务体，未导出以便 InstallPrivate 之类的
// 调用方在同一事务里复用（避免嵌套事务或先提交再补写导致的中间状态可见）。
//
// importerID 为空串时写 NULL，表示该版本不由某个特定用户导入（例如内置主题）。
//
// 入参校验也放在这里而不是公开包装函数 UpsertVersion：InstallPrivate 直接调用
// 本函数，不经过 UpsertVersion，校验若只留在包装函数里，私有安装路径上一个
// 坏 sourceType 就会绕过检查、一路撞到 SQL 层变成一次不可读的 500，而不是
// 这里给出的域错误。
func upsertVersionTx(ctx context.Context, tx *sql.Tx, packageID string, compiled Compiled, sourceType, sourceRef, importerID string, now time.Time) (string, error) {
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		return "", errors.New("themes: package id is empty")
	}
	if compiled.VersionID == "" || compiled.ContentHash == "" {
		return "", errors.New("themes: compiled package is missing version id or content hash")
	}
	if !allowedSourceTypes[sourceType] {
		return "", fmt.Errorf("themes: unsupported source type %q", sourceType)
	}

	manifestJSON, err := json.Marshal(compiled.Manifest)
	if err != nil {
		return "", fmt.Errorf("themes: marshal manifest: %w", err)
	}

	var exists int
	switch err := tx.QueryRowContext(ctx, `SELECT 1 FROM themes WHERE id = ?`, packageID).Scan(&exists); {
	case errors.Is(err, sql.ErrNoRows):
		return "", fmt.Errorf("%w: theme %q", ErrNotFound, packageID)
	case err != nil:
		return "", err
	}

	// 显式指定冲突目标：只有内容重复才是「已存在」。主键冲突（同一 id 落在
	// 另一个 theme_id 上）应当报错而不是被静默吞掉。
	result, err := tx.ExecContext(ctx, `
		INSERT INTO theme_versions(
			id, theme_id, version, source_ref, manifest_json,
			compiled_css, content_hash, status, imported_by, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)
		ON CONFLICT (theme_id, content_hash) DO NOTHING`,
		compiled.VersionID, packageID, compiled.Manifest.Version, sourceRef, string(manifestJSON),
		compiled.CSS, compiled.ContentHash, sql.NullString{String: importerID, Valid: importerID != ""}, dbTime(now))
	if err != nil {
		return "", err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return "", err
	}

	var versionID, status string
	if err := tx.QueryRowContext(ctx,
		`SELECT id, status FROM theme_versions WHERE theme_id = ? AND content_hash = ?`,
		packageID, compiled.ContentHash,
	).Scan(&versionID, &status); err != nil {
		return "", err
	}

	if inserted > 0 {
		if err := insertVersionAssets(ctx, tx, versionID, compiled.Assets); err != nil {
			return "", err
		}
	}

	// 撤销过的版本不因一次重新导入而复活：撤销是运维动作，静默回滚它会让
	// kill switch 形同虚设。触发器也会拦，但这里给出可读的原因。
	if status != VersionStatusActive {
		return "", fmt.Errorf("themes: version %s of theme %s is %s and cannot become current", versionID, packageID, status)
	}

	// 回写 manifest 展示元数据：themes 行的这些列否则永远停留在迁移种子值，
	// 列表 API 会吐与实际包不符的文案（内置与第三方同样受益）。
	preview := ""
	for _, asset := range compiled.Assets {
		if asset.Path == "preview.png" {
			preview = AssetBasePath(compiled.VersionID) + "preview.png"
			break
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE themes SET current_version_id = ?, source_type = ?, name = ?, author = ?, description = ?,
		       mode = ?, version = ?, preview = ?, spec_version = ?, updated_at = ?
		WHERE id = ?`,
		versionID, sourceType, compiled.Manifest.Name, compiled.Manifest.Author, compiled.Manifest.Description,
		compiled.Manifest.Mode, compiled.Manifest.Version, preview, compiled.Manifest.SpecVersion,
		dbTime(now), packageID); err != nil {
		return "", fmt.Errorf("update theme pointer: %w", err)
	}
	return versionID, nil
}

func insertVersionAssets(ctx context.Context, tx *sql.Tx, versionID string, assets []Asset) error {
	if len(assets) == 0 {
		return nil
	}
	statement, err := tx.PrepareContext(ctx, `
		INSERT INTO theme_assets(id, theme_version_id, path, mime, bytes, sha256, data)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = statement.Close() }()

	for _, asset := range assets {
		// 资产 ID 由版本 ID 与路径派生，因此重放同一版本不会产生新的随机主键。
		assetID := assetRowID(versionID, asset)
		if _, err := statement.ExecContext(ctx, assetID, versionID, asset.Path, asset.MIME,
			len(asset.Data), asset.SHA256, asset.Data); err != nil {
			return err
		}
	}
	return nil
}

func assetRowID(versionID string, asset Asset) string {
	return "thas_" + versionID + "_" + asset.SHA256[:16]
}

// VersionCSS 返回某个版本的编译 CSS、内容哈希与状态。
//
// status 一并返回，让端点能区分 404（从未存在）与 410（曾存在、已撤销）。
func (s *Store) VersionCSS(ctx context.Context, versionID string) ([]byte, string, string, error) {
	var (
		css         []byte
		contentHash string
		status      string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT compiled_css, content_hash, status FROM theme_versions WHERE id = ?`,
		versionID,
	).Scan(&css, &contentHash, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", "", fmt.Errorf("%w: theme version %q", ErrNotFound, versionID)
	}
	if err != nil {
		return nil, "", "", err
	}
	return css, contentHash, status, nil
}

// VersionAsset 按包内路径返回某个版本的资产。
//
// 只接受精确登记的路径：不做任何归一化或前缀匹配，越界路径自然落空。
func (s *Store) VersionAsset(ctx context.Context, versionID, path string) (Asset, error) {
	asset := Asset{Path: path}
	err := s.db.QueryRowContext(ctx,
		`SELECT mime, sha256, data FROM theme_assets WHERE theme_version_id = ? AND path = ?`,
		versionID, path,
	).Scan(&asset.MIME, &asset.SHA256, &asset.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, fmt.Errorf("%w: asset %q of version %q", ErrNotFound, path, versionID)
	}
	if err != nil {
		return Asset{}, err
	}
	return asset, nil
}

func dbTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

// 可用性谓词（eligibility）。列表、选择、预览、发布必须复用同一份判定，
// 各写一份 SQL 就会出现「选择器里能选中、发布时却静默回落」这种不一致。
//
// 三条缺一不可：
//   - themes.enabled = 1 —— 同时作用于目录主题与私有主题。卸载私有主题
//     正是通过 enabled = 0 实现的，私有分支漏掉这条，已卸载的主题会继续
//     出现在选择器里。
//   - 当前版本必须属于本主题且处于 active。
//   - 目录主题人人可用；私有主题只有归属者可用。
const (
	// EligibilityJoin 把主题与它的当前版本连起来。
	EligibilityJoin = `JOIN theme_versions
		ON theme_versions.id = themes.current_version_id
		AND theme_versions.theme_id = themes.id`

	// EligibilityWhere 需要绑定一个参数：调用方的用户 ID。
	// 匿名调用传空串——私有主题的 owner_id 非空，因此永不匹配。
	EligibilityWhere = `themes.enabled = 1
		AND theme_versions.status = 'active'
		AND (themes.scope = 'catalog' OR (themes.scope = 'private' AND themes.owner_id = ?))`
)

// ResolveEligibleVersion 解析出某个主体可用的主题版本。
//
// 谓词不成立时回落默认主题——而不是报错。用户不该因为别人下架了一个主题
// 就被卡住无法发布。但如果**默认主题自己**也不可用，那是实例级故障，必须
// 响亮失败，否则会产出一个引用空版本的快照。
// Queryer 让解析既能在 *sql.DB 上跑，也能在事务里跑。发布必须走后者：
// 解析与写快照之间若隔着事务边界，主题在这个空档被撤销就会留下一条指向
// 已撤销版本的快照，公开页从此拿不到样式。
type Queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ResolveEligibleVersion 在给定的 Queryer 上解析可用版本。
func ResolveEligibleVersion(ctx context.Context, q Queryer, themeID, actorID string) (string, error) {
	candidates := []string{strings.TrimSpace(themeID)}
	if alias, ok := themeIDAliases[strings.TrimSpace(themeID)]; ok {
		candidates = append(candidates, alias)
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		var versionID string
		err := q.QueryRowContext(ctx, `
			SELECT themes.current_version_id
			FROM themes `+EligibilityJoin+`
			WHERE themes.id = ? AND `+EligibilityWhere, candidate, actorID).Scan(&versionID)
		switch {
		case err == nil:
			return versionID, nil
		case errors.Is(err, sql.ErrNoRows):
			continue
		default:
			return "", err
		}
	}

	var fallback string
	err := q.QueryRowContext(ctx, `
		SELECT themes.current_version_id
		FROM themes `+EligibilityJoin+`
		WHERE themes.is_default = 1 AND `+EligibilityWhere, "").Scan(&fallback)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrDefaultThemeUnavailable
	}
	if err != nil {
		return "", err
	}
	return fallback, nil
}

// ResolveEligibleVersion 是实例方法形式，供不在事务中的调用方使用。
func (s *Store) ResolveEligibleVersion(ctx context.Context, themeID, actorID string) (string, error) {
	return ResolveEligibleVersion(ctx, s.db, themeID, actorID)
}
