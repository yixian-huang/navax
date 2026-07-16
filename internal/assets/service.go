// Package assets stores validated raster images on the local filesystem and
// records their immutable metadata in SQLite.
package assets

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
)

var (
	ErrInvalidKind   = errors.New("invalid asset kind")
	ErrTooLarge      = errors.New("asset exceeds configured upload limit")
	ErrUnsupported   = errors.New("unsupported image type")
	ErrInvalidImage  = errors.New("invalid image data")
	ErrInvalidObject = errors.New("invalid asset object key")
	ErrNotFound      = errors.New("asset not found")
	ErrInvalidOwner  = errors.New("invalid asset owner")
)

const (
	publicURLPrefix = "/api/v1/assets/"
	maxImagePixels  = uint64(40_000_000)
)

var objectKeyPattern = regexp.MustCompile(`^(avatar|background|site-icon)/[a-f0-9]{32}\.(png|jpg|gif)$`)

type Asset struct {
	ID        string
	OwnerID   string
	Kind      string
	ObjectKey string
	URL       string
	MIMEType  string
	Size      int64
	SHA256    string
	CreatedAt time.Time
}

type Service struct {
	db   *sql.DB
	root string
	now  func() time.Time
}

func NewService(db *sql.DB, root string) (*Service, error) {
	if db == nil || strings.TrimSpace(root) == "" {
		return nil, errors.New("asset database and storage directory are required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve asset directory: %w", err)
	}
	if err := ensurePrivateDirectory(absolute); err != nil {
		return nil, err
	}
	for _, kind := range []string{"avatar", "background", "site-icon"} {
		if err := ensurePrivateDirectory(filepath.Join(absolute, kind)); err != nil {
			return nil, err
		}
	}
	return &Service{db: db, root: absolute, now: time.Now}, nil
}

func (s *Service) MaxUploadBytes(ctx context.Context) (int64, error) {
	var maximum int64
	if err := s.db.QueryRowContext(ctx, "SELECT max_upload_bytes FROM system_settings WHERE id = 1").Scan(&maximum); err != nil {
		return 0, fmt.Errorf("read asset upload limit: %w", err)
	}
	return maximum, nil
}

func (s *Service) Upload(ctx context.Context, ownerID, kind, filename, declaredMIME string, source io.Reader) (Asset, error) {
	if strings.TrimSpace(ownerID) == "" {
		return Asset{}, ErrInvalidOwner
	}
	if !validKind(kind) {
		return Asset{}, ErrInvalidKind
	}
	maximum, err := s.MaxUploadBytes(ctx)
	if err != nil {
		return Asset{}, err
	}
	temporary, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return Asset{}, fmt.Errorf("create upload staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return Asset{}, fmt.Errorf("secure upload staging file: %w", err)
	}
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hasher), io.LimitReader(source, maximum+1))
	if err != nil {
		return Asset{}, fmt.Errorf("write upload staging file: %w", err)
	}
	if written > maximum {
		return Asset{}, ErrTooLarge
	}
	if written == 0 {
		return Asset{}, ErrInvalidImage
	}
	if err := temporary.Sync(); err != nil {
		return Asset{}, fmt.Errorf("sync upload staging file: %w", err)
	}
	detectedMIME, extension, err := inspectImage(temporary, filename, declaredMIME)
	if err != nil {
		return Asset{}, err
	}
	randomName, err := randomHex(16)
	if err != nil {
		return Asset{}, err
	}
	objectKey := kind + "/" + randomName + extension
	finalPath, err := s.pathForKey(objectKey)
	if err != nil {
		return Asset{}, err
	}
	if err := temporary.Close(); err != nil {
		return Asset{}, fmt.Errorf("close upload staging file: %w", err)
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return Asset{}, fmt.Errorf("commit asset file: %w", err)
	}
	temporaryPath = finalPath
	if err := os.Chmod(finalPath, 0o600); err != nil {
		return Asset{}, fmt.Errorf("secure asset file: %w", err)
	}
	id, err := identity.New("ast")
	if err != nil {
		return Asset{}, err
	}
	now := s.now().UTC()
	asset := Asset{
		ID: id, OwnerID: ownerID, Kind: kind, ObjectKey: objectKey,
		URL: publicURLPrefix + objectKey, MIMEType: detectedMIME, Size: written,
		SHA256: hex.EncodeToString(hasher.Sum(nil)), CreatedAt: now,
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO assets(id, owner_id, kind, storage_driver, object_key, url, mime_type, size_bytes, sha256, created_at)
		VALUES (?, ?, ?, 'local', ?, ?, ?, ?, ?, ?)`, asset.ID, asset.OwnerID, asset.Kind,
		asset.ObjectKey, asset.URL, asset.MIMEType, asset.Size, asset.SHA256, dbTime(asset.CreatedAt))
	if err != nil {
		return Asset{}, fmt.Errorf("record asset: %w", err)
	}
	committed = true
	return asset, nil
}

func (s *Service) Open(ctx context.Context, objectKey string) (Asset, *os.File, error) {
	path, err := s.pathForKey(objectKey)
	if err != nil {
		return Asset{}, nil, err
	}
	asset, err := s.assetByKey(ctx, objectKey)
	if err != nil {
		return Asset{}, nil, err
	}
	pathInfo, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Asset{}, nil, ErrNotFound
	}
	if err != nil {
		return Asset{}, nil, fmt.Errorf("inspect asset path: %w", err)
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return Asset{}, nil, ErrNotFound
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Asset{}, nil, ErrNotFound
	}
	if err != nil {
		return Asset{}, nil, fmt.Errorf("open asset file: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != asset.Size {
		_ = file.Close()
		if err != nil {
			return Asset{}, nil, fmt.Errorf("inspect asset file: %w", err)
		}
		return Asset{}, nil, ErrNotFound
	}
	return asset, file, nil
}

func (s *Service) assetByKey(ctx context.Context, objectKey string) (Asset, error) {
	var asset Asset
	var createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, owner_id, kind, object_key, url, mime_type, size_bytes, sha256, created_at
		FROM assets WHERE object_key = ? AND storage_driver = 'local'`, objectKey).Scan(
		&asset.ID, &asset.OwnerID, &asset.Kind, &asset.ObjectKey, &asset.URL,
		&asset.MIMEType, &asset.Size, &asset.SHA256, &createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	if err != nil {
		return Asset{}, err
	}
	asset.CreatedAt, err = parseDBTime(createdAt)
	return asset, err
}

