package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"cpanel/internal/domain"
	"gopkg.in/yaml.v3"
)

type clashDocument struct {
	MixedPort   int              `yaml:"mixed-port"`
	Mode        string           `yaml:"mode"`
	LogLevel    string           `yaml:"log-level"`
	IPv6        bool             `yaml:"ipv6"`
	Proxies     []map[string]any `yaml:"proxies"`
	ProxyGroups []map[string]any `yaml:"proxy-groups"`
	Rules       []string         `yaml:"rules"`
}

func BuildClashMeta(data domain.Subscription) ([]byte, error) {
	proxies := make([]map[string]any, 0, len(data.Nodes))
	names := make([]string, 0, len(data.Nodes))
	for _, node := range data.Nodes {
		proxy, err := proxyForNode(node)
		if err != nil {
			return nil, fmt.Errorf("node %q: %w", node.Name, err)
		}
		proxies = append(proxies, proxy)
		names = append(names, node.Name)
	}
	selectNames := append([]string{"DIRECT"}, names...)
	groups := []map[string]any{{"name": "CPanel", "type": "select", "proxies": selectNames}}
	if len(names) > 0 {
		selectNames = append([]string{"自动选择"}, selectNames...)
		groups[0]["proxies"] = selectNames
		groups = append(groups, map[string]any{"name": "自动选择", "type": "url-test", "url": "https://www.gstatic.com/generate_204", "interval": 300, "proxies": names})
	}
	document := clashDocument{
		MixedPort: 7890, Mode: "rule", LogLevel: "info", IPv6: true, Proxies: proxies,
		ProxyGroups: groups,
		Rules:       []string{"MATCH,CPanel"},
	}
	return yaml.Marshal(document)
}

func proxyForNode(node domain.SubscriptionNode) (map[string]any, error) {
	var config map[string]any
	if len(node.NodeConfig) > 0 {
		if err := json.Unmarshal(node.NodeConfig, &config); err != nil {
			return nil, fmt.Errorf("invalid node config: %w", err)
		}
	}
	protocol := strings.ToLower(node.Protocol)
	proxy := map[string]any{"name": node.Name, "server": node.Host, "port": node.Port}
	switch protocol {
	case "vmess":
		proxy["type"] = "vmess"
		proxy["uuid"] = node.UserUUID
		proxy["alterId"] = 0
		proxy["cipher"] = "auto"
	case "vless":
		proxy["type"] = "vless"
		proxy["uuid"] = node.UserUUID
		copyOptional(proxy, config, "flow", "flow")
	case "trojan":
		proxy["type"] = "trojan"
		proxy["password"] = node.UserUUID
	case "shadowsocks":
		proxy["type"] = "ss"
		cipher, _ := config["cipher"].(string)
		if cipher == "" {
			return nil, fmt.Errorf("missing cipher")
		}
		proxy["cipher"] = cipher
		proxy["password"] = node.UserUUID
		if userKey, ok := ss2022UserKey(cipher, node.UserUUID); ok {
			serverKey, _ := config["server_key"].(string)
			serverKey = strings.TrimSpace(serverKey)
			if serverKey == "" {
				return nil, fmt.Errorf("missing SS2022 server key")
			}
			proxy["password"] = serverKey + ":" + userKey
		}
	case "hysteria", "hysteria2":
		proxy["type"] = "hysteria2"
		proxy["password"] = node.UserUUID
	case "tuic":
		proxy["type"] = "tuic"
		proxy["uuid"] = node.UserUUID
		proxy["password"] = node.UserUUID
	case "anytls":
		proxy["type"] = "anytls"
		proxy["password"] = node.UserUUID
	default:
		return nil, fmt.Errorf("protocol %q is not supported by Clash Meta subscription", protocol)
	}
	copyOptional(proxy, config, "servername", "server_name")
	if _, ok := config["transport"]; ok {
		copyOptional(proxy, config, "network", "transport")
	} else {
		copyOptional(proxy, config, "network", "network")
	}
	if tlsValue, ok := config["tls"]; ok {
		proxy["tls"] = tlsEnabled(tlsValue)
		if tls, ok := tlsValue.(map[string]any); ok {
			copyOptional(proxy, tls, "servername", "server_name")
			if realityEnabled(tls["enabled"]) {
				copyOptional(proxy, tls, "client-fingerprint", "fingerprint")
				publicKey, _ := tls["public_key"].(string)
				shortID, _ := tls["short_id"].(string)
				if publicKey != "" && shortID != "" {
					proxy["reality-opts"] = map[string]any{"public-key": publicKey, "short-id": shortID}
				}
			}
		}
	}
	return proxy, nil
}

func ss2022UserKey(cipher, udid string) (string, bool) {
	keySizes := map[string]int{
		"2022-blake3-aes-128-gcm":       16,
		"2022-blake3-aes-256-gcm":       32,
		"2022-blake3-chacha20-poly1305": 32,
	}
	size, ok := keySizes[cipher]
	if !ok {
		return "", false
	}
	raw := make([]byte, size)
	copy(raw, udid)
	return base64.StdEncoding.EncodeToString(raw), true
}

func copyOptional(target map[string]any, source map[string]any, targetKey, sourceKey string) {
	if value, ok := source[sourceKey]; ok && value != nil && value != "" {
		target[targetKey] = value
	}
}

func tlsEnabled(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed > 0
	case string:
		return typed != "" && typed != "0" && typed != "false"
	case map[string]any:
		return tlsEnabled(typed["enabled"])
	default:
		return false
	}
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
