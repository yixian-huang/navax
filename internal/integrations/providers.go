package integrations

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/netguard"
	"github.com/yixian-huang/navax/internal/security"
)

var (
	ErrInvalidProvider   = errors.New("invalid provider kind")
	ErrInvalidSettings   = errors.New("invalid provider settings")
	ErrMasterKeyRequired = errors.New("master key is required to store provider secrets")
)

type Kind string

const (
	SMTP    Kind = "smtp"
	Storage Kind = "storage"
	DNS     Kind = "dns"
)

type Provider struct {
	Kind       Kind           `json:"kind"`
	Enabled    bool           `json:"enabled"`
	Configured bool           `json:"configured"`
	HasSecret  bool           `json:"hasSecret"`
	Settings   map[string]any `json:"settings"`
	UpdatedAt  *time.Time     `json:"updatedAt"`
}

type Update struct {
	Enabled  bool
	Settings map[string]any
	Secrets  map[string]string
}

type TestResult struct {
	Success    bool   `json:"success"`
	DurationMS int64  `json:"durationMs"`
	Message    string `json:"message"`
}

type Service struct {
	db  *sql.DB
	box *security.SecretBox
	// probeHTTP/probeSMTP are seams so tests can exercise connectivity against a
	// loopback fixture that the SSRF guard would otherwise (correctly) reject.
	probeHTTP func(ctx context.Context, validator netguard.Validator, endpoint, bearer string) error
	probeSMTP func(ctx context.Context, validator netguard.Validator, settings map[string]any, secrets map[string]string) error
}

func NewService(db *sql.DB, masterKey []byte) (*Service, error) {
	var box *security.SecretBox
	var err error
	if len(masterKey) > 0 {
		box, err = security.NewSecretBox(masterKey)
		if err != nil {
			return nil, err
		}
	}
	return &Service{db: db, box: box, probeHTTP: probeHTTP, probeSMTP: probeSMTP}, nil
}

