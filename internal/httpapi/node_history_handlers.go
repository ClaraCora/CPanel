package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleNodeRevisions(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListNodeRevisions(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleRestoreNodeRevision(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Revision int `json:"revision"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Revision < 1 {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "请选择有效的历史版本", map[string]string{"revision": "版本必须大于零"})
		return
	}
	item, err := s.store.RestoreNodeRevision(r.Context(), chi.URLParam(r, "id"), input.Revision)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	_ = s.store.WriteAudit(r.Context(), currentAdmin(r).ID, "node.restore", "node", item.ID, map[string]int{"revision": input.Revision}, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, item)
}
