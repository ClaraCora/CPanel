package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cpanel/internal/agentv2"
	"cpanel/internal/store"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

const agentV2StateKey contextKey = "agent_v2_state"

type agentV2RequestState struct {
	record agentV2SessionRecord
}

func (s *Server) handleAgentHandshakeEntry(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), agentv2.ContentType) {
		s.legacyAgent(http.HandlerFunc(s.handleAgentHandshake)).ServeHTTP(w, r)
		return
	}
	body, ok := readAgentV2Body(w, r, 2<<20)
	if !ok {
		return
	}
	if len(body) > 0 && body[0] == agentv2.EnvelopeVersion {
		r.Body = io.NopCloser(bytes.NewReader(body))
		s.requireAgentV2(http.HandlerFunc(s.handleAgentV2Bootstrap)).ServeHTTP(w, r)
		return
	}
	var request agentv2.HandshakeRequest
	if err := agentv2.DecodeHandshake(body, &request); err != nil {
		writeAgentV2FixedError(w, http.StatusBadRequest)
		return
	}
	response, err := s.agentV2.acceptHandshake(r.Context(), request, time.Now().UTC())
	if err != nil {
		writeAgentV2FixedError(w, http.StatusUnauthorized)
		return
	}
	encoded, err := agentv2.EncodeHandshake(response)
	if err != nil {
		writeAgentV2FixedError(w, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", agentv2.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

func (s *Server) legacyAgent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.store.LegacyAgentAllowed(r.Context()) {
			writeAgentV2FixedError(w, http.StatusNotFound)
			return
		}
		s.requireAgent(next).ServeHTTP(w, r)
	})
}

func (s *Server) dualAgentEndpoint(legacy, v2 http.HandlerFunc) http.HandlerFunc {
	legacyHandler := s.legacyAgent(http.HandlerFunc(legacy))
	v2Handler := s.requireAgentV2(http.HandlerFunc(v2))
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), agentv2.ContentType) {
			v2Handler.ServeHTTP(w, r)
			return
		}
		legacyHandler.ServeHTTP(w, r)
	}
}

func (s *Server) handleAgentStreamEntry(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		s.legacyAgent(http.HandlerFunc(s.handleAgentStream)).ServeHTTP(w, r)
		return
	}
	s.handleAgentV2Stream(w, r)
}

