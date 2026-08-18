package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"cpanel/internal/auth"
	"cpanel/internal/domain"
	"cpanel/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func validOneOf(value *string, allowed ...string) bool {
	if value == nil {
		return true
	}
	for _, candidate := range allowed {
		if *value == candidate {
			return true
		}
	}
	return false
}

func validPlanResetStrategy(value string) bool {
	return value == "calendar_month" || value == "never"
}

func validNodeEndpoints(items []domain.NodeEndpoint, fields map[string]string) {
	for index, item := range items {
		prefix := fmt.Sprintf("endpoints.%d", index)
		if !required(item.Name) {
			fields[prefix+".name"] = "required"
		}
		if !required(item.Host) {
			fields[prefix+".host"] = "required"
		}
		if item.Port < 1 || item.Port > 65535 {
			fields[prefix+".port"] = "invalid"
		}
		if item.Status != "" && item.Status != "active" && item.Status != "disabled" {
			fields[prefix+".status"] = "invalid"
		}
	}
}

func (s *Server) handleListMachines(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListMachines(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	latestVersion := s.agentRelease.Latest(r.Context())
	for index := range items {
		items[index].LatestAgentVersion = latestVersion
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleCreateMachine(w http.ResponseWriter, r *http.Request) {
	var input domain.MachineCreate
	if !decodeJSON(w, r, &input) {
		return
	}
	if !required(input.Name) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请填写服务器名称", map[string]string{"name": "required"})
		return
	}
	if input.KernelType != "" && input.KernelType != "singbox" && input.KernelType != "xray" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "内核类型无效", map[string]string{"kernel_type": "invalid"})
		return
	}
	item, err := s.store.CreateMachine(r.Context(), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditCreate(s, r, "machine", item.ID, input)
	writeData(w, r, http.StatusCreated, item)
}

func (s *Server) handleUpdateMachine(w http.ResponseWriter, r *http.Request) {
	var input domain.MachineUpdate
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Name != nil && !required(*input.Name) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请填写服务器名称", map[string]string{"name": "required"})
		return
	}
	if !validOneOf(input.KernelType, "singbox", "xray") || !validOneOf(input.Status, "pending", "online", "offline", "disabled") {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "服务器状态或内核无效", nil)
		return
	}
	item, err := s.store.UpdateMachine(r.Context(), chi.URLParam(r, "id"), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditUpdate(s, r, "machine", item.ID, input)
	writeData(w, r, http.StatusOK, item)
}

func (s *Server) handleDeleteMachine(w http.ResponseWriter, r *http.Request) {
	s.handleArchiveResource(w, r, "machine", s.store.ArchiveMachine)
}

func (s *Server) handleCreateMachineCredential(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.CreateMachineCredential(r.Context(), chi.URLParam(r, "id"), nil)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditCreate(s, r, "machine_credential", item.ID, map[string]string{"machine_id": item.MachineID})
	writeData(w, r, http.StatusCreated, item)
}

func (s *Server) handleMachineInstallation(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")
	machines, err := s.store.ListMachines(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	found := false
	for _, machine := range machines {
		if machine.ID == machineID {
			found = true
			break
		}
	}
	if !found {
		writeStoreError(w, r, store.ErrNotFound)
		return
	}
	sealed, err := s.store.RawSetting(r.Context(), "agent", "communication_key")
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "AGENT_KEY_NOT_CONFIGURED", "请先在系统设置中配置 Agent 统一通讯密钥", nil)
		return
	}
	plain, err := s.secureBox.Open(sealed)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "AGENT_KEY_UNAVAILABLE", "Agent 通讯密钥无法读取", nil)
		return
	}
	var key string
	if json.Unmarshal(plain, &key) != nil || key == "" {
		writeError(w, r, http.StatusInternalServerError, "AGENT_KEY_UNAVAILABLE", "Agent 通讯密钥无法读取", nil)
		return
	}
	controlURL := s.store.SettingString(r.Context(), "agent", "external_url", s.cfg.ExternalURL)
	installerURL := s.store.SettingString(r.Context(), "agent", "installer_url", "https://raw.githubusercontent.com/ClaraCora/CPP/main/corade-install.sh")
	panelPublicKey, err := s.agentV2.panelPublicKey(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "AGENT_IDENTITY_UNAVAILABLE", "Agent V2 面板身份无法读取", nil)
		return
	}
	encodedPanelPublicKey := base64.RawURLEncoding.EncodeToString(panelPublicKey)
	command := agentInstallCommand(installerURL, controlURL, key, machineID, encodedPanelPublicKey)
	writeData(w, r, http.StatusOK, map[string]string{
		"machine_id": machineID, "control_url": controlURL, "installer_url": installerURL,
		"panel_public_key": encodedPanelPublicKey, "command": command,
	})
}

