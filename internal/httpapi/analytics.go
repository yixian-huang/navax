package httpapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/analytics"
)

type AnalyticsHandler struct {
	service *analytics.Service
}

func NewAnalyticsHandler(service *analytics.Service) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

func (h *AnalyticsHandler) MountPublic(router chi.Router) {
	router.Post("/public/events", h.record)
}

func (h *AnalyticsHandler) MountProtected(router chi.Router) {
	router.Get("/me/analytics/overview", h.overview)
	router.Get("/me/analytics/trends", h.trends)
	router.Get("/me/analytics/breakdown", h.breakdown)
}

func (h *AnalyticsHandler) record(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Type          string `json:"type"`
		PageID        string `json:"pageId"`
		SnapshotID    string `json:"snapshotId"`
		SiteID        string `json:"siteId"`
		ClientEventID string `json:"clientEventId"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "统计事件格式无效", nil)
		return
	}
	err := h.service.Record(r.Context(), analytics.Event{
		Type: request.Type, PageID: request.PageID, SnapshotID: request.SnapshotID,
		SiteID: request.SiteID, ClientEventID: request.ClientEventID,
		ClientAddress: remoteAddress(r), UserAgent: r.UserAgent(), Referrer: r.Referer(),
	})
	switch {
	case err == nil, errors.Is(err, analytics.ErrDisabled), errors.Is(err, analytics.ErrNotFound):
		WriteJSON(w, r, http.StatusAccepted, nil)
	case errors.Is(err, analytics.ErrInvalid):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "统计事件无效", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "记录统计事件失败", nil)
	}
}

func (h *AnalyticsHandler) overview(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Overview(r.Context(), currentUserID(r), queryInt(r, "period", 30))
	if err != nil {
		h.writeReadError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, value)
}

func (h *AnalyticsHandler) trends(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Trends(r.Context(), currentUserID(r), queryInt(r, "period", 30))
	if err != nil {
		h.writeReadError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, value)
}

func (h *AnalyticsHandler) breakdown(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Breakdown(r.Context(), currentUserID(r), queryInt(r, "period", 30))
	if err != nil {
		h.writeReadError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, value)
}

func (h *AnalyticsHandler) writeReadError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, analytics.ErrNotFound) {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "个人导航页不存在", nil)
		return
	}
	WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取访问统计失败", nil)
}

func currentUserID(r *http.Request) string {
	session, _ := SessionFromContext(r.Context())
	return session.User.ID
}

func remoteAddress(r *http.Request) string {
	value := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return value
}
