package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	verrors "github.com/pb33f/libopenapi-validator/errors"
)

// apiClient 按角色（游客/用户/管理员）持有独立 Cookie Jar 的 HTTP 客户端，
// 每次请求都会将请求与响应交给 OpenAPI 校验器。
type apiClient struct {
	http *http.Client
}

func newAPIClient(t *testing.T) *apiClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("创建 cookie jar: %v", err)
	}
	return &apiClient{http: &http.Client{Jar: jar, Timeout: 15 * time.Second}}
}

type callOptions struct {
	headers            map[string]string
	skipReqValidation  bool
	skipRespValidation bool
}

type callOption func(*callOptions)

func withHeader(key, value string) callOption {
	return func(o *callOptions) {
		if o.headers == nil {
			o.headers = map[string]string{}
		}
		o.headers[key] = value
	}
}

// withoutRequestValidation 用于刻意构造“不满足契约前置条件”的请求
// （例如未认证访问受保护端点），此时只校验响应。
func withoutRequestValidation() callOption {
	return func(o *callOptions) { o.skipReqValidation = true }
}

type apiResult struct {
	status int
	header http.Header
	body   []byte
	json   map[string]any
}

// data 返回响应 envelope 中的 data 对象；非对象时返回 nil。
func (r apiResult) data() map[string]any {
	if r.json == nil {
		return nil
	}
	data, _ := r.json["data"].(map[string]any)
	return data
}

func (c *apiClient) call(t *testing.T, method, path string, body any, opts ...callOption) apiResult {
	t.Helper()

	options := callOptions{}
	for _, apply := range opts {
		apply(&options)
	}

	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("编码请求体 %s %s: %v", method, path, err)
		}
		payload = encoded
	}

	request, err := http.NewRequest(method, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("构造请求 %s %s: %v", method, path, err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet && method != http.MethodHead {
		request.Header.Set("Origin", baseURL)
	}
	for key, value := range options.headers {
		request.Header.Set(key, value)
	}

	response, err := c.http.Do(request)
	if err != nil {
		t.Fatalf("请求 %s %s: %v", method, path, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("读取响应 %s %s: %v", method, path, err)
	}

	// client.Do 已消耗请求体，校验前重置。
	request.Body = io.NopCloser(bytes.NewReader(payload))
	if !options.skipReqValidation {
		if ok, validationErrs := apiValidator.ValidateHttpRequest(request); !ok {
			reportValidationErrors(t, "请求", method, path, validationErrs)
		}
	}
	if !options.skipRespValidation {
		request.Body = io.NopCloser(bytes.NewReader(payload))
		response.Body = io.NopCloser(bytes.NewReader(responseBody))
		if ok, validationErrs := apiValidator.ValidateHttpResponse(request, response); !ok {
			reportValidationErrors(t, "响应", method, path, validationErrs)
		}
	}

	result := apiResult{status: response.StatusCode, header: response.Header, body: responseBody}
	if strings.Contains(response.Header.Get("Content-Type"), "application/json") && len(responseBody) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(responseBody, &parsed); err != nil {
			t.Fatalf("解析响应 JSON %s %s: %v\n%s", method, path, err, responseBody)
		}
		result.json = parsed
	}
	return result
}

// uploadMultipart 以 multipart/form-data 向任意端点上传字段与一个文件，只
// 校验响应（multipart 请求体不做契约请求校验）。
func (c *apiClient) uploadMultipart(t *testing.T, path string, fields map[string]string, fileField, filename string, content []byte) apiResult {
	t.Helper()

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("写入字段 %s: %v", key, err)
		}
	}
	part, err := writer.CreateFormFile(fileField, filename)
	if err != nil {
		t.Fatalf("创建 %s 字段: %v", fileField, err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("写入文件内容: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("关闭 multipart writer: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, baseURL+path, &buffer)
	if err != nil {
		t.Fatalf("构造上传请求 %s: %v", path, err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", baseURL)

	response, err := c.http.Do(request)
	if err != nil {
		t.Fatalf("上传请求 %s: %v", path, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("读取上传响应 %s: %v", path, err)
	}

	response.Body = io.NopCloser(bytes.NewReader(responseBody))
	if ok, validationErrs := apiValidator.ValidateHttpResponse(request, response); !ok {
		reportValidationErrors(t, "响应", http.MethodPost, path, validationErrs)
	}

	result := apiResult{status: response.StatusCode, header: response.Header, body: responseBody}
	if strings.Contains(response.Header.Get("Content-Type"), "application/json") && len(responseBody) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(responseBody, &parsed); err != nil {
			t.Fatalf("解析上传响应 JSON %s: %v\n%s", path, err, responseBody)
		}
		result.json = parsed
	}
	return result
}

// uploadPNG 是 uploadMultipart 的薄封装：上传一张最小 PNG 到资源端点。
func (c *apiClient) uploadPNG(t *testing.T, kind string, png []byte) apiResult {
	t.Helper()
	return c.uploadMultipart(t, "/api/v1/assets", map[string]string{"kind": kind}, "file", "background.png", png)
}

func reportValidationErrors(t *testing.T, kind, method, path string, validationErrs []*verrors.ValidationError) {
	t.Helper()
	for _, validationErr := range validationErrs {
		details := validationErr.Message
		if validationErr.Reason != "" {
			details += " — " + validationErr.Reason
		}
		for _, schemaErr := range validationErr.SchemaValidationErrors {
			details += "\n    schema: " + schemaErr.Reason + " @ " + schemaErr.FieldPath
		}
		t.Errorf("%s契约校验失败 %s %s:\n  %s", kind, method, path, details)
	}
}

// mustStatus 断言状态码并在失败时打印响应体。
func mustStatus(t *testing.T, result apiResult, expected int, context string) {
	t.Helper()
	if result.status != expected {
		t.Fatalf("%s: 期望状态 %d，实际 %d\n%s", context, expected, result.status, result.body)
	}
}

func stringField(t *testing.T, object map[string]any, field, context string) string {
	t.Helper()
	value, ok := object[field].(string)
	if !ok || value == "" {
		t.Fatalf("%s: 缺少字符串字段 %q，对象为 %v", context, field, object)
	}
	return value
}

func numberField(t *testing.T, object map[string]any, field, context string) int {
	t.Helper()
	value, ok := object[field].(float64)
	if !ok {
		t.Fatalf("%s: 缺少数字字段 %q，对象为 %v", context, field, object)
	}
	return int(value)
}