func agentInstallCommand(installerURL, controlURL, key, machineID, panelPublicKey string) string {
	return fmt.Sprintf("curl -fsSL %s | sudo sh -s -- --control-url %s --communication-key %s --machine-id %s --panel-public-key %s",
		shellQuote(installerURL), shellQuote(controlURL), shellQuote(key), shellQuote(machineID), shellQuote(panelPublicKey))
}

func (s *Server) handleResetMachineAgentIdentity(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")
	if err := s.store.ResetAgentIdentity(r.Context(), machineID); err != nil {
		writeStoreError(w, r, err)
		return
	}
	s.agentV2.deleteMachineSessions(machineID)
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, "machine.agent_identity_reset", "machine", machineID, nil, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, map[string]any{"machine_id": machineID, "reset": true})
}

func (s *Server) handleMachineAgentUpgrade(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")
	latestVersion := s.agentRelease.Latest(r.Context())
	if latestVersion == "" {
		writeError(w, r, http.StatusServiceUnavailable, "AGENT_RELEASE_UNAVAILABLE", "暂时无法获取 Agent 最新版本，请稍后重试", nil)
		return
	}
	task, err := s.store.RequestMachineAgentUpgrade(r.Context(), machineID, latestVersion)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, "machine.agent_upgrade", "machine", machineID,
		map[string]string{"task_id": task.ID}, clientIP(r), requestID(r))
	writeData(w, r, http.StatusAccepted, task)
}

func (s *Server) handleAgentArtifact(w http.ResponseWriter, r *http.Request) {
	artifact := chi.URLParam(r, "artifact")
	allowed := map[string]bool{
		"corade-linux-amd64":        true,
		"corade-linux-amd64.sha256": true,
		"corade-linux-arm64":        true,
		"corade-linux-arm64.sha256": true,
	}
	if !allowed[artifact] {
		http.NotFound(w, r)
		return
	}
	filePath := filepath.Join(s.cfg.AgentArtifactDir, artifact)
	info, err := os.Stat(filePath)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, artifact))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, filePath)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListNodes(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.GetNode(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, item)
}

func (s *Server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	var input domain.NodeCreate
	if !decodeJSON(w, r, &input) {
		return
	}
	fields := map[string]string{}
	if !required(input.Name) {
		fields["name"] = "required"
	}
	if !required(input.MachineID) {
		fields["machine_id"] = "required"
	}
	if !required(input.Protocol) {
		fields["protocol"] = "required"
	}
	if input.ServerPort < 1 || input.ServerPort > 65535 {
		fields["server_port"] = "invalid"
	}
	if message := nodeConfigValidationMessage(input.Config); message != "" {
		fields["config"] = message
	}
	validNodeEndpoints(input.Endpoints, fields)
	if len(fields) > 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "节点配置不完整", fields)
		return
	}
	item, err := s.store.CreateNode(r.Context(), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditCreate(s, r, "node", item.ID, input)
	writeData(w, r, http.StatusCreated, item)
}