func (s *Server) requireAgentV2(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		envelope, ok := readAgentV2Body(w, r, 2<<20)
		if !ok {
			return
		}
		record, err := s.agentV2.sessionForEnvelope(envelope, time.Now().UTC())
		if err != nil {
			writeAgentV2FixedError(w, http.StatusPreconditionFailed)
			return
		}
		plain, err := record.session.Open(r.Method, r.URL.Path, envelope)
		if err != nil {
			writeAgentV2FixedError(w, http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(plain))
		r.ContentLength = int64(len(plain))
		ctx := context.WithValue(r.Context(), agentKey, record.machine)
		ctx = context.WithValue(ctx, agentV2StateKey, agentV2RequestState{record: record})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func readAgentV2Body(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, bool) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil || int64(len(body)) > limit {
		writeAgentV2FixedError(w, http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return body, true
}

func writeAgentV2FixedError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", agentv2.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte{0})
}

func writeAgentV2(w http.ResponseWriter, r *http.Request, status int, value any) {
	state, ok := r.Context().Value(agentV2StateKey).(agentV2RequestState)
	if !ok || state.record.session == nil {
		writeAgentV2FixedError(w, http.StatusInternalServerError)
		return
	}
	plain, err := json.Marshal(value)
	if err != nil {
		writeAgentV2FixedError(w, http.StatusInternalServerError)
		return
	}
	envelope, err := state.record.session.Seal(r.Method, r.URL.Path, plain)
	if err != nil {
		writeAgentV2FixedError(w, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", agentv2.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(envelope)
}

func writeAgentV2Error(w http.ResponseWriter, r *http.Request, status int, code string) {
	writeAgentV2(w, r, status, map[string]any{"cw": map[string]string{"dm": code}})
}

func writeAgentV2StoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeAgentV2Error(w, r, http.StatusNotFound, "bcz")
	case errors.Is(err, store.ErrConflict):
		writeAgentV2Error(w, r, http.StatusConflict, "ct")
	default:
		writeAgentV2Error(w, r, http.StatusInternalServerError, "nbcw")
	}
}

func decodeAgentV2(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeAgentV2Error(w, r, http.StatusBadRequest, "gs")
		return false
	}
	return true
}

func (s *Server) handleAgentV2Bootstrap(w http.ResponseWriter, r *http.Request) {
	machine := currentAgent(r)
	heartbeatInterval, telemetryInterval, fallbackInterval := s.agentIntervals(r.Context())
	cursor, err := s.store.LatestAgentCursor(r.Context(), machine.ID)
	if err != nil {
		writeAgentV2StoreError(w, r, err)
		return
	}
	externalURL := s.store.SettingString(r.Context(), "agent", "external_url", s.cfg.ExternalURL)
	streamURL, _ := url.Parse(strings.TrimRight(externalURL, "/") + "/ca/cc/td")
	if streamURL.Scheme == "https" {
		streamURL.Scheme = "wss"
	} else {
		streamURL.Scheme = "ws"
	}
	writeAgentV2(w, r, http.StatusOK, map[string]any{
		"xybb": agentv2.ProtocolVersion,
		"fwq":  map[string]any{"bh": machine.ID, "mc": machine.Name},
		"td":   map[string]any{"qy": true, "dz": streamURL.String()},
		"jg": map[string]int{
			"xtjg": int(heartbeatInterval.Seconds()),
			"ycjg": int(telemetryInterval.Seconds()),
			"ldjg": int(fallbackInterval.Seconds()),
		},
		"yb": strconv.FormatInt(cursor, 10),
	})
}

func (s *Server) handleAgentV2Nodes(w http.ResponseWriter, r *http.Request) {
	machine := currentAgent(r)
	_, telemetryInterval, fallbackInterval := s.agentIntervals(r.Context())
	items, err := s.store.AgentNodes(r.Context(), machine.ID)
	if err != nil {
		writeAgentV2StoreError(w, r, err)
		return
	}
	nodes := make([]map[string]any, 0, len(items))
	for _, item := range items {
		nodes = append(nodes, map[string]any{"bh": item.AgentID, "lx": item.Type, "mc": item.Name})
	}
	writeAgentV2(w, r, http.StatusOK, map[string]any{
		"jd":   nodes,
		"jcpz": map[string]int{"tsjg": int(telemetryInterval.Seconds()), "lqjg": int(fallbackInterval.Seconds())},
	})
}

func agentV2NodeID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value, err := strconv.ParseInt(chi.URLParam(r, "jdbh"), 10, 64)
	if err != nil || value <= 0 {
		writeAgentV2Error(w, r, http.StatusBadRequest, "jdbh")
		return 0, false
	}
	return value, true
}

func (s *Server) handleAgentV2NodeSpec(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := agentV2NodeID(w, r)
	if !ok {
		return
	}
	machine := currentAgent(r)
	_, telemetryInterval, fallbackInterval := s.agentIntervals(r.Context())
	spec, err := s.store.AgentNodeSpec(r.Context(), machine.ID, nodeID)
	if err != nil {
		writeAgentV2StoreError(w, r, err)
		return
	}
	writeAgentV2(w, r, http.StatusOK, map[string]any{
		"jdbh": spec.NodeID, "bb": spec.Revision, "xy": spec.Protocol,
		"jtdz": spec.ListenIP, "fwqdk": spec.ServerPort, "nh": spec.KernelType, "sz": spec.Settings,
		"jcpz": map[string]int{"tsjg": int(telemetryInterval.Seconds()), "lqjg": int(fallbackInterval.Seconds())},
	})
}

