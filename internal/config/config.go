package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/security"
)

const (
	defaultAddr         = ":8080"
	defaultInstanceName = "nav.ax"
)

// Config contains process-level settings. Mutable product settings live in SQLite.
type Config struct {
	Addr              string
	DataDir           string
	DatabasePath      string
	PublicBaseURL     string
	RootDomain        string
	InstanceName      string
	SetupToken        string
	MasterKey         []byte
	SecureCookies     bool
	SessionTTL        time.Duration
	ShutdownTimeout   time.Duration
	UpdateManifestURL string
	UpdatePublicKey   []byte
	// TrustedProxies lists CIDRs/IPs of reverse proxies whose X-Forwarded-For
	// may be trusted to derive the real client IP. Empty ⇒ trust none (use RemoteAddr).
	TrustedProxies []netip.Prefix
}

// Load reads and validates environment configuration.
func Load() (Config, error) {
	dataDir := env("NAVAX_DATA_DIR", "./data")
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve data directory: %w", err)
	}

	publicBaseURL := strings.TrimRight(env("PUBLIC_BASE_URL", "http://localhost:8080"), "/")
	parsedURL, err := url.ParseRequestURI(publicBaseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return Config{}, errors.New("PUBLIC_BASE_URL must be an absolute http(s) URL")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return Config{}, errors.New("PUBLIC_BASE_URL must use http or https")
	}

	setupToken := strings.TrimSpace(os.Getenv("NAVAX_SETUP_TOKEN"))
	if setupToken == "" {
		setupToken, err = randomToken(32)
		if err != nil {
			return Config{}, fmt.Errorf("generate setup token: %w", err)
		}
	}
	if len(setupToken) < 32 {
		return Config{}, errors.New("NAVAX_SETUP_TOKEN must contain at least 32 characters")
	}

	masterKey, err := optionalKey(os.Getenv("NAVAX_MASTER_KEY"))
	if err != nil {
		return Config{}, err
	}
	if len(masterKey) == 0 {
		masterKey, err = security.LoadOrCreateKey(filepath.Join(absDataDir, "master.key"), 32)
		if err != nil {
			return Config{}, fmt.Errorf("load or create instance master key: %w", err)
		}
	}
	updatePublicKey, err := optionalPublicKey(os.Getenv("NAVAX_UPDATE_PUBLIC_KEY"))
	if err != nil {
		return Config{}, err
	}

	secureCookies, err := envBool("NAVAX_SECURE_COOKIES", parsedURL.Scheme == "https")
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := envDuration("NAVAX_SESSION_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := envDuration("NAVAX_SHUTDOWN_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	trustedProxies, err := parseTrustedProxies(os.Getenv("NAVAX_TRUSTED_PROXIES"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Addr:              env("NAVAX_ADDR", defaultAddr),
		DataDir:           absDataDir,
		DatabasePath:      filepath.Join(absDataDir, "navax.db"),
		PublicBaseURL:     publicBaseURL,
		RootDomain:        strings.ToLower(strings.TrimSpace(os.Getenv("ROOT_DOMAIN"))),
		InstanceName:      env("INSTANCE_NAME", defaultInstanceName),
		SetupToken:        setupToken,
		MasterKey:         masterKey,
		SecureCookies:     secureCookies,
		SessionTTL:        sessionTTL,
		ShutdownTimeout:   shutdownTimeout,
		UpdateManifestURL: strings.TrimSpace(os.Getenv("NAVAX_UPDATE_MANIFEST_URL")),
		UpdatePublicKey:   updatePublicKey,
		TrustedProxies:    trustedProxies,
	}, nil
}

// parseTrustedProxies accepts a comma-separated list of CIDRs or bare IPs.
func parseTrustedProxies(raw string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(part); err == nil {
			prefixes = append(prefixes, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("NAVAX_TRUSTED_PROXIES contains an invalid entry %q", part)
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return prefixes, nil
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return parsed, nil
}

func optionalKey(encoded string) ([]byte, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, nil
	}
	key, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(encoded)
	}
	if err != nil || len(key) != 32 {
		return nil, errors.New("NAVAX_MASTER_KEY must be a base64-encoded 32-byte key")
	}
	return key, nil
}

func optionalPublicKey(encoded string) ([]byte, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, nil
	}
	key, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(encoded)
	}
	if err != nil || len(key) != 32 {
		return nil, errors.New("NAVAX_UPDATE_PUBLIC_KEY must be a base64-encoded Ed25519 public key")
	}
	return key, nil
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
