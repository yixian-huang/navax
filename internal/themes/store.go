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
// 与 web/src/lib/themeResolve.ts 的 THEME_ID_ALIASES 保持一致：解析统一收敛到
// 服务端一处，前端不再自行兜底。
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

	versionID := ""
	err = database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var exists int
		switch err := tx.QueryRowContext(ctx, `SELECT 1 FROM themes WHERE id = ?`, packageID).Scan(&exists); {
		case errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("%w: theme %q", ErrNotFound, packageID)
		case err != nil:
			return err
		}

		// 显式指定冲突目标：只有内容重复才是「已存在」。主键冲突（同一 id 落在
		// 另一个 theme_id 上）应当报错而不是被静默吞掉。
		result, err := tx.ExecContext(ctx, `
			INSERT INTO theme_versions(
				id, theme_id, version, source_ref, manifest_json,
				compiled_css, content_hash, status, imported_by, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'active', NULL, ?)
			ON CONFLICT (theme_id, content_hash) DO NOTHING`,
			compiled.VersionID, packageID, compiled.Manifest.Version, sourceRef, string(manifestJSON),
			compiled.CSS, compiled.ContentHash, dbTime(now))
		if err != nil {
			return err
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return err
		}

		var status string
		if err := tx.QueryRowContext(ctx,
			`SELECT id, status FROM theme_versions WHERE theme_id = ? AND content_hash = ?`,
			packageID, compiled.ContentHash,
		).Scan(&versionID, &status); err != nil {
			return err
		}

		if inserted > 0 {
			if err := insertVersionAssets(ctx, tx, versionID, compiled.Assets); err != nil {
				return err
			}
		}

		// 撤销过的版本不因一次重新导入而复活：撤销是运维动作，静默回滚它会让
		// kill switch 形同虚设。触发器也会拦，但这里给出可读的原因。
		if status != VersionStatusActive {
			return fmt.Errorf("themes: version %s of theme %s is %s and cannot become current", versionID, packageID, status)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE themes
			SET current_version_id = ?, source_type = ?, updated_at = ?
			WHERE id = ?`,
			versionID, sourceType, dbTime(now), packageID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
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

// ResolvePackageVersion 把一个 themeId 解析成可下发的版本 ID。
//
// 顺序：直接命中 → 别名表 → 默认主题。未知、已下架、当前版本被撤销的主题都
// 走到回落，因此调用方永远拿到一个可用版本或一个明确的错误，不会拿到空串。
// 注意这里只做包级可用性判定；私有主题的归属判定（eligible 谓词的 actor 分支）
// 属于调用方。
func (s *Store) ResolvePackageVersion(ctx context.Context, themeID string) (string, error) {
	candidates := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(themeID); trimmed != "" {
		candidates = append(candidates, trimmed)
		if alias, ok := themeIDAliases[trimmed]; ok {
			candidates = append(candidates, alias)
		}
	}
	for _, candidate := range candidates {
		versionID, err := s.serviceableVersion(ctx, `themes.id = ?`, candidate)
		if err != nil {
			return "", err
		}
		if versionID != "" {
			return versionID, nil
		}
	}

	versionID, err := s.serviceableVersion(ctx, `themes.is_default = 1`)
	if err != nil {
		return "", err
	}
	if versionID == "" {
		return "", ErrDefaultThemeUnavailable
	}
	return versionID, nil
}

// serviceableVersion 返回匹配主题的当前版本 ID，不可下发时返回空串。
// 「可下发」= 主题启用 + 有当前版本 + 该版本仍是 active。
func (s *Store) serviceableVersion(ctx context.Context, condition string, args ...any) (string, error) {
	var versionID string
	err := s.db.QueryRowContext(ctx, `
		SELECT themes.current_version_id
		FROM themes
		JOIN theme_versions ON theme_versions.id = themes.current_version_id
		WHERE `+condition+`
		  AND themes.enabled = 1
		  AND theme_versions.status = 'active'
		LIMIT 1`, args...).Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return versionID, nil
}

func dbTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
