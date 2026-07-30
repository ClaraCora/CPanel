package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cpanel/internal/domain"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

const agentStreamKeepaliveInterval = 25 * time.Second

func (s *Server) handleAgentHandshake(w http.ResponseWriter, r *http.Request) {
	machine := currentAgent(r)
	heartbeatInterval, telemetryInterval, fallbackInterval := s.agentIntervals(r.Context())
	cursor, err := s.store.LatestAgentCursor(r.Context(), machine.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	externalURL := s.store.SettingString(r.Context(), "agent", "external_url", s.cfg.ExternalURL)
	streamURL, _ := url.Parse(strings.TrimRight(externalURL, "/") + "/ca/cc/td")
	if streamURL.Scheme == "https" {
		streamURL.Scheme = "wss"
	} else {
		streamURL.Scheme = "ws"
	}
	panelPublicKey, err := s.agentV2.panelPublicKey(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "AGENT_IDENTITY_UNAVAILABLE", "Agent V2 panel identity is unavailable", nil)
		return
	}
	writeData(w, r, http.StatusOK, map[string]any{
		"protocol_version": "1.0",
		"machine":          map[string]any{"id": machine.ID, "name": machine.Name},
		"stream":           map[string]any{"enabled": true, "url": streamURL.String()},
		"intervals": map[string]int{
			"heartbeat_seconds":     int(heartbeatInterval.Seconds()),
			"telemetry_seconds":     int(telemetryInterval.Seconds()),
			"fallback_pull_seconds": int(fallbackInterval.Seconds()),
		},
		"cursor": strconv.FormatInt(cursor, 10),
		"aq":     map[string]string{"mbgy": base64.RawURLEncoding.EncodeToString(panelPublicKey)},
	})
}

func (s *Server) handleAgentNodes(w http.ResponseWriter, r *http.Request) {
	machine := currentAgent(r)
	_, telemetryInterval, fallbackInterval := s.agentIntervals(r.Context())
	items, err := s.store.AgentNodes(r.Context(), machine.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, map[string]any{"nodes": items, "base_config": map[string]int{
		"push_interval": int(telemetryInterval.Seconds()), "pull_interval": int(fallbackInterval.Seconds()),
	}})
}

func agentNodeID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value, err := strconv.ParseInt(chi.URLParam(r, "jdbh"), 10, 64)
	if err != nil || value <= 0 {
		writeError(w, r, http.StatusBadRequest, "INVALID_NODE_NUMBER", "节点编号无效", nil)
		return 0, false
	}
	return value, true
}

func (s *Server) handleAgentNodeSpec(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := agentNodeID(w, r)
	if !ok {
		return
	}
	machine := currentAgent(r)
	_, telemetryInterval, fallbackInterval := s.agentIntervals(r.Context())
	spec, err := s.store.AgentNodeSpec(r.Context(), machine.ID, nodeID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, agentNodeSpecPayload(spec, telemetryInterval, fallbackInterval))
}

func (s *Server) agentIntervals(ctx context.Context) (time.Duration, time.Duration, time.Duration) {
	heartbeat := s.store.SettingInt(ctx, "agent", "heartbeat_seconds", int(s.cfg.HeartbeatInterval.Seconds()), 10)
	telemetry := s.store.SettingInt(ctx, "node_defaults", "telemetry_seconds", int(s.cfg.TelemetryInterval.Seconds()), 10)
	fallback := s.store.SettingInt(ctx, "agent", "fallback_pull_seconds", int(s.cfg.FallbackPullInterval.Seconds()), 15)
	return time.Duration(heartbeat) * time.Second, time.Duration(telemetry) * time.Second, time.Duration(fallback) * time.Second
}

