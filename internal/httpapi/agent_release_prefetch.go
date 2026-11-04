package httpapi

import (
	"context"
	"net/http"
	"strings"
)

type prefetchedAgentReleaseKey struct{}

// Resolve remote release metadata before administrative transactions start.
// Otherwise GitHub latency prolongs presence row locks on list requests and the
// global resource lock on upgrade requests, delaying unrelated writes/heartbeats.
func (s *Server) prefetchAgentRelease(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ca/ht/"), "/")
		list := r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "fwq"
		upgrade := r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "fwq" && parts[1] != "" && parts[2] == "sj"
		if list || upgrade {
			version := s.agentRelease.Latest(r.Context())
			ctx := context.WithValue(r.Context(), prefetchedAgentReleaseKey{}, version)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) agentReleaseVersion(r *http.Request) string {
	if version, ok := r.Context().Value(prefetchedAgentReleaseKey{}).(string); ok {
		return version // An empty result was fetched too; do not perform more I/O.
	}
	return s.agentRelease.Latest(r.Context())
}