func (s *Server) handleAgentV2NodeUsers(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := agentV2NodeID(w, r)
	if !ok {
		return
	}
	machine := currentAgent(r)
	items, err := s.store.AgentNodeUsers(r.Context(), machine.ID, nodeID)
	if err != nil {
		writeAgentV2StoreError(w, r, err)
		return
	}
	users := make([]map[string]any, 0, len(items))
	for _, item := range items {
		users = append(users, map[string]any{"bh": item.ID, "wybs": item.UUID, "xs": item.SpeedLimit, "sbs": item.DeviceLimit})
	}
	writeAgentV2(w, r, http.StatusOK, map[string]any{"yh": users})
}

func (s *Server) handleAgentV2Changes(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Cursor string `json:"yb"`
	}
	if !decodeAgentV2(w, r, &input) {
		return
	}
	after := int64(0)
	if input.Cursor != "" {
		parsed, err := strconv.ParseInt(input.Cursor, 10, 64)
		if err != nil || parsed < 0 {
			writeAgentV2Error(w, r, http.StatusBadRequest, "yb")
			return
		}
		after = parsed
	}
	machine := currentAgent(r)
	items, err := s.store.AgentChanges(r.Context(), machine.ID, after, 200)
	if err != nil {
		writeAgentV2StoreError(w, r, err)
		return
	}
	next := after
	changes := make([]map[string]any, 0, len(items))
	for _, item := range items {
		changes = append(changes, map[string]any{
			"yb": item.Cursor, "jdbh": item.NodeID, "lx": item.EventType, "bb": item.Revision,
			"sj": item.Payload, "fssj": item.CreatedAt,
		})
		next = item.Cursor
	}
	writeAgentV2(w, r, http.StatusOK, map[string]any{"bg": changes, "xyb": strconv.FormatInt(next, 10)})
}

func (s *Server) handleAgentV2Heartbeat(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Version      string          `json:"bb"`
		Kernel       string          `json:"nh"`
		Capabilities json.RawMessage `json:"nl"`
		Metrics      json.RawMessage `json:"zb"`
	}
	if !decodeAgentV2(w, r, &input) {
		return
	}
	machine := currentAgent(r)
	if input.Kernel == "" {
		input.Kernel = machine.KernelType
	}
	if err := s.store.RecordMachineHeartbeat(r.Context(), machine, input.Version, input.Kernel, input.Capabilities, input.Metrics); err != nil {
		writeAgentV2StoreError(w, r, err)
		return
	}
	commands := make([]map[string]string, 0, 1)
	command, claimed, err := s.store.ClaimMachineAgentUpgrade(r.Context(), machine.ID)
	if err != nil {
		writeAgentV2StoreError(w, r, err)
		return
	}
	if claimed {
		commands = append(commands, map[string]string{"bh": command.ID, "lx": command.Type})
	}
	writeAgentV2(w, r, http.StatusOK, map[string]any{"js": true, "rw": commands})
}

type agentV2TelemetryEvent struct {
	Type       string          `json:"lx"`
	NodeID     int64           `json:"jdbh,omitempty"`
	OccurredAt string          `json:"fssj"`
	Data       json.RawMessage `json:"sj"`
}

func (s *Server) handleAgentV2Telemetry(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IdempotencyKey string                  `json:"mdj"`
		Events         []agentV2TelemetryEvent `json:"sj"`
	}
	if !decodeAgentV2(w, r, &input) {
		return
	}
	if input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 {
		writeAgentV2Error(w, r, http.StatusBadRequest, "mdj")
		return
	}
	legacyEvents := make([]map[string]any, 0, len(input.Events))
	for _, event := range input.Events {
		legacyEvents = append(legacyEvents, map[string]any{
			"type": event.Type, "node_id": event.NodeID, "occurred_at": event.OccurredAt, "data": event.Data,
		})
	}
	payload, err := json.Marshal(map[string]any{"events": legacyEvents})
	if err != nil {
		writeAgentV2Error(w, r, http.StatusBadRequest, "gs")
		return
	}
	machine := currentAgent(r)
	created, err := s.store.RecordTelemetryBatch(r.Context(), machine.ID, input.IdempotencyKey, payload)
	if err != nil {
		writeAgentV2StoreError(w, r, err)
		return
	}
	writeAgentV2(w, r, http.StatusOK, map[string]bool{"js": created, "cf": !created})
}

