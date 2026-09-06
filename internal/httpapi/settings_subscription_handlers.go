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
	"cpanel/internal/domain"
	"cpanel/internal/store"
	"cpanel/internal/subscription"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleHistoricalData(w http.ResponseWriter, r *http.Request) {
	view := r.URL.Query().Get("view")
	query := parseListQuery(r, "created_at", map[string]bool{"created_at": true})
	if view != "" && (query.Paginated || view == "traffic" || view == "machines" || view == "nodes" || view == "devices") {
		var items any
		var total int
		var err error
		switch view {
		case "traffic":
			items, total, err = s.store.ListTrafficPage(r.Context(), s.store.SettingInt(r.Context(), "retention", "traffic_days", 90, 1), query.Page, query.PageSize)
		case "machines", "nodes":
			items, total, err = s.store.ListMetricSamplesPage(r.Context(), view, query.Page, query.PageSize)
		case "devices":
			items, total, err = s.store.ListDeviceHistoryPage(r.Context(), s.store.SettingInt(r.Context(), "retention", "devices_days", 30, 1), query.Page, query.PageSize)
		}
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, map[string]any{"view": view, "items": items, "page": query.Page, "page_size": query.PageSize, "total": total, "has_more": query.Page*query.PageSize < total})
		return
	}
	data, err := s.store.HistoricalData(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, data)
}

func (s *Server) handleListSubscriptionAccess(w http.ResponseWriter, r *http.Request) {
	query := parseListQuery(r, "created_at", map[string]bool{"created_at": true, "status_code": true})
	if query.Paginated {
		items, total, err := s.store.ListSubscriptionAccessPage(r.Context(), query.Page, query.PageSize, query.Query, query.Sort, query.Order)
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, listPage[domain.SubscriptionAccessEvent]{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total, HasMore: query.Page*query.PageSize < total})
		return
	}
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

var sensitiveSettingKeys = map[string]map[string]bool{
	"agent":       {"communication_key": true},
	"certificate": {"dns_api_token": true},
	"tgbot":       {"bot_token": true},
}

var allowedSettingKeys = map[string]map[string]bool{
	"site":          {"platform_name": true, "site_url": true, "timezone": true, "default_language": true, "footer_text": true},
	"agent":         {"external_url": true, "installer_url": true, "communication_key": true, "heartbeat_seconds": true, "offline_threshold_seconds": true, "fallback_pull_seconds": true, "max_message_bytes": true, "allow_legacy_protocol": true},
	"security":      {"session_ttl_minutes": true, "password_min_length": true, "max_login_failures": true, "trusted_proxy_cidrs": true},
	"node_defaults": {"default_kernel": true, "listen_ip": true, "telemetry_seconds": true, "certificate_mode": true},
	"certificate":   {"acme_email": true, "dns_provider": true, "dns_api_token": true, "http01_port": true},
	"subscription":  {"base_url": true, "cache_seconds": true, "format": true, "block_browser_access": true, "ua_whitelist": true},
	"retention":     {"devices_days": true, "traffic_days": true, "audit_days": true, "subscription_access_days": true},
	"tgbot":         {"enabled": true, "bot_token": true, "admin_telegram_id": true, "daily_report_enabled": true, "daily_report_time": true},
}

func isSensitiveSetting(section, key string) bool {
	return sensitiveSettingKeys[section][key]
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
		Values map[string]json.RawMessage `json:"values"`
		// Kept for request compatibility only; the server schema is authoritative.
		SensitiveKeys []string `json:"sensitive_keys"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if len(input.Values) == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "没有需要保存的设置", nil)
		return
	}
	sensitive := make(map[string]bool)
	var sharedKey string
	for key, value := range input.Values {
		if strings.TrimSpace(key) == "" || !json.Valid(value) {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "设置键或值无效", map[string]string{key: "invalid"})
			return
		}
		if !allowedSettingKeys[section][key] {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "设置项不受支持", map[string]string{key: "unknown_setting"})
			return
		}
		sensitive[key] = isSensitiveSetting(section, key)
		if sensitive[key] {
			if section == "agent" && key == "communication_key" {
				if err := json.Unmarshal(value, &sharedKey); err != nil {
					writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Agent 通讯密钥格式无效", map[string]string{key: "invalid"})
					return
				}
				sharedKey = strings.TrimSpace(sharedKey)
				if len(sharedKey) < 32 || len(sharedKey) > 512 {
					writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Agent 通讯密钥需要 32 到 512 个字符", map[string]string{key: "invalid_length"})
					return
				}
				input.Values[key], _ = json.Marshal(sharedKey)
				value = input.Values[key]
			}
			if section == "tgbot" && key == "bot_token" {
				var token string
				if err := json.Unmarshal(value, &token); err != nil || !validTelegramBotToken(token) {
					writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "TG Bot 密钥格式无效", map[string]string{key: "请填写 BotFather 提供的完整密钥"})
					return
				}
				input.Values[key], _ = json.Marshal(strings.TrimSpace(token))
				value = input.Values[key]
			}
			if section == "certificate" && key == "dns_api_token" {
				var token string
				if err := json.Unmarshal(value, &token); err != nil || strings.TrimSpace(token) == "" || len(token) > 4096 {
					writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "DNS API Token 格式无效", map[string]string{key: "请填写有效的 Token"})
					return
				}
				input.Values[key], _ = json.Marshal(strings.TrimSpace(token))
				value = input.Values[key]
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
		if section == "security" && key == "session_ttl_minutes" {
			var minutes int
			if err := json.Unmarshal(value, &minutes); err != nil || minutes < minimumSessionTTLMinutes || minutes > maximumSessionTTLMinutes {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "会话有效期必须是 15 分钟到 30 天", map[string]string{key: "请输入 15 到 43200 之间的整数"})
				return
			}
		}
		if section == "security" && key == "max_login_failures" {
			var maximum int
			if err := json.Unmarshal(value, &maximum); err != nil || maximum < 3 || maximum > 100 {
				writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "登录失败锁定次数必须是 3 到 100", map[string]string{key: "请输入 3 到 100 之间的整数"})
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
	if sharedKey != "" {
		if err := s.store.UpsertSettingsWithAgentCredential(
			r.Context(), section, admin.ID, input.Values, sensitive,
			auth.HashSecret(sharedKey), auth.Prefix(sharedKey, 12),
		); err != nil {
			writeStoreError(w, r, err)
			return
		}
	} else if err := s.store.UpsertSettings(r.Context(), section, admin.ID, input.Values, sensitive); err != nil {
		writeStoreError(w, r, err)
		return
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
	if len(value) > 256 {
		return false
	}
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
	ipAddress := clientIP(r)
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
