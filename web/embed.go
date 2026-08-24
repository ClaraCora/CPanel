package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
)

//go:embed dist
var assets embed.FS

func Handler() http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject parent segments before path.Clean. Cleaning first would make a
		// request such as /assets/portal/../secure/app.js indistinguishable from
		// a legitimate path and could cross the public/protected asset boundary.
		if hasParentPathSegment(r.URL.EscapedPath()) {
			http.NotFound(w, r)
			return
		}
		assetPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if assetPath == "." {
			assetPath = ""
		}

		if isBackendPath(assetPath) {
			http.NotFound(w, r)
			return
		}

		if assetPath != "" {
			info, statErr := fs.Stat(dist, assetPath)
			if statErr == nil && !info.IsDir() {
				if strings.HasPrefix(assetPath, "assets/secure/") || strings.HasPrefix(assetPath, "assets/portal/") {
					w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
					w.Header().Add("Vary", "Cookie")
				} else if strings.HasPrefix(assetPath, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(assetPath, "assets/") {
				http.NotFound(w, r)
				return
			}
		}

		// The SPA document is an entry point for both login shells.  Keep it out
		// of browser/proxy caches so a future server-side change cannot expose a
		// stale authenticated shell or inline configuration.
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Add("Vary", "Cookie")
		request := r.Clone(r.Context())
		request.URL.Path = "/"
		files.ServeHTTP(w, request)
	})
}

func hasParentPathSegment(value string) bool {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return true
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func isBackendPath(value string) bool {
	for _, prefix := range []string{"api", "ca", "health"} {
		if value == prefix || strings.HasPrefix(value, prefix+"/") {
			return true
		}
	}
	return false
}
