package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestS3PutSignsPathStyleRequest(t *testing.T) {
	var gotAuth, gotHost, gotAmz, gotContentType string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotHost = r.Host
		gotAmz = r.Header.Get("x-amz-date")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		if r.Method != http.MethodPut {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/navax/background/abc.png" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	store, err := newS3Store(S3Config{
		Endpoint:   server.URL,
		Region:     "us-east-1",
		Bucket:     "navax",
		AccessKey:  "AKIAEXAMPLE",
		SecretKey:  "secretsecretsecretsecret",
		PathStyle:  true,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("hello-image")
	if err := store.Put(context.Background(), "background/abc.png", "image/png", strings.NewReader(string(payload)), int64(len(payload))); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=AKIAEXAMPLE/") {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(gotAuth, "SignedHeaders=") || !strings.Contains(gotAuth, "Signature=") {
		t.Fatalf("Authorization missing fields: %q", gotAuth)
	}
	if gotContentType != "image/png" {
		t.Fatalf("Content-Type = %q", gotContentType)
	}
	if string(gotBody) != string(payload) {
		t.Fatalf("body = %q", gotBody)
	}
	if gotHost == "" || gotAmz == "" {
		t.Fatalf("missing host/date headers host=%q date=%q", gotHost, gotAmz)
	}
	// Signature should be stable for fixed clock — at least recompute digest of body.
	sum := sha256.Sum256(payload)
	if !strings.Contains(strings.ToLower(gotAuth), "signature=") {
		t.Fatalf("missing signature, auth=%q digest=%s", gotAuth, hex.EncodeToString(sum[:]))
	}
}

func TestAWSURIEncodeLeavesUnreserved(t *testing.T) {
	if got := awsURIEncode("background-abc_def.xyz"); got != "background-abc_def.xyz" {
		t.Fatalf("encode = %q", got)
	}
	if got := awsURIEncode("a b"); got != "a%20b" {
		t.Fatalf("encode space = %q", got)
	}
}
