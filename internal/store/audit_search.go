package store

import (
	"sort"
	"strings"
)

// Keep these labels aligned with web/src/auditActions.ts. Canonical actions
// remain unchanged in storage; aliases only extend the paginated search.
var auditActionLabels = map[string]string{
	"admin.login":                  "管理员登录",
	"admin.logout":                 "管理员退出",
	"admin.profile.update":         "修改管理员资料",
	"admin.password.update":        "修改管理员密码",
	"machine.create":               "添加服务器",
	"machine.update":               "编辑服务器",
	"machine.delete":               "删除服务器",
	"machine.agent_identity_reset": "重置 Agent 身份",
	"machine.agent_upgrade":        "升级 Agent",
	"machine_credential.create":    "创建服务器凭据",
	"agent_enrollment.create":      "生成 Agent 安装凭据",
	"node.create":                  "创建节点",
	"node.update":                  "编辑节点",
	"node.delete":                  "删除节点",
	"node.publish":                 "发布节点",
	"node.restore":                 "恢复节点版本",
	"access_group.create":          "创建权限组",
	"access_group.update":          "编辑权限组",
	"access_group.delete":          "删除权限组",
	"plan.create":                  "创建套餐",
	"plan.update":                  "编辑套餐",
	"plan.delete":                  "删除套餐",
	"user.create":                  "添加账号",
	"user.update":                  "编辑订阅账号",
	"user.delete":                  "删除订阅账号",
	"user.subscription.copy":       "复制订阅地址",
	"user.portal.delegate.create":  "创建门户只读代入",
	"user.portal.delegate.redeem":  "使用门户只读代入",
	"route_policy.create":          "创建路由策略",
	"route_policy.update":          "编辑路由策略",
	"route_policy.delete":          "删除路由策略",
	"outbound.create":              "创建出站",
	"outbound.update":              "编辑出站",
	"outbound.delete":              "删除出站",
	"settings.update":              "修改设置",
	"tgbot.test":                   "发送 TG Bot 测试消息",
}

func matchingAuditActions(query string) []string {
	needle := strings.ToLower(strings.TrimSpace(query))
	matches := make([]string, 0)
	if needle == "" {
		return matches
	}
	for action, label := range auditActionLabels {
		if strings.Contains(strings.ToLower(label), needle) {
			matches = append(matches, action)
		}
	}
	sort.Strings(matches)
	return matches
}
