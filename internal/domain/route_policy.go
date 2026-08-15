package domain

import (
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

var geoIPCategoryPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func ValidateRoutePolicyRules(rules []RoutePolicyRule) error {
	for index := range rules {
		rule := &rules[index]
		rule.Name = strings.TrimSpace(rule.Name)
		normalizeRouteMatch(&rule.Match)
		rule.Action.Type = strings.ToLower(strings.TrimSpace(rule.Action.Type))
		rule.Action.Target = strings.TrimSpace(rule.Action.Target)

		if rule.Disabled {
			continue
		}
		if routeMatchEmpty(rule.Match) {
			return fmt.Errorf("第 %d 条规则至少需要一个匹配条件", index+1)
		}
		for _, value := range append(append([]string{}, rule.Match.IPCIDRs...), rule.Match.SourceCIDRs...) {
			if _, err := netip.ParsePrefix(value); err != nil {
				return fmt.Errorf("第 %d 条规则包含无效的 IP/CIDR：%s", index+1, value)
			}
		}
		for _, value := range append(append([]string{}, rule.Match.Ports...), rule.Match.SourcePorts...) {
			if !validRoutePort(value) {
				return fmt.Errorf("第 %d 条规则包含无效端口：%s", index+1, value)
			}
		}
		for _, network := range rule.Match.Networks {
			if network != "tcp" && network != "udp" {
				return fmt.Errorf("第 %d 条规则的网络类型必须是 TCP 或 UDP", index+1)
			}
		}
		for _, expression := range rule.Match.DomainRegexes {
			if _, err := regexp.Compile(expression); err != nil {
				return fmt.Errorf("第 %d 条规则包含无效域名正则：%s", index+1, expression)
			}
		}
		for _, category := range rule.Match.GeoIPs {
			if !geoIPCategoryPattern.MatchString(category) {
				return fmt.Errorf("第 %d 条规则包含无效 GeoIP 分类：%s", index+1, category)
			}
		}
		switch rule.Action.Type {
		case "direct", "block":
			if rule.Action.Target != "" {
				return fmt.Errorf("第 %d 条规则只有指定出站时才能填写出站目标", index+1)
			}
		case "route":
			if rule.Action.Target == "" {
				return fmt.Errorf("第 %d 条规则请选择出站目标", index+1)
			}
		default:
			return fmt.Errorf("第 %d 条规则的执行动作无效", index+1)
		}
	}
	return nil
}

func normalizeRouteMatch(match *RoutePolicyMatch) {
	match.Domains = normalizeRouteValues(match.Domains, true)
	match.DomainSuffixes = normalizeRouteValues(match.DomainSuffixes, true)
	match.DomainRegexes = normalizeRouteRegexes(match.DomainRegexes)
	match.GeoIPs = normalizeGeoIPValues(match.GeoIPs)
	match.IPCIDRs = normalizeRouteValues(match.IPCIDRs, false)
	match.Ports = normalizeRouteValues(match.Ports, true)
	match.Networks = normalizeRouteValues(match.Networks, true)
	match.SourceCIDRs = normalizeRouteValues(match.SourceCIDRs, false)
	match.SourcePorts = normalizeRouteValues(match.SourcePorts, true)
}

func normalizeGeoIPValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimSpace(strings.TrimPrefix(value, "geoip:"))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeRouteRegexes(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) >= len("regexp:") && strings.EqualFold(value[:len("regexp:")], "regexp:") {
			value = strings.TrimSpace(value[len("regexp:"):])
		}
		// Rules copied from JSON often contain escaped backslashes. Domain names
		// cannot contain a backslash, so normalize those expressions for Xray.
		value = strings.ReplaceAll(value, `\\`, `\`)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeRouteValues(values []string, lower bool) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func routeMatchEmpty(match RoutePolicyMatch) bool {
	return len(match.Domains)+len(match.DomainSuffixes)+len(match.DomainRegexes)+len(match.GeoIPs)+len(match.IPCIDRs)+len(match.Ports)+
		len(match.Networks)+len(match.SourceCIDRs)+len(match.SourcePorts) == 0
}

func validRoutePort(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil || start < 1 || start > 65535 {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	end, err := strconv.Atoi(parts[1])
	return err == nil && end >= start && end <= 65535
}
