package iplocation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"cpanel/internal/domain"
)

const (
	defaultEndpoint = "https://ipwho.is"
	maxResponseSize = 64 << 10
	maxFieldRunes   = 160
)

var (
	ErrInvalidAddress      = errors.New("invalid IP address")
	ErrProviderRateLimited = errors.New("IP location provider rate limited")
	ErrProviderUnavailable = errors.New("IP location provider unavailable")
	nonPublicPrefixes      = parsePrefixes([]string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
		"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
		"::/128", "::1/128", "100::/64", "2001:db8::/32", "fc00::/7", "fe80::/10", "ff00::/8",
	})
)

type Resolver struct {
	client   *http.Client
	endpoint *url.URL
	now      func() time.Time
}

type providerResponse struct {
	Success     bool   `json:"success"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Region      string `json:"region"`
	City        string `json:"city"`
	Connection  struct {
		ISP string `json:"isp"`
	} `json:"connection"`
}

func New(client *http.Client, endpoint string) *Resolver {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultEndpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		parsed, _ = url.Parse(defaultEndpoint)
	}
	if client == nil {
		expectedHost := parsed.Host
		client = &http.Client{
			Timeout: 4 * time.Second,
			CheckRedirect: func(request *http.Request, _ []*http.Request) error {
				if request.URL.Host != expectedHost {
					return errors.New("IP location redirect host rejected")
				}
				return nil
			},
		}
	}
	return &Resolver{client: client, endpoint: parsed, now: time.Now}
}

func (r *Resolver) Lookup(ctx context.Context, rawAddress string) (domain.UserAccessIPLocation, error) {
	address, err := netip.ParseAddr(strings.TrimSpace(rawAddress))
	if err != nil || address.Zone() != "" {
		return domain.UserAccessIPLocation{}, ErrInvalidAddress
	}
	address = address.Unmap()
	if isNonPublic(address) {
		return domain.UserAccessIPLocation{
			Scope: "private", Country: "内网/保留地址", ResolvedAt: r.now().UTC(),
		}, nil
	}

	target := *r.endpoint
	target.Path = strings.TrimRight(target.Path, "/") + "/" + url.PathEscape(address.String())
	query := target.Query()
	query.Set("lang", "zh-CN")
	target.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return domain.UserAccessIPLocation{}, fmt.Errorf("create IP location request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "CPanel-IP-Location/1")
	response, err := r.client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.UserAccessIPLocation{}, errors.Join(ErrProviderUnavailable, context.DeadlineExceeded)
		}
		return domain.UserAccessIPLocation{}, ErrProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		return domain.UserAccessIPLocation{}, ErrProviderRateLimited
	}
	if response.StatusCode != http.StatusOK {
		return domain.UserAccessIPLocation{}, fmt.Errorf("%w: status %d", ErrProviderUnavailable, response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return domain.UserAccessIPLocation{}, fmt.Errorf("%w: read response", ErrProviderUnavailable)
	}
	if len(payload) > maxResponseSize {
		return domain.UserAccessIPLocation{}, fmt.Errorf("%w: response too large", ErrProviderUnavailable)
	}
	var provider providerResponse
	if err := json.Unmarshal(payload, &provider); err != nil || !provider.Success {
		return domain.UserAccessIPLocation{}, fmt.Errorf("%w: invalid response", ErrProviderUnavailable)
	}
	location := domain.UserAccessIPLocation{
		Scope:       "public",
		CountryCode: cleanField(provider.CountryCode),
		Country:     cleanField(provider.Country),
		Province:    cleanField(provider.Region),
		City:        cleanField(provider.City),
		ISP:         cleanField(provider.Connection.ISP),
		ResolvedAt:  r.now().UTC(),
	}
	if location.Country == "" && location.Province == "" && location.City == "" {
		return domain.UserAccessIPLocation{}, fmt.Errorf("%w: empty location", ErrProviderUnavailable)
	}
	return location, nil
}

func isNonPublic(address netip.Addr) bool {
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return !address.IsGlobalUnicast()
}

func cleanField(value string) string {
	value = strings.TrimSpace(value)
	if value == "0" || value == "-" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maxFieldRunes {
		value = string(runes[:maxFieldRunes])
	}
	return value
}

func parsePrefixes(values []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}
