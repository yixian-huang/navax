package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/assets"
)

func TestAssetHandlerUploadAndPublicRead(t *testing.T) {
	db, authService, _, _, token := setupHandlerServices(t)
	service, err := assets.NewService(db, filepath.Join(t.TempDir(), "assets"))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAssetHandler(service)
	router := NewRouter(RouterOptions{
		PublicBaseURL: "https://nav.ax",
		MountAPI: func(api chi.Router) {
			handler.MountPublic(api)
			api.Group(func(protected chi.Router) {
				protected.Use(RequireSession(authService))
				handler.MountProtected(protected)
			})
		},
	})

	payload := httpTestPNG(t)
	request := multipartAssetRequest(t, "/api/v1/assets", "avatar", "avatar.png", "image/png", payload, "")
	request.Header.Set("Origin", "https://nav.ax")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload status = %d", response.Code)
	}

	request = multipartAssetRequest(t, "/api/v1/assets", "avatar", "avatar.png", "image/png", payload, token)
	request.Header.Set("Origin", "https://nav.ax")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("upload status = %d: %s", response.Code, response.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	assetURL := data["url"].(string)
	if !strings.HasPrefix(assetURL, "/api/v1/assets/avatar/") || data["mimeType"] != "image/png" {
		t.Fatalf("asset response = %+v", data)
	}

	readRequest := httptest.NewRequest(http.MethodGet, assetURL, nil)
	readResponse := httptest.NewRecorder()
	router.ServeHTTP(readResponse, readRequest)
	if readResponse.Code != http.StatusOK || !bytes.Equal(readResponse.Body.Bytes(), payload) {
		t.Fatalf("read status=%d body-size=%d", readResponse.Code, readResponse.Body.Len())
	}
	if readResponse.Header().Get("X-Content-Type-Options") != "nosniff" || readResponse.Header().Get("ETag") == "" || readResponse.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("security/cache headers = %+v", readResponse.Header())
	}
	conditional := httptest.NewRequest(http.MethodGet, assetURL, nil)
	conditional.Header.Set("If-None-Match", readResponse.Header().Get("ETag"))
	conditionalResponse := httptest.NewRecorder()
	router.ServeHTTP(conditionalResponse, conditional)
	if conditionalResponse.Code != http.StatusNotModified {
		t.Fatalf("conditional read status = %d", conditionalResponse.Code)
	}

	svgRequest := multipartAssetRequest(t, "/api/v1/assets", "avatar", "avatar.svg", "image/svg+xml", []byte(`<svg><script>alert(1)</script></svg>`), token)
	svgRequest.Header.Set("Origin", "https://nav.ax")
	svgResponse := httptest.NewRecorder()
	router.ServeHTTP(svgResponse, svgRequest)
	if svgResponse.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("SVG upload status = %d: %s", svgResponse.Code, svgResponse.Body.String())
	}
}

func multipartAssetRequest(t *testing.T, target, kind, filename, mimeType string, payload []byte, token string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("kind", kind); err != nil {
		t.Fatal(err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	header.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	}
	return request
}

func httpTestPNG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x * 16), G: uint8(y * 16), B: 96, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
