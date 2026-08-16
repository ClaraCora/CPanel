package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"cpanel/internal/auth"
	"cpanel/internal/domain"
	"cpanel/internal/store"
	"cpanel/internal/subscription"
	"github.com/go-chi/chi/v5"
)

const (
	portalGrantTTL       = 90 * time.Second
	portalLoginWindow    = 15 * time.Minute
	portalLoginBlockTime = 15 * time.Minute
	portalLoginMaxTries  = 5
)

func (s *Server) handlePortalLogin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	login := strings.TrimSpace(input.Login)
	key := clientIP(r) + "\x00" + strings.ToLower(login)
	now := time.Now()
	allowed := s.portalLoginAllowed(key, now)
	user, err := s.store.FindPortalUserByLogin(r.Context(), login)
	hash := s.dummyPasswordHash
	if err == nil && user.PasswordHash != "" {
		hash = user.PasswordHash
	}
	valid := auth.VerifyPassword(hash, input.Password)
	if !allowed || err != nil || !valid {
		s.recordPortalLoginFailure(key, now)
		writeError(w, r, http.StatusUnauthorized, "PORTAL_LOGIN_FAILED", "账号或密码错误", nil)
		return
	}
	s.clearPortalLoginFailures(key)
	plain, tokenHash, err := auth.NewSecret("cpedu_", 32)
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
	if _, err := s.store.CreatePortalSession(r.Context(), user.ID, tokenHash, csrf, "", clientIP(r), r.UserAgent(), false, expiresAt); err != nil {
		writeStoreError(w, r, err)
		return
	}
	s.setPortalSessionCookie(w, plain, expiresAt)
	writeData(w, r, http.StatusOK, portalSessionPayload(domain.PortalSession{
		User: user.User, CSRFToken: csrf, ExpiresAt: expiresAt,
	}))
}

func (s *Server) handleCreatePortalGrant(w http.ResponseWriter, r *http.Request) {
	plain, tokenHash, err := auth.NewSecret("cpedug_", 32)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	admin := currentAdmin(r)
	expiresAt := time.Now().Add(portalGrantTTL)
	userID := strings.TrimSpace(chi.URLParam(r, "id"))
	if err := s.store.CreatePortalGrant(r.Context(), admin.ID, userID, tokenHash, expiresAt); err != nil {
		writeStoreError(w, r, err)
		return
	}
	_ = s.store.WriteAudit(r.Context(), admin.ID, "user.portal.delegate.create", "user", userID,
		map[string]any{"expires_at": expiresAt}, clientIP(r), requestID(r))
	noStore(w)
	writeData(w, r, http.StatusCreated, map[string]any{"grant": plain, "expires_at": expiresAt})
}

