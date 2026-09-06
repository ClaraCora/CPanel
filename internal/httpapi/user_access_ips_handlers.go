package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"cpanel/internal/domain"
	"cpanel/internal/iplocation"
)

const (
	userAccessIPLocationTTL     = 30 * 24 * time.Hour
	userAccessIPLookupCooldown  = time.Minute
	userAccessIPLookupRetention = time.Hour
)

type ipLocationResolver interface {
	Lookup(context.Context, string) (domain.UserAccessIPLocation, error)
}

type userAccessIPLocationInput struct {
	IPAddress string `json:"ip_address"`
	Refresh   bool   `json:"refresh"`
}

type userAccessIPLocationResult struct {
	IPAddress string                      `json:"ip_address"`
	Location  domain.UserAccessIPLocation `json:"location"`
	Cached    bool                        `json:"cached"`
}

func (s *Server) handleListUserAccessIPs(w http.ResponseWriter, r *http.Request) {
	query := parseListQuery(r, "name", map[string]bool{"name": true, "last_seen_at": true})
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	if role != "" && role != "admin" && role != "user" && role != "friend" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "账号分类无效", map[string]string{"role": "请选择有效的账号分类"})
		return
	}
	query.Paginated = query.Paginated || role != ""
	if query.Paginated {
		items, total, err := s.store.ListUserAccessIPsPage(r.Context(), query.Page, query.PageSize, query.Query, query.Sort, query.Order, role)
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, listPage[domain.UserAccessIPAccount]{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total, HasMore: query.Page*query.PageSize < total})
		return
	}
	items, err := s.store.ListUserAccessIPs(r.Context())
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, items)
}

func (s *Server) handleResolveUserAccessIPLocation(w http.ResponseWriter, r *http.Request) {
	var input userAccessIPLocationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	address, err := netip.ParseAddr(strings.TrimSpace(input.IPAddress))
	if err != nil || address.Zone() != "" {
		writeError(w, r, http.StatusUnprocessableEntity, "IP_ADDRESS_INVALID", "IP 地址格式不正确", map[string]string{"ip_address": "请输入有效的 IPv4 或 IPv6 地址"})
		return
	}
	address = address.Unmap()
	normalized := address.String()
	cached, hasLocation, err := s.store.GetUserAccessIPLocation(r.Context(), normalized)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	if !input.Refresh && hasLocation && cached.ResolvedAt.After(time.Now().Add(-userAccessIPLocationTTL)) {
		writeData(w, r, http.StatusOK, userAccessIPLocationResult{IPAddress: normalized, Location: cached, Cached: true})
		return
	}
	if !s.allowUserAccessIPLocationLookup(normalized, time.Now()) {
		writeError(w, r, http.StatusTooManyRequests, "IP_LOCATION_RATE_LIMITED", "该 IP 的归属地刚刚查询过，请稍后再试", nil)
		return
	}
	if s.ipLocation == nil {
		writeError(w, r, http.StatusServiceUnavailable, "IP_LOCATION_UNAVAILABLE", "归属地查询服务暂时不可用", nil)
		return
	}
	location, err := s.ipLocation.Lookup(r.Context(), normalized)
	if err != nil {
		slog.Warn("IP location lookup failed", "request_id", requestID(r), "error", err)
		switch {
		case errors.Is(err, iplocation.ErrProviderRateLimited):
			writeError(w, r, http.StatusTooManyRequests, "IP_LOCATION_RATE_LIMITED", "归属地服务请求过多，请稍后重试", nil)
		case errors.Is(err, context.DeadlineExceeded):
			writeError(w, r, http.StatusGatewayTimeout, "IP_LOCATION_TIMEOUT", "归属地查询超时，请重试", nil)
		default:
			writeError(w, r, http.StatusBadGateway, "IP_LOCATION_LOOKUP_FAILED", "归属地服务暂时不可用，请稍后重试", nil)
		}
		return
	}
	if err := s.store.UpdateUserAccessIPLocation(r.Context(), normalized, location); err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, userAccessIPLocationResult{IPAddress: normalized, Location: location})
}

func (s *Server) allowUserAccessIPLocationLookup(ipAddress string, now time.Time) bool {
	s.ipLocationRateMu.Lock()
	defer s.ipLocationRateMu.Unlock()
	if s.ipLocationLookups == nil {
		s.ipLocationLookups = make(map[string]time.Time)
	}
	if previous, exists := s.ipLocationLookups[ipAddress]; exists && now.Sub(previous) < userAccessIPLookupCooldown {
		return false
	}
	for address, attemptedAt := range s.ipLocationLookups {
		if now.Sub(attemptedAt) > userAccessIPLookupRetention {
			delete(s.ipLocationLookups, address)
		}
	}
	s.ipLocationLookups[ipAddress] = now
	return true
}
