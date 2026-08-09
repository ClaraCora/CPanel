package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"cpanel/internal/auth"
	"cpanel/internal/store"
	"cpanel/internal/subscription"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleHistoricalData(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.HistoricalData(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, data)
}

func (s *Server) handleListSubscriptionAccess(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListSubscriptionAccess(r.Context(), 200)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

var allowedSettingSections = map[string]bool{
	"site": true, "agent": true, "security": true, "node_defaults": true,
	"certificate": true, "subscription": true, "retention": true,
	"tgbot": true,
}

var settingSectionPaths = map[string]string{
	"zd": "site", "dl": "agent", "aq": "security", "jdmr": "node_defaults",
	"zs": "certificate", "dy": "subscription", "bl": "retention",
	"tg": "tgbot",
}

func settingSection(r *http.Request) string {
	value := chi.URLParam(r, "section")
	return settingSectionPaths[value]
}

func (s *Server) handleListSettings(w http.ResponseWriter, r *http.Request) {
	section := settingSection(r)
	if !allowedSettingSections[section] {
		writeError(w, r, http.StatusNotFound, "SETTING_SECTION_NOT_FOUND", "设置分区不存在", nil)
		return
	}
	items, err := s.store.ListSettings(r.Context(), section)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	section := settingSection(r)
	if !allowedSettingSections[section] {
		writeError(w, r, http.StatusNotFound, "SETTING_SECTION_NOT_FOUND", "设置分区不存在", nil)
		return
	}
	var input struct {
		Values        map[string]json.RawMessage `json:"values"`
		SensitiveKeys []string                   `json:"sensitive_keys"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if len(input.Values) == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "没有需要保存的设置", nil)
		return
	}
	sensitive := make(map[string]bool, len(input.SensitiveKeys))
	for _, key := range input.SensitiveKeys {
		sensitive[key] = true
	}
	var sharedKey string
	for key, value := range input.Values {
		if strings.TrimSpace(key) == "" || !json.Valid(value) {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "设置键或值无效", map[string]string{key: "invalid"})
			return
		}
		if section == "tgbot" && key == "bot_token" {
			sensitive[key] = true
		}
		if sensitive[key] {
			if section == "agent" && key == "communication_key" {
				if err := json.Unmarshal(value, &sharedKey); err != nil || len(sharedKey) < 32 {
					writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Agent 通讯密钥至少需要 32 个字符", map[string]string{key: "too_short"})
					return
				}
			}
			if section == "tgbot" && key == "bot_token" {
				var token string
				if err := json.Unmarshal(value, &token); err != nil || !validTelegramBotToken(token) {
					writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "TG Bot 密钥格式无效", map[string]string{key: "请填写 BotFather 提供的完整密钥"})
					return
				}
			}
			sealed, err := s.secureBox.Seal(value)
			if err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, "SETTING_ENCRYPT_FAILED", "敏感设置无法加密", map[string]string{key: "invalid"})
				return
			}
			input.Values[key] = sealed
		}
		if section == "subscription" && key == "block_browser_access" {
			var enabled bool
			if err := json.Unmarshal(value, &enabled); err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "浏览器拦截开关必须是布尔值", map[string]string{key: "请选择启用或关闭"})
				return
			}
		}
		if section == "subscription" && key == "ua_whitelist" {
			var configured string
			if err := json.Unmarshal(value, &configured); err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "UA 白名单格式无效", map[string]string{key: "请每行填写一个 UA 关键字"})
				return
			}
			normalized, err := normalizeUAWhitelist(configured)
			if err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "UA 白名单最多 100 条，每条最多 256 个字符", map[string]string{key: "请精简白名单记录"})
				return
			}
			input.Values[key], _ = json.Marshal(normalized)
		}
		if section == "security" && key == "password_min_length" {
			var minimum int
			if err := json.Unmarshal(value, &minimum); err != nil || minimum < 8 || minimum > 128 {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "密码最小长度必须是 8 到 128", map[string]string{key: "请输入 8 到 128 之间的整数"})
				return
			}
		}
		if section == "agent" && key == "allow_legacy_protocol" {
			var allowed bool
			if err := json.Unmarshal(value, &allowed); err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "旧通讯开关必须是布尔值", map[string]string{key: "invalid_boolean"})
				return
			}
			if !allowed {
				blockers, err := s.store.LegacyAgentBlockers(r.Context())
				if err != nil {
					writeStoreError(w, r, err)
					return
				}
				if len(blockers) > 0 {
					writeError(w, r, http.StatusConflict, "LEGACY_AGENTS_REMAIN", "仍有服务器使用旧通讯，请先完成 V2 升级", map[string]string{"machine_ids": strings.Join(blockers, ",")})
					return
				}
			}
		}
		if section == "tgbot" && (key == "enabled" || key == "daily_report_enabled") {
			var enabled bool
			if err := json.Unmarshal(value, &enabled); err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "TG Bot 开关必须是布尔值", map[string]string{key: "请选择启用或关闭"})
				return
			}
		}
		if section == "tgbot" && key == "admin_telegram_id" {
			var configured string
			if err := json.Unmarshal(value, &configured); err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "管理员 Telegram ID 格式无效", map[string]string{key: "请输入数字 ID"})
				return
			}
			configured = strings.TrimSpace(configured)
			if configured != "" {
				id, err := strconv.ParseInt(configured, 10, 64)
				if err != nil || id <= 0 {
					writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "管理员 Telegram ID 必须是正整数", map[string]string{key: "请输入 /id 返回的数字"})
					return
				}
			}
		}
		if section == "tgbot" && key == "daily_report_time" {
			var configured string
			if err := json.Unmarshal(value, &configured); err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "日报推送时间格式无效", map[string]string{key: "请选择推送时间"})
				return
			}
			if _, err := time.Parse("15:04", configured); err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "日报推送时间格式无效", map[string]string{key: "请选择有效时间"})
				return
			}
		}
	}
	admin := currentAdmin(r)
	if err := s.store.UpsertSettings(r.Context(), section, admin.ID, input.Values, sensitive); err != nil {
		writeStoreError(w, r, err)
		return
	}
	if sharedKey != "" {
		if err := s.store.SetAgentSharedCredential(r.Context(), admin.ID, auth.HashSecret(sharedKey), auth.Prefix(sharedKey, 12)); err != nil {
			writeStoreError(w, r, err)
			return
		}
	}
	keys := make([]string, 0, len(input.Values))
	for key := range input.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	_ = s.store.WriteAudit(r.Context(), admin.ID, "settings.update", "settings", section,
		map[string]any{"keys": keys}, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, map[string]any{"section": section, "updated_keys": keys})
}

func validTelegramBotToken(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 || len(parts[1]) < 20 {
		return false
	}
	if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		return false
	}
	for _, char := range parts[1] {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	userAgent := sanitizeUserAgent(r.UserAgent())
	ipAddress := resolvedClientIP(r, s.store.SettingString(r.Context(), "security", "trusted_proxy_cidrs", ""))
	recordAccess := func(userID *string, userName, outcome string, statusCode int) {
		if err := s.store.RecordSubscriptionAccess(r.Context(), userID, userName, ipAddress, userAgent, outcome, statusCode); err != nil {
			slog.Warn("record subscription access failed", "request_id", requestID(r), "error", err)
		}
	}
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		recordAccess(nil, "", "not_found", http.StatusNotFound)
		http.NotFound(w, r)
		return
	}
	data, err := s.store.SubscriptionByToken(r.Context(), auth.HashSecret(token))
	if errors.Is(err, store.ErrNotFound) {
		recordAccess(nil, "", "not_found", http.StatusNotFound)
		http.NotFound(w, r)
		return
	}
	if err != nil {
		recordAccess(nil, "", "failed", http.StatusServiceUnavailable)
		http.Error(w, "subscription unavailable", http.StatusServiceUnavailable)
		return
	}
	userID := data.UserID
	if s.store.SettingBool(r.Context(), "subscription", "block_browser_access", false) &&
		isConventionalBrowserUA(userAgent) &&
		!uaWhitelistMatches(userAgent, s.store.SettingString(r.Context(), "subscription", "ua_whitelist", "")) {
		recordAccess(&userID, data.UserName, "blocked", http.StatusNotFound)
		http.NotFound(w, r)
		return
	}
	output, err := subscription.BuildClashMeta(data)
	if err != nil {
		recordAccess(&userID, data.UserName, "failed", http.StatusUnprocessableEntity)
		http.Error(w, "subscription configuration unavailable", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="cpanel.yaml"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Profile-Title", "CPanel")
	recordAccess(&userID, data.UserName, "allowed", http.StatusOK)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}
