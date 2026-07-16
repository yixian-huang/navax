package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yixian-huang/navax/internal/integrations"
)

type ProviderHandler struct {
	service *integrations.Service
}

func NewProviderHandler(service *integrations.Service) *ProviderHandler {
	return &ProviderHandler{service: service}
}

func (h *ProviderHandler) Mount(router chi.Router) {
	router.Get("/providers", h.list)
	router.Get("/providers/{kind}", h.get)
	router.Patch("/providers/{kind}", h.update)
	router.Post("/providers/{kind}/test", h.test)
}

func (h *ProviderHandler) list(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.List(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "读取服务配置失败", nil)
		return
	}
	summaries := make([]map[string]any, 0, len(providers))
	for _, provider := range providers {
		summaries = append(summaries, map[string]any{
			"kind": provider.Kind, "enabled": provider.Enabled, "configured": provider.Configured,
			"hasSecret": provider.HasSecret, "updatedAt": provider.UpdatedAt,
		})
	}
	WriteJSON(w, r, http.StatusOK, summaries)
}

func (h *ProviderHandler) get(w http.ResponseWriter, r *http.Request) {
	provider, err := h.service.Get(r.Context(), integrations.Kind(chi.URLParam(r, "kind")))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, provider)
}

func (h *ProviderHandler) update(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Enabled  bool              `json:"enabled"`
		Settings map[string]any    `json:"settings"`
		Secrets  map[string]string `json:"secrets"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	provider, err := h.service.Update(r.Context(), integrations.Kind(chi.URLParam(r, "kind")), integrations.Update{
		Enabled: request.Enabled, Settings: request.Settings, Secrets: request.Secrets,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, provider)
}

func (h *ProviderHandler) test(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Test(r.Context(), integrations.Kind(chi.URLParam(r, "kind")))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, result)
}

func (h *ProviderHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, integrations.ErrInvalidProvider):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "服务类型不存在", nil)
	case errors.Is(err, integrations.ErrInvalidSettings), errors.Is(err, integrations.ErrMasterKeyRequired):
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务配置操作失败", nil)
	}
}
