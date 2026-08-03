package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"cpanel/internal/auth"
	"cpanel/internal/domain"
	"cpanel/internal/store"
)

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	admin, err := s.store.FindAdminByEmail(r.Context(), input.Email)
	hash := s.dummyPasswordHash
	if err == nil {
		hash = admin.PasswordHash
	}
	valid := auth.VerifyPassword(hash, input.Password)
	if err != nil || !valid || admin.Status != "active" {
		writeError(w, r, http.StatusUnauthorized, "LOGIN_FAILED", "邮箱或密码错误", nil)
		return
	}
	plain, tokenHash, err := auth.NewSecret("cpsess_", 32)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	csrf, _, err := auth.NewSecret("csrf_", 32)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	expiresAt := time.Now().Add(s.cfg.SessionTTL)
	if _, err := s.store.CreateAdminSession(r.Context(), admin.ID, tokenHash, csrf, clientIP(r), r.UserAgent(), expiresAt); err != nil {
		writeStoreError(w, r, err)
		return
	}
	_ = s.store.TouchAdminLogin(r.Context(), admin.ID)
	_ = s.store.WriteAudit(r.Context(), admin.ID, "admin.login", "admin", admin.ID, map[string]any{}, clientIP(r), requestID(r))
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: plain, Path: "/", HttpOnly: true,
		Secure: s.cfg.CookieSecure, SameSite: http.SameSiteStrictMode,
		Expires: expiresAt, MaxAge: int(s.cfg.SessionTTL.Seconds()),
	})
	writeData(w, r, http.StatusOK, map[string]any{"admin": admin.Admin, "csrf_token": csrf})
}

func (s *Server) handleCurrentSession(w http.ResponseWriter, r *http.Request) {
	session, _ := r.Context().Value(sessionKey).(domain.AdminSession)
	writeData(w, r, http.StatusOK, map[string]any{
		"admin": session.Admin, "csrf_token": session.CSRFToken, "expires_at": session.ExpiresAt,
	})
}

func (s *Server) handleUpdateAdminProfile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	fields := map[string]string{}
	if input.Name == "" || len([]rune(input.Name)) > 100 {
		fields["name"] = "管理员名称应为 1 到 100 个字符"
	}
	parsed, err := mail.ParseAddress(input.Email)
	if err != nil || !strings.EqualFold(parsed.Address, input.Email) || len(input.Email) > 254 {
		fields["email"] = "请填写有效的管理员邮箱"
	}
	if len(fields) > 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "管理员资料不完整", fields)
		return
	}
	admin := currentAdmin(r)
	updated, err := s.store.UpdateAdminProfile(r.Context(), admin.ID, input.Email, input.Name)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, r, http.StatusConflict, "ADMIN_EMAIL_CONFLICT", "该邮箱已被其他管理员使用", map[string]string{"email": "邮箱已存在"})
		return
	}
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	_ = s.store.WriteAudit(r.Context(), admin.ID, "admin.profile.update", "admin", admin.ID,
		map[string]any{"name": updated.Name, "email": updated.Email}, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, updated)
}

func (s *Server) handleUpdateAdminPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	minimumLength := s.store.SettingInt(r.Context(), "security", "password_min_length", 8, 8)
	fields := map[string]string{}
	if input.CurrentPassword == "" {
		fields["current_password"] = "请输入当前密码"
	}
	if utf8.RuneCountInString(input.NewPassword) < minimumLength {
		fields["new_password"] = fmt.Sprintf("新密码至少需要 %d 个字符", minimumLength)
	}
	if input.NewPassword != input.ConfirmPassword {
		fields["confirm_password"] = "两次输入的新密码不一致"
	}
	if len(fields) > 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "密码信息不完整", fields)
		return
	}
	admin := currentAdmin(r)
	authAdmin, err := s.store.FindAdminByID(r.Context(), admin.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if !auth.VerifyPassword(authAdmin.PasswordHash, input.CurrentPassword) {
		writeError(w, r, http.StatusUnprocessableEntity, "CURRENT_PASSWORD_INVALID", "当前密码不正确", map[string]string{"current_password": "当前密码不正确"})
		return
	}
	if auth.VerifyPassword(authAdmin.PasswordHash, input.NewPassword) {
		writeError(w, r, http.StatusUnprocessableEntity, "PASSWORD_UNCHANGED", "新密码不能与当前密码相同", map[string]string{"new_password": "请使用不同的新密码"})
		return
	}
	hash, err := auth.HashPassword(input.NewPassword)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "PASSWORD_INVALID", "新密码不符合安全要求", map[string]string{"new_password": fmt.Sprintf("新密码至少需要 %d 个字符", minimumLength)})
		return
	}
	session, _ := r.Context().Value(sessionKey).(domain.AdminSession)
	revoked, err := s.store.UpdateAdminPassword(r.Context(), admin.ID, session.ID, hash)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	_ = s.store.WriteAudit(r.Context(), admin.ID, "admin.password.update", "admin", admin.ID,
		map[string]any{"other_sessions_revoked": revoked}, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, map[string]any{"password_updated": true, "other_sessions_revoked": revoked})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = s.store.RevokeAdminSession(r.Context(), auth.HashSecret(cookie.Value))
	}
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, "admin.logout", "admin", admin.ID, map[string]any{}, clientIP(r), requestID(r))
	s.expireSessionCookie(w)
	writeData(w, r, http.StatusOK, map[string]bool{"logged_out": true})
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}
	if period != "today" && period != "yesterday" && period != "7d" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "流量排行时间范围无效", nil)
		return
	}
	value, err := s.store.Overview(r.Context(), period)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, value)
}

func (s *Server) handleListAuditEvents(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAuditEvents(r.Context(), 200)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func auditCreate(s *Server, r *http.Request, resourceType, resourceID string, changes any) {
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, resourceType+".create", resourceType, resourceID, changes, clientIP(r), requestID(r))
}

func required(value string) bool {
	return strings.TrimSpace(value) != ""
}
