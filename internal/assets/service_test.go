package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/security"
)

func TestLocalAssetUploadAndOpen(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	insertAssetOwner(t, db, "usr_asset_owner")
	root := filepath.Join(t.TempDir(), "assets")
	service, err := NewService(db, root)
	if err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	payload := testPNG(t)
	asset, err := service.Upload(ctx, "usr_asset_owner", "avatar", "avatar.png", "image/png", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	if asset.MIMEType != "image/png" || asset.Size != int64(len(payload)) || asset.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("asset metadata = %+v", asset)
	}
	if asset.Driver != "local" {
		t.Fatalf("driver = %q, want local", asset.Driver)
	}
	if !objectKeyPattern.MatchString(asset.ObjectKey) || asset.URL != publicURLPrefix+asset.ObjectKey {
		t.Fatalf("asset object key/url = %q %q", asset.ObjectKey, asset.URL)
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(filepath.Join(root, filepath.FromSlash(asset.ObjectKey)))
	if err != nil {
		t.Fatal(err)
	}
	if rootInfo.Mode().Perm() != 0o700 || fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("permissions root=%o file=%o", rootInfo.Mode().Perm(), fileInfo.Mode().Perm())
	}
	stored, file, err := service.Open(ctx, asset.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if stored.ID != asset.ID {
		t.Fatalf("opened asset = %+v", stored)
	}
	if _, _, err := service.Open(ctx, "../navax.db"); !errors.Is(err, ErrInvalidObject) {
		t.Fatalf("path traversal error = %v", err)
	}
}

// Without any object-storage config (or with a broken resolver), uploads must
// still succeed on the local data directory.
func TestUploadFallsBackToLocalWhenStorageUnavailable(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	insertAssetOwner(t, db, "usr_asset_fallback")
	root := filepath.Join(t.TempDir(), "assets")
	service, err := NewService(db, root)
	if err != nil {
		t.Fatal(err)
	}

	// Resolver returns nil config (no S3) → local.
	service.SetStorageResolver(func(context.Context) (*S3Config, error) {
		return nil, nil
	})
	payload := testPNGAt(t, 128, 96)
	asset, err := service.Upload(ctx, "usr_asset_fallback", "background", "bg.png", "image/png", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("upload without S3: %v", err)
	}
	if asset.Driver != "local" || !strings.HasPrefix(asset.URL, publicURLPrefix) {
		t.Fatalf("want local asset, got %+v", asset)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(asset.ObjectKey))); err != nil {
		t.Fatalf("local file missing: %v", err)
	}

	// Resolver errors → still local, not a hard failure.
	service.SetStorageResolver(func(context.Context) (*S3Config, error) {
		return nil, errors.New("storage provider unreachable")
	})
	asset2, err := service.Upload(ctx, "usr_asset_fallback", "site-icon", "icon.png", "image/png", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("upload with broken resolver: %v", err)
	}
	if asset2.Driver != "local" {
		t.Fatalf("driver = %q after resolver error, want local", asset2.Driver)
	}

	// Incomplete S3 config (nil after resolve returns empty) already covered;
	// invalid client construction also falls back.
	service.SetStorageResolver(func(context.Context) (*S3Config, error) {
		return &S3Config{Endpoint: "http://127.0.0.1:1", Bucket: "b", AccessKey: "a", SecretKey: "s", Region: "us-east-1", PathStyle: true}, nil
	})
	asset3, err := service.Upload(ctx, "usr_asset_fallback", "avatar", "a.png", "image/png", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("upload with unreachable S3 endpoint: %v", err)
	}
	if asset3.Driver != "local" {
		t.Fatalf("driver = %q after S3 put failure, want local", asset3.Driver)
	}
}

func TestAssetValidationAndLimit(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	insertAssetOwner(t, db, "usr_asset_check")
	if _, err := db.ExecContext(ctx, "UPDATE system_settings SET max_upload_bytes = 1024 WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, filepath.Join(t.TempDir(), "assets"))
	if err != nil {
		t.Fatal(err)
	}
	pngPayload := testPNG(t)
	tests := []struct {
		name     string
		kind     string
		filename string
		mimeType string
		payload  []byte
		want     error
	}{
		{name: "invalid kind", kind: "document", filename: "a.png", mimeType: "image/png", payload: pngPayload, want: ErrInvalidKind},
		{name: "svg", kind: "avatar", filename: "a.svg", mimeType: "image/svg+xml", payload: []byte(`<svg onload="alert(1)"></svg>`), want: ErrUnsupported},
		{name: "html polyglot name", kind: "avatar", filename: "a.png", mimeType: "image/png", payload: []byte(`<html><script>alert(1)</script></html>`), want: ErrUnsupported},
		{name: "malformed png", kind: "avatar", filename: "a.png", mimeType: "image/png", payload: append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...), want: ErrInvalidImage},
		{name: "too large", kind: "background", filename: "large.png", mimeType: "image/png", payload: bytes.Repeat([]byte{0}, 1025), want: ErrTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Upload(ctx, "usr_asset_check", test.kind, test.filename, test.mimeType, bytes.NewReader(test.payload))
			if !errors.Is(err, test.want) {
				t.Fatalf("Upload() error = %v, want %v", err, test.want)
			}
		})
	}
	// Content wins over wrong client extension / declared MIME aliases.
	t.Run("accepts png with jpg name and jpeg claim", func(t *testing.T) {
		asset, err := service.Upload(ctx, "usr_asset_check", "avatar", "photo.jpg", "image/jpg", bytes.NewReader(pngPayload))
		if err != nil {
			t.Fatalf("Upload() error = %v", err)
		}
		if asset.MIMEType != "image/png" || !strings.HasSuffix(asset.ObjectKey, ".png") {
			t.Fatalf("expected png storage from content, got %+v", asset)
		}
	})
	t.Run("accepts image/jpg alias for jpeg payload", func(t *testing.T) {
		jpegPayload := testJPEG(t)
		asset, err := service.Upload(ctx, "usr_asset_check", "avatar", "shot.JPEG", "image/jpg", bytes.NewReader(jpegPayload))
		if err != nil {
			t.Fatalf("Upload() error = %v", err)
		}
		if asset.MIMEType != "image/jpeg" || !strings.HasSuffix(asset.ObjectKey, ".jpg") {
			t.Fatalf("expected jpeg storage, got %+v", asset)
		}
	})
	t.Run("rejects tiny background wallpaper", func(t *testing.T) {
		tiny := testPNGAt(t, 16, 16)
		_, err := service.Upload(ctx, "usr_asset_check", "background", "tiny.png", "image/png", bytes.NewReader(tiny))
		if !errors.Is(err, ErrImageTooSmall) {
			t.Fatalf("Upload() error = %v, want ErrImageTooSmall", err)
		}
	})
	matches, err := filepath.Glob(filepath.Join(service.root, ".upload-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staging files left behind: %v, %v", matches, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM assets").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("accepted uploads recorded %d assets, want 2", count)
	}
}

func insertAssetOwner(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	hash, err := security.HashPassword("integration-password")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(`
		INSERT INTO users(id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, 'asset-owner', 'asset@example.com', ?, 'user', 'active', ?, ?)`, id, hash, now, now)
	if err != nil {
		t.Fatal(err)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	return testPNGAt(t, 16, 16)
}

func testPNGAt(t *testing.T, w, h int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 120, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			canvas.Set(x, y, color.RGBA{R: 200, G: uint8(x * 8), B: uint8(y * 8), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
