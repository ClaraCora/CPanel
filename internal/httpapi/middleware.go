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
	adminKey     contextKey = "admin"
	sessionKey   contextKey = "session"
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
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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

func requestID(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey).(string)
	return value
}

func currentAdmin(r *http.Request) domain.Admin {
	value, _ := r.Context().Value(adminKey).(domain.Admin)
	return value
}

func currentAgent(r *http.Request) domain.AgentMachine {
	value, _ := r.Context().Value(agentKey).(domain.AgentMachine)
	return value
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) expireSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: s.cfg.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1,
		Expires: time.Unix(1, 0),
	})
}
