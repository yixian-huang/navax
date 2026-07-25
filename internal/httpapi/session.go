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

// OptionalSession 尝试用 Cookie 解析会话；解析成功则写入 context，未带
// Cookie 或 Cookie 无效都不阻断请求——供既对匿名开放、又想在“恰好登录”时
// 返回个性化内容的端点使用（例如主题列表要在带会话时叠加调用者的私有
// 主题）。与 RequireSession 的区别只在于失败路径：那边 401，这边放行。
func OptionalSession(service *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if session, err := service.Authenticate(r.Context(), readSessionCookie(r)); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, session))
			}
			next.ServeHTTP(w, r)
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
