package themes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
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
	return store.AssertDefaultThemeUsable(ctx)
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
