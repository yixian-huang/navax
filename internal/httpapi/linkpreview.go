package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/linkpreview"
)

type LinkPreviewHandler struct {
	service *linkpreview.Service
}

func NewLinkPreviewHandler(service *linkpreview.Service) *LinkPreviewHandler {
	return &LinkPreviewHandler{service: service}
}

func (h *LinkPreviewHandler) MountProtected(router chi.Router) {
	router.Post("/link-preview", h.preview)
}

func (h *LinkPreviewHandler) preview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	body.URL = strings.TrimSpace(body.URL)
	if body.URL == "" {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请提供 url", nil)
		return
	}
	result, err := h.service.Preview(r.Context(), body.URL)
	if err != nil {
		switch {
		case errors.Is(err, linkpreview.ErrInvalidURL):
			WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "链接格式无效", nil)
		case errors.Is(err, linkpreview.ErrBlocked):
			WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "不允许预览该地址", nil)
		default:
			WriteError(w, r, http.StatusBadGateway, "UPSTREAM_ERROR", "无法获取站点信息", nil)
		}
		return
	}
	WriteJSON(w, r, http.StatusOK, result)
}
