package maintenance

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/auth"
	"github.com/yixian-huang/navax/internal/database"
	_ "modernc.org/sqlite"
)

func TestCreateBackupProducesValidSQLite(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service, err := NewBackupService(db, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	backup, err := service.Create(ctx, "manual", "")
	if err != nil {
		t.Fatal(err)
	}
	if backup.Size == 0 || len(backup.SHA256) != 64 {
		t.Fatalf("backup = %+v", backup)
	}
	path, err := service.Path(ctx, backup.ID)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup permissions = %v, %v", info.Mode().Perm(), err)
	}
	copyDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var integrity string
	if err := copyDB.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q, %v", integrity, err)
	}
	var migrationCount int
	if err := copyDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil || migrationCount != 3 {
		t.Fatalf("migration count = %d, %v", migrationCount, err)
	}
}

func TestRestoreUsesOneTimeTokenAndStartupSwap(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	databasePath := filepath.Join(root, "navax.db")
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: databasePath, MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	authService := auth.NewService(auth.NewSQLStore(db), "01234567890123456789012345678901", time.Hour)
	session, _, err := authService.Bootstrap(ctx, "01234567890123456789012345678901", auth.BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "strong password",
		InstanceName: "before-restore", PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewBackupService(db, filepath.Join(root, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ConfigureRestore(databasePath); err != nil {
		t.Fatal(err)
	}
	backup, err := service.Create(ctx, "manual", session.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE system_settings SET instance_name = 'after-backup' WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	token, err := service.CreateRestoreToken(ctx, backup.ID, session.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.StageRestore(ctx, backup.ID, session.User.ID, token.Token); err != nil {
		t.Fatal(err)
	}
	if err := service.StageRestore(ctx, backup.ID, session.User.ID, token.Token); !errors.Is(err, ErrRestoreToken) {
		t.Fatalf("reused restore token error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	swap, err := ApplyPendingRestore(databasePath)
	if err != nil || !swap.Applied() {
		t.Fatalf("ApplyPendingRestore() = %+v, %v", swap, err)
	}
	restored, err := database.OpenAndMigrate(ctx, database.Config{Path: databasePath, MaxOpenConns: 1})
	if err != nil {
		_ = swap.Rollback()
		t.Fatal(err)
	}
	defer restored.Close()
	var instanceName string
	if err := restored.QueryRowContext(ctx, "SELECT instance_name FROM system_settings WHERE id = 1").Scan(&instanceName); err != nil {
		t.Fatal(err)
	}
	if instanceName != "before-restore" {
		t.Fatalf("restored instance name = %q", instanceName)
	}
	if err := swap.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestInstanceArchiveRestoresDatabaseAssetsAndKeys(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	databasePath := filepath.Join(root, "navax.db")
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: databasePath, MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	authService := auth.NewService(auth.NewSQLStore(db), "01234567890123456789012345678901", time.Hour)
	session, _, err := authService.Bootstrap(ctx, "01234567890123456789012345678901", auth.BootstrapInput{
		Username: "owner", Email: "owner@example.com", Password: "strong password",
		InstanceName: "archive-source", PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "assets", "usr_owner"), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, value := range map[string]string{
		filepath.Join(root, "assets", "usr_owner", "logo.png"): "asset-before",
		filepath.Join(root, "master.key"):                      "master-before",
		filepath.Join(root, "analytics.key"):                   "analytics-before",
	} {
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	service, err := NewBackupService(db, filepath.Join(root, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ConfigureRestore(databasePath); err != nil {
		t.Fatal(err)
	}
	backup, err := service.Create(ctx, "manual", session.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	archivePath, err := service.Path(ctx, backup.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(archivePath) != ".navbak" {
		t.Fatalf("archive path = %q", archivePath)
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool)
	for _, file := range archive.File {
		names[file.Name] = true
	}
	_ = archive.Close()
	for _, name := range []string{"manifest.json", "navax.db", "master.key", "analytics.key", "assets/usr_owner/logo.png"} {
		if !names[name] {
			t.Fatalf("archive missing %q", name)
		}
	}
	if _, err := db.ExecContext(ctx, "UPDATE system_settings SET instance_name = 'changed' WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "usr_owner", "logo.png"), []byte("asset-after"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "master.key"), []byte("master-after"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := service.CreateRestoreToken(ctx, backup.ID, session.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.StageRestore(ctx, backup.ID, session.User.ID, token.Token); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	swap, err := ApplyPendingRestore(databasePath)
	if err != nil || !swap.Applied() {
		t.Fatalf("ApplyPendingRestore() = %+v, %v", swap, err)
	}
	for path, expected := range map[string]string{
		filepath.Join(root, "assets", "usr_owner", "logo.png"): "asset-before",
		filepath.Join(root, "master.key"):                      "master-before",
		filepath.Join(root, "analytics.key"):                   "analytics-before",
	} {
		actual, err := os.ReadFile(path)
		if err != nil || string(actual) != expected {
			t.Fatalf("restored %s = %q, %v", path, actual, err)
		}
	}
	restored, err := database.OpenAndMigrate(ctx, database.Config{Path: databasePath, MaxOpenConns: 1})
	if err != nil {
		_ = swap.Rollback()
		t.Fatal(err)
	}
	defer restored.Close()
	var instanceName string
	if err := restored.QueryRowContext(ctx, "SELECT instance_name FROM system_settings WHERE id = 1").Scan(&instanceName); err != nil {
		t.Fatal(err)
	}
	if instanceName != "archive-source" {
		t.Fatalf("restored instance name = %q", instanceName)
	}
	if err := swap.Commit(); err != nil {
		t.Fatal(err)
	}
}
