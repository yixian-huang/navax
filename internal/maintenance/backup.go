package maintenance

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
	"github.com/yixian-huang/navax/internal/security"
	_ "modernc.org/sqlite"
)

var (
	ErrRestoreNotConfigured = errors.New("backup restore is not configured")
	ErrRestoreToken         = errors.New("restore token is invalid or expired")
	ErrBackupInvalid        = errors.New("backup file failed validation")
)

type Backup struct {
	ID        string    `json:"id"`
	Reason    string    `json:"reason"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"createdAt"`
	path      string
}

type BackupService struct {
	db            *sql.DB
	directory     string
	databasePath  string
	dataDirectory string
	restoreMu     sync.Mutex
}

func NewBackupService(db *sql.DB, directory string) (*BackupService, error) {
	if db == nil {
		return nil, errors.New("backup database is required")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve backup directory: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.Chmod(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("secure backup directory: %w", err)
	}
	return &BackupService{db: db, directory: absolute}, nil
}

// ConfigureRestore selects the live database path that will be replaced on
// the next process start after an administrator stages a restore.
func (s *BackupService) ConfigureRestore(databasePath string) error {
	absolute, err := filepath.Abs(databasePath)
	if err != nil {
		return fmt.Errorf("resolve live database path: %w", err)
	}
	if absolute == "" || absolute == s.directory {
		return ErrRestoreNotConfigured
	}
	s.databasePath = absolute
	s.dataDirectory = filepath.Dir(absolute)
	return nil
}

func (s *BackupService) Create(ctx context.Context, reason, createdBy string) (Backup, error) {
	if reason != "manual" && reason != "pre-update" && reason != "scheduled" {
		return Backup{}, errors.New("invalid backup reason")
	}
	id, err := identity.New("bak")
	if err != nil {
		return Backup{}, err
	}
	databaseSnapshot := filepath.Join(s.directory, "."+id+".sqlite3")
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO "+sqliteString(databaseSnapshot)); err != nil {
		return Backup{}, fmt.Errorf("create SQLite backup: %w", err)
	}
	defer os.Remove(databaseSnapshot)
	path := filepath.Join(s.directory, id+".sqlite3")
	if s.dataDirectory != "" {
		path = filepath.Join(s.directory, id+".navbak")
		if err := createInstanceArchive(path, databaseSnapshot, s.dataDirectory, time.Now().UTC()); err != nil {
			return Backup{}, err
		}
	} else if err := os.Rename(databaseSnapshot, path); err != nil {
		return Backup{}, fmt.Errorf("commit SQLite backup: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(path)
		}
	}()
	if err := os.Chmod(path, 0o600); err != nil {
		return Backup{}, fmt.Errorf("secure backup file: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return Backup{}, err
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(hasher, file)
	closeErr := file.Close()
	if copyErr != nil {
		return Backup{}, fmt.Errorf("hash backup: %w", copyErr)
	}
	if closeErr != nil {
		return Backup{}, fmt.Errorf("close backup: %w", closeErr)
	}
	now := time.Now().UTC()
	digest := hex.EncodeToString(hasher.Sum(nil))
	var actor any
	if createdBy != "" {
		actor = createdBy
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO backups(id, reason, path, size_bytes, sha256, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, reason, path, size, digest, actor, now.Format(time.RFC3339Nano)); err != nil {
		return Backup{}, fmt.Errorf("record backup: %w", err)
	}
	cleanup = false
	return Backup{ID: id, Reason: reason, Size: size, SHA256: digest, CreatedAt: now, path: path}, nil
}

func (s *BackupService) List(ctx context.Context) ([]Backup, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, reason, path, size_bytes, sha256, created_at FROM backups ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	backups := make([]Backup, 0)
	for rows.Next() {
		var backup Backup
		var createdAt string
		if err := rows.Scan(&backup.ID, &backup.Reason, &backup.path, &backup.Size, &backup.SHA256, &createdAt); err != nil {
			return nil, err
		}
		backup.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		backups = append(backups, backup)
	}
	return backups, rows.Err()
}

func (s *BackupService) Path(ctx context.Context, id string) (string, error) {
	var path string
	if err := s.db.QueryRowContext(ctx, "SELECT path FROM backups WHERE id = ?", id).Scan(&path); err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(s.directory, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("backup path escapes backup directory")
	}
	if _, err := os.Stat(absolute); err != nil {
		return "", err
	}
	return absolute, nil
}

type RestoreToken struct {
	Token     string    `json:"restoreToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (s *BackupService) CreateRestoreToken(ctx context.Context, backupID, userID string) (RestoreToken, error) {
	if _, err := s.Path(ctx, backupID); err != nil {
		return RestoreToken{}, err
	}
	token, tokenHash, err := security.NewToken()
	if err != nil {
		return RestoreToken{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(10 * time.Minute)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO restore_tokens(token_hash, backup_id, user_id, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`, tokenHash, backupID, userID,
		expiresAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return RestoreToken{}, err
	}
	return RestoreToken{Token: token, ExpiresAt: expiresAt}, nil
}

// StageRestore validates the one-time token and backup, then writes a pending
// database beside the live database. The process must restart to apply it.
func (s *BackupService) StageRestore(ctx context.Context, backupID, userID, token string) error {
	if s.databasePath == "" {
		return ErrRestoreNotConfigured
	}
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()

	tokenHash := security.HashToken(token)
	var path, expectedSHA, expiresAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT b.path, b.sha256, rt.expires_at
		FROM restore_tokens rt JOIN backups b ON b.id = rt.backup_id
		WHERE rt.token_hash = ? AND rt.backup_id = ? AND rt.user_id = ? AND rt.used_at IS NULL`,
		tokenHash, backupID, userID,
	).Scan(&path, &expectedSHA, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRestoreToken
	}
	if err != nil {
		return err
	}
	expires, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil || !expires.After(time.Now().UTC()) {
		return ErrRestoreToken
	}
	if err := validateBackup(path, expectedSHA); err != nil {
		return err
	}
	if strings.EqualFold(filepath.Ext(path), ".navbak") {
		if err := stageInstanceArchive(path, s.dataDirectory); err != nil {
			return fmt.Errorf("stage instance restore: %w", err)
		}
	} else {
		pending := s.databasePath + ".restore-pending"
		if err := copyFileAtomic(path, pending); err != nil {
			return fmt.Errorf("stage restore: %w", err)
		}
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE restore_tokens SET used_at = ?
		WHERE token_hash = ? AND backup_id = ? AND user_id = ? AND used_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339Nano), tokenHash, backupID, userID)
	if err != nil {
		removePendingRestore(s.dataDirectory, s.databasePath)
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		removePendingRestore(s.dataDirectory, s.databasePath)
		return ErrRestoreToken
	}
	return nil
}

type RestoreSwap struct {
	components []restoreComponent
	applied    bool
}

type restoreComponent struct {
	path        string
	previous    string
	hadPrevious bool
}

// ApplyPendingRestore swaps a previously staged database before SQLite opens.
// Call Commit after migrations succeed, or Rollback if startup validation fails.
func ApplyPendingRestore(databasePath string) (*RestoreSwap, error) {
	absolute, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, err
	}
	swap := &RestoreSwap{}
	root := filepath.Dir(absolute)
	paths := []string{absolute, filepath.Join(root, "assets"), filepath.Join(root, "master.key"), filepath.Join(root, "analytics.key")}
	for _, path := range paths {
		pending := path + ".restore-pending"
		if _, err := os.Stat(pending); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			_ = swap.Rollback()
			return nil, err
		}
		component := restoreComponent{path: path, previous: path + ".pre-restore"}
		_ = os.RemoveAll(component.previous)
		if _, err := os.Stat(path); err == nil {
			component.hadPrevious = true
			if err := os.Rename(path, component.previous); err != nil {
				_ = swap.Rollback()
				return nil, fmt.Errorf("preserve pre-restore component %s: %w", filepath.Base(path), err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			_ = swap.Rollback()
			return nil, err
		}
		if path == absolute {
			_ = os.Remove(absolute + "-wal")
			_ = os.Remove(absolute + "-shm")
		}
		if err := os.Rename(pending, path); err != nil {
			if component.hadPrevious {
				_ = os.Rename(component.previous, component.path)
			}
			_ = swap.Rollback()
			return nil, fmt.Errorf("apply pending component %s: %w", filepath.Base(path), err)
		}
		swap.components = append(swap.components, component)
		swap.applied = true
	}
	if swap.applied {
		if err := syncDirectory(root); err != nil {
			_ = swap.Rollback()
			return nil, err
		}
	}
	return swap, nil
}

func (s *RestoreSwap) Applied() bool { return s != nil && s.applied }

func (s *RestoreSwap) Commit() error {
	if !s.Applied() {
		return nil
	}
	for _, component := range s.components {
		if err := os.RemoveAll(component.previous); err != nil {
			return err
		}
	}
	return nil
}

func (s *RestoreSwap) Rollback() error {
	if !s.Applied() {
		return nil
	}
	var firstErr error
	for index := len(s.components) - 1; index >= 0; index-- {
		component := s.components[index]
		_ = os.RemoveAll(component.path)
		if component.hadPrevious {
			if err := os.Rename(component.previous, component.path); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	s.applied = false
	return firstErr
}

func validateBackup(path, expectedSHA string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), expectedSHA) {
		return ErrBackupInvalid
	}
	if strings.EqualFold(filepath.Ext(path), ".navbak") {
		return validateInstanceArchive(path)
	}
	return validateSQLite(path)
}

type instanceBackupManifest struct {
	Format    string    `json:"format"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
}

func createInstanceArchive(destination, databaseSnapshot, dataDirectory string, createdAt time.Time) error {
	temporary := destination + ".tmp"
	_ = os.Remove(temporary)
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		_ = output.Close()
		if cleanup {
			_ = os.Remove(temporary)
		}
	}()
	archive := zip.NewWriter(output)
	manifest, err := json.Marshal(instanceBackupManifest{Format: "nav.ax-instance", Version: 1, CreatedAt: createdAt})
	if err != nil {
		return err
	}
	if err := writeZipBytes(archive, "manifest.json", manifest, 0o600); err != nil {
		return err
	}
	if err := writeZipFile(archive, "navax.db", databaseSnapshot); err != nil {
		return err
	}
	for _, name := range []string{"master.key", "analytics.key"} {
		path := filepath.Join(dataDirectory, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			if err := writeZipFile(archive, name, path); err != nil {
				return err
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	assetsRoot := filepath.Join(dataDirectory, "assets")
	if err := archiveAssets(archive, assetsRoot); err != nil {
		return err
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("finalize instance archive: %w", err)
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	_ = os.Remove(destination)
	if err := os.Rename(temporary, destination); err != nil {
		return err
	}
	cleanup = false
	return syncDirectory(filepath.Dir(destination))
}

func archiveAssets(archive *zip.Writer, root string) error {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return writeZipBytes(archive, "assets/", nil, 0o700|os.ModeDir)
	} else if err != nil {
		return err
	}
	if err := writeZipBytes(archive, "assets/", nil, 0o700|os.ModeDir); err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("asset symlinks are not supported: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := "assets/" + filepath.ToSlash(relative)
		if entry.IsDir() {
			return writeZipBytes(archive, name+"/", nil, 0o700|os.ModeDir)
		}
		return writeZipFile(archive, name, path)
	})
}

func writeZipFile(archive *zip.Writer, name, path string) error {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ErrBackupInvalid
	}
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o600)
	header.SetModTime(time.Unix(0, 0).UTC())
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, input)
	return err
}

