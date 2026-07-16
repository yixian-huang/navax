package maintenance

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/identity"
)

var (
	ErrUpdateNotConfigured = errors.New("automatic update is not configured")
	ErrInvalidManifest     = errors.New("invalid signed update manifest")
	ErrContainerManaged    = errors.New("container deployments must be updated by the container runtime")
)

const maxManifestBytes = 2 << 20

type UpdateAsset struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type UpdateManifest struct {
	Version      string        `json:"version"`
	ReleaseNotes string        `json:"releaseNotes"`
	PublishedAt  time.Time     `json:"publishedAt"`
	Assets       []UpdateAsset `json:"assets"`
}

type signedManifest struct {
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
}

type UpdateState struct {
	CurrentVersion    string     `json:"currentVersion"`
	LatestVersion     *string    `json:"latestVersion"`
	Deployment        string     `json:"deployment"`
	Channel           string     `json:"channel"`
	AutoCheck         bool       `json:"autoCheck"`
	AutoApply         bool       `json:"autoApply"`
	MaintenanceWindow *string    `json:"maintenanceWindow"`
	Status            string     `json:"status"`
	ReleaseNotes      string     `json:"releaseNotes"`
	CheckedAt         *time.Time `json:"checkedAt"`
	Error             string     `json:"error"`
}

type UpdateService struct {
	db          *sql.DB
	current     string
	deployment  string
	manifestURL string
	publicKey   ed25519.PublicKey
	client      *http.Client
	backups     *BackupService
	executable  func() (string, error)
}

func NewUpdateService(db *sql.DB, currentVersion, deployment, manifestURL string, publicKey []byte) *UpdateService {
	return &UpdateService{
		db: db, current: currentVersion, deployment: deployment,
		manifestURL: strings.TrimSpace(manifestURL), publicKey: ed25519.PublicKey(publicKey),
		client: &http.Client{Timeout: 15 * time.Second}, executable: os.Executable,
	}
}

func (s *UpdateService) AttachBackups(backups *BackupService) { s.backups = backups }

func (s *UpdateService) Initialize(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE update_state SET current_version = ?, deployment = ?, updated_at = ? WHERE id = 1",
		s.current, s.deployment, dbTimestamp(time.Now()),
	)
	return err
}

func (s *UpdateService) UpdateSettings(ctx context.Context, autoCheck, autoApply *bool, maintenanceWindow *string) (UpdateState, error) {
	state, err := s.State(ctx)
	if err != nil {
		return UpdateState{}, err
	}
	if autoCheck != nil {
		state.AutoCheck = *autoCheck
	}
	if autoApply != nil {
		if *autoApply && s.deployment == "container" {
			return UpdateState{}, ErrContainerManaged
		}
		state.AutoApply = *autoApply
		if *autoApply {
			state.AutoCheck = true
		}
	}
	if maintenanceWindow != nil {
		if *maintenanceWindow == "" {
			state.MaintenanceWindow = nil
		} else {
			if _, _, err := parseMaintenanceWindow(*maintenanceWindow); err != nil {
				return UpdateState{}, err
			}
			state.MaintenanceWindow = maintenanceWindow
		}
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE update_state SET auto_check = ?, auto_apply = ?, maintenance_window = ?, updated_at = ? WHERE id = 1`,
		state.AutoCheck, state.AutoApply, state.MaintenanceWindow, dbTimestamp(time.Now()),
	)
	if err != nil {
		return UpdateState{}, err
	}
	return s.State(ctx)
}

func parseMaintenanceWindow(value string) (int, int, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, errors.New("maintenance window must use HH:MM-HH:MM")
	}
	parse := func(raw string) (int, error) {
		parsed, err := time.Parse("15:04", raw)
		if err != nil {
			return 0, errors.New("maintenance window must use HH:MM-HH:MM")
		}
		return parsed.Hour()*60 + parsed.Minute(), nil
	}
	start, err := parse(parts[0])
	if err != nil {
		return 0, 0, err
	}
	end, err := parse(parts[1])
	if err != nil || start == end {
		return 0, 0, errors.New("maintenance window must contain a non-zero interval")
	}
	return start, end, nil
}

func (s *UpdateService) State(ctx context.Context) (UpdateState, error) {
	var state UpdateState
	var latest, window sql.NullString
	var checkedRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT current_version, latest_version, deployment, channel, auto_check, auto_apply,
		       maintenance_window, status, release_notes, COALESCE(checked_at, ''), error
		FROM update_state WHERE id = 1`,
	).Scan(&state.CurrentVersion, &latest, &state.Deployment, &state.Channel, &state.AutoCheck,
		&state.AutoApply, &window, &state.Status, &state.ReleaseNotes, &checkedRaw, &state.Error)
	if err != nil {
		return UpdateState{}, err
	}
	if latest.Valid {
		state.LatestVersion = &latest.String
	}
	if window.Valid {
		state.MaintenanceWindow = &window.String
	}
	if checkedRaw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, checkedRaw)
		if err != nil {
			return UpdateState{}, err
		}
		state.CheckedAt = &parsed
	}
	return state, nil
}

