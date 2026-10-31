package webui

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
)

//go:embed public/corade-install.sh
var agentInstaller []byte

func AgentInstaller() []byte       { return append([]byte(nil), agentInstaller...) }
func AgentInstallerSHA256() string { return fmt.Sprintf("%x", sha256.Sum256(agentInstaller)) }
