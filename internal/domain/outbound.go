package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var outboundTagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func ValidateOutbound(tag, protocol string, settings json.RawMessage) error {
	if !outboundTagPattern.MatchString(strings.ToLower(strings.TrimSpace(tag))) {
		return fmt.Errorf("出站标记只能包含小写字母、数字、点、下划线和连字符")
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	allowed := map[string]bool{"vless": true, "vmess": true, "trojan": true, "shadowsocks": true, "socks": true, "http": true, "wireguard": true}
	if !allowed[protocol] {
		return fmt.Errorf("不支持该出站协议")
	}
	var decoded map[string]any
	if len(settings) == 0 || json.Unmarshal(settings, &decoded) != nil {
		return fmt.Errorf("出站参数无效")
	}
	if protocol == "wireguard" {
		if !nonBlank(decoded["secretKey"]) || len(stringValues(decoded["address"])) == 0 {
			return fmt.Errorf("WireGuard 私钥和本地地址不能为空")
		}
		peer := firstMap(decoded["peers"])
		if !nonBlank(peer["publicKey"]) || !nonBlank(peer["endpoint"]) {
			return fmt.Errorf("WireGuard 对端公钥和地址不能为空")
		}
		return nil
	}
	listKey := "servers"
	if protocol == "vless" || protocol == "vmess" {
		listKey = "vnext"
	}
	server := firstMap(decoded[listKey])
	if !nonBlank(server["address"]) {
		return fmt.Errorf("服务器地址不能为空")
	}
	port, err := asOutboundPort(server["port"])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("服务器端口必须在 1 到 65535 之间")
	}
	if protocol == "vless" || protocol == "vmess" {
		if !nonBlank(firstMap(server["users"])["id"]) {
			return fmt.Errorf("用户 UUID 不能为空")
		}
	}
	if (protocol == "trojan" || protocol == "shadowsocks") && !nonBlank(server["password"]) {
		return fmt.Errorf("认证密码不能为空")
	}
	return nil
}

func firstMap(value any) map[string]any {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return map[string]any{}
	}
	item, _ := items[0].(map[string]any)
	return item
}

func stringValues(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, text)
		}
	}
	return result
}

func nonBlank(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func asOutboundPort(value any) (int, error) {
	switch typed := value.(type) {
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("not an integer")
		}
		return int(typed), nil
	case int:
		return typed, nil
	default:
		return 0, fmt.Errorf("not an integer")
	}
}