func (s *UpdateService) Check(ctx context.Context) (UpdateState, error) {
	if s.manifestURL == "" || len(s.publicKey) != ed25519.PublicKeySize {
		return UpdateState{}, ErrUpdateNotConfigured
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE update_state SET status = 'checking', error = '', updated_at = ? WHERE id = 1", dbTimestamp(time.Now())); err != nil {
		return UpdateState{}, err
	}
	raw, manifest, err := s.fetchManifest(ctx)
	if err != nil {
		s.recordUpdateFailure(ctx, err)
		return UpdateState{}, err
	}
	now := time.Now().UTC()
	status := "idle"
	if compareVersions(manifest.Version, s.current) > 0 {
		status = "available"
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE update_state
		SET current_version = ?, latest_version = ?, deployment = ?, status = ?, release_notes = ?,
		    manifest_json = ?, checked_at = ?, error = '', updated_at = ?
		WHERE id = 1`,
		s.current, manifest.Version, s.deployment, status, manifest.ReleaseNotes,
		string(raw), dbTimestamp(now), dbTimestamp(now),
	)
	if err != nil {
		return UpdateState{}, err
	}
	return s.State(ctx)
}

func (s *UpdateService) Apply(ctx context.Context, requestedVersion, actorID string) (UpdateState, error) {
	if s.deployment != "binary" {
		return UpdateState{}, ErrContainerManaged
	}
	if s.backups == nil {
		return UpdateState{}, errors.New("pre-update backup service is unavailable")
	}
	var raw string
	var latest sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT manifest_json, latest_version FROM update_state WHERE id = 1").Scan(&raw, &latest); err != nil {
		return UpdateState{}, err
	}
	if !latest.Valid || requestedVersion == "" || requestedVersion != latest.String {
		return UpdateState{}, fmt.Errorf("requested update version is not available")
	}
	manifest, err := s.verifyManifest([]byte(raw))
	if err != nil || manifest.Version != requestedVersion || compareVersions(manifest.Version, s.current) <= 0 {
		return UpdateState{}, fmt.Errorf("%w: stored manifest cannot be applied", ErrInvalidManifest)
	}
	asset, ok := s.PlatformAsset(manifest)
	if !ok {
		return UpdateState{}, fmt.Errorf("no update asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE update_state SET status = 'downloading', error = '', updated_at = ? WHERE id = 1", dbTimestamp(time.Now())); err != nil {
		return UpdateState{}, err
	}
	executablePath, err := s.executable()
	if err != nil {
		s.recordUpdateFailure(ctx, err)
		return UpdateState{}, err
	}
	executablePath, err = filepath.EvalSymlinks(executablePath)
	if err != nil {
		s.recordUpdateFailure(ctx, err)
		return UpdateState{}, err
	}
	staging, err := s.downloadAsset(ctx, asset, filepath.Dir(executablePath))
	if err != nil {
		s.recordUpdateFailure(ctx, err)
		return UpdateState{}, err
	}
	defer os.Remove(staging)
	if _, err := s.backups.Create(ctx, "pre-update", actorID); err != nil {
		s.recordUpdateFailure(ctx, err)
		return UpdateState{}, fmt.Errorf("create pre-update backup: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE update_state SET status = 'applying', updated_at = ? WHERE id = 1", dbTimestamp(time.Now())); err != nil {
		return UpdateState{}, err
	}
	rollbackPath := executablePath + ".rollback-" + strings.TrimPrefix(s.current, "v")
	_ = os.Remove(rollbackPath)
	if err := os.Rename(executablePath, rollbackPath); err != nil {
		s.recordUpdateFailure(ctx, err)
		return UpdateState{}, fmt.Errorf("preserve current binary: %w", err)
	}
	if err := os.Rename(staging, executablePath); err != nil {
		rollbackErr := os.Rename(rollbackPath, executablePath)
		if rollbackErr != nil {
			err = fmt.Errorf("install update: %v; rollback failed: %w", err, rollbackErr)
		}
		s.recordUpdateFailure(ctx, err)
		return UpdateState{}, err
	}
	if err := syncDirectory(filepath.Dir(executablePath)); err != nil {
		_ = os.Remove(executablePath)
		_ = os.Rename(rollbackPath, executablePath)
		s.recordUpdateFailure(ctx, err)
		return UpdateState{}, err
	}
	historyID, _ := identity.New("upd")
	now := dbTimestamp(time.Now())
	_, _ = s.db.ExecContext(ctx, `
		INSERT INTO update_history(id, from_version, to_version, status, detail, started_at, finished_at)
		VALUES (?, ?, ?, 'succeeded', ?, ?, ?)`, historyID, s.current, manifest.Version, rollbackPath, now, now)
	if _, err := s.db.ExecContext(ctx,
		"UPDATE update_state SET status = 'restart-required', error = '', updated_at = ? WHERE id = 1", now,
	); err != nil {
		return UpdateState{}, err
	}
	return s.State(ctx)
}

func (s *UpdateService) fetchManifest(ctx context.Context) ([]byte, UpdateManifest, error) {
	parsedURL, err := url.Parse(s.manifestURL)
	if err != nil || !validUpdateURL(parsedURL) {
		return nil, UpdateManifest{}, fmt.Errorf("%w: invalid manifest URL", ErrInvalidManifest)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, UpdateManifest{}, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, UpdateManifest{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, UpdateManifest{}, fmt.Errorf("manifest server returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxManifestBytes+1))
	if err != nil {
		return nil, UpdateManifest{}, err
	}
	if len(body) > maxManifestBytes {
		return nil, UpdateManifest{}, fmt.Errorf("%w: manifest is too large", ErrInvalidManifest)
	}
	manifest, err := s.verifyManifest(body)
	if err != nil {
		return nil, UpdateManifest{}, err
	}
	return body, manifest, nil
}

func (s *UpdateService) verifyManifest(body []byte) (UpdateManifest, error) {
	var signed signedManifest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&signed); err != nil || len(signed.Payload) == 0 {
		return UpdateManifest{}, fmt.Errorf("%w: malformed envelope", ErrInvalidManifest)
	}
	signature, err := base64.RawStdEncoding.DecodeString(signed.Signature)
	if err != nil {
		signature, err = base64.StdEncoding.DecodeString(signed.Signature)
	}
	if err != nil || !ed25519.Verify(s.publicKey, signed.Payload, signature) {
		return UpdateManifest{}, fmt.Errorf("%w: signature verification failed", ErrInvalidManifest)
	}
	var manifest UpdateManifest
	if err := json.Unmarshal(signed.Payload, &manifest); err != nil {
		return UpdateManifest{}, fmt.Errorf("%w: invalid payload", ErrInvalidManifest)
	}
	if !validVersion(manifest.Version) || len(manifest.Assets) == 0 {
		return UpdateManifest{}, fmt.Errorf("%w: version or assets missing", ErrInvalidManifest)
	}
	for _, asset := range manifest.Assets {
		if asset.OS == "" || asset.Arch == "" || asset.Size <= 0 || len(asset.SHA256) != 64 {
			return UpdateManifest{}, fmt.Errorf("%w: invalid asset", ErrInvalidManifest)
		}
		if _, err := hex.DecodeString(asset.SHA256); err != nil {
			return UpdateManifest{}, fmt.Errorf("%w: invalid asset digest", ErrInvalidManifest)
		}
		assetURL, err := url.Parse(asset.URL)
		if err != nil || !validUpdateURL(assetURL) {
			return UpdateManifest{}, fmt.Errorf("%w: invalid asset URL", ErrInvalidManifest)
		}
	}
	return manifest, nil
}

func validUpdateURL(value *url.URL) bool {
	if value == nil || value.Host == "" {
		return false
	}
	if value.Scheme == "https" {
		return true
	}
	if value.Scheme != "http" {
		return false
	}
	host := strings.Trim(strings.ToLower(value.Hostname()), "[]")
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func (s *UpdateService) downloadAsset(ctx context.Context, asset UpdateAsset, directory string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("asset server returned %s", response.Status)
	}
	if response.ContentLength > 0 && response.ContentLength != asset.Size {
		return "", errors.New("update asset size does not match manifest")
	}
	file, err := os.CreateTemp(directory, ".navax-update-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(response.Body, asset.Size+1))
	if err != nil || written != asset.Size {
		return "", errors.New("update asset size does not match manifest")
	}
	if !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), asset.SHA256) {
		return "", errors.New("update asset SHA-256 verification failed")
	}
	if err := file.Chmod(0o755); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	cleanup = false
	return path, nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (s *UpdateService) PlatformAsset(manifest UpdateManifest) (UpdateAsset, bool) {
	for _, asset := range manifest.Assets {
		if asset.OS == runtime.GOOS && asset.Arch == runtime.GOARCH {
			return asset, true
		}
	}
	return UpdateAsset{}, false
}

func (s *UpdateService) recordUpdateFailure(ctx context.Context, updateErr error) {
	now := dbTimestamp(time.Now())
	_, _ = s.db.ExecContext(ctx, "UPDATE update_state SET status = 'failed', error = ?, checked_at = ?, updated_at = ? WHERE id = 1", updateErr.Error(), now, now)
}

func validVersion(version string) bool {
	_, ok := versionParts(version)
	return ok
}

func compareVersions(left, right string) int {
	l, lok := versionParts(left)
	r, rok := versionParts(right)
	if !lok || !rok {
		return strings.Compare(left, right)
	}
	for index := range l {
		if l[index] < r[index] {
			return -1
		}
		if l[index] > r[index] {
			return 1
		}
	}
	return 0
}

func versionParts(version string) ([3]int, bool) {
	var result [3]int
	value := strings.TrimPrefix(strings.TrimSpace(version), "v")
	value = strings.SplitN(value, "-", 2)[0]
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return result, false
	}
	for index, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 {
			return result, false
		}
		result[index] = parsed
	}
	return result, true
}

func dbTimestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func assetDigest(reader io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
