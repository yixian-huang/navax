package maintenance

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/database"
)

func TestCheckVerifiesSignedManifest(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(UpdateManifest{
		Version: "1.2.0", ReleaseNotes: "security fixes", PublishedAt: time.Now().UTC(),
		Assets: []UpdateAsset{{OS: "linux", Arch: "amd64", URL: "https://example.com/navax", SHA256: string(make([]byte, 64)), Size: 1024}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["assets"].([]any)[0].(map[string]any)["sha256"] = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	payload, _ = json.Marshal(manifest)
	signed, _ := json.Marshal(signedManifest{Payload: payload, Signature: base64.RawStdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(signed) }))
	defer server.Close()

	db, err := database.OpenAndMigrate(context.Background(), database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewUpdateService(db, "1.0.0", "binary", server.URL, publicKey)
	state, err := service.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "available" || state.LatestVersion == nil || *state.LatestVersion != "1.2.0" {
		t.Fatalf("state = %+v", state)
	}
}

func TestCheckRejectsTamperedManifest(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signed, _ := json.Marshal(signedManifest{Payload: json.RawMessage(`{"version":"9.9.9"}`), Signature: base64.RawStdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(signed) }))
	defer server.Close()
	db, err := database.OpenAndMigrate(context.Background(), database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewUpdateService(db, "1.0.0", "binary", server.URL, publicKey)
	if _, err := service.Check(context.Background()); err == nil {
		t.Fatal("Check() accepted a tampered manifest")
	}
}

func TestApplyReplacesBinaryAfterBackupAndVerification(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	newBinary := []byte("new signed navax binary")
	digest := sha256.Sum256(newBinary)
	var signed []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset" {
			_, _ = w.Write(newBinary)
			return
		}
		_, _ = w.Write(signed)
	}))
	defer server.Close()
	payload, err := json.Marshal(UpdateManifest{
		Version: "1.1.0", PublishedAt: time.Now().UTC(),
		Assets: []UpdateAsset{{OS: runtime.GOOS, Arch: runtime.GOARCH, URL: server.URL + "/asset", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(newBinary))}},
	})
	if err != nil {
		t.Fatal(err)
	}
	signed, _ = json.Marshal(signedManifest{Payload: payload, Signature: base64.RawStdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))})

	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	backups, err := NewBackupService(db, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "navax")
	if err := os.WriteFile(executable, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	service := NewUpdateService(db, "1.0.0", "binary", server.URL, publicKey)
	service.AttachBackups(backups)
	service.executable = func() (string, error) { return executable, nil }
	if err := service.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Check(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := service.Apply(ctx, "1.1.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "restart-required" {
		t.Fatalf("state = %+v", state)
	}
	installed, err := os.ReadFile(executable)
	if err != nil || string(installed) != string(newBinary) {
		t.Fatalf("installed = %q, %v", installed, err)
	}
	rollback, err := os.ReadFile(executable + ".rollback-1.0.0")
	if err != nil || string(rollback) != "old binary" {
		t.Fatalf("rollback = %q, %v", rollback, err)
	}
	list, err := backups.List(ctx)
	if err != nil || len(list) != 1 || list[0].Reason != "pre-update" {
		t.Fatalf("backups = %+v, %v", list, err)
	}
}
