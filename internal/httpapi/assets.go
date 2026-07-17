package httpapi

import (
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/assets"
)

const multipartOverheadAllowance int64 = 1 << 20

type AssetHandler struct {
	service *assets.Service
}

func NewAssetHandler(service *assets.Service) *AssetHandler {
	return &AssetHandler{service: service}
}

// MountPublic exposes immutable raster assets. Object keys are validated and
// checked against SQLite before any filesystem path is opened.
func (h *AssetHandler) MountPublic(router chi.Router) {
	router.Get("/assets/*", h.read)
	router.Head("/assets/*", h.read)
}

// MountProtected mounts upload on a router that already requires a session.
func (h *AssetHandler) MountProtected(router chi.Router) {
	router.Post("/assets", h.upload)
}

func (h *AssetHandler) upload(w http.ResponseWriter, r *http.Request) {
	maximum, err := h.service.MaxUploadBytes(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取上传限制失败", nil)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		WriteError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "上传必须使用 multipart/form-data", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maximum+multipartOverheadAllowance)
	if err := r.ParseMultipartForm(minimum(maximum, multipartOverheadAllowance)); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) || errors.Is(err, multipart.ErrMessageTooLarge) {
			WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "图片超过系统上传限制", nil)
			return
		}
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "multipart 上传内容无效", nil)
		return
	}
	defer r.MultipartForm.RemoveAll()
	if len(r.MultipartForm.Value["kind"]) != 1 || len(r.MultipartForm.File["file"]) != 1 {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "kind 和 file 必须各提供一次", nil)
		return
	}
	kind := r.MultipartForm.Value["kind"][0]
	header := r.MultipartForm.File["file"][0]
	if header.Size > maximum {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "图片超过系统上传限制", nil)
		return
	}
	file, err := header.Open()
	if err != nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "无法读取上传图片", nil)
		return
	}
	defer file.Close()
	session, ok := SessionFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录", nil)
		return
	}
	asset, err := h.service.Upload(
		r.Context(), session.User.ID, kind, header.Filename, header.Header.Get("Content-Type"), file,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, map[string]any{
		"id": asset.ID, "kind": asset.Kind, "url": asset.URL,
		"mimeType": asset.MIMEType, "size": asset.Size, "createdAt": asset.CreatedAt,
	})
}

func (h *AssetHandler) read(w http.ResponseWriter, r *http.Request) {
	objectKey := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	asset, body, err := h.service.Open(r.Context(), objectKey)
	if err != nil {
		if errors.Is(err, assets.ErrInvalidObject) || errors.Is(err, assets.ErrNotFound) {
			WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "图片不存在", nil)
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取图片失败", nil)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", asset.MIMEType)
	w.Header().Set("Content-Length", strconv.FormatInt(asset.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+asset.SHA256+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
	// Allow video playback elements; still sandbox scripts.
	if strings.HasPrefix(asset.MIMEType, "video/") {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; media-src 'self'; sandbox")
		w.Header().Set("Accept-Ranges", "bytes")
	} else {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	}
	http.ServeContent(w, r, objectKey, asset.CreatedAt, body)
}

func (h *AssetHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, assets.ErrTooLarge):
		WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "图片超过系统上传限制", nil)
	case errors.Is(err, assets.ErrUnsupported), errors.Is(err, assets.ErrInvalidImage):
		WriteError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "仅支持有效的 PNG、JPEG、GIF 或 WebP 图片", nil)
	case errors.Is(err, assets.ErrImageTooSmall):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "背景图过小（宽高至少 64px），请上传清晰的壁纸图", nil)
	case errors.Is(err, assets.ErrInvalidKind):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "kind 必须是 avatar、background 或 site-icon", nil)
	case errors.Is(err, assets.ErrInvalidOwner):
		WriteError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录", nil)
	case errors.Is(err, assets.ErrStorage):
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "对象存储暂不可用", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "保存图片失败", nil)
	}
}

func minimum(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