func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	var input domain.NodeUpdate
	if !decodeJSON(w, r, &input) {
		return
	}
	fields := map[string]string{}
	if input.Name != nil && !required(*input.Name) {
		fields["name"] = "required"
	}
	if input.MachineID != nil && !required(*input.MachineID) {
		fields["machine_id"] = "required"
	}
	if input.Protocol != nil && !required(*input.Protocol) {
		fields["protocol"] = "required"
	}
	if input.ServerPort != nil && (*input.ServerPort < 1 || *input.ServerPort > 65535) {
		fields["server_port"] = "invalid"
	}
	if input.KernelType != nil && *input.KernelType != "singbox" && *input.KernelType != "xray" {
		fields["kernel_type"] = "invalid"
	}
	if input.Config != nil {
		if message := nodeConfigValidationMessage(*input.Config); message != "" {
			fields["config"] = message
		}
	}
	if !validOneOf(input.Status, "draft", "published", "disabled", "error") {
		fields["status"] = "invalid"
	}
	if input.Endpoints != nil {
		validNodeEndpoints(*input.Endpoints, fields)
	}
	if len(fields) > 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "节点配置不完整", fields)
		return
	}
	item, err := s.store.UpdateNode(r.Context(), chi.URLParam(r, "id"), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, "node.update", "node", item.ID, input, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, item)
}

