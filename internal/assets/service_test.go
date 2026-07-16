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
	"image/png"
	"os"
	"path/filepath"
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
		{name: "extension mismatch", kind: "avatar", filename: "a.jpg", mimeType: "image/png", payload: pngPayload, want: ErrUnsupported},
		{name: "mime mismatch", kind: "avatar", filename: "a.png", mimeType: "image/jpeg", payload: pngPayload, want: ErrUnsupported},
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
	matches, err := filepath.Glob(filepath.Join(service.root, ".upload-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staging files left behind: %v, %v", matches, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM assets").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid uploads recorded %d assets", count)
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
	canvas := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 120, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
