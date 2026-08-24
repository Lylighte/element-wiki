package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"element-wiki/internal/permission"
	authservice "element-wiki/internal/service/authservice"
)

// ctxKey actor 上下文键。
type ctxKey int

const actorKey ctxKey = 1

// WithActor 将已解析身份写入请求上下文。
func WithActor(r *http.Request, a permission.Actor) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), actorKey, a))
}

// ActorFrom 取回身份；未注入时返回 nil。
func ActorFrom(r *http.Request) permission.Actor {
	a, _ := r.Context().Value(actorKey).(permission.Actor)
	return a
}

const sessionCookie = "access_token"

// authMiddleware 解析身份并注入上下文。
// 优先级：Authorization: Bearer → 会话 cookie → 匿名（按站点开关）。
func authMiddleware(auth *authservice.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, err := resolveActor(r, auth)
		switch {
		case err == nil:
			// 有效身份
		case strings.Contains(err.Error(), "disabled"):
			// 禁用账号显式 403，绝不降级为匿名
			writeErr(w, http.StatusForbidden, "account disabled")
			return
		default:
			actor = auth.AnonymousActor()
			// 契约 §14：匿名模式关闭时，/v1 一律 401（auth 域除外）
			if !auth.AnonymousEnabled() &&
				strings.HasPrefix(r.URL.Path, "/v1/") &&
				!strings.HasPrefix(r.URL.Path, "/v1/auth/") {
				writeErr(w, http.StatusUnauthorized, "unauthenticated")
				return
			}
		}
		next.ServeHTTP(w, WithActor(r, actor))
	})
}

func resolveActor(r *http.Request, auth *authservice.Service) (permission.Actor, error) {
	ctx := r.Context()
	if h := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return auth.ActorFromBearer(ctx, strings.TrimSpace(h[7:]))
	}
	c, cerr := r.Cookie(sessionCookie)
	if cerr == nil && c.Value != "" {
		return auth.ActorFromSession(ctx, c.Value)
	}
	return nil, authservice.ErrUnauthenticated
}

// sessionCookieOpts 按 secure_cookies 配置输出会话 cookie。
func setSessionCookie(w http.ResponseWriter, cfg cookieCfg, val string, expiresMs int64, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func clearSessionCookie(w http.ResponseWriter, cfg cookieCfg) {
	setSessionCookie(w, cfg, "", 0, -1)
}

type cookieCfg struct{ secureCookies bool }

// Deps 附件/上传辅助取值（main 装配时注入）。
func (d *Deps) maxUploadBytes() int64 { return d.UploadMaxBytes }

func (d *Deps) attachRoot() string { return d.AttachDir }

func sanitizeHeaderFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\r' || r == '\n' {
			r = '-'
		}
		out = append(out, r)
	}
	return string(out)
}

func modTimeZero() time.Time { return time.Time{} }