func (s *Server) handlePublishNode(w http.ResponseWriter, r *http.Request) {
	admin := currentAdmin(r)
	item, err := s.store.PublishNode(r.Context(), chi.URLParam(r, "id"), admin.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	_ = s.store.WriteAudit(r.Context(), admin.ID, "node.publish", "node", item.ID, map[string]int{"revision": item.CurrentRevision}, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, item)
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	s.handleArchiveResource(w, r, "node", s.store.ArchiveNode)
}

func (s *Server) handleListAccessGroups(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAccessGroups(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleCreateAccessGroup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name    string   `json:"name"`
		Notes   string   `json:"notes"`
		NodeIDs []string `json:"node_ids"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !required(input.Name) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请填写权限组名称", map[string]string{"name": "required"})
		return
	}
	item, err := s.store.CreateAccessGroup(r.Context(), input.Name, input.Notes, input.NodeIDs)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditCreate(s, r, "access_group", item.ID, input)
	writeData(w, r, http.StatusCreated, item)
}

func (s *Server) handleUpdateAccessGroup(w http.ResponseWriter, r *http.Request) {
	var input domain.AccessGroupUpdate
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Name != nil && !required(*input.Name) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请填写权限组名称", map[string]string{"name": "required"})
		return
	}
	if !validOneOf(input.Status, "active", "disabled") {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "权限组状态无效", nil)
		return
	}
	item, err := s.store.UpdateAccessGroup(r.Context(), chi.URLParam(r, "id"), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditUpdate(s, r, "access_group", item.ID, input)
	writeData(w, r, http.StatusOK, item)
}

func (s *Server) handleDeleteAccessGroup(w http.ResponseWriter, r *http.Request) {
	s.handleArchiveResource(w, r, "access_group", s.store.ArchiveAccessGroup)
}

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListPlans(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var input domain.PlanCreate
	if !decodeJSON(w, r, &input) {
		return
	}
	if !required(input.Name) || !required(input.AccessGroupID) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "套餐名称和权限组必填", nil)
		return
	}
	input.ResetStrategy = strings.TrimSpace(input.ResetStrategy)
	if input.ResetStrategy == "" {
		input.ResetStrategy = "calendar_month"
	}
	if !validPlanResetStrategy(input.ResetStrategy) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "套餐流量重置策略无效", map[string]string{"reset_strategy": "请选择有效的流量重置策略"})
		return
	}
	item, err := s.store.CreatePlan(r.Context(), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditCreate(s, r, "plan", item.ID, input)
	writeData(w, r, http.StatusCreated, item)
}

func (s *Server) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	var input domain.PlanUpdate
	if !decodeJSON(w, r, &input) {
		return
	}
	if (input.Name != nil && !required(*input.Name)) || (input.AccessGroupID != nil && !required(*input.AccessGroupID)) ||
		(input.TrafficLimitBytes != nil && *input.TrafficLimitBytes < 0) ||
		(input.SpeedLimitMbps != nil && *input.SpeedLimitMbps < 0) || (input.DeviceLimit != nil && *input.DeviceLimit < 0) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "套餐配置无效", nil)
		return
	}
	if !validOneOf(input.Status, "active", "disabled") {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "套餐状态无效", nil)
		return
	}
	if input.ResetStrategy != nil {
		strategy := strings.TrimSpace(*input.ResetStrategy)
		input.ResetStrategy = &strategy
		if !validPlanResetStrategy(strategy) {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "套餐流量重置策略无效", map[string]string{"reset_strategy": "请选择有效的流量重置策略"})
			return
		}
	}
	item, err := s.store.UpdatePlan(r.Context(), chi.URLParam(r, "id"), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditUpdate(s, r, "plan", item.ID, input)
	writeData(w, r, http.StatusOK, item)
}

func (s *Server) handleDeletePlan(w http.ResponseWriter, r *http.Request) {
	s.handleArchiveResource(w, r, "plan", s.store.ArchivePlan)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var input domain.UserCreate
	if !decodeJSON(w, r, &input) {
		return
	}
	if !required(input.Name) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请填写账号名称", map[string]string{"name": "required"})
		return
	}
	if input.Role != "" && input.Role != "user" && input.Role != "friend" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "账号角色只能是用户或朋友", map[string]string{"role": "invalid"})
		return
	}
	input.PortalLogin = strings.TrimSpace(input.PortalLogin)
	if input.PortalLogin != "" || input.PortalPassword != "" {
		fields := map[string]string{}
		if !validPortalLogin(input.PortalLogin) {
			fields["portal_login"] = "请填写 3 到 100 位的账号、邮箱或 . _ - @ 组合"
		}
		minimumLength := s.store.SettingInt(r.Context(), "security", "password_min_length", 8, 8)
		if utf8.RuneCountInString(input.PortalPassword) < minimumLength {
			fields["portal_password"] = fmt.Sprintf("门户密码至少需要 %d 个字符", minimumLength)
		}
		if len(fields) > 0 {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "门户登录信息不完整", fields)
			return
		}
		hash, err := auth.HashPassword(input.PortalPassword)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "PASSWORD_INVALID", "门户密码不符合安全要求", map[string]string{"portal_password": "密码不符合安全要求"})
			return
		}
		input.PortalPasswordHash = hash
		input.PortalPassword = ""
	}
	item, err := s.store.CreateUser(r.Context(), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditCreate(s, r, "user", item.ID, map[string]any{"role": item.Role, "name": item.Name})
	writeData(w, r, http.StatusCreated, item)
}

func (s *Server) handleQuickCreateUser(w http.ResponseWriter, r *http.Request) {
	role := chi.URLParam(r, "role")
	role = map[string]string{"yh": "user", "py": "friend"}[role]
	label := "用户"
	if role == "friend" {
		label = "朋友"
	} else if role != "user" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "账号角色只能是用户或朋友", nil)
		return
	}

	input := domain.UserCreate{
		Role:  role,
		Name:  fmt.Sprintf("待编辑%s %s", label, time.Now().Format("20060102-150405.000")),
		Notes: "一键添加，待编辑",
	}
	item, err := s.store.CreateUser(r.Context(), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditCreate(s, r, "user", item.ID, map[string]any{"role": item.Role, "name": item.Name, "quick_create": true})
	writeData(w, r, http.StatusCreated, item)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	var input domain.UserUpdate
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Name != nil && !required(*input.Name) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请填写账号名称", map[string]string{"name": "required"})
		return
	}
	if !validOneOf(input.Role, "admin", "user", "friend") || !validOneOf(input.Status, "active", "paused", "expired") {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "账号分类或状态无效", nil)
		return
	}
	if input.UUID != nil {
		canonical, valid := canonicalUserUUID(*input.UUID)
		if !valid {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "UDID 格式无效", map[string]string{"uuid": "请输入标准 UUID"})
			return
		}
		input.UUID = &canonical
	}
	if input.ExpiresAt != nil && *input.ExpiresAt != "" {
		if _, err := time.Parse(time.RFC3339, *input.ExpiresAt); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "到期时间无效", map[string]string{"expires_at": "invalid"})
			return
		}
	}
	if input.PortalLogin != nil {
		value := strings.TrimSpace(*input.PortalLogin)
		input.PortalLogin = &value
		if value != "" && !validPortalLogin(value) {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "门户账号格式无效", map[string]string{"portal_login": "请填写 3 到 100 位的账号、邮箱或 . _ - @ 组合"})
			return
		}
	}
	if input.PortalPassword != nil && *input.PortalPassword != "" {
		minimumLength := s.store.SettingInt(r.Context(), "security", "password_min_length", 8, 8)
		if utf8.RuneCountInString(*input.PortalPassword) < minimumLength {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "门户密码不符合安全要求", map[string]string{"portal_password": fmt.Sprintf("门户密码至少需要 %d 个字符", minimumLength)})
			return
		}
		hash, err := auth.HashPassword(*input.PortalPassword)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "PASSWORD_INVALID", "门户密码不符合安全要求", map[string]string{"portal_password": "密码不符合安全要求"})
			return
		}
		input.PortalPasswordHash = &hash
		input.PortalPassword = nil
	}
	item, err := s.store.UpdateUser(r.Context(), chi.URLParam(r, "id"), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditUpdate(s, r, "user", item.ID, input)
	writeData(w, r, http.StatusOK, item)
}

func canonicalUserUUID(value string) (string, bool) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	return parsed.String(), true
}

func (s *Server) handleGetUserSubscription(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	token, err := s.store.UserSubscriptionToken(r.Context(), userID)
	if errors.Is(err, store.ErrSubscriptionTokenUnavailable) {
		writeError(w, r, http.StatusConflict, "SUBSCRIPTION_TOKEN_UNAVAILABLE",
			"该账号的原始订阅令牌未保存，无法恢复；请重新导入旧数据", nil)
		return
	}
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	baseURL := s.store.SettingString(r.Context(), "subscription", "base_url", "")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.store.SettingString(r.Context(), "site", "site_url", s.cfg.ExternalURL)
	}
	link := strings.TrimRight(baseURL, "/") + "/ca/x/" + url.PathEscape(token)
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, "user.subscription.copy", "user", userID,
		map[string]any{}, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, domain.UserSubscription{URL: link})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if err := s.store.ArchiveUser(r.Context(), userID); err != nil {
		writeStoreError(w, r, err)
		return
	}
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, "user.delete", "user", userID,
		map[string]any{"status": "archived"}, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) handleListRoutePolicies(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListRoutePolicies(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleCreateRoutePolicy(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name               string                   `json:"name"`
		Notes              string                   `json:"notes"`
		Scope              string                   `json:"scope"`
		DefaultOutboundTag string                   `json:"default_outbound_tag"`
		Rules              []domain.RoutePolicyRule `json:"rules"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !required(input.Name) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请填写路由策略名称", nil)
		return
	}
	if input.Scope != "" && input.Scope != "default" && input.Scope != "admin" && input.Scope != "member" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "路由策略适用范围无效", map[string]string{"scope": "invalid"})
		return
	}
	if err := domain.ValidateRoutePolicyRules(input.Rules); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), map[string]string{"rules": err.Error()})
		return
	}
	admin := currentAdmin(r)
	item, err := s.store.CreateRoutePolicy(r.Context(), input.Name, input.Notes, input.Scope, input.DefaultOutboundTag, admin.ID, input.Rules)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditCreate(s, r, "route_policy", item.ID, input)
	writeData(w, r, http.StatusCreated, item)
}

