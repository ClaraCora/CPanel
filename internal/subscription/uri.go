package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"cpanel/internal/domain"
)

// BuildNodeURI serializes the same normalized node data used by subscriptions.
// The portal never accepts a client-provided node configuration for this path.
func BuildNodeURI(node domain.SubscriptionNode) (string, error) {
	if strings.TrimSpace(node.Host) == "" || node.Port < 1 || node.Port > 65535 {
		return "", fmt.Errorf("missing endpoint")
	}
	if strings.TrimSpace(node.UserUUID) == "" {
		return "", fmt.Errorf("missing user credential")
	}
	config, err := parseNodeConfig(node.NodeConfig)
	if err != nil {
		return "", err
	}
	protocol := strings.ToLower(strings.TrimSpace(node.Protocol))
	switch protocol {
	case "vless":
		return buildVLESSURI(node, config), nil
	case "vmess":
		return buildVMessURI(node, config)
	case "trojan":
		return buildTrojanURI(node, config), nil
	case "shadowsocks":
		return buildShadowsocksURI(node, config)
	case "hysteria", "hysteria2":
		return buildHysteriaURI(node, config)
	case "tuic":
		return buildTUICURI(node, config), nil
	case "anytls":
		return buildAnyTLSURI(node, config), nil
	default:
		return "", fmt.Errorf("protocol %q does not support direct import", protocol)
	}
}

func parseNodeConfig(raw json.RawMessage) (map[string]any, error) {
	config := map[string]any{}
	if len(raw) == 0 {
		return config, nil
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("invalid node configuration: %w", err)
	}
	return config, nil
}

func buildVLESSURI(node domain.SubscriptionNode, config map[string]any) string {
	query := url.Values{}
	query.Set("encryption", stringConfig(config, "decryption", "none"))
	addTransportQuery(query, config)
	addTLSQuery(query, config)
	if flow := stringConfig(config, "flow", ""); flow != "" {
		query.Set("flow", flow)
	}
	return endpointURL("vless", url.User(node.UserUUID), node, query)
}

