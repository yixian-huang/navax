package httpapi

import (
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/backgrounds"
)

type BackgroundHandler struct {
	service *backgrounds.Service
}

func NewBackgroundHandler(service *backgrounds.Service) *BackgroundHandler {
	return &BackgroundHandler{service: service}
}

func (h *BackgroundHandler) MountProtected(router chi.Router) {
	router.Get("/backgrounds/presets", h.listPresets)
	router.Post("/backgrounds/presets", h.uploadPreset)
	router.Delete("/backgrounds/presets/{id}", h.deletePreset)
	router.Get("/backgrounds/mine", h.listMine)
	router.Post("/backgrounds/mine", h.uploadMine)
	router.Delete("/backgrounds/mine/{id}", h.deleteMine)
}

func (h *BackgroundHandler) listPresets(w http.ResponseWriter, r *http.Request) {
	// Non-admin only sees enabled; admin sees all when ?all=1
	session, _ := SessionFromContext(r.Context())
	includeDisabled := session.User.Role == "admin" && r.URL.Query().Get("all") == "1"
	list, err := h.service.ListPresets(r.Context(), includeDisabled)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取预设背景失败", nil)
		return
	}
	WriteJSON(w, r, http.StatusOK, list)
}

func (h *BackgroundHandler) listMine(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录", nil)
		return
	}
	list, err := h.service.ListMine(r.Context(), session.User.ID)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取我的背景失败", nil)
		return
	}
	WriteJSON(w, r, http.StatusOK, list)
}

func (h *BackgroundHandler) uploadPreset(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok || session.User.Role != "admin" {
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "仅管理员可上传预设背景", nil)
		return
	}
	file, header, err := h.parseFile(r)
	if err != nil {
		h.writeUploadErr(w, r, err)
		return
	}
	defer file.Close()
	media, err := h.service.UploadPreset(r.Context(), session.User.ID, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		h.writeServiceErr(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, media)
}

func (h *BackgroundHandler) uploadMine(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录", nil)
		return
	}
	file, header, err := h.parseFile(r)
	if err != nil {
		h.writeUploadErr(w, r, err)
		return
	}
	defer file.Close()
	media, err := h.service.UploadMine(r.Context(), session.User.ID, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		h.writeServiceErr(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, media)
}

func (h *BackgroundHandler) deletePreset(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok || session.User.Role != "admin" {
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "仅管理员可删除预设背景", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), id, session.User.ID, true); err != nil {
		h.writeServiceErr(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{"deleted": true})
}

func (h *BackgroundHandler) deleteMine(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), id, session.User.ID, session.User.Role == "admin"); err != nil {
		h.writeServiceErr(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, map[string]any{"deleted": true})
}

func (h *BackgroundHandler) parseFile(r *http.Request) (multipart.File, *multipart.FileHeader, error) {
	const maxBytes = 40 << 20 // 40MB pre-compress
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		return nil, nil, errUnsupportedMedia
	}
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes+multipartOverheadAllowance)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return nil, nil, err
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, nil, err
	}
	return file, header, nil
}

var errUnsupportedMedia = errors.New("unsupported media")

func (h *BackgroundHandler) writeUploadErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errUnsupportedMedia) {
		WriteError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "请使用 multipart 上传文件", nil)
		return
	}
	WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "无法读取上传文件", nil)
}

func (h *BackgroundHandler) writeServiceErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, backgrounds.ErrQuota):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), nil)
	case errors.Is(err, backgrounds.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "背景媒体不存在", nil)
	case errors.Is(err, backgrounds.ErrForbidden):
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "无权操作该背景", nil)
	case errors.Is(err, backgrounds.ErrFFmpegRequired):
		WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "服务器未安装 ffmpeg，暂不支持视频背景", nil)
	case errors.Is(err, backgrounds.ErrVideoTooLong):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "视频时长不能超过 15 秒", nil)
	case errors.Is(err, backgrounds.ErrInvalidFile):
		WriteError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "不支持的文件类型或内容损坏", nil)
	case errors.Is(err, backgrounds.ErrInvalidFile) || strings.Contains(err.Error(), "too small"):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), nil)
	default:
		msg := "保存背景失败"
		if err != nil && strings.TrimSpace(err.Error()) != "" {
			// Surface a short cause for operators without leaking stack traces.
			msg = "保存背景失败: " + err.Error()
			if len(msg) > 200 {
				msg = msg[:200]
			}
		}
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", msg, nil)
	}
}
