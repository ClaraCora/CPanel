package tgbot

import (
	"fmt"
	"strings"
	"time"

	"cpanel/internal/domain"
)

func formatStatus(platform string, overview domain.Overview) string {
	return fmt.Sprintf("%s 运行状态\n\n服务器：%d 在线 / %d 总数\n节点：%d 已发布 / %d 总数\n订阅账号：%d 管理员 / %d 用户 / %d 朋友\n今日流量：%s",
		cleanLabel(platform), overview.MachinesOnline, overview.MachinesTotal,
		overview.NodesPublished, overview.NodesTotal, overview.AdminsActive,
		overview.UsersActive, overview.FriendsActive, formatBytes(overview.TrafficToday))
}

func formatTraffic(title string, summary domain.TrafficSummary) string {
	return fmt.Sprintf("%s\n\n上传：%s\n下载：%s\n合计：%s", title,
		formatBytes(summary.UploadBytes), formatBytes(summary.DownloadBytes), formatBytes(summary.TotalBytes))
}

func formatMachines(items []domain.Machine, location *time.Location) string {
	if len(items) == 0 {
		return "服务器状态\n\n当前没有服务器。"
	}
	lines := []string{"服务器状态", ""}
	limit := len(items)
	if limit > 20 {
		limit = 20
	}
	for _, item := range items[:limit] {
		lastSeen := "从未上报"
		if item.LastHeartbeat != nil {
			lastSeen = item.LastHeartbeat.In(location).Format("01-02 15:04:05")
		}
		lines = append(lines, fmt.Sprintf("[%s] %s  最后心跳 %s", machineStatus(item.Status), cleanLabel(item.Name), lastSeen))
	}
	if len(items) > limit {
		lines = append(lines, fmt.Sprintf("另有 %d 台服务器未显示", len(items)-limit))
	}
	return strings.Join(lines, "\n")
}

func formatRanking(title string, summary domain.TrafficSummary, nodes, users []domain.TrafficRank) string {
	lines := []string{title, "", "上传：" + formatBytes(summary.UploadBytes), "下载：" + formatBytes(summary.DownloadBytes), "合计：" + formatBytes(summary.TotalBytes), "", "节点 Top 5"}
	lines = append(lines, formatRankingItems(nodes)...)
	lines = append(lines, "", "用户 Top 5")
	lines = append(lines, formatRankingItems(users)...)
	return strings.Join(lines, "\n")
}

func formatRankingItems(items []domain.TrafficRank) []string {
	if len(items) == 0 {
		return []string{"暂无流量数据"}
	}
	limit := len(items)
	if limit > 5 {
		limit = 5
	}
	lines := make([]string, 0, limit)
	for index, item := range items[:limit] {
		lines = append(lines, fmt.Sprintf("%d. %s  %s", index+1, cleanLabel(item.Name), formatBytes(item.TotalBytes)))
	}
	return lines
}

func formatHelp() string {
	return strings.Join([]string{
		"CPanel TG Bot 命令",
		"",
		"/status - 面板运行状态",
		"/traffic - 今日上传与下载流量",
		"/machines - 服务器在线状态",
		"/ranking - 昨日节点和用户流量排行",
		"/id - 查看当前 Telegram 数字 ID",
		"/help - 查看命令说明",
	}, "\n")
}

func formatBytes(value int64) string {
	if value < 0 {
		value = 0
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", value, units[unit])
	}
	return fmt.Sprintf("%.2f %s", size, units[unit])
}

func cleanLabel(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if value == "" {
		return "未命名"
	}
	if len([]rune(value)) > 40 {
		return string([]rune(value)[:40])
	}
	return value
}

func machineStatus(value string) string {
	if value == "online" {
		return "在线"
	}
	return "离线"
}
