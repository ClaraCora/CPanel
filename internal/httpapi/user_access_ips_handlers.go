package httpapi

import (
	"net/http"
)

func (s *Server) handleListUserAccessIPs(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListUserAccessIPs(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}
