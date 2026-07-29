package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestNodeConfigValidationAcceptsGeneratedRealityCredentials(t *testing.T) {
	credentials, err := newRealityCredentials()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"tls": map[string]any{
		"enabled": 2, "private_key": credentials.PrivateKey, "public_key": credentials.PublicKey,
		"short_id": credentials.ShortID, "server_name": "www.example.com", "dest": "www.example.com:443", "fingerprint": "chrome",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if message := nodeConfigValidationMessage(raw); message != "" {
		t.Fatalf("unexpected validation message: %s", message)
	}
}

func TestNodeConfigValidationRejectsMismatchedRealityPublicKey(t *testing.T) {
	first, _ := newRealityCredentials()
	second, _ := newRealityCredentials()
	raw, _ := json.Marshal(map[string]any{"tls": map[string]any{
		"enabled": 2, "private_key": first.PrivateKey, "public_key": second.PublicKey,
		"short_id": first.ShortID, "server_name": "www.example.com", "dest": "www.example.com:443", "fingerprint": "chrome",
	}})
	if message := nodeConfigValidationMessage(raw); !strings.Contains(message, "不匹配") {
		t.Fatalf("validation message = %q", message)
	}
}

func TestNodeConfigValidationChecksSS2022KeySize(t *testing.T) {
	valid128 := base64.StdEncoding.EncodeToString(make([]byte, 16))
	valid256 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cases := []struct {
		name    string
		cipher  string
		key     string
		wantErr bool
	}{
		{name: "aes128", cipher: "2022-blake3-aes-128-gcm", key: valid128},
		{name: "aes256", cipher: "2022-blake3-aes-256-gcm", key: valid256},
		{name: "wrong size", cipher: "2022-blake3-aes-256-gcm", key: valid128, wantErr: true},
		{name: "invalid base64", cipher: "2022-blake3-aes-128-gcm", key: "not-base64", wantErr: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"cipher": test.cipher, "server_key": test.key})
			message := nodeConfigValidationMessage(raw)
			if (message != "") != test.wantErr {
				t.Fatalf("validation message = %q, wantErr=%v", message, test.wantErr)
			}
		})
	}
}
