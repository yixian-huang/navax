package themes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

// SyncBuiltin 把内置主题经真实校验器编译后幂等落库。
//
// 内置主题走的是和第三方完全相同的管线，这是有意的：规范因此在每次启动
// （以及每次 CI）都被自家主题验证一遍，目录与实现不会悄悄漂移。代价是
// 启动多几十毫秒。
//
// 内置主题不合规属于构建缺陷，不是运行时状况，所以这里直接返回错误让启动
// 失败，而不是跳过并降级。
func SyncBuiltin(ctx context.Context, store *Store, now time.Time) error {
	packages, err := BuiltinPackages()
	if err != nil {
		return fmt.Errorf("load builtin themes: %w", err)
	}
	for _, pkg := range packages {
		compiled, compileErr := Compile(pkg, pkg.Manifest.ID)
		if compileErr != nil {
			return fmt.Errorf("compile builtin theme %s: %w", pkg.Manifest.ID, compileErr)
		}
		if _, upsertErr := store.UpsertVersion(ctx, pkg.Manifest.ID, compiled, "builtin", "builtin", now); upsertErr != nil {
			return fmt.Errorf("store builtin theme %s: %w", pkg.Manifest.ID, upsertErr)
		}
	}
	if err := store.EnsureDefaultTheme(ctx, now); err != nil {
		return err
	}
	return store.AssertDefaultThemeUsable(ctx)
}

// BaselineThemeID 是自愈时优先提升为默认的主题。它同时是前端保留的基线
// 令牌来源，所以指向它的结果最可预测。
const BaselineThemeID = "slate"

// EnsureDefaultTheme 修复不可用的默认主题，而不是让实例起不来。
//
// 这条路径来自一次真实事故（2026-07-23）：迁移 0013 停用了 culled 主题，
// 却没有把 is_default 从其中一个（mono）挪走。启动断言正确地发现了这个坏
// 状态，但当时的反应是拒绝启动，于是任何处于该状态的既有实例升级即挂。
//
// 它也覆盖一条与历史迁移无关的常驻风险：管理员在后台停用了当前默认主题，
// 重启后同样会踩到。
//
// 只有在完全没有可用目录主题时才返回错误——那才是真正无法自动恢复的状态。
func (s *Store) EnsureDefaultTheme(ctx context.Context, now time.Time) error {
	if err := s.AssertDefaultThemeUsable(ctx); err == nil {
		return nil
	}

	return database.WithinTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var replacement string
		// 优先基线主题；它不可用时退而求其次取任意可用目录主题，顺序固定
		// 以保证结果可预测。
		err := tx.QueryRowContext(ctx, `
			SELECT themes.id
			FROM themes `+EligibilityJoin+`
			WHERE themes.scope = 'catalog' AND `+EligibilityWhere+`
			ORDER BY CASE WHEN themes.id = ? THEN 0 ELSE 1 END, themes.id
			LIMIT 1`, "", BaselineThemeID).Scan(&replacement)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: no usable catalog theme to promote", ErrDefaultThemeUnavailable)
		}
		if err != nil {
			return err
		}

		var previous string
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(id, '') FROM themes WHERE is_default = 1`).Scan(&previous); err != nil &&
			!errors.Is(err, sql.ErrNoRows) {
			return err
		}

		// idx_themes_single_default 是部分唯一索引，旧默认必须先清掉。
		if _, err := tx.ExecContext(ctx,
			`UPDATE themes SET is_default = 0, updated_at = ? WHERE is_default = 1`, dbTime(now)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE themes SET is_default = 1, updated_at = ? WHERE id = ?`, dbTime(now), replacement); err != nil {
			return err
		}
		slog.WarnContext(ctx, "promoted a usable theme to default",
			"previous", previous, "replacement", replacement,
			"reason", "previous default theme had no active catalog version")
		return nil
	})
}

// AssertDefaultThemeUsable 校验回落目标本身可用。
//
// 没有这条，回落只是把问题推后一步：一个指向被停用主题的默认标记会让
// 每次发布都落到一个取不到样式的版本上。
func (s *Store) AssertDefaultThemeUsable(ctx context.Context) error {
	var (
		count     int
		themeID   string
		versionID string
	)
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM themes WHERE is_default = 1`).Scan(&count); err != nil {
		return fmt.Errorf("count default themes: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("%w: expected exactly one default theme, found %d", ErrDefaultThemeUnavailable, count)
	}
	err := s.db.QueryRowContext(ctx, `
		SELECT themes.id, theme_versions.id
		FROM themes
		JOIN theme_versions ON theme_versions.id = themes.current_version_id
		WHERE themes.is_default = 1
		  AND themes.enabled = 1
		  AND themes.scope = 'catalog'
		  AND theme_versions.theme_id = themes.id
		  AND theme_versions.status = ?`, VersionStatusActive).Scan(&themeID, &versionID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("%w: default theme has no active catalog version", ErrDefaultThemeUnavailable)
	case err != nil:
		return fmt.Errorf("%w: %v", ErrDefaultThemeUnavailable, err)
	}
	return nil
}
