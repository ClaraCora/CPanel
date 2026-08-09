package httpapi

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const defaultAgentReleaseURL = "https://github.com/ClaraCora/CPanelde/releases/download/latest/agent-version.txt"

var semanticAgentVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9a-z.-]+)?$`)

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
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("User-Agent", "CPanel")
	response, err := r.client.Do(request)
	if err != nil {
		return r.version
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return r.version
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 128))
	if err != nil {
		return r.version
	}
	if version := normalizeAgentVersion(string(payload)); version != "" {
		r.version = version
	}
	return r.version
}

func normalizeAgentVersion(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if semanticAgentVersionPattern.MatchString(value) {
		return value
	}
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
