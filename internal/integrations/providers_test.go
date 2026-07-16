package integrations

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yixian-huang/navax/internal/database"
)

func TestProviderSecretsAreEncryptedAndNeverReturned(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service, err := NewService(db, bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	provider, err := service.Update(ctx, SMTP, Update{
		Enabled:  true,
		Settings: map[string]any{"host": "smtp.example.com", "port": 587, "tlsMode": "starttls", "fromAddress": "noreply@example.com"},
		Secrets:  map[string]string{"password": "plaintext-password"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !provider.HasSecret || !provider.Enabled {
		t.Fatalf("provider = %+v", provider)
	}
	var stored string
	if err := db.QueryRowContext(ctx, "SELECT CAST(secrets_ciphertext AS TEXT) FROM provider_configs WHERE kind = 'smtp'").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "" || stored == "plaintext-password" || bytes.Contains([]byte(stored), []byte("plaintext-password")) {
		t.Fatalf("secret was not encrypted: %q", stored)
	}
	provider, err = service.Get(ctx, SMTP)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := provider.Settings["password"]; leaked {
		t.Fatal("provider response leaked password")
	}
}

func TestDNSProviderTestUsesStoredToken(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer dns-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := NewService(db, bytes.Repeat([]byte{4}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, DNS, Update{
		Enabled: true,
		Settings: map[string]any{
			"provider": "generic", "zoneId": "zone", "ttl": 300, "apiEndpoint": server.URL,
		},
		Secrets: map[string]string{"token": "dns-secret"},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := service.Test(ctx, DNS)
	if err != nil || !result.Success {
		t.Fatalf("Test() = %+v, %v", result, err)
	}
}

func TestProviderSecretRequiresMasterKey(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service, err := NewService(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Update(ctx, DNS, Update{
		Settings: map[string]any{"provider": "generic", "zoneId": "zone", "ttl": 300},
		Secrets:  map[string]string{"token": "secret"},
	})
	if err != ErrMasterKeyRequired {
		t.Fatalf("Update() error = %v", err)
	}
}
