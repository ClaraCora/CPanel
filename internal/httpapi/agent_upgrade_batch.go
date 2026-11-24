package httpapi

import (
	"net/http"
	"strings"

	"cpanel/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGetUpgradeBatch(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.AgentUpgradeBatch(r.Context(), chi.URLParam(r, "batch"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, result)
}

func (s *Server) handleCreateUpgradeBatch(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TargetVersion string `json:"target_version"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.TargetVersion) == "" {
		writeError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "请先确认目标 Agent 版本", nil)
		return
	}
	target := s.agentReleaseVersion(r)
	if target == "" {
		writeError(w, r, http.StatusServiceUnavailable, "AGENT_RELEASE_UNAVAILABLE", "暂时无法获取 Agent 最新版本，请稍后重试", nil)
		return
	}
	if input.TargetVersion != target {
		writeStoreError(w, r, store.ErrAgentReleaseChanged)
		return
	}
	result, err := s.store.CreateAgentUpgradeBatch(r.Context(), target)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if err = s.store.WriteAudit(r.Context(), currentAdmin(r).ID, "machine.agent_upgrade_batch", "agent_upgrade_batch", result.ID,
		map[string]any{"target_version": target, "machine_count": len(result.Items)}, clientIP(r), requestID(r)); err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusAccepted, result)
}

func (s *Server) handleRetryUpgradeBatchItem(w http.ResponseWriter, r *http.Request) {
	id, machine := chi.URLParam(r, "batch"), chi.URLParam(r, "machine")
	target := s.agentReleaseVersion(r)
	if target == "" {
		writeError(w, r, http.StatusServiceUnavailable, "AGENT_RELEASE_UNAVAILABLE", "暂时无法获取 Agent 最新版本，请稍后重试", nil)
		return
	}
	result, err := s.store.RetryAgentUpgradeBatchItem(r.Context(), id, machine, target)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if err = s.store.WriteAudit(r.Context(), currentAdmin(r).ID, "machine.agent_upgrade_retry", "machine", machine, map[string]string{"batch_id": id, "target_version": target}, clientIP(r), requestID(r)); err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusAccepted, result)
}
