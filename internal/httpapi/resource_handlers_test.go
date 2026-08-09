package httpapi

import (
	"strings"
	"testing"
)

func TestAgentInstallCommandUsesPOSIXShell(t *testing.T) {
	command := agentInstallCommand(
		"https://example.com/install.sh",
		"https://panel.example.com",
		"secret-value",
		"mch_test",
		"panel-public-key",
	)
	if !strings.Contains(command, "| sudo sh -s --") {
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