func (s *Service) pathForKey(objectKey string) (string, error) {
	if !objectKeyPattern.MatchString(objectKey) || strings.Contains(objectKey, `\`) {
		return "", ErrInvalidObject
	}
	path := filepath.Join(s.root, filepath.FromSlash(objectKey))
	relative, err := filepath.Rel(s.root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrInvalidObject
	}
	return path, nil
}

func inspectImage(file *os.File, filename, declaredMIME string) (string, string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	header := make([]byte, 512)
	count, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", "", ErrInvalidImage
	}
	header = header[:count]
	detected := http.DetectContentType(header)
	extension, accepted := imageTypes[detected]
	if !accepted {
		return "", "", ErrUnsupported
	}
	declaredMIME = strings.ToLower(strings.TrimSpace(strings.Split(declaredMIME, ";")[0]))
	if declaredMIME != "" && declaredMIME != "application/octet-stream" && declaredMIME != detected {
		return "", "", ErrUnsupported
	}
	filenameExtension := strings.ToLower(filepath.Ext(filename))
	if !acceptedExtension(detected, filenameExtension) {
		return "", "", ErrUnsupported
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	config, format, err := image.DecodeConfig(file)
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return "", "", ErrInvalidImage
	}
	if !formatMatchesMIME(format, detected) || uint64(config.Width)*uint64(config.Height) > maxImagePixels {
		return "", "", ErrInvalidImage
	}
	return detected, extension, nil
}

var imageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
}

func acceptedExtension(mimeType, extension string) bool {
	switch mimeType {
	case "image/jpeg":
		return extension == ".jpg" || extension == ".jpeg"
	case "image/png":
		return extension == ".png"
	case "image/gif":
		return extension == ".gif"
	default:
		return false
	}
}

func formatMatchesMIME(format, mimeType string) bool {
	return format == "jpeg" && mimeType == "image/jpeg" || format == "png" && mimeType == "image/png" || format == "gif" && mimeType == "image/gif"
}

func validKind(kind string) bool {
	return kind == "avatar" || kind == "background" || kind == "site-icon"
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create asset directory: %w", err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("inspect asset directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("asset storage path must be a real directory")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure asset directory: %w", err)
	}
	return nil
}

func randomHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate asset object key: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func dbTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseDBTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse database time %q: %w", value, err)
	}
	return parsed, nil
}