func (s *Server) handleAgentV2Stream(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	handshakeCtx, handshakeCancel := context.WithTimeout(r.Context(), 15*time.Second)
	messageType, body, err := conn.Read(handshakeCtx)
	handshakeCancel()
	if err != nil || messageType != websocket.MessageBinary {
		_ = conn.Close(websocket.StatusPolicyViolation, "")
		return
	}
	var request agentv2.HandshakeRequest
	if err := agentv2.DecodeHandshake(body, &request); err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "")
		return
	}
	response, err := s.agentV2.acceptHandshake(r.Context(), request, time.Now().UTC())
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "")
		return
	}
	record, err := s.agentV2.sessionByID(response.SessionID, time.Now().UTC())
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "")
		return
	}
	encodedHandshake, err := agentv2.EncodeHandshake(response)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "")
		return
	}
	writeCtx, writeCancel := context.WithTimeout(r.Context(), 10*time.Second)
	err = conn.Write(writeCtx, websocket.MessageBinary, encodedHandshake)
	writeCancel()
	if err != nil {
		return
	}

	cursor, err := s.store.LatestAgentCursor(r.Context(), record.machine.ID)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "")
		return
	}
	ready := map[string]any{"bh": "zb_" + strconv.FormatInt(cursor, 10), "lx": "dl.jx", "bb": 0, "fssj": time.Now().UTC(), "sj": map[string]any{"yb": cursor}}
	if err := writeAgentV2WS(r.Context(), conn, record.session, r.URL.Path, ready); err != nil {
		return
	}

	streamCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		for {
			messageType, message, readErr := conn.Read(streamCtx)
			if readErr != nil {
				errCh <- readErr
				return
			}
			if messageType != websocket.MessageBinary {
				errCh <- errors.New("plaintext Agent stream message rejected")
				return
			}
			if _, openErr := record.session.Open("WS", r.URL.Path, message); openErr != nil {
				errCh <- openErr
				return
			}
		}
	}()

	pollTicker := time.NewTicker(2 * time.Second)
	defer pollTicker.Stop()
	keepaliveTicker := time.NewTicker(agentStreamKeepaliveInterval)
	defer keepaliveTicker.Stop()
	sessionTimer := time.NewTimer(time.Until(record.expiresAt))
	defer sessionTimer.Stop()
	for {
		select {
		case <-streamCtx.Done():
			return
		case <-sessionTimer.C:
			_ = conn.Close(websocket.StatusNormalClosure, "")
			return
		case <-errCh:
			return
		case <-keepaliveTicker.C:
			if err := writeAgentV2WS(streamCtx, conn, record.session, r.URL.Path, map[string]string{"lx": ""}); err != nil {
				return
			}
		case <-pollTicker.C:
			if _, sessionErr := s.agentV2.sessionByID(response.SessionID, time.Now().UTC()); sessionErr != nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "")
				return
			}
			items, listErr := s.store.AgentChanges(streamCtx, record.machine.ID, cursor, 200)
			if listErr != nil {
				_ = conn.Close(websocket.StatusInternalError, "")
				return
			}
			for _, item := range items {
				event := map[string]any{
					"bh": "bg_" + strconv.FormatInt(item.Cursor, 10), "lx": item.EventType,
					"bb": item.Revision, "fssj": item.CreatedAt, "sj": item.Payload,
				}
				if err := writeAgentV2WS(streamCtx, conn, record.session, r.URL.Path, event); err != nil {
					return
				}
				cursor = item.Cursor
			}
		}
	}
}

func writeAgentV2WS(ctx context.Context, conn *websocket.Conn, session *agentv2.Session, path string, value any) error {
	plain, err := json.Marshal(value)
	if err != nil {
		return err
	}
	envelope, err := session.Seal("WS", path, plain)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageBinary, envelope)
}