func writeZipBytes(archive *zip.Writer, name string, value []byte, mode os.FileMode) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	header.SetModTime(time.Unix(0, 0).UTC())
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(value)
	return err
}

func validateInstanceArchive(path string) error {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return ErrBackupInvalid
	}
	defer archive.Close()
	var manifest instanceBackupManifest
	var database *zip.File
	for _, file := range archive.File {
		if !validArchiveEntry(file) {
			return ErrBackupInvalid
		}
		switch file.Name {
		case "manifest.json":
			reader, err := file.Open()
			if err != nil {
				return ErrBackupInvalid
			}
			err = json.NewDecoder(io.LimitReader(reader, 64<<10)).Decode(&manifest)
			_ = reader.Close()
			if err != nil {
				return ErrBackupInvalid
			}
		case "navax.db":
			database = file
		}
	}
	if manifest.Format != "nav.ax-instance" || manifest.Version != 1 || database == nil {
		return ErrBackupInvalid
	}
	temporary, err := os.CreateTemp("", "navax-backup-*.sqlite3")
	if err != nil {
		return err
	}
	pathCopy := temporary.Name()
	defer os.Remove(pathCopy)
	reader, err := database.Open()
	if err != nil {
		_ = temporary.Close()
		return ErrBackupInvalid
	}
	_, copyErr := io.Copy(temporary, io.LimitReader(reader, 4<<30))
	closeReaderErr := reader.Close()
	closeFileErr := temporary.Close()
	if copyErr != nil || closeReaderErr != nil || closeFileErr != nil {
		return ErrBackupInvalid
	}
	return validateSQLite(pathCopy)
}