func (s *Server) handlePortalGrantRedemption(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var input struct {
		Grant string `json:"grant"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Grant) == "" {
		writeError(w, r, http.StatusUnauthorized, "PORTAL_GRANT_INVALID", "管理员授权已失效，请返回后台重新进入", nil)
		return
	}
	userID, adminID, err := s.store.RedeemPortalGrant(r.Context(), auth.HashSecret(strings.TrimSpace(input.Grant)))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusUnauthorized, "PORTAL_GRANT_INVALID", "管理员授权已失效，请返回后台重新进入", nil)
		return
	}
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	user, err := s.store.FindPortalUserByID(r.Context(), userID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusUnauthorized, "PORTAL_ACCOUNT_UNAVAILABLE", "该订阅账号当前不可用", nil)
		return
	}
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	plain, tokenHash, err := auth.NewSecret("cpedu_", 32)
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
	if _, err := s.store.CreatePortalSession(r.Context(), userID, tokenHash, csrf, adminID, clientIP(r), r.UserAgent(), true, expiresAt); err != nil {
		writeStoreError(w, r, err)
		return
	}
	_ = s.store.WriteAudit(r.Context(), adminID, "user.portal.delegate.redeem", "user", userID,
		map[string]any{}, clientIP(r), requestID(r))
	s.setPortalSessionCookie(w, plain, expiresAt)
	writeData(w, r, http.StatusOK, portalSessionPayload(domain.PortalSession{
		User: user.User, CSRFToken: csrf, ExpiresAt: expiresAt, ReadOnly: true, DelegatedByID: adminID,
	}))
}

func (s *Server) handlePortalCurrentSession(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	writeData(w, r, http.StatusOK, portalSessionPayload(currentPortalSession(r)))
}

func (s *Server) handlePortalDashboard(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	session := currentPortalSession(r)
	dashboard, err := s.store.PortalDashboard(r.Context(), session.User.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	token, err := s.store.UserSubscriptionToken(r.Context(), session.User.ID)
	if errors.Is(err, store.ErrSubscriptionTokenUnavailable) {
		writeError(w, r, http.StatusConflict, "SUBSCRIPTION_TOKEN_UNAVAILABLE", "该账号的原始订阅令牌未保存，暂时无法在订阅中心展示", nil)
		return
	}
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	dashboard.SubscriptionURL = s.subscriptionURL(r, token)
	data, err := s.store.SubscriptionByUserID(r.Context(), session.User.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	dashboard.Nodes = make([]domain.PortalNode, 0, len(data.Nodes))
	for _, node := range data.Nodes {
		item := domain.PortalNode{Name: node.Name, EntryName: node.EntryName, Protocol: node.Protocol, Status: node.Status}
		uri, uriErr := subscription.BuildNodeURI(node)
		if uriErr != nil {
			item.Error = "该节点暂不支持单链接导入"
		} else {
			item.URI = uri
		}
		dashboard.Nodes = append(dashboard.Nodes, item)
	}
	writeData(w, r, http.StatusOK, dashboard)
}

func (s *Server) handlePortalPassword(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	session := currentPortalSession(r)
	if rejectPortalAccountMutation(w, r, session, "管理员查看模式不能修改账号密码") {
		return
	}
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
	user, err := s.store.FindPortalUserByID(r.Context(), session.User.ID)
	if err != nil || !auth.VerifyPassword(user.PasswordHash, input.CurrentPassword) {
		writeError(w, r, http.StatusUnauthorized, "PASSWORD_INVALID", "当前密码错误", map[string]string{"current_password": "当前密码错误"})
		return
	}
	hash, err := auth.HashPassword(input.NewPassword)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "PASSWORD_INVALID", "新密码不符合安全要求", map[string]string{"new_password": "新密码不符合安全要求"})
		return
	}
	revoked, err := s.store.UpdatePortalPassword(r.Context(), session.User.ID, session.ID, hash)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, map[string]any{"password_updated": true, "other_sessions_revoked": revoked})
}

func (s *Server) handlePortalSubscriptionRotation(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	session := currentPortalSession(r)
	if rejectPortalAccountMutation(w, r, session, "管理员查看模式不能重置订阅地址") {
		return
	}
	var input struct {
		CurrentPassword string `json:"current_password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.store.FindPortalUserByID(r.Context(), session.User.ID)
	if err != nil || !auth.VerifyPassword(user.PasswordHash, input.CurrentPassword) {
		writeError(w, r, http.StatusUnauthorized, "PASSWORD_INVALID", "当前密码错误", map[string]string{"current_password": "当前密码错误"})
		return
	}
	token, err := s.store.RotateUserSubscriptionToken(r.Context(), session.User.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, domain.UserSubscription{URL: s.subscriptionURL(r, token)})
}

func (s *Server) handlePortalLogout(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if cookie, err := r.Cookie(portalSessionCookieName); err == nil && cookie.Value != "" {
		_ = s.store.RevokePortalSession(r.Context(), auth.HashSecret(cookie.Value))
	}
	s.expirePortalSessionCookie(w)
	writeData(w, r, http.StatusOK, map[string]bool{"logged_out": true})
}

func portalSessionPayload(session domain.PortalSession) map[string]any {
	return map[string]any{
		"user":       map[string]any{"id": session.User.ID, "name": session.User.Name, "email": session.User.Email, "role": session.User.Role, "status": session.User.Status},
		"csrf_token": session.CSRFToken, "expires_at": session.ExpiresAt, "read_only": session.ReadOnly,
		"delegated_by_name": session.DelegatedByName,
	}
}

func (s *Server) setPortalSessionCookie(w http.ResponseWriter, value string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: portalSessionCookieName, Value: value, Path: "/", HttpOnly: true,
		Secure: s.cfg.CookieSecure, SameSite: http.SameSiteStrictMode,
		Expires: expiresAt, MaxAge: int(time.Until(expiresAt).Seconds()),
	})
}

func (s *Server) subscriptionURL(r *http.Request, token string) string {
	baseURL := s.store.SettingString(r.Context(), "subscription", "base_url", "")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.store.SettingString(r.Context(), "site", "site_url", s.cfg.ExternalURL)
	}
	return strings.TrimRight(baseURL, "/") + "/ca/x/" + url.PathEscape(token)
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
}

func rejectPortalAccountMutation(w http.ResponseWriter, r *http.Request, session domain.PortalSession, message string) bool {
	if !session.ReadOnly {
		return false
	}
	writeError(w, r, http.StatusForbidden, "PORTAL_READ_ONLY", message, nil)
	return true
}

func validPortalLogin(value string) bool {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) < 3 || utf8.RuneCountInString(value) > 100 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("._-@", character) {
			continue
		}
		return false
	}
	return true
}

func (s *Server) portalLoginAllowed(key string, now time.Time) bool {
	s.portalLoginMu.Lock()
	defer s.portalLoginMu.Unlock()
	for candidate, attempt := range s.portalLogin {
		if now.Sub(attempt.windowStart) > 2*portalLoginWindow && now.After(attempt.blockedTill) {
			delete(s.portalLogin, candidate)
		}
	}
	attempt := s.portalLogin[key]
	return !now.Before(attempt.blockedTill)
}

func (s *Server) recordPortalLoginFailure(key string, now time.Time) {
	s.portalLoginMu.Lock()
	defer s.portalLoginMu.Unlock()
	attempt := s.portalLogin[key]
	if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) > portalLoginWindow {
		attempt = portalLoginAttempt{windowStart: now}
	}
	attempt.count++
	if attempt.count >= portalLoginMaxTries {
		attempt.blockedTill = now.Add(portalLoginBlockTime)
		attempt.count = 0
		attempt.windowStart = now
	}
	s.portalLogin[key] = attempt
}

func (s *Server) clearPortalLoginFailures(key string) {
	s.portalLoginMu.Lock()
	defer s.portalLoginMu.Unlock()
	delete(s.portalLogin, key)
}
