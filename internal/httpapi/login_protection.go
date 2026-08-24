package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLoginMaxFailures = 8
	loginFailureWindow      = 15 * time.Minute
	loginBlockDuration      = 15 * time.Minute
	maxLoginRateEntries     = 10000
)

type loginAttempt struct {
	count       int
	windowStart time.Time
	blockedTill time.Time
	lastSeen    time.Time
}

func (s *Server) loginFailureLimit(ctx context.Context) int {
	if s.store == nil {
		return defaultLoginMaxFailures
	}
	return s.store.SettingInt(ctx, "security", "max_login_failures", defaultLoginMaxFailures, 3)
}

func (s *Server) loginAllowed(scope, ipAddress, login string, maximum int, now time.Time) bool {
	s.loginRateMu.Lock()
	defer s.loginRateMu.Unlock()
	s.pruneLoginAttemptsLocked(now)
	return !s.loginBlockedLocked(loginAccountKey(scope, ipAddress, login), now) &&
		!s.loginBlockedLocked(loginIPKey(scope, ipAddress), now)
}

func (s *Server) recordLoginFailure(scope, ipAddress, login string, maximum int, now time.Time) {
	s.loginRateMu.Lock()
	defer s.loginRateMu.Unlock()
	s.pruneLoginAttemptsLocked(now)
	s.recordLoginAttemptLocked(loginAccountKey(scope, ipAddress, login), maximum, now)
	s.recordLoginAttemptLocked(loginIPKey(scope, ipAddress), aggregateLoginFailureLimit(maximum), now)
}

func (s *Server) clearLoginFailures(scope, ipAddress, login string) {
	s.loginRateMu.Lock()
	defer s.loginRateMu.Unlock()
	delete(s.loginAttempts, loginAccountKey(scope, ipAddress, login))
}

func (s *Server) loginBlockedLocked(key string, now time.Time) bool {
	attempt := s.loginAttempts[key]
	return now.Before(attempt.blockedTill)
}

func (s *Server) recordLoginAttemptLocked(key string, maximum int, now time.Time) {
	if s.loginAttempts == nil {
		s.loginAttempts = make(map[string]loginAttempt)
	}
	attempt := s.loginAttempts[key]
	if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) > loginFailureWindow {
		attempt = loginAttempt{windowStart: now}
	}
	attempt.count++
	attempt.lastSeen = now
	if attempt.count >= maximum {
		attempt.blockedTill = now.Add(loginBlockDuration)
		attempt.count = 0
		attempt.windowStart = now
	}
	s.loginAttempts[key] = attempt
}

func (s *Server) pruneLoginAttemptsLocked(now time.Time) {
	for key, attempt := range s.loginAttempts {
		if now.Sub(attempt.lastSeen) > 2*loginFailureWindow && now.After(attempt.blockedTill) {
			delete(s.loginAttempts, key)
		}
	}
	for len(s.loginAttempts) >= maxLoginRateEntries {
		for key := range s.loginAttempts {
			delete(s.loginAttempts, key)
			break
		}
	}
}

func loginAccountKey(scope, ipAddress, login string) string {
	login = strings.TrimSpace(login)
	if len(login) > 254 {
		login = login[:254]
	}
	return "account\x00" + scope + "\x00" + strings.TrimSpace(ipAddress) + "\x00" + strings.ToLower(login)
}

func loginIPKey(scope, ipAddress string) string {
	return "ip\x00" + scope + "\x00" + strings.TrimSpace(ipAddress)
}

func aggregateLoginFailureLimit(maximum int) int {
	aggregate := maximum * 4
	if aggregate < 20 {
		return 20
	}
	return aggregate
}

func writeLoginRateLimited(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", strconv.Itoa(int(loginBlockDuration.Seconds())))
	writeError(w, r, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "登录尝试过于频繁，请稍后重试", nil)
}