func buildVMessURI(node domain.SubscriptionNode, config map[string]any) (string, error) {
	tls := tlsConfig(config)
	network := transportConfig(config)
	payload := map[string]string{
		"v": "2", "ps": displayName(node), "add": node.Host, "port": strconv.Itoa(node.Port),
		"id": node.UserUUID, "aid": "0", "scy": "auto", "net": network.kind, "type": "none",
	}
	if network.kind == "ws" || network.kind == "httpupgrade" || network.kind == "h2" || network.kind == "xhttp" {
		payload["path"] = network.path
		payload["host"] = network.host
	}
	if network.kind == "grpc" {
		payload["path"] = network.serviceName
	}
	if tls.enabled {
		payload["tls"] = "tls"
		payload["sni"] = tls.serverName
		payload["fp"] = tls.fingerprint
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.RawStdEncoding.EncodeToString(encoded), nil
}

func buildTrojanURI(node domain.SubscriptionNode, config map[string]any) string {
	query := url.Values{}
	addTransportQuery(query, config)
	addTLSQuery(query, config)
	return endpointURL("trojan", url.User(node.UserUUID), node, query)
}

func buildShadowsocksURI(node domain.SubscriptionNode, config map[string]any) (string, error) {
	cipher := stringConfig(config, "cipher", "")
	if cipher == "" {
		return "", fmt.Errorf("missing Shadowsocks cipher")
	}
	password := node.UserUUID
	if key, ok := ss2022UserKey(cipher, node.UserUUID); ok {
		serverKey := strings.TrimSpace(stringConfig(config, "server_key", ""))
		if serverKey == "" {
			return "", fmt.Errorf("missing SS2022 server key")
		}
		password = serverKey + ":" + key
	}
	credentials := base64.RawURLEncoding.EncodeToString([]byte(cipher + ":" + password))
	return "ss://" + credentials + "@" + hostPort(node.Host, node.Port) + "#" + url.QueryEscape(displayName(node)), nil
}

func buildHysteriaURI(node domain.SubscriptionNode, config map[string]any) (string, error) {
	query := url.Values{}
	addTLSQuery(query, config)
	if value := stringConfig(config, "obfs", ""); value != "" {
		query.Set("obfs", value)
		if password := stringConfig(config, "obfs_password", stringConfig(config, "obfs-password", "")); password != "" {
			query.Set("obfs-password", password)
		}
	}
	if numberConfig(config, "version", 2) == 1 {
		if value := numberConfig(config, "up_mbps", 0); value > 0 {
			query.Set("upmbps", strconv.Itoa(value))
		}
		if value := numberConfig(config, "down_mbps", 0); value > 0 {
			query.Set("downmbps", strconv.Itoa(value))
		}
		return endpointURL("hysteria", url.User(node.UserUUID), node, query), nil
	}
	return endpointURL("hysteria2", url.User(node.UserUUID), node, query), nil
}

func buildTUICURI(node domain.SubscriptionNode, config map[string]any) string {
	query := url.Values{}
	addTLSQuery(query, config)
	if congestion := stringConfig(config, "congestion_control", ""); congestion != "" {
		query.Set("congestion_control", congestion)
	}
	return endpointURL("tuic", url.UserPassword(node.UserUUID, node.UserUUID), node, query)
}

func buildAnyTLSURI(node domain.SubscriptionNode, config map[string]any) string {
	query := url.Values{}
	addTLSQuery(query, config)
	if padding := stringConfig(config, "padding_scheme", ""); padding != "" {
		query.Set("padding", padding)
	}
	return endpointURL("anytls", url.User(node.UserUUID), node, query)
}

func endpointURL(scheme string, user *url.Userinfo, node domain.SubscriptionNode, query url.Values) string {
	return (&url.URL{Scheme: scheme, User: user, Host: hostPort(node.Host, node.Port), RawQuery: query.Encode(), Fragment: displayName(node)}).String()
}

func hostPort(host string, port int) string {
	if parsed := net.ParseIP(strings.Trim(strings.TrimSpace(host), "[]")); parsed != nil && strings.Contains(parsed.String(), ":") {
		return net.JoinHostPort(parsed.String(), strconv.Itoa(port))
	}
	return net.JoinHostPort(strings.Trim(strings.TrimSpace(host), "[]"), strconv.Itoa(port))
}

type transportDetails struct {
	kind        string
	path        string
	host        string
	serviceName string
	mode        string
}

func transportConfig(config map[string]any) transportDetails {
	kind := stringConfig(config, "transport", stringConfig(config, "network", "tcp"))
	settings := objectConfig(config, "network_settings")
	if len(settings) == 0 {
		settings = objectConfig(config, "networkSettings")
	}
	return transportDetails{
		kind:        kind,
		path:        stringConfig(settings, "path", ""),
		host:        stringConfig(settings, "host", ""),
		serviceName: stringConfig(settings, "service_name", stringConfig(settings, "serviceName", "")),
		mode:        stringConfig(settings, "mode", ""),
	}
}

func addTransportQuery(query url.Values, config map[string]any) {
	transport := transportConfig(config)
	query.Set("type", transport.kind)
	switch transport.kind {
	case "ws", "httpupgrade", "h2", "xhttp":
		if transport.path != "" {
			query.Set("path", transport.path)
		}
		if transport.host != "" {
			query.Set("host", transport.host)
		}
		if transport.kind == "xhttp" && transport.mode != "" {
			query.Set("mode", transport.mode)
		}
	case "grpc":
		if transport.serviceName != "" {
			query.Set("serviceName", transport.serviceName)
		}
	}
}

type tlsDetails struct {
	enabled     bool
	reality     bool
	serverName  string
	fingerprint string
	publicKey   string
	shortID     string
}

func tlsConfig(config map[string]any) tlsDetails {
	tls := objectConfig(config, "tls")
	if len(tls) == 0 {
		return tlsDetails{}
	}
	reality := realityEnabled(tls["enabled"])
	return tlsDetails{
		enabled:     tlsEnabled(tls["enabled"]),
		reality:     reality,
		serverName:  stringConfig(tls, "server_name", stringConfig(tls, "servername", "")),
		fingerprint: stringConfig(tls, "fingerprint", stringConfig(tls, "client-fingerprint", "")),
		publicKey:   stringConfig(tls, "public_key", ""),
		shortID:     stringConfig(tls, "short_id", ""),
	}
}

func addTLSQuery(query url.Values, config map[string]any) {
	tls := tlsConfig(config)
	if !tls.enabled {
		return
	}
	if tls.reality {
		query.Set("security", "reality")
		if tls.publicKey != "" {
			query.Set("pbk", tls.publicKey)
		}
		if tls.shortID != "" {
			query.Set("sid", tls.shortID)
		}
	} else {
		query.Set("security", "tls")
	}
	if tls.serverName != "" {
		query.Set("sni", tls.serverName)
	}
	if tls.fingerprint != "" {
		query.Set("fp", tls.fingerprint)
	}
}

func objectConfig(config map[string]any, key string) map[string]any {
	value, _ := config[key].(map[string]any)
	return value
}

func stringConfig(config map[string]any, key, fallback string) string {
	if value, ok := config[key].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func numberConfig(config map[string]any, key string, fallback int) int {
	switch value := config[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func displayName(node domain.SubscriptionNode) string {
	entry := strings.TrimSpace(node.EntryName)
	if entry != "" {
		return entry
	}
	return strings.TrimSpace(node.Name)
}