func (s *Server) handleUpdateRoutePolicy(w http.ResponseWriter, r *http.Request) {
	var input domain.RoutePolicyUpdate
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Name != nil && !required(*input.Name) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请填写路由策略名称", nil)
		return
	}
	if input.Scope != nil && *input.Scope != "default" && *input.Scope != "admin" && *input.Scope != "member" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "路由策略适用范围无效", map[string]string{"scope": "invalid"})
		return
	}
	if !validOneOf(input.Status, "published", "disabled") {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "路由策略状态无效", nil)
		return
	}
	if input.Rules != nil {
		if err := domain.ValidateRoutePolicyRules(*input.Rules); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), map[string]string{"rules": err.Error()})
			return
		}
	}
	admin := currentAdmin(r)
	item, err := s.store.UpdateRoutePolicy(r.Context(), chi.URLParam(r, "id"), admin.ID, input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditUpdate(s, r, "route_policy", item.ID, input)
	writeData(w, r, http.StatusOK, item)
}

func (s *Server) handleDeleteRoutePolicy(w http.ResponseWriter, r *http.Request) {
	s.handleArchiveResource(w, r, "route_policy", s.store.ArchiveRoutePolicy)
}

func (s *Server) handleListOutbounds(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListOutbounds(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleCreateOutbound(w http.ResponseWriter, r *http.Request) {
	var input domain.Outbound
	if !decodeJSON(w, r, &input) {
		return
	}
	if !required(input.Name) || !required(input.Tag) || !required(input.Protocol) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "出站名称、标记和协议必填", nil)
		return
	}
	if len(input.Settings) > 0 && !json.Valid(input.Settings) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "出站设置不是有效 JSON", nil)
		return
	}
	input.Tag = strings.ToLower(strings.TrimSpace(input.Tag))
	if err := domain.ValidateOutbound(input.Tag, input.Protocol, input.Settings); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), map[string]string{"settings": err.Error()})
		return
	}
	item, err := s.store.CreateOutbound(r.Context(), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditCreate(s, r, "outbound", item.ID, map[string]string{"tag": item.Tag})
	writeData(w, r, http.StatusCreated, item)
}

