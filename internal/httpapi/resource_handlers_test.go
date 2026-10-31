package httpapi

import (
	"strings"
	"testing"

	"cpanel/internal/domain"
)

func TestAgentInstallCommandUsesPOSIXShell(t *testing.T) {
	command := agentInstallCommand(
		"https://example.com/install.sh",
		"https://panel.example.com",
		"secret-value",
		"mch_test",
		"panel-public-key",
	)
	if !strings.Contains(command, `sudo sh "$installer"`) || !strings.Contains(command, "sha256sum -c -") {
		t.Fatalf("install command does not use POSIX sh: %q", command)
	}
	if strings.Contains(command, "bash") {
		t.Fatalf("install command must not require bash: %q", command)
	}
}

func TestValidPlanResetStrategy(t *testing.T) {
	for _, value := range []string{"calendar_month", "never"} {
		if !validPlanResetStrategy(value) {
			t.Errorf("validPlanResetStrategy(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"", "month", "daily", "CALENDAR_MONTH"} {
		if validPlanResetStrategy(value) {
			t.Errorf("validPlanResetStrategy(%q) = true, want false", value)
		}
	}
}

func TestAgentControlURLRequiresTLSOutsideNumericLoopback(t *testing.T) {
	for _, value := range []string{"https://panel.example.com", "http://127.0.0.1:8256", "http://[::1]:8256"} {
		if err := validateAgentControlURL(value); err != nil {
			t.Errorf("valid %q: %v", value, err)
		}
	}
	for _, value := range []string{"http://panel.example.com", "http://localhost", "https://user:password@panel.example.com", "https://panel.example.com?key=abc", "ftp://panel.example.com", "https:///missing-host"} {
		if err := validateAgentControlURL(value); err == nil {
			t.Errorf("unsafe control URL accepted: %q", value)
		}
	}
}

func TestValidNodeEndpointsAccessScope(t *testing.T) {
	fields := map[string]string{}
	validNodeEndpoints([]domain.NodeEndpoint{
		{Name: "默认入口", Host: "edge.example.com", Port: 443, AccessScope: "default"},
		{Name: "管理入口", Host: "admin.example.com", Port: 8443, AccessScope: "admin"},
		{Name: "旧入口", Host: "legacy.example.com", Port: 443},
	}, fields)
	if len(fields) != 0 {
		t.Fatalf("valid endpoint access scopes rejected: %#v", fields)
	}

	fields = map[string]string{}
	validNodeEndpoints([]domain.NodeEndpoint{{Name: "入口", Host: "edge.example.com", Port: 443, AccessScope: "private"}}, fields)
	if got := fields["endpoints.0.access_scope"]; got != "invalid" {
		t.Fatalf("invalid access scope error = %q, want invalid", got)
	}
}