func (s *Service) List(ctx context.Context) ([]Provider, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT kind, enabled, settings_json, secrets_ciphertext IS NOT NULL, updated_at
		FROM provider_configs ORDER BY kind`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	providers := make([]Provider, 0, 3)
	for rows.Next() {
		provider, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		provider.Settings = nil
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func (s *Service) Get(ctx context.Context, kind Kind) (Provider, error) {
	if !kind.Valid() {
		return Provider{}, ErrInvalidProvider
	}
	return scanProvider(s.db.QueryRowContext(ctx, `
		SELECT kind, enabled, settings_json, secrets_ciphertext IS NOT NULL, updated_at
		FROM provider_configs WHERE kind = ?`, kind))
}

func (s *Service) Update(ctx context.Context, kind Kind, update Update) (Provider, error) {
	if !kind.Valid() {
		return Provider{}, ErrInvalidProvider
	}
	if err := validateSettings(kind, update.Settings); err != nil {
		return Provider{}, err
	}
	settingsJSON, err := json.Marshal(update.Settings)
	if err != nil {
		return Provider{}, fmt.Errorf("encode provider settings: %w", err)
	}

	var ciphertext []byte
	var nonceMarker []byte
	if len(update.Secrets) > 0 {
		if s.box == nil {
			return Provider{}, ErrMasterKeyRequired
		}
		secrets, err := s.mergedSecrets(ctx, kind, update.Secrets)
		if err != nil {
			return Provider{}, err
		}
		encoded, err := json.Marshal(secrets)
		if err != nil {
			return Provider{}, err
		}
		sealed, err := s.box.Encrypt(encoded, "provider:"+string(kind))
		if err != nil {
			return Provider{}, err
		}
		ciphertext = []byte(sealed)
		nonceMarker = []byte("aes-gcm-v1-embedded")
	}

	now := time.Now().UTC()
	if len(update.Secrets) > 0 {
		_, err = s.db.ExecContext(ctx, `
			UPDATE provider_configs
			SET enabled = ?, settings_json = ?, secrets_ciphertext = ?, secret_nonce = ?, updated_at = ?
			WHERE kind = ?`, update.Enabled, string(settingsJSON), ciphertext, nonceMarker, now.Format(time.RFC3339Nano), kind)
	} else {
		_, err = s.db.ExecContext(ctx, `
			UPDATE provider_configs SET enabled = ?, settings_json = ?, updated_at = ? WHERE kind = ?`,
			update.Enabled, string(settingsJSON), now.Format(time.RFC3339Nano), kind)
	}
	if err != nil {
		return Provider{}, err
	}
	return s.Get(ctx, kind)
}

func (s *Service) mergedSecrets(ctx context.Context, kind Kind, incoming map[string]string) (map[string]string, error) {
	allowed := allowedSecrets(kind)
	merged := make(map[string]string)
	var existing []byte
	err := s.db.QueryRowContext(ctx, "SELECT secrets_ciphertext FROM provider_configs WHERE kind = ?", kind).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if len(existing) > 0 {
		if s.box == nil {
			return nil, ErrMasterKeyRequired
		}
		plaintext, err := s.box.Decrypt(string(existing), "provider:"+string(kind))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(plaintext, &merged); err != nil {
			return nil, fmt.Errorf("decode stored provider secrets: %w", err)
		}
	}
	for name, value := range incoming {
		if !allowed[name] {
			return nil, fmt.Errorf("%w: unsupported secret %q", ErrInvalidSettings, name)
		}
		if value != "" {
			merged[name] = value
		}
	}
	return merged, nil
}

// ActiveS3Config returns S3 credentials when storage is enabled with driver s3.
// A nil config means callers should use local disk storage.
func (s *Service) ActiveS3Config(ctx context.Context) (endpoint, region, bucket, prefix, accessKey, secretKey, publicBaseURL string, pathStyle bool, ok bool, err error) {
	provider, err := s.Get(ctx, Storage)
	if err != nil {
		return "", "", "", "", "", "", "", false, false, err
	}
	if !provider.Enabled || !provider.Configured {
		return "", "", "", "", "", "", "", false, false, nil
	}
	if stringSetting(provider.Settings, "driver") != "s3" {
		return "", "", "", "", "", "", "", false, false, nil
	}
	secrets, err := s.mergedSecrets(ctx, Storage, nil)
	if err != nil {
		return "", "", "", "", "", "", "", false, false, err
	}
	if secrets["secretKey"] == "" {
		return "", "", "", "", "", "", "", false, false, fmt.Errorf("%w: secretKey is required", ErrInvalidSettings)
	}
	pathStyle = boolSetting(provider.Settings, "pathStyle")
	return stringSetting(provider.Settings, "endpoint"),
		stringSetting(provider.Settings, "region"),
		stringSetting(provider.Settings, "bucket"),
		stringSetting(provider.Settings, "prefix"),
		stringSetting(provider.Settings, "accessKey"),
		secrets["secretKey"],
		stringSetting(provider.Settings, "publicBaseUrl"),
		pathStyle,
		true,
		nil
}

func (s *Service) Test(ctx context.Context, kind Kind) (TestResult, error) {
	started := time.Now()
	provider, err := s.Get(ctx, kind)
	if err != nil {
		return TestResult{}, err
	}
	if !provider.Configured {
		return TestResult{}, ErrInvalidSettings
	}
	secrets, err := s.mergedSecrets(ctx, kind, nil)
	if err != nil {
		return TestResult{}, err
	}
	probeErr := error(nil)
	message := "连接成功"
	switch kind {
	case SMTP:
		// SMTP and object storage may legitimately be an internal relay in a
		// self-hosted deployment, so permit private ranges but still block
		// loopback and cloud-metadata (see netguard.NewInternalValidator).
		probeErr = s.probeSMTP(ctx, netguard.NewInternalValidator(nil), provider.Settings, secrets)
		message = "SMTP 握手与认证成功"
	case Storage:
		if stringSetting(provider.Settings, "driver") == "local" {
			message = "本地存储配置有效"
		} else {
			if secrets["secretKey"] == "" {
				return TestResult{}, fmt.Errorf("%w: secretKey is required", ErrInvalidSettings)
			}
			probeErr = s.probeHTTP(ctx, netguard.NewInternalValidator(nil), stringSetting(provider.Settings, "endpoint"), "")
			message = "S3 端点可达；未执行写入操作"
		}
	case DNS:
		if secrets["token"] == "" {
			return TestResult{}, fmt.Errorf("%w: token is required", ErrInvalidSettings)
		}
		endpoint := stringSetting(provider.Settings, "apiEndpoint")
		if endpoint == "" && strings.EqualFold(stringSetting(provider.Settings, "provider"), "cloudflare") {
			endpoint = "https://api.cloudflare.com/client/v4/user/tokens/verify"
		}
		// DNS control-plane APIs are always public; enforce the strict guard.
		probeErr = s.probeHTTP(ctx, netguard.NewValidator(nil), endpoint, secrets["token"])
		message = "DNS API 凭据验证成功"
	}
	duration := time.Since(started).Milliseconds()
	if probeErr != nil {
		return TestResult{Success: false, DurationMS: duration, Message: probeErr.Error()}, nil
	}
	return TestResult{Success: true, DurationMS: duration, Message: message}, nil
}

func (kind Kind) Valid() bool {
	return kind == SMTP || kind == Storage || kind == DNS
}

func allowedSecrets(kind Kind) map[string]bool {
	switch kind {
	case SMTP:
		return map[string]bool{"password": true}
	case Storage:
		return map[string]bool{"secretKey": true}
	case DNS:
		return map[string]bool{"token": true}
	default:
		return map[string]bool{}
	}
}

func validateSettings(kind Kind, settings map[string]any) error {
	if settings == nil {
		return fmt.Errorf("%w: settings are required", ErrInvalidSettings)
	}
	requireString := func(name string) error {
		value, ok := settings[name].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: %s is required", ErrInvalidSettings, name)
		}
		return nil
	}
	switch kind {
	case SMTP:
		for _, field := range []string{"host", "tlsMode", "fromAddress"} {
			if err := requireString(field); err != nil {
				return err
			}
		}
		port, ok := numericSetting(settings, "port")
		if !ok || port < 1 || port > 65535 {
			return fmt.Errorf("%w: port must be between 1 and 65535", ErrInvalidSettings)
		}
		if mode := stringSetting(settings, "tlsMode"); mode != "none" && mode != "starttls" && mode != "tls" {
			return fmt.Errorf("%w: tlsMode is invalid", ErrInvalidSettings)
		}
		if address, err := mail.ParseAddress(stringSetting(settings, "fromAddress")); err != nil || address.Address != stringSetting(settings, "fromAddress") {
			return fmt.Errorf("%w: fromAddress is invalid", ErrInvalidSettings)
		}
	case Storage:
		if err := requireString("driver"); err != nil {
			return err
		}
		if settings["driver"] == "s3" {
			for _, field := range []string{"endpoint", "region", "bucket", "accessKey"} {
				if err := requireString(field); err != nil {
					return err
				}
			}
			if err := validateHTTPURL(stringSetting(settings, "endpoint")); err != nil {
				return err
			}
		} else if settings["driver"] != "local" {
			return fmt.Errorf("%w: driver is invalid", ErrInvalidSettings)
		}
	case DNS:
		for _, field := range []string{"provider", "zoneId"} {
			if err := requireString(field); err != nil {
				return err
			}
		}
		if ttl, ok := numericSetting(settings, "ttl"); !ok || ttl < 60 || ttl > 86400 {
			return fmt.Errorf("%w: ttl must be between 60 and 86400", ErrInvalidSettings)
		}
		if endpoint := stringSetting(settings, "apiEndpoint"); endpoint != "" {
			if err := validateHTTPURL(endpoint); err != nil {
				return err
			}
		}
	default:
		return ErrInvalidProvider
	}
	return nil
}

func probeSMTP(ctx context.Context, validator netguard.Validator, settings map[string]any, secrets map[string]string) error {
	host := stringSetting(settings, "host")
	port, _ := numericSetting(settings, "port")
	address := net.JoinHostPort(host, strconv.Itoa(port))
	// Dial through the guard so the host is resolved and vetted before connecting,
	// and the TCP connection targets the vetted IP (no DNS-rebinding window).
	dialer := netguard.Dialer{Validator: validator, Dialer: net.Dialer{Timeout: 8 * time.Second}}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("连接 SMTP 失败: %w", err)
	}
	if stringSetting(settings, "tlsMode") == "tls" {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if handshakeErr := tlsConnection.HandshakeContext(ctx); handshakeErr != nil {
			_ = connection.Close()
			return fmt.Errorf("SMTP TLS 握手失败: %w", handshakeErr)
		}
		connection = tlsConnection
	}
	defer connection.Close()
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("SMTP 握手失败: %w", err)
	}
	defer client.Close()
	if stringSetting(settings, "tlsMode") == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("SMTP 服务器不支持 STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("SMTP STARTTLS 失败: %w", err)
		}
	}
	username := stringSetting(settings, "username")
	if username != "" {
		password := secrets["password"]
		if password == "" {
			return errors.New("SMTP 密码尚未配置")
		}
		if err := client.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}
	return client.Quit()
}

func probeHTTP(ctx context.Context, validator netguard.Validator, endpoint, bearer string) error {
	if err := validateHTTPURL(endpoint); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	// GuardedClient validates the target (and any redirect) and dials only the
	// vetted resolved IP. maxRedirects=0 keeps the probe to the configured host.
	response, err := netguard.GuardedClient(validator, 8*time.Second, 0).Do(request)
	if err != nil {
		return fmt.Errorf("连接端点失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("端点返回 %s", response.Status)
	}
	return nil
}

func validateHTTPURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: endpoint must be an absolute http(s) URL", ErrInvalidSettings)
	}
	return nil
}

func stringSetting(settings map[string]any, name string) string {
	value, _ := settings[name].(string)
	return strings.TrimSpace(value)
}

func boolSetting(settings map[string]any, name string) bool {
	value, _ := settings[name].(bool)
	return value
}

func numericSetting(settings map[string]any, name string) (int, bool) {
	switch value := settings[name].(type) {
	case float64:
		if value != float64(int(value)) {
			return 0, false
		}
		return int(value), true
	case int:
		return value, true
	case json.Number:
		parsed, err := strconv.Atoi(value.String())
		return parsed, err == nil
	default:
		return 0, false
	}
}

type scanner interface{ Scan(...any) error }

func scanProvider(row scanner) (Provider, error) {
	var provider Provider
	var settingsJSON string
	var updatedAt sql.NullString
	if err := row.Scan(&provider.Kind, &provider.Enabled, &settingsJSON, &provider.HasSecret, &updatedAt); err != nil {
		return Provider{}, err
	}
	if err := json.Unmarshal([]byte(settingsJSON), &provider.Settings); err != nil {
		return Provider{}, fmt.Errorf("decode provider settings: %w", err)
	}
	provider.Configured = len(provider.Settings) > 0
	if updatedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, updatedAt.String)
		if err != nil {
			return Provider{}, err
		}
		provider.UpdatedAt = &parsed
	}
	return provider, nil
}
