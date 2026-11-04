package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cpanel/internal/store"
	"github.com/jackc/pgx/v5"
)

type adminConsistencyRoute struct {
	resource string
	id       string
	section  string
	response bool
	intent   bool
}

func consistencyRoute(r *http.Request) adminConsistencyRoute {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/ca/ht/"), "/"), "/")
	resources := map[string]string{"fwq": "machines", "jd": "nodes", "yh": "users", "tc": "plans", "qxz": "access_groups", "ly": "route_policies", "ck": "outbounds"}
	if len(parts) == 2 && parts[0] == "sz" && allowedSettingSections[settingSectionPaths[parts[1]]] {
		return adminConsistencyRoute{section: settingSectionPaths[parts[1]], response: true}
	}
	resource := resources[parts[0]]
	if resource == "" {
		return adminConsistencyRoute{}
	}
	if len(parts) == 1 {
		return adminConsistencyRoute{resource: resource, response: true, intent: r.Method == http.MethodPost}
	}
	if len(parts) == 2 {
		return adminConsistencyRoute{resource: resource, id: parts[1], response: true}
	}
	if resource == "users" && len(parts) == 3 && parts[1] == "ks" {
		return adminConsistencyRoute{resource: resource, response: true, intent: r.Method == http.MethodPost}
	}
	if resource == "nodes" && len(parts) == 3 && (parts[2] == "fb" || parts[2] == "hf") {
		return adminConsistencyRoute{resource: resource, id: parts[1], response: true, intent: r.Method == http.MethodPost}
	}
	return adminConsistencyRoute{}
}

type bufferedAdminResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newBufferedAdminResponse() *bufferedAdminResponse {
	return &bufferedAdminResponse{header: make(http.Header)}
}
func (w *bufferedAdminResponse) Header() http.Header { return w.header }
func (w *bufferedAdminResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *bufferedAdminResponse) Write(data []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.body.Write(data)
}
func (w *bufferedAdminResponse) flush(target http.ResponseWriter) {
	for key, values := range w.header {
		target.Header()[key] = append([]string(nil), values...)
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	target.WriteHeader(w.status)
	_, _ = target.Write(w.body.Bytes())
}

func (s *Server) adminConsistency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := consistencyRoute(r)
		read := r.Method == http.MethodGet
		if read && !route.response || standaloneAdminOperation(r) {
			next.ServeHTTP(w, r)
			return
		}
		var body []byte
		var err error
		if !read && r.Body != nil {
			body, err = io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "请求内容过大或无法读取", nil)
				return
			}
		}
		for attempt := 0; ; attempt++ {
			response, err := s.runConsistentAdminRequest(r, next, route, body, read)
			if read && store.IsRetryableTransactionError(err) && attempt < 2 {
				continue
			}
			if err != nil {
				if store.IsRetryableTransactionError(err) {
					writeError(w, r, http.StatusConflict, "CONCURRENT_WRITE_RETRY", "配置正在被其他请求修改，请保留输入并重试", nil)
				} else {
					writeStoreError(w, r, err)
				}
				return
			}
			response.flush(w)
			return
		}
	})
}

// These operations have no configuration transaction to commit. In particular,
// revoking a session must survive an unavailable audit sink, and an already sent
// Telegram message cannot be rolled back. Authentication and CSRF still run in
// requireAdmin before this middleware.
func standaloneAdminOperation(r *http.Request) bool {
	switch r.Method + " " + r.URL.Path {
	case "DELETE /ca/ht/hh/dq", "POST /ca/ht/sz/tg/cs", "POST /ca/ht/gj/xsmy", "POST /ca/ht/yh/fwjl/gs":
		return true
	default:
		return false
	}
}