func validArchiveEntry(file *zip.File) bool {
	name := file.Name
	if name == "manifest.json" || name == "navax.db" || name == "master.key" || name == "analytics.key" || name == "assets/" {
		return file.Mode()&os.ModeSymlink == 0
	}
	cleaned := pathpkg.Clean(name)
	return strings.HasPrefix(name, "assets/") && cleaned == strings.TrimSuffix(name, "/") &&
		!strings.Contains(name, "\\") && file.Mode()&os.ModeSymlink == 0
}

func stageInstanceArchive(path, dataDirectory string) error {
	if dataDirectory == "" {
		return ErrRestoreNotConfigured
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return ErrBackupInvalid
	}
	defer archive.Close()
	databasePath := filepath.Join(dataDirectory, "navax.db")
	removePendingRestore(dataDirectory, databasePath)
	assetsPending := filepath.Join(dataDirectory, "assets.restore-pending")
	if err := os.MkdirAll(assetsPending, 0o700); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			removePendingRestore(dataDirectory, databasePath)
		}
	}()
	for _, file := range archive.File {
		if !validArchiveEntry(file) {
			return ErrBackupInvalid
		}
		switch {
		case file.Name == "navax.db":
			if err := extractZipFile(file, databasePath+".restore-pending", 0o600); err != nil {
				return err
			}
		case file.Name == "master.key" || file.Name == "analytics.key":
			if err := extractZipFile(file, filepath.Join(dataDirectory, file.Name)+".restore-pending", 0o600); err != nil {
				return err
			}
		case strings.HasPrefix(file.Name, "assets/") && file.Name != "assets/":
			relative := strings.TrimPrefix(file.Name, "assets/")
			destination := filepath.Join(assetsPending, filepath.FromSlash(relative))
			if file.FileInfo().IsDir() {
				if err := os.MkdirAll(destination, 0o700); err != nil {
					return err
				}
			} else if err := extractZipFile(file, destination, 0o600); err != nil {
				return err
			}
		}
	}
	if err := validateSQLite(databasePath + ".restore-pending"); err != nil {
		return err
	}
	cleanup = false
	return syncDirectory(dataDirectory)
}

