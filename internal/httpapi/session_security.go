package httpapi

import (
	"context"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	minimumSessionTTLMinutes  = 15
	maximumSessionTTLMinutes  = 30 * 24 * 60
	maximumLoginPasswordRunes = 1024
)

func (s *Server) sessionTTL(ctx context.Context) time.Duration {
	fallback := s.cfg.SessionTTL
	if fallback <= 0 {
		fallback = 24 * time.Hour
	}
	fallbackMinutes := int(fallback / time.Minute)
	if fallbackMinutes < minimumSessionTTLMinutes {
		fallbackMinutes = minimumSessionTTLMinutes
	}
	if fallbackMinutes > maximumSessionTTLMinutes {
		fallbackMinutes = maximumSessionTTLMinutes
	}
	if s.store == nil {
		return time.Duration(fallbackMinutes) * time.Minute
	}
	minutes := s.store.SettingInt(ctx, "security", "session_ttl_minutes", fallbackMinutes, minimumSessionTTLMinutes)
	if minutes > maximumSessionTTLMinutes {
		minutes = maximumSessionTTLMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func (s *Server) secureCookies(ctx context.Context) bool {
	if s.cfg.CookieSecure {
		return true
	}
	values := []string{s.cfg.ExternalURL}
	if s.store != nil {
		values = append(values, s.store.SettingString(ctx, "site", "site_url", ""))
	}
	for _, value := range values {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err == nil && strings.EqualFold(parsed.Scheme, "https") {
			return true
		}
	}
	return false
}

func validLoginPassword(value string) bool {
	count := utf8.RuneCountInString(value)
	return count > 0 && count <= maximumLoginPasswordRunes
}

func validAdminLogin(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 254
}
