package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultAgentReleaseURL = "https://api.github.com/repos/ClaraCora/CPanelde/git/ref/tags/latest"

type agentReleaseResolver struct {
	mu        sync.Mutex
	client    *http.Client
	url       string
	version   string
	checkedAt time.Time
}

func newAgentReleaseResolver(client *http.Client, url string) *agentReleaseResolver {
	if client == nil {
		client = &http.Client{Timeout: 4 * time.Second}
	}
	if strings.TrimSpace(url) == "" {
		url = defaultAgentReleaseURL
	}
	return &agentReleaseResolver{client: client, url: url}
}

func (r *agentReleaseResolver) Latest(ctx context.Context) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	ttl := time.Minute
	if r.version != "" {
		ttl = 10 * time.Minute
	}
	if !r.checkedAt.IsZero() && time.Since(r.checkedAt) < ttl {
		return r.version
	}
	r.checkedAt = time.Now()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return r.version
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "CPanel")
	response, err := r.client.Do(request)
	if err != nil {
		return r.version
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return r.version
	}
	var payload struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if json.NewDecoder(response.Body).Decode(&payload) != nil {
		return r.version
	}
	if version := normalizeAgentVersion(payload.Object.SHA); version != "" {
		r.version = version
	}
	return r.version
}

func normalizeAgentVersion(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) < 12 {
		return ""
	}
	for _, char := range value[:12] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return ""
		}
	}
	return value[:12]
}
