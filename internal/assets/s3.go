package assets

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yixian-huang/navax/internal/netguard"
)

// S3Config holds credentials and endpoint details for an S3-compatible bucket.
type S3Config struct {
	Endpoint      string
	Region        string
	Bucket        string
	Prefix        string
	AccessKey     string
	SecretKey     string
	PathStyle     bool
	PublicBaseURL string
	HTTPClient    *http.Client
}

type s3Store struct {
	cfg S3Config
}

func newS3Store(cfg S3Config) (*s3Store, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3 endpoint and bucket are required")
	}
	if strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, fmt.Errorf("s3 access key and secret key are required")
	}
	if strings.TrimSpace(cfg.Region) == "" {
		cfg.Region = "us-east-1"
	}
	cfg.Prefix = strings.Trim(strings.TrimSpace(cfg.Prefix), "/")
	if cfg.HTTPClient == nil {
		// Storage endpoints may be private in self-hosted deploys; block loopback/metadata.
		cfg.HTTPClient = netguard.GuardedClient(netguard.NewInternalValidator(nil), 30*time.Second, 3)
	}
	return &s3Store{cfg: cfg}, nil
}

func (s *s3Store) Put(ctx context.Context, objectKey, contentType string, body io.Reader, size int64) error {
	payload, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil {
		return fmt.Errorf("read s3 upload body: %w", err)
	}
	if int64(len(payload)) != size {
		return fmt.Errorf("s3 upload size mismatch")
	}
	key := s.objectKey(objectKey)
	req, err := s.newRequest(ctx, http.MethodPut, key, payload, contentType)
	if err != nil {
		return err
	}
	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("s3 put object: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("s3 put object: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

func (s *s3Store) Open(ctx context.Context, objectKey string) (io.ReadCloser, int64, string, error) {
	key := s.objectKey(objectKey)
	req, err := s.newRequest(ctx, http.MethodGet, key, nil, "")
	if err != nil {
		return nil, 0, "", err
	}
	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("s3 get object: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		return nil, 0, "", ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		return nil, 0, "", fmt.Errorf("s3 get object: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return resp.Body, resp.ContentLength, resp.Header.Get("Content-Type"), nil
}

func (s *s3Store) PublicURL(objectKey string) string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.PublicBaseURL), "/")
	if base == "" {
		return publicURLPrefix + objectKey
	}
	key := s.objectKey(objectKey)
	return base + "/" + strings.TrimPrefix(key, "/")
}

func (s *s3Store) objectKey(objectKey string) string {
	if s.cfg.Prefix == "" {
		return objectKey
	}
	return s.cfg.Prefix + "/" + objectKey
}

func (s *s3Store) newRequest(ctx context.Context, method, objectKey string, payload []byte, contentType string) (*http.Request, error) {
	endpoint, err := url.Parse(s.cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse s3 endpoint: %w", err)
	}
	host := endpoint.Host
	var path string
	if s.cfg.PathStyle || isIPHost(host) {
		path = "/" + s.cfg.Bucket + "/" + objectKey
	} else {
		host = s.cfg.Bucket + "." + host
		path = "/" + objectKey
	}
	target := *endpoint
	target.Host = host
	target.Path = path
	target.RawQuery = ""

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payloadHash := sha256Hex(payload)
	if payload == nil {
		payloadHash = sha256Hex(nil)
	}

	headers := map[string]string{
		"host":                 host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	if contentType != "" {
		headers["content-type"] = contentType
	}

	canonicalHeaders, signedHeaders := canonicalSignedHeaders(headers)
	canonicalRequest := strings.Join([]string{
		method,
		uriEncodePath(path),
		"",
		canonicalHeaders + "\n",
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := dateStamp + "/" + s.cfg.Region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := hex.EncodeToString(hmacSHA256(signingKey(s.cfg.SecretKey, dateStamp, s.cfg.Region, "s3"), []byte(stringToSign)))
	authorization := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AccessKey, credentialScope, signedHeaders, signature,
	)

	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	for name, value := range headers {
		if name == "host" {
			continue
		}
		req.Header.Set(name, value)
	}
	req.Header.Set("Authorization", authorization)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if payload != nil {
		req.ContentLength = int64(len(payload))
	}
	return req, nil
}

func isIPHost(host string) bool {
	h := host
	if i := strings.LastIndex(host, ":"); i >= 0 && strings.Count(host, ":") == 1 {
		h = host[:i]
	}
	return net.ParseIP(h) != nil
}

func canonicalSignedHeaders(headers map[string]string) (string, string) {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte(':')
		b.WriteString(strings.TrimSpace(headers[name]))
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n") + "\n", strings.Join(names, ";")
}

func uriEncodePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
		parts[i] = strings.ReplaceAll(parts[i], "+", "%20")
	}
	return strings.Join(parts, "/")
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func signingKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}
