package themes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/identity"
)

// ErrQuotaExceeded 表示该用户的私有主题数已达实例配额。
var ErrQuotaExceeded = errors.New("private theme quota exceeded")

// InstalledTheme 是一次导入(新装或升级)的结果。
type InstalledTheme struct {
	ThemeID   string
	Slug      string
	VersionID string
	Upgraded  bool
}

// InstallPrivate 安装或升级一个私有主题。themeID 在事务内确定(既有行复用、
// 新行用不透明 ULID),compile 回调据此产出以该 ID 为 CSS 作用域的编译产物——
// 作用域用行 ID 而不是 slug,两个用户同名主题天然无碰撞(设计 §7.2)。
// 同 owner 同 slug 重复导入即升级:不占新配额,切换 current 指针,已发布快照
// 因锁版本不受影响。
func (s *Store) InstallPrivate(ctx context.Context, ownerID, slug, sourceType, sourceURL, sourceRef string, quota int, compile func(themeID string) (Compiled, error), now time.Time) (InstalledTheme, error) {
	ownerID = strings.TrimSpace(ownerID)
	slug = strings.TrimSpace(slug)
	if ownerID == "" || slug == "" {
		return InstalledTheme{}, errors.New("themes: owner and slug are required")
	}
	var result InstalledTheme
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var themeID string
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM themes WHERE scope = 'private' AND owner_id = ? AND slug = ?`,
			ownerID, slug).Scan(&themeID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// 新装:配额按行数计(含墓碑——被历史发布引用的已卸载主题仍占位)。
			var owned int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM themes WHERE owner_id = ?`, ownerID).Scan(&owned); err != nil {
				return err
			}
			if owned >= quota {
				return ErrQuotaExceeded
			}
			themeID, err = identity.New("thm")
			if err != nil {
				return err
			}
			compiled, err := compile(themeID)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default,
				                    created_at, updated_at, slug, scope, owner_id, source_type, source_url, spec_version)
				VALUES (?, ?, ?, ?, ?, ?, '', 1, 0, ?, ?, ?, 'private', ?, ?, ?, ?)`,
				themeID, compiled.Manifest.Name, compiled.Manifest.Version, compiled.Manifest.Author,
				compiled.Manifest.Description, compiled.Manifest.Mode, dbTime(now), dbTime(now),
				slug, ownerID, sourceType, sourceURL, compiled.Manifest.SpecVersion); err != nil {
				return fmt.Errorf("insert private theme: %w", err)
			}
			versionID, err := upsertVersionTx(ctx, tx, themeID, compiled, sourceType, sourceRef, now)
			if err != nil {
				return err
			}
			result = InstalledTheme{ThemeID: themeID, Slug: slug, VersionID: versionID}
			return nil
		case err != nil:
			return err
		default:
			// 升级:同一行换版本。source_url 跟随本次来源(github 重新拉取会刷新)。
			compiled, err := compile(themeID)
			if err != nil {
				return err
			}
			versionID, err := upsertVersionTx(ctx, tx, themeID, compiled, sourceType, sourceRef, now)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE themes SET source_url = ?, updated_at = ? WHERE id = ?`,
				sourceURL, dbTime(now), themeID); err != nil {
				return err
			}
			result = InstalledTheme{ThemeID: themeID, Slug: slug, VersionID: versionID, Upgraded: true}
			return nil
		}
	})
	if err != nil {
		return InstalledTheme{}, err
	}
	return result, nil
}
