package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"cpanel/internal/domain"
)

func TestAgentNodeSpecPayloadKeepsIdentitySeparateFromSettings(t *testing.T) {
	payload := agentNodeSpecPayload(domain.AgentNodeSpec{
		NodeID: 42, Revision: 7, Protocol: "vless", ListenIP: "0.0.0.0", ServerPort: 443,
		KernelType: "singbox", Settings: json.RawMessage(`{"transport":"tcp","tls":{"enabled":true}}`),
	}, 45*time.Second, 90*time.Second)

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		NodeID     int64          `json:"node_id"`
		Revision   int            `json:"revision"`
		Protocol   string         `json:"protocol"`
		ListenIP   string         `json:"listen_ip"`
		ServerPort int            `json:"server_port"`
		KernelType string         `json:"kernel_type"`
		Settings   map[string]any `json:"settings"`
		BaseConfig struct {
			Push int `json:"push_interval"`
			Pull int `json:"pull_interval"`
		} `json:"base_config"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.NodeID != 42 || decoded.Revision != 7 || decoded.Protocol != "vless" || decoded.ServerPort != 443 {
		t.Fatalf("unexpected node identity: %+v", decoded)
	}
	if decoded.ListenIP != "0.0.0.0" || decoded.KernelType != "singbox" {
		t.Fatalf("unexpected runtime target: %+v", decoded)
	}
	if decoded.Settings["transport"] != "tcp" || decoded.BaseConfig.Push != 45 || decoded.BaseConfig.Pull != 90 {
		t.Fatalf("unexpected settings or intervals: %+v", decoded)
	}
}
