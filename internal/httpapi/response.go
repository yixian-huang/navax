package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

type responseMeta struct {
	RequestID string            `json:"requestId"`
	Message   string            `json:"message,omitempty"`
	Detail    string            `json:"detail,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

type envelope struct {
	Code string       `json:"code"`
	Data any          `json:"data"`
	Meta responseMeta `json:"meta"`
}

func WriteJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	writeEnvelope(w, r, status, envelope{
		Code: "OK",
		Data: data,
		Meta: responseMeta{RequestID: middleware.GetReqID(r.Context())},
	})
}

func WriteRawJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Request-ID", middleware.GetReqID(r.Context()))
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.ErrorContext(r.Context(), "encode raw HTTP response", "error", err)
	}
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, err error) {
	detail := ""
	if err != nil && status < http.StatusInternalServerError {
		detail = err.Error()
	}
	writeEnvelope(w, r, status, envelope{
		Code: code,
		Data: nil,
		Meta: responseMeta{RequestID: middleware.GetReqID(r.Context()), Message: message, Detail: detail},
	})
}

func writeEnvelope(w http.ResponseWriter, r *http.Request, status int, value envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Request-ID", value.Meta.RequestID)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.ErrorContext(r.Context(), "encode HTTP response", "error", err)
	}
}
