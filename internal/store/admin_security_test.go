package store

import (
	"encoding/json"
	"testing"
)

func TestMarshalAuditChangesRedactsNestedSecrets(t *testing.T) {
	input := map[string]any{
		"name": "node-a",
		"config": json.RawMessage(`{
			"tls":{"private_key":"reality-private","public_key":"reality-public","key_content":"pem-private"},
			"server_key":"ss2022-key",
			"servers":[{"address":"127.0.0.1","password":"outbound-password"}],
			"secretKey":"wireguard-private"
		}`),
	}
	data, err := marshalAuditChanges("node", input)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["config"] != auditRedactedValue {
		t.Fatalf("node configuration was not redacted: %#v", result)
	}
	if result["name"] != "node-a" {
		t.Fatalf("non-sensitive audit field changed: %#v", result)
	}
}

func TestMarshalAuditChangesRedactsGenericNestedCredentialFields(t *testing.T) {
	data, err := marshalAuditChanges("user", map[string]any{
		"name":   "user-a",
		"nested": map[string]any{"password": "secret", "public_key": "public"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	nested := result["nested"].(map[string]any)
	if nested["password"] != auditRedactedValue || nested["public_key"] != "public" {
		t.Fatalf("generic nested redaction failed: %#v", nested)
	}
}

func TestSensitiveAuditKeyCoversCredentialsWithoutHidingPublicKeys(t *testing.T) {
	for _, key := range []string{"subscription_token", "portal_password_hash", "uuid", "obfs-password", "dns_api_token"} {
		if !sensitiveAuditKey(key) {
			t.Errorf("sensitiveAuditKey(%q) = false", key)
		}
	}
	for _, key := range []string{"public_key", "token_prefix", "name", "host"} {
		if sensitiveAuditKey(key) {
			t.Errorf("sensitiveAuditKey(%q) = true", key)
		}
	}
}