func extractZipFile(file *zip.File, destination string, mode os.FileMode) error {
	if file.UncompressedSize64 > 4<<30 {
		return ErrBackupInvalid
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	temporary := destination + ".tmp"
	_ = os.Remove(temporary)
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		_ = output.Close()
		if cleanup {
			_ = os.Remove(temporary)
		}
	}()
	written, err := io.Copy(output, io.LimitReader(reader, int64(file.UncompressedSize64)+1))
	if err != nil || written != int64(file.UncompressedSize64) {
		return ErrBackupInvalid
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	_ = os.Remove(destination)
	if err := os.Rename(temporary, destination); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func removePendingRestore(dataDirectory, databasePath string) {
	_ = os.Remove(databasePath + ".restore-pending")
	if dataDirectory == "" {
		return
	}
	_ = os.RemoveAll(filepath.Join(dataDirectory, "assets.restore-pending"))
	_ = os.Remove(filepath.Join(dataDirectory, "master.key.restore-pending"))
	_ = os.Remove(filepath.Join(dataDirectory, "analytics.key.restore-pending"))
}

func validateSQLite(path string) error {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return ErrBackupInvalid
	}
	defer db.Close()
	var result string
	if err := db.QueryRow("PRAGMA quick_check").Scan(&result); err != nil || result != "ok" {
		return ErrBackupInvalid
	}
	return nil
}

func copyFileAtomic(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	temporary := destination + ".tmp"
	_ = os.Remove(temporary)
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		_ = output.Close()
		if cleanup {
			_ = os.Remove(temporary)
		}
	}()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	_ = os.Remove(destination)
	if err := os.Rename(temporary, destination); err != nil {
		return err
	}
	cleanup = false
	return syncDirectory(filepath.Dir(destination))
}

func sqliteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
