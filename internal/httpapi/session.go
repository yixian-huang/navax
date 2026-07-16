package httpapi

import (
	"context"
	"net/http"

	"github.com/yixian-huang/navax/internal/auth"
)

type sessionContextKey struct{}

func RequireSession(service *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := service.Authenticate(r.Context(), readSessionCookie(r))
			if err != nil {
				WriteError(w, r, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录", nil)
				return
			}
			ctx := context.WithValue(r.Context(), sessionContextKey{}, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := SessionFromContext(r.Context())
		if !ok || session.User.Role != "admin" {
			WriteError(w, r, http.StatusForbidden, "ADMIN_REQUIRED", "需要管理员权限", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func SessionFromContext(ctx context.Context) (auth.Session, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(auth.Session)
	return session, ok
}
