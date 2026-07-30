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
	case errors.Is(err, store.ErrNodePortInUse):
		writeError(w, r, http.StatusConflict, "NODE_PORT_IN_USE", "该服务器上的监听端口已被其他节点占用，请更换端口", map[string]string{"server_port": "该端口已被占用"})
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
