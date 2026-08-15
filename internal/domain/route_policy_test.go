package domain

import (
	"strings"
	"testing"
)

func TestValidateRoutePolicyRulesNormalizesValues(t *testing.T) {
	rules := []RoutePolicyRule{{
		Name: "  media  ",
		Match: RoutePolicyMatch{
			DomainSuffixes: []string{" Example.COM ", "example.com"},
			DomainRegexes:  []string{" regexp:^(.+\\.)?Example\\.COM$ ", "^(.+\\.)?Example\\.COM$", `^.*(\\.torrent|announce\\.php\\?passkey=).*$`},
			GeoIPs:         []string{" geoip:Google ", "google", " CN "},
			Ports:          []string{"443", "8000-9000"},
			Networks:       []string{" TCP ", "udp"},
		},
		Action: RoutePolicyAction{Type: " DIRECT "},
	}}
	if err := ValidateRoutePolicyRules(rules); err != nil {
		t.Fatal(err)
	}
	if rules[0].Name != "media" || rules[0].Action.Type != "direct" {
		t.Fatalf("rule was not normalized: %+v", rules[0])
	}
	if got := rules[0].Match.DomainSuffixes; len(got) != 1 || got[0] != "example.com" {
		t.Fatalf("domain suffixes were not normalized: %#v", got)
	}
	if got := rules[0].Match.DomainRegexes; len(got) != 2 || got[0] != `^(.+\.)?Example\.COM$` || got[1] != `^.*(\.torrent|announce\.php\?passkey=).*$` {
		t.Fatalf("domain regexes were not normalized or preserved case: %#v", got)
	}
	if got := rules[0].Match.GeoIPs; len(got) != 2 || got[0] != "google" || got[1] != "cn" {
		t.Fatalf("GeoIP categories were not normalized: %#v", got)
	}
}

func TestValidateRoutePolicyRulesRejectsInvalidRule(t *testing.T) {
	tests := []struct {
		name string
		rule RoutePolicyRule
		want string
	}{
		{name: "empty match", rule: RoutePolicyRule{Action: RoutePolicyAction{Type: "direct"}}, want: "至少需要一个匹配条件"},
		{name: "invalid cidr", rule: RoutePolicyRule{Match: RoutePolicyMatch{IPCIDRs: []string{"10.0.0.1"}}, Action: RoutePolicyAction{Type: "direct"}}, want: "无效的 IP/CIDR"},
		{name: "invalid port range", rule: RoutePolicyRule{Match: RoutePolicyMatch{Ports: []string{"9000-8000"}}, Action: RoutePolicyAction{Type: "direct"}}, want: "无效端口"},
		{name: "invalid domain regex", rule: RoutePolicyRule{Match: RoutePolicyMatch{DomainRegexes: []string{"regexp:("}}, Action: RoutePolicyAction{Type: "block"}}, want: "无效域名正则"},
		{name: "invalid GeoIP category", rule: RoutePolicyRule{Match: RoutePolicyMatch{GeoIPs: []string{"google/cn"}}, Action: RoutePolicyAction{Type: "block"}}, want: "无效 GeoIP 分类"},
		{name: "missing outbound", rule: RoutePolicyRule{Match: RoutePolicyMatch{Domains: []string{"example.com"}}, Action: RoutePolicyAction{Type: "route"}}, want: "请选择出站目标"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateRoutePolicyRules([]RoutePolicyRule{test.rule})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestValidateRoutePolicyRulesAllowsIncompleteDisabledRule(t *testing.T) {
	rules := []RoutePolicyRule{{Disabled: true}}
	if err := ValidateRoutePolicyRules(rules); err != nil {
		t.Fatalf("disabled rules should not block publishing: %v", err)
	}
}