func (s *Server) runConsistentAdminRequest(r *http.Request, next http.Handler, route adminConsistencyRoute, body []byte, read bool) (*bufferedAdminResponse, error) {
	options := pgx.TxOptions{}
	if read {
		// Resource reads retain existing presence/admin/monthly maintenance. A
		// repeatable-read read/write transaction gives the record and its edit
		// version one coherent snapshot without prohibiting that maintenance.
		options.IsoLevel = pgx.RepeatableRead
	}
	ctx, transaction, err := s.store.BeginRequest(r.Context(), options)
	if err != nil {
		return nil, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = transaction.Rollback(cleanup)
	}()
	r = r.WithContext(ctx)
	if !read {
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err := s.store.ConfigureAdminMutation(ctx); err != nil {
			return nil, err
		}
	}
	response := newBufferedAdminResponse()
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var hash [sha256.Size]byte
	if route.intent && key != "" {
		if !validIntentKey(key) {
			writeError(response, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID", "操作标识格式无效，请重新打开编辑页面", nil)
			return response, nil
		}
		if s.secureBox == nil {
			return nil, fmt.Errorf("idempotency result encryption is unavailable")
		}
		hash = sha256.Sum256(append([]byte(r.Method+"\n"+r.URL.RequestURI()+"\n"), body...))
		replayed, err := s.store.ClaimIdempotency(ctx, currentAdmin(r).ID, key, hash[:])
		if err != nil {
			return nil, err
		}
		if replayed != nil {
			plain, err := s.secureBox.Open(replayed.Response)
			if err != nil {
				return nil, err
			}
			response.status = replayed.Status
			response.Header().Set("Content-Type", "application/json; charset=utf-8")
			response.Header().Set("Idempotency-Replayed", "true")
			response.body.Write(withCurrentRequestID(plain, requestID(r)))
			if err := transaction.Commit(ctx); err != nil {
				return nil, err
			}
			return response, nil
		}
	}
	if !read || route.resource == "users" && route.id == "" {
		if err := s.store.AcquireAdminConsistencyLocks(ctx, route.resource); err != nil {
			return nil, err
		}
	}
	if !read && r.Header.Get("If-Match") != "" {
		expected, err := parseEditVersion(r.Header.Get("If-Match"))
		if err != nil {
			writeError(response, r, http.StatusBadRequest, "EDIT_VERSION_INVALID", "配置版本无效，请重新加载当前记录", nil)
			return response, nil
		}
		if route.section != "" {
			actual, err := s.store.SettingSectionConfigVersion(ctx, route.section, true)
			if err != nil {
				return nil, err
			}
			if actual != expected {
				return nil, store.ErrEditVersionConflict
			}
		} else if route.resource != "" && route.id != "" {
			if err := s.store.CheckResourceConfigVersion(ctx, route.resource, route.id, expected); err != nil {
				return nil, err
			}
		}
	}
	next.ServeHTTP(response, r)
	if err := transaction.Error(); err != nil {
		return nil, err
	}
	if response.status >= http.StatusBadRequest {
		return response, nil // deferred rollback also releases any intent claim
	}
	if route.response {
		if err := s.attachConfigVersions(ctx, response, route); err != nil {
			return nil, err
		}
	}
	if route.intent && key != "" {
		sealed, err := s.secureBox.Seal(response.body.Bytes())
		if err != nil {
			return nil, err
		}
		if err := s.store.CompleteIdempotency(ctx, currentAdmin(r).ID, key, hash[:], response.status, sealed); err != nil {
			return nil, err
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

func validIntentKey(value string) bool {
	if len(value) < 8 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._:-", char) {
			continue
		}
		return false
	}
	return true
}

func parseEditVersion(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) && len(value) > 2 {
		value = value[1 : len(value)-1]
	}
	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("invalid edit version")
	}
	return version, nil
}

func withCurrentRequestID(data []byte, id string) []byte {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(data, &envelope) != nil {
		return data
	}
	var meta map[string]json.RawMessage
	_ = json.Unmarshal(envelope["meta"], &meta)
	if meta == nil {
		meta = make(map[string]json.RawMessage)
	}
	meta["request_id"], _ = json.Marshal(id)
	envelope["meta"], _ = json.Marshal(meta)
	result, err := json.Marshal(envelope)
	if err != nil {
		return data
	}
	return result
}

func (s *Server) attachConfigVersions(ctx context.Context, response *bufferedAdminResponse, route adminConsistencyRoute) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(response.body.Bytes(), &envelope); err != nil {
		return err
	}
	if route.section != "" {
		version, err := s.store.SettingSectionConfigVersion(ctx, route.section, false)
		if err != nil {
			return err
		}
		var meta map[string]json.RawMessage
		if err := json.Unmarshal(envelope["meta"], &meta); err != nil {
			return err
		}
		meta["config_version"], _ = json.Marshal(version)
		envelope["meta"], _ = json.Marshal(meta)
		response.Header().Set("ETag", fmt.Sprintf(`"%d"`, version))
	} else if route.resource != "" {
		data := bytes.TrimSpace(envelope["data"])
		var records []map[string]json.RawMessage
		list := len(data) > 0 && data[0] == '['
		if list {
			if err := json.Unmarshal(data, &records); err != nil {
				return err
			}
		} else if len(data) > 0 && data[0] == '{' {
			var record map[string]json.RawMessage
			if err := json.Unmarshal(data, &record); err != nil {
				return err
			}
			records = []map[string]json.RawMessage{record}
		}
		ids := make([]string, 0, len(records))
		for _, record := range records {
			var id string
			if json.Unmarshal(record["id"], &id) == nil && id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			versions, err := s.store.ResourceConfigVersions(ctx, route.resource, ids)
			if err != nil {
				return err
			}
			for _, record := range records {
				var id string
				_ = json.Unmarshal(record["id"], &id)
				if version, ok := versions[id]; ok {
					record["config_version"], _ = json.Marshal(version)
					if !list {
						response.Header().Set("ETag", fmt.Sprintf(`"%d"`, version))
					}
				} else {
					return store.ErrNotFound
				}
			}
			if list {
				envelope["data"], _ = json.Marshal(records)
			} else {
				envelope["data"], _ = json.Marshal(records[0])
			}
		}
	}
	result, err := json.Marshal(envelope)
	if err == nil {
		response.body.Reset()
		response.body.Write(result)
	}
	return err
}
