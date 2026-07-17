// Package assets stores validated raster images and records immutable metadata
// in SQLite. When the storage provider is enabled with driver "s3", blobs are
// written to S3-compatible object storage; otherwise they stay on local disk.
package assets

import (
	"bytes"
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
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

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
	ErrStorage       = errors.New("object storage is unavailable")
)

const (
	publicURLPrefix = "/api/v1/assets/"
	maxImagePixels  = uint64(40_000_000)
)

var objectKeyPattern = regexp.MustCompile(`^(avatar|background|site-icon)/[a-f0-9]{32}\.(png|jpg|gif|webp)$`)

// StorageResolver returns the active object storage backend for uploads.
// Returning (nil, nil) means local filesystem. Errors fail the upload.
type StorageResolver func(ctx context.Context) (*S3Config, error)

type Asset struct {
	ID        string
	OwnerID   string
	Kind      string
	ObjectKey string
	URL       string
	MIMEType  string
	Size      int64
	SHA256    string
	Driver    string
	CreatedAt time.Time
}

// OpenResult is a seekable reader for public asset serving.
type OpenResult struct {
	Asset  Asset
	Body   io.ReadSeekCloser
	Size   int64
	Closer func() error
}

type Service struct {
	db      *sql.DB
	root    string
	now     func() time.Time
	resolve StorageResolver
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

// SetStorageResolver wires optional S3 configuration (typically from integrations).
func (s *Service) SetStorageResolver(resolve StorageResolver) {
	s.resolve = resolve
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
	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		return Asset{}, err
	}

	driver := "local"
	publicURL := publicURLPrefix + objectKey
	var s3cfg *S3Config
	if s.resolve != nil {
		s3cfg, err = s.resolve(ctx)
		if err != nil {
			slog.Error("resolve object storage", "error", err)
			return Asset{}, fmt.Errorf("%w: %v", ErrStorage, err)
		}
	}
	if s3cfg != nil {
		store, err := newS3Store(*s3cfg)
		if err != nil {
			slog.Error("init object storage client", "error", err)
			return Asset{}, fmt.Errorf("%w: %v", ErrStorage, err)
		}
		if err := store.Put(ctx, objectKey, detectedMIME, temporary, written); err != nil {
			slog.Error("object storage put", "error", err, "object_key", objectKey, "bytes", written)
			return Asset{}, fmt.Errorf("%w: %v", ErrStorage, err)
		}
		driver = "s3"
		publicURL = store.PublicURL(objectKey)
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		temporaryPath = ""
	} else {
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
	}

	id, err := identity.New("ast")
	if err != nil {
		return Asset{}, err
	}
	now := s.now().UTC()
	asset := Asset{
		ID: id, OwnerID: ownerID, Kind: kind, ObjectKey: objectKey,
		URL: publicURL, MIMEType: detectedMIME, Size: written, Driver: driver,
		SHA256: hex.EncodeToString(hasher.Sum(nil)), CreatedAt: now,
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO assets(id, owner_id, kind, storage_driver, object_key, url, mime_type, size_bytes, sha256, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, asset.ID, asset.OwnerID, asset.Kind, driver,
		asset.ObjectKey, asset.URL, asset.MIMEType, asset.Size, asset.SHA256, dbTime(asset.CreatedAt))
	if err != nil {
		return Asset{}, fmt.Errorf("record asset: %w", err)
	}
	committed = true
	return asset, nil
}

// Open returns a seekable body for local assets, or streams from S3 when needed.
// For external publicBaseUrl S3 objects the handler may redirect; Open still
// proxies content so relative /api/v1/assets URLs always work.
func (s *Service) Open(ctx context.Context, objectKey string) (Asset, io.ReadSeekCloser, error) {
	if _, err := s.pathForKey(objectKey); err != nil {
		return Asset{}, nil, err
	}
	asset, err := s.assetByKey(ctx, objectKey)
	if err != nil {
		return Asset{}, nil, err
	}
	if asset.Driver == "s3" {
		var s3cfg *S3Config
		if s.resolve != nil {
			s3cfg, err = s.resolve(ctx)
			if err != nil {
				return Asset{}, nil, fmt.Errorf("%w: %v", ErrStorage, err)
			}
		}
		if s3cfg == nil {
			return Asset{}, nil, ErrNotFound
		}
		store, err := newS3Store(*s3cfg)
		if err != nil {
			return Asset{}, nil, err
		}
		body, size, contentType, err := store.Open(ctx, objectKey)
		if err != nil {
			return Asset{}, nil, err
		}
		if contentType != "" {
			asset.MIMEType = contentType
		}
		if size > 0 {
			asset.Size = size
		}
		// Buffer to satisfy ServeContent Seek requirement.
		data, err := io.ReadAll(io.LimitReader(body, asset.Size+1))
		_ = body.Close()
		if err != nil {
			return Asset{}, nil, err
		}
		return asset, &bytesReadSeekCloser{data: data}, nil
	}

	path, err := s.pathForKey(objectKey)
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
		SELECT id, owner_id, kind, storage_driver, object_key, url, mime_type, size_bytes, sha256, created_at
		FROM assets WHERE object_key = ?`, objectKey).Scan(
		&asset.ID, &asset.OwnerID, &asset.Kind, &asset.Driver, &asset.ObjectKey, &asset.URL,
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

type bytesReadSeekCloser struct {
	data []byte
	off  int64
}

func (b *bytesReadSeekCloser) Read(p []byte) (int, error) {
	if b.off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.off:])
	b.off += int64(n)
	return n, nil
}

func (b *bytesReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = b.off + offset
	case io.SeekEnd:
		next = int64(len(b.data)) + offset
	default:
		return 0, fmt.Errorf("invalid seek whence")
	}
	if next < 0 {
		return 0, fmt.Errorf("negative position")
	}
	b.off = next
	return next, nil
}

func (b *bytesReadSeekCloser) Close() error { return nil }

// inspectImage identifies raster type from file content. Client MIME and
// filename extension are treated as soft hints only: browsers often send
// image/jpg, empty type, or mismatched extensions for otherwise valid files.
// Storage extension always follows the verified content type.
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
	if isBlockedImagePayload(header) {
		return "", "", ErrUnsupported
	}

	sniffed := normalizeImageMIME(http.DetectContentType(header))
	if _, ok := imageTypes[sniffed]; !ok {
		// Fall back to format decoders when the sniffer is inconclusive
		// (e.g. application/octet-stream for short or unusual headers).
		sniffed = ""
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	config, format, err := image.DecodeConfig(file)
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return "", "", ErrInvalidImage
	}
	if uint64(config.Width)*uint64(config.Height) > maxImagePixels {
		return "", "", ErrInvalidImage
	}
	detected, ok := formatToMIME[format]
	if !ok {
		return "", "", ErrUnsupported
	}
	// Sniffer and decoder must agree when both identify an image type.
	if sniffed != "" && sniffed != detected {
		return "", "", ErrInvalidImage
	}
	_ = declaredMIME // client MIME is a soft hint only; content is authoritative
	filenameExtension := strings.ToLower(filepath.Ext(filename))
	if filenameExtension == ".svg" || filenameExtension == ".svgz" {
		return "", "", ErrUnsupported
	}
	return detected, imageTypes[detected], nil
}

var imageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var formatToMIME = map[string]string{
	"png":  "image/png",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"webp": "image/webp",
}

func normalizeImageMIME(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch value {
	case "image/jpg", "image/pjpeg", "image/x-jpeg":
		return "image/jpeg"
	case "image/x-png":
		return "image/png"
	default:
		return value
	}
}

func isBlockedImagePayload(header []byte) bool {
	// Reject obvious non-raster / active content before decoder registration
	// could confuse type detection (SVG, HTML, XML polyglots).
	trimmed := bytes.TrimSpace(header)
	if len(trimmed) == 0 {
		return true
	}
	lower := bytes.ToLower(trimmed[:min(len(trimmed), 256)])
	switch {
	case bytes.HasPrefix(lower, []byte("<svg")),
		bytes.HasPrefix(lower, []byte("<?xml")),
		bytes.HasPrefix(lower, []byte("<!doctype html")),
		bytes.HasPrefix(lower, []byte("<html")),
		bytes.Contains(lower, []byte("<script")):
		return true
	default:
		return false
	}
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
