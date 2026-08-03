package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateOutbound(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		protocol string
		settings string
		want     string
	}{
		{name: "vless", tag: "sg-vless", protocol: "vless", settings: `{"vnext":[{"address":"sg.example.com","port":443,"users":[{"id":"c6b6f019-0523-43ab-980b-30a251bac363"}]}]}`},
		{name: "wireguard", tag: "warp", protocol: "wireguard", settings: `{"secretKey":"private","address":["172.16.0.2/32"],"peers":[{"publicKey":"public","endpoint":"engage.cloudflareclient.com:2408"}]}`},
		{name: "invalid tag", tag: "SG Proxy", protocol: "socks", settings: `{"servers":[{"address":"127.0.0.1","port":1080}]}`, want: "出站标记"},
		{name: "missing password", tag: "ss", protocol: "shadowsocks", settings: `{"servers":[{"address":"127.0.0.1","port":8388}]}`, want: "认证密码"},
		{name: "invalid port", tag: "socks", protocol: "socks", settings: `{"servers":[{"address":"127.0.0.1","port":70000}]}`, want: "服务器端口"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateOutbound(test.tag, test.protocol, json.RawMessage(test.settings))
			if test.want == "" && err != nil {
				t.Fatal(err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}
