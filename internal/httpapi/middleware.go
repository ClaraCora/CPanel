package httpapi

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"cpanel/internal/auth"
	"cpanel/internal/domain"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	clientIPKey  contextKey = "client_ip"
	adminKey     contextKey = "admin"
	sessionKey   contextKey = "session"
	portalKey    contextKey = "portal"
	agentKey     contextKey = "agent"
)

func (s *Server) commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" || len(requestID) > 100 {
			requestID = domain.MustID("req")
		}
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if sensitiveResponsePath(r.URL.Path) {
			noStore(w)
		}

		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		if clientIPRequired(r.URL.Path) && s.store != nil {
			trustedProxies := s.store.SettingString(ctx, "security", "trusted_proxy_cidrs", "")
			ctx = context.WithValue(ctx, clientIPKey, resolvedClientIP(r, trustedProxies))
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func sensitiveResponsePath(path string) bool {
	for _, prefix := range []string{"/ca/ht/", "/ca/edu/", "/ca/x/", "/ca/cc/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func clientIPRequired(path string) bool {
	for _, prefix := range []string{"/ca/ht/", "/ca/edu/", "/ca/x/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = debug.Stack()
				writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func localHealth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !isLocalHealthHost(remoteHost) {
			http.NotFound(w, r)
			return
		}
		requestHost := r.Host
		if host, _, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
			requestHost = host
		}
		if !isLocalHealthHost(requestHost) {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

func isLocalHealthHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, r, http.StatusUnauthorized, "ADMIN_AUTH_REQUIRED", "请先登录管理后台", nil)
			return
		}
		session, err := s.store.FindAdminSession(r.Context(), auth.HashSecret(cookie.Value))
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "ADMIN_SESSION_INVALID", "登录状态已失效", nil)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			provided := r.Header.Get("X-CSRF-Token")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(session.CSRFToken)) != 1 {
				writeError(w, r, http.StatusForbidden, "CSRF_TOKEN_INVALID", "安全令牌无效，请刷新页面后重试", nil)
				return
			}
		}
		ctx := context.WithValue(r.Context(), adminKey, session.Admin)
		ctx = context.WithValue(ctx, sessionKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireAgent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, r, http.StatusUnauthorized, "AGENT_AUTH_REQUIRED", "Agent 凭据缺失", nil)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		tokenHash := auth.HashSecret(token)
		machineID := strings.TrimSpace(r.Header.Get("X-CPanel-Machine-ID"))
		machine, err := s.store.AuthenticateSharedAgent(r.Context(), machineID, tokenHash)
		if err != nil {
			machine, err = s.store.AuthenticateAgent(r.Context(), tokenHash)
		}
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "AGENT_TOKEN_INVALID", "Agent 凭据无效", nil)
			return
		}
		ctx := context.WithValue(r.Context(), agentKey, machine)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requirePortal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(portalSessionCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, r, http.StatusUnauthorized, "PORTAL_AUTH_REQUIRED", "请先登录订阅中心", nil)
			return
		}
		session, err := s.store.FindPortalSession(r.Context(), auth.HashSecret(cookie.Value))
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "PORTAL_SESSION_INVALID", "登录状态已失效，请重新登录", nil)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			provided := r.Header.Get("X-CSRF-Token")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(session.CSRFToken)) != 1 {
				writeError(w, r, http.StatusForbidden, "CSRF_TOKEN_INVALID", "安全令牌无效，请刷新页面后重试", nil)
				return
			}
		}
		ctx := context.WithValue(r.Context(), portalKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireWebSession protects chunks shared by the administrator and portal
// applications. Shared chunks contain only common UI/runtime code, but they
// must still require one of the two authenticated sessions.
func (s *Server) requireWebSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
			session, findErr := s.store.FindAdminSession(r.Context(), auth.HashSecret(cookie.Value))
			if findErr == nil {
				ctx := context.WithValue(r.Context(), adminKey, session.Admin)
				ctx = context.WithValue(ctx, sessionKey, session)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		if cookie, err := r.Cookie(portalSessionCookieName); err == nil && cookie.Value != "" {
			session, findErr := s.store.FindPortalSession(r.Context(), auth.HashSecret(cookie.Value))
			if findErr == nil {
				ctx := context.WithValue(r.Context(), portalKey, session)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		writeError(w, r, http.StatusUnauthorized, "WEB_AUTH_REQUIRED", "请先登录", nil)
	})
}

func requestID(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey).(string)
	return value
}

func currentAdmin(r *http.Request) domain.Admin {
	value, _ := r.Context().Value(adminKey).(domain.Admin)
	return value
}

func currentPortalSession(r *http.Request) domain.PortalSession {
	session, _ := r.Context().Value(portalKey).(domain.PortalSession)
	return session
}

func currentAgent(r *http.Request) domain.AgentMachine {
	value, _ := r.Context().Value(agentKey).(domain.AgentMachine)
	return value
}

func clientIP(r *http.Request) string {
	if value, ok := r.Context().Value(clientIPKey).(string); ok && value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) expireSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: s.secureCookies(r.Context()), SameSite: http.SameSiteStrictMode, MaxAge: -1,
		Expires: time.Unix(1, 0),
	})
}

func (s *Server) expirePortalSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: portalSessionCookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: s.secureCookies(r.Context()), SameSite: http.SameSiteStrictMode, MaxAge: -1,
		Expires: time.Unix(1, 0),
	})
}
