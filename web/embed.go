package webui

import (
	"embed"
	"io/fs"
	"net/http"
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
				if strings.HasPrefix(assetPath, "assets/secure/") {
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

		request := r.Clone(r.Context())
		request.URL.Path = "/"
		files.ServeHTTP(w, request)
	})
}

func isBackendPath(value string) bool {
	for _, prefix := range []string{"api", "ca", "health"} {
		if value == prefix || strings.HasPrefix(value, prefix+"/") {
			return true
		}
	}
	return false
}
