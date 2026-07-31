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
