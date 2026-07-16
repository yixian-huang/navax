package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/linkcheck"
	"github.com/yixian-huang/navax/internal/navigation"
)

type LinkCheckHandler struct {
	service *linkcheck.Service
}

func NewLinkCheckHandler(service *linkcheck.Service) *LinkCheckHandler {
	return &LinkCheckHandler{service: service}
}

// MountProtected must be called inside the session-protected /api/v1 group.
func (h *LinkCheckHandler) MountProtected(router chi.Router) {
	router.Post("/pages/{pageId}/link-checks", h.check)
}

func (h *LinkCheckHandler) check(w http.ResponseWriter, r *http.Request) {
	var request struct {
		SiteIDs []string `json:"siteIds"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	results, err := h.service.Check(r.Context(), navigationActor(r), chi.URLParam(r, "pageId"), request.SiteIDs)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, results)
}

func (h *LinkCheckHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, linkcheck.ErrBusy):
		w.Header().Set("Retry-After", "5")
		WriteError(w, r, http.StatusTooManyRequests, "LINK_CHECK_BUSY", "链接检查任务繁忙，请稍后重试", nil)
	case errors.Is(err, linkcheck.ErrInvalid):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), nil)
	case errors.Is(err, navigation.ErrForbidden):
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "没有该导航页的访问权限", nil)
	case errors.Is(err, navigation.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "导航页不存在", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "链接检查失败", nil)
	}
}
