package httpapi

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
)

var ss2022KeySizes = map[string]int{
	"2022-blake3-aes-128-gcm": 16,
	"2022-blake3-aes-256-gcm": 32,
}

var realityFingerprints = map[string]struct{}{
	"chrome": {}, "firefox": {}, "safari": {}, "ios": {}, "android": {},
	"edge": {}, "360": {}, "qq": {}, "random": {},
}

func nodeConfigValidationMessage(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return "节点协议配置必须是有效的对象"
	}
	if message := validateSS2022Config(config); message != "" {
		return message
	}
	return validateRealityConfig(config)
}

func validateSS2022Config(config map[string]any) string {
	cipher, _ := config["cipher"].(string)
	expectedBytes, ok := ss2022KeySizes[cipher]
	if !ok {
		return ""
	}
	serverKey, _ := config["server_key"].(string)
	serverKey = strings.TrimSpace(serverKey)
	if serverKey == "" {
		return "SS2022 需要生成服务端密钥"
	}
	decoded, err := base64.StdEncoding.DecodeString(serverKey)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != serverKey {
		return "SS2022 服务端密钥必须是标准 Base64 编码"
	}
	if len(decoded) != expectedBytes {
		return "SS2022 服务端密钥长度与加密方式不匹配"
	}
	return ""
}

func validateRealityConfig(config map[string]any) string {
	tls, ok := config["tls"].(map[string]any)
	if !ok || !realityEnabled(tls["enabled"]) {
		return ""
	}
	privateKey, _ := tls["private_key"].(string)
	publicKey, _ := tls["public_key"].(string)
	shortID, _ := tls["short_id"].(string)
	serverName, _ := tls["server_name"].(string)
	destination, _ := tls["dest"].(string)
	fingerprint, _ := tls["fingerprint"].(string)
	if strings.TrimSpace(serverName) == "" || strings.TrimSpace(destination) == "" {
		return "Reality 需要填写伪装域名和目标地址"
	}
	if _, ok := realityFingerprints[fingerprint]; !ok {
		return "Reality 客户端伪装指纹无效"
	}
	privateBytes, ok := decodeX25519Key(privateKey)
	if !ok {
		return "Reality 私钥必须是有效的 X25519 Base64URL 密钥"
	}
	publicBytes, ok := decodeX25519Key(publicKey)
	if !ok {
		return "Reality 公钥必须是有效的 X25519 Base64URL 密钥"
	}
	private, err := ecdh.X25519().NewPrivateKey(privateBytes)
	if err != nil || !bytes.Equal(private.PublicKey().Bytes(), publicBytes) {
		return "Reality 公钥与私钥不匹配，请重新随机生成"
	}
	shortBytes, err := hex.DecodeString(shortID)
	if err != nil || len(shortBytes) < 1 || len(shortBytes) > 8 {
		return "Reality Short ID 必须是 2 到 16 位偶数长度的十六进制字符"
	}
	return ""
}

func decodeX25519Key(value string) ([]byte, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return decoded, err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == value
}

func realityEnabled(value any) bool {
	switch typed := value.(type) {
	case float64:
		return typed == 2
	case int:
		return typed == 2
	case string:
		return typed == "2"
	default:
		return false
	}
}
