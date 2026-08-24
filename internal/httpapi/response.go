package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"cpanel/internal/store"
)

type envelope struct {
	Data  any       `json:"data"`
	Meta  meta      `json:"meta"`
	Error *apiError `json:"error"`
}

type meta struct {
	RequestID string `json:"request_id"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeData(w http.ResponseWriter, r *http.Request, status int, data any) {
	writeJSON(w, status, envelope{Data: data, Meta: meta{RequestID: requestID(r)}})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, fields map[string]string) {
	writeJSON(w, status, envelope{
		Data:  nil,
		Meta:  meta{RequestID: requestID(r)},
		Error: &apiError{Code: code, Message: message, Fields: fields},
	})
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrMachineHasNodes):
		writeError(w, r, http.StatusConflict, "MACHINE_HAS_NODES", "请先删除或迁移该服务器下的节点", nil)
	case errors.Is(err, store.ErrAccessGroupInUse):
		writeError(w, r, http.StatusConflict, "ACCESS_GROUP_IN_USE", "该权限组仍被套餐或账号使用，请先解除关联", nil)
	case errors.Is(err, store.ErrPlanInUse):
		writeError(w, r, http.StatusConflict, "PLAN_IN_USE", "该套餐仍有账号使用，请先调整这些账号的套餐", nil)
	case errors.Is(err, store.ErrRoutePolicyInUse):
		writeError(w, r, http.StatusConflict, "ROUTE_POLICY_IN_USE", "该路由策略仍被节点使用，请先解除节点绑定", nil)
	case errors.Is(err, store.ErrRoutePolicyUnavailable):
		writeError(w, r, http.StatusConflict, "ROUTE_POLICY_UNAVAILABLE", "路由策略必须是已发布状态才能绑定节点", nil)
	case errors.Is(err, store.ErrRoutePolicyScopeMismatch):
		writeError(w, r, http.StatusConflict, "ROUTE_POLICY_SCOPE_MISMATCH", "路由策略适用范围与节点策略槽位不匹配", nil)
	case errors.Is(err, store.ErrRouteOutboundUnavailable):
		writeError(w, r, http.StatusUnprocessableEntity, "ROUTE_OUTBOUND_UNAVAILABLE", "规则引用的出站不存在、已停用或不支持当前节点内核，请重新选择", map[string]string{"rules": "请选择可用的出站"})
	case errors.Is(err, store.ErrOutboundInUse):
		writeError(w, r, http.StatusConflict, "OUTBOUND_IN_USE", "该出站仍被路由策略或链式出站引用，请先解除引用", nil)
	case errors.Is(err, store.ErrOutboundKernelUnsupported):
		writeError(w, r, http.StatusConflict, "OUTBOUND_KERNEL_UNSUPPORTED", "出站不支持当前节点内核", map[string]string{"kernel_support": "请保留节点使用的内核"})
	case errors.Is(err, store.ErrOutboundKernelInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "OUTBOUND_KERNEL_INVALID", "出站内核支持只能是 singbox 或 xray", map[string]string{"kernel_support": "至少选择一个有效内核"})
	case errors.Is(err, store.ErrMachineUnavailable):
		writeError(w, r, http.StatusConflict, "MACHINE_UNAVAILABLE", "服务器已停用或归档，不能绑定节点", map[string]string{"machine_id": "请选择可用服务器"})
	case errors.Is(err, store.ErrNodePortInUse):
		writeError(w, r, http.StatusConflict, "NODE_PORT_IN_USE", "该服务器上的监听端口已被其他节点占用，请更换端口", map[string]string{"server_port": "该端口已被占用"})
	case errors.Is(err, store.ErrAgentNotConnected):
		writeError(w, r, http.StatusConflict, "AGENT_NOT_CONNECTED", "Agent 尚未连接，暂时无法下发升级任务", nil)
	case errors.Is(err, store.ErrAgentAlreadyLatest):
		writeError(w, r, http.StatusConflict, "AGENT_ALREADY_LATEST", "Agent 已是最新版本，无需升级", nil)
	case errors.Is(err, store.ErrAgentUpgradePending):
		writeError(w, r, http.StatusConflict, "AGENT_UPGRADE_PENDING", "Agent 升级任务正在处理，请等待下一次心跳", nil)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "资源不存在", nil)
	case errors.Is(err, store.ErrConflict):
		writeError(w, r, http.StatusConflict, "RESOURCE_CONFLICT", "资源存在冲突或仍被引用", nil)
	default:
		slog.Error("request failed", "request_id", requestID(r), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用", nil)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "请求内容格式不正确", nil)
		return false
	}
	return true
}
