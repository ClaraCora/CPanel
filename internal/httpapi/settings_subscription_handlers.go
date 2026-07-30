package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

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

var allowedSettingSections = map[string]bool{
	"site": true, "agent": true, "security": true, "node_defaults": true,
	"certificate": true, "subscription": true, "retention": true,
}

func (s *Server) handleListSettings(w http.ResponseWriter, r *http.Request) {
	section := chi.URLParam(r, "section")
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
	section := chi.URLParam(r, "section")
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
		if sensitive[key] {
			if section == "agent" && key == "communication_key" {
				if err := json.Unmarshal(value, &sharedKey); err != nil || len(sharedKey) < 32 {
					writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Agent 通讯密钥至少需要 32 个字符", map[string]string{key: "too_short"})
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

func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		http.NotFound(w, r)
		return
	}
	data, err := s.store.SubscriptionByToken(r.Context(), auth.HashSecret(token))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "subscription unavailable", http.StatusServiceUnavailable)
		return
	}
	output, err := subscription.BuildClashMeta(data)
	if err != nil {
		http.Error(w, "subscription configuration unavailable", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="cpanel.yaml"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Profile-Title", "CPanel")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}