func (s *Server) handleUpdateOutbound(w http.ResponseWriter, r *http.Request) {
	var input domain.OutboundUpdate
	if !decodeJSON(w, r, &input) {
		return
	}
	if (input.Name != nil && !required(*input.Name)) || (input.Tag != nil && !required(*input.Tag)) ||
		(input.Protocol != nil && !required(*input.Protocol)) || (input.Settings != nil && !json.Valid(*input.Settings)) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "出站配置无效", nil)
		return
	}
	if input.Tag != nil {
		value := strings.ToLower(strings.TrimSpace(*input.Tag))
		input.Tag = &value
	}
	if input.Tag != nil && input.Protocol != nil && input.Settings != nil {
		if err := domain.ValidateOutbound(*input.Tag, *input.Protocol, *input.Settings); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", err.Error(), map[string]string{"settings": err.Error()})
			return
		}
	}
	if !validOneOf(input.Status, "active", "disabled") {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "出站状态无效", nil)
		return
	}
	item, err := s.store.UpdateOutbound(r.Context(), chi.URLParam(r, "id"), input)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	auditUpdate(s, r, "outbound", item.ID, input)
	writeData(w, r, http.StatusOK, item)
}

func (s *Server) handleDeleteOutbound(w http.ResponseWriter, r *http.Request) {
	s.handleArchiveResource(w, r, "outbound", s.store.ArchiveOutbound)
}

func auditUpdate(s *Server, r *http.Request, resourceType, resourceID string, changes any) {
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, resourceType+".update", resourceType, resourceID, changes, clientIP(r), requestID(r))
}

func (s *Server) handleArchiveResource(w http.ResponseWriter, r *http.Request, resourceType string, archive func(context.Context, string) error) {
	resourceID := chi.URLParam(r, "id")
	if err := archive(r.Context(), resourceID); err != nil {
		writeStoreError(w, r, err)
		return
	}
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, resourceType+".delete", resourceType, resourceID,
		map[string]any{"status": "archived"}, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, map[string]any{"deleted": true})
}
