package integrations

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yixian-huang/navax/internal/database"
	"github.com/yixian-huang/navax/internal/netguard"
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
	// The httptest server binds loopback, which the SSRF guard (correctly)
	// rejects. Substitute an unguarded probe so this test can still assert that
	// Test() decrypts and forwards the stored bearer token to the endpoint.
	service.probeHTTP = func(ctx context.Context, _ netguard.Validator, endpoint, bearer string) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		if bearer != "" {
			request.Header.Set("Authorization", "Bearer "+bearer)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode >= 300 {
			return fmt.Errorf("端点返回 %s", response.Status)
		}
		return nil
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

func TestDNSProviderTestBlocksNonPublicEndpoint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := database.OpenAndMigrate(ctx, database.Config{Path: ":memory:", MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := NewService(db, bytes.Repeat([]byte{5}, 32))
	if err != nil {
		t.Fatal(err)
	}
	// A DNS control-plane endpoint pointed at the cloud metadata service must be
	// rejected by the strict guard rather than probed.
	if _, err := service.Update(ctx, DNS, Update{
		Enabled: true,
		Settings: map[string]any{
			"provider": "generic", "zoneId": "zone", "ttl": 300, "apiEndpoint": "http://169.254.169.254/latest/meta-data/",
		},
		Secrets: map[string]string{"token": "dns-secret"},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := service.Test(ctx, DNS)
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if result.Success {
		t.Fatal("Test() succeeded against a metadata endpoint; SSRF guard not applied")
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

func TestActiveS3ConfigUsesLocalWithoutS3(t *testing.T) {
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

	// Unconfigured storage → local (ok=false, no error).
	_, _, _, _, _, _, _, _, ok, err := service.ActiveS3Config(ctx)
	if err != nil || ok {
		t.Fatalf("unconfigured: ok=%v err=%v", ok, err)
	}

	// Explicit local driver → local.
	if _, err := service.Update(ctx, Storage, Update{Enabled: true, Settings: map[string]any{"driver": "local"}}); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, _, _, _, _, ok, err = service.ActiveS3Config(ctx)
	if err != nil || ok {
		t.Fatalf("local driver: ok=%v err=%v", ok, err)
	}

	// Incomplete S3 (enabled, no secretKey) must not error — fall back to local.
	if _, err := service.Update(ctx, Storage, Update{Enabled: true, Settings: map[string]any{
		"driver": "s3", "endpoint": "https://s3.example.com", "region": "us-east-1",
		"bucket": "navax", "accessKey": "AKIAEXAMPLE",
	}}); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, _, _, _, _, ok, err = service.ActiveS3Config(ctx)
	if err != nil {
		t.Fatalf("incomplete s3 returned error (must fall back silently): %v", err)
	}
	if ok {
		t.Fatal("incomplete s3 should not report ok=true")
	}
}