func agentNodeSpecPayload(spec domain.AgentNodeSpec, telemetryInterval, fallbackInterval time.Duration) map[string]any {
	return map[string]any{
		"node_id": spec.NodeID, "revision": spec.Revision, "protocol": spec.Protocol,
		"listen_ip": spec.ListenIP, "server_port": spec.ServerPort, "kernel_type": spec.KernelType,
		"settings": spec.Settings,
		"base_config": map[string]int{
			"push_interval": int(telemetryInterval.Seconds()),
			"pull_interval": int(fallbackInterval.Seconds()),
		},
	}
}

func (s *Server) handleAgentNodeUsers(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := agentNodeID(w, r)
	if !ok {
		return
	}
	machine := currentAgent(r)
	items, err := s.store.AgentNodeUsers(r.Context(), machine.ID, nodeID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, map[string]any{"users": items})
}

func (s *Server) handleAgentChanges(w http.ResponseWriter, r *http.Request) {
	after := int64(0)
	if value := r.URL.Query().Get("yb"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 0 {
			writeError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "游标无效", nil)
			return
		}
		after = parsed
	}
	machine := currentAgent(r)
	items, err := s.store.AgentChanges(r.Context(), machine.ID, after, 200)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	next := after
	if len(items) > 0 {
		next = items[len(items)-1].Cursor
	}
	writeData(w, r, http.StatusOK, map[string]any{"changes": items, "next_cursor": strconv.FormatInt(next, 10)})
}

func (s *Server) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Version      string          `json:"version"`
		Kernel       string          `json:"kernel"`
		Capabilities json.RawMessage `json:"capabilities"`
		Metrics      json.RawMessage `json:"metrics"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	machine := currentAgent(r)
	if input.Kernel == "" {
		input.Kernel = machine.KernelType
	}
	if err := s.store.RecordMachineHeartbeat(r.Context(), machine, input.Version, input.Kernel, input.Capabilities, input.Metrics); err != nil {
		writeStoreError(w, r, err)
		return
	}
	commands := make([]domain.AgentCommand, 0, 1)
	command, claimed, err := s.store.ClaimMachineAgentUpgrade(r.Context(), machine.ID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if claimed {
		commands = append(commands, command)
	}
	writeData(w, r, http.StatusOK, map[string]any{"accepted": true, "commands": commands})
}

func (s *Server) handleAgentTelemetry(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		writeError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "遥测请求必须包含有效幂等键", nil)
		return
	}
	var payload json.RawMessage
	if !decodeJSON(w, r, &payload) {
		return
	}
	machine := currentAgent(r)
	created, err := s.store.RecordTelemetryBatch(r.Context(), machine.ID, key, payload)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, map[string]bool{"accepted": created, "duplicate": !created})
}

func (s *Server) handleAgentStream(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	machine := currentAgent(r)
	cursor, err := s.store.LatestAgentCursor(r.Context(), machine.ID)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "cursor unavailable")
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		for {
			_, _, err := conn.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}
		}
	}()
	ready := map[string]any{"id": "evt_ready", "type": "agent.ready", "revision": 0, "occurred_at": time.Now().UTC(), "data": map[string]any{"cursor": cursor}}
	if err := wsWriteJSON(ctx, conn, ready); err != nil {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	keepaliveTicker := time.NewTicker(agentStreamKeepaliveInterval)
	defer keepaliveTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			return
		case <-keepaliveTicker.C:
			if err := wsWriteJSON(ctx, conn, map[string]string{"type": ""}); err != nil {
				return
			}
		case <-ticker.C:
			items, err := s.store.AgentChanges(ctx, machine.ID, cursor, 200)
			if err != nil {
				_ = conn.Close(websocket.StatusInternalError, "change feed unavailable")
				return
			}
			for _, item := range items {
				event := map[string]any{"id": "chg_" + strconv.FormatInt(item.Cursor, 10), "type": item.EventType, "revision": item.Revision, "occurred_at": item.CreatedAt, "data": item.Payload}
				if err := wsWriteJSON(ctx, conn, event); err != nil {
					return
				}
				cursor = item.Cursor
			}
		}
	}
}

func wsWriteJSON(ctx context.Context, conn *websocket.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, data)
}

var _ = errors.New
