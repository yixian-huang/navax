package themes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
				// 配额满:先回收无快照引用的墓碑(它们对 owner 不可见却占位),再重算。
				if _, err := reclaimTombstones(ctx, tx, ownerID, now); err != nil {
					return err
				}
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM themes WHERE owner_id = ?`, ownerID).Scan(&owned); err != nil {
					return err
				}
				if owned >= quota {
					return ErrQuotaExceeded
				}
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
			versionID, err := upsertVersionTx(ctx, tx, themeID, compiled, sourceType, sourceRef, ownerID, now)
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
			versionID, err := upsertVersionTx(ctx, tx, themeID, compiled, sourceType, sourceRef, ownerID, now)
			if err != nil {
				return err
			}
			// enabled = 1：重装同 slug 等同于重新安装，必须能唤醒一个先前被
			// UninstallPrivate 墓碑化(enabled = 0)的行，否则用户会永远卡在
			// 一个自己看不到、也无法再次启用的主题上。
			if _, err := tx.ExecContext(ctx, `UPDATE themes SET source_url = ?, enabled = 1, updated_at = ? WHERE id = ?`,
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

// UninstallPrivate 卸载私有主题。无任何已发布快照引用其版本时物理删除
// (配额立即释放);仍被引用时转墓碑(enabled=0):版本与资产保留,公开页
// 继续可用——撤销语义见设计 §8.1.1。不存在与非本人统一返回 ErrNotFound
// (不区分,防止探测他人主题是否存在)。
func (s *Store) UninstallPrivate(ctx context.Context, ownerID, themeID string, now time.Time) (bool, error) {
	var removed bool
	err := database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var one int
		err := tx.QueryRowContext(ctx, `
			SELECT 1 FROM themes WHERE id = ? AND scope = 'private' AND owner_id = ?`,
			themeID, ownerID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var refs int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM published_snapshots
			WHERE theme_version_id IN (SELECT id FROM theme_versions WHERE theme_id = ?)`,
			themeID).Scan(&refs); err != nil {
			return err
		}
		if refs > 0 {
			_, err := tx.ExecContext(ctx, `UPDATE themes SET enabled = 0, updated_at = ? WHERE id = ?`, dbTime(now), themeID)
			return err
		}
		// 物理删除:无快照引用,可安全回收。
		if err := deleteThemeTx(ctx, tx, themeID, now); err != nil {
			return err
		}
		removed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return removed, nil
}

// deleteThemeTx 物理删除单个主题:先摘 current 指针(theme_versions_current_guard
// 才放行删版本),再删版本与行。调用方需已确认该主题无 published_snapshots 引用。
func deleteThemeTx(ctx context.Context, tx *sql.Tx, themeID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `UPDATE themes SET current_version_id = NULL, updated_at = ? WHERE id = ?`, dbTime(now), themeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM theme_versions WHERE theme_id = ?`, themeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM themes WHERE id = ?`, themeID); err != nil {
		return err
	}
	return nil
}

// reclaimTombstones 回收该 owner 的可回收墓碑:scope=private、enabled=0、其全部
// 版本都不再被任何 published_snapshots 引用、且没有任何版本带 kill-switch
// 停用记忆(status='disabled')。返回回收数量。墓碑对 owner 不可见却占配额,
// 回收在配额压力时自动发生。
func reclaimTombstones(ctx context.Context, tx *sql.Tx, ownerID string, now time.Time) (int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM themes WHERE scope = 'private' AND owner_id = ? AND enabled = 0`, ownerID)
	if err != nil {
		return 0, err
	}
	var tombstones []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		tombstones = append(tombstones, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	reclaimed := 0
	for _, themeID := range tombstones {
		var refs int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM published_snapshots
			WHERE theme_version_id IN (SELECT id FROM theme_versions WHERE theme_id = ?)`,
			themeID).Scan(&refs); err != nil {
			return 0, err
		}
		if refs > 0 {
			continue
		}
		// 任一版本曾被管理员 kill-switch 停用(status='disabled'),说明该墓碑
		// 承载着一段管理判断("这版本有问题")。物理删除会连它的版本行一起
		// 抹掉,记忆也随之消失——如果同名 slug 未来被重装,新行拿到新 ULID,
		// 不会继承任何"这曾经被停用过"的痕迹。配额压力下宁可不回收,也不
		// 丢掉这条记忆。
		var disabledVersions int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM theme_versions WHERE theme_id = ? AND status = ?`,
			themeID, VersionStatusDisabled).Scan(&disabledVersions); err != nil {
			return 0, err
		}
		if disabledVersions > 0 {
			continue
		}
		if err := deleteThemeTx(ctx, tx, themeID, now); err != nil {
			return 0, err
		}
		slog.WarnContext(ctx, "reclaiming unreferenced private tombstone",
			"themeID", themeID, "ownerID", ownerID)
		reclaimed++
	}
	return reclaimed, nil
}
