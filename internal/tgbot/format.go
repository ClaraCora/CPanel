package tgbot

import (
	"fmt"
	"html"
	"strings"
	"time"
	"unicode"

	"cpanel/internal/domain"
)

type tableColumn struct {
	Title string
	Width int
	Right bool
}

func formatStatus(platform string, overview domain.Overview) string {
	table := renderTable([]tableColumn{
		{Title: "项目", Width: 10},
		{Title: "当前", Width: 14, Right: true},
		{Title: "总数", Width: 10, Right: true},
	}, [][]string{
		{"服务器", fmt.Sprintf("在线 %d", overview.MachinesOnline), fmt.Sprint(overview.MachinesTotal)},
		{"节点", fmt.Sprintf("已发布 %d", overview.NodesPublished), fmt.Sprint(overview.NodesTotal)},
		{"管理员", fmt.Sprint(overview.AdminsActive), "-"},
		{"用户", fmt.Sprint(overview.UsersActive), "-"},
		{"朋友", fmt.Sprint(overview.FriendsActive), "-"},
		{"今日流量", formatBytes(overview.TrafficToday), "-"},
	})
	return wrapTable(cleanLabel(platform)+" 运行状态", table)
}

func formatTraffic(title string, summary domain.TrafficSummary) string {
	table := renderTable([]tableColumn{
		{Title: "方向", Width: 12},
		{Title: "流量", Width: 16, Right: true},
	}, [][]string{
		{"上传", formatBytes(summary.UploadBytes)},
		{"下载", formatBytes(summary.DownloadBytes)},
		{"合计", formatBytes(summary.TotalBytes)},
	})
	return wrapTable(title, table)
}

func formatMachines(items []domain.Machine, location *time.Location) string {
	columns := []tableColumn{
		{Title: "状态", Width: 6},
		{Title: "服务器", Width: 18},
		{Title: "最后心跳", Width: 14},
	}
	rows := make([][]string, 0, 20)
	limit := len(items)
	if limit > 20 {
		limit = 20
	}
	for _, item := range items[:limit] {
		lastSeen := "从未上报"
		if item.LastHeartbeat != nil {
			lastSeen = item.LastHeartbeat.In(location).Format("01-02 15:04:05")
		}
		rows = append(rows, []string{machineStatus(item.Status), cleanLabel(item.Name), lastSeen})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "暂无服务器", "-"})
	}
	message := wrapTable("服务器状态", renderTable(columns, rows))
	if len(items) > limit {
		message += "\n" + wrapTable("更多", renderTable([]tableColumn{{Title: "说明", Width: 28}}, [][]string{{fmt.Sprintf("另有 %d 台服务器未显示", len(items)-limit)}}))
	}
	return message
}

func formatRanking(title string, summary domain.TrafficSummary, nodes, users []domain.TrafficRank) string {
	return wrapTable(title, formatTrafficSummaryTable(summary)) + "\n" +
		wrapTable("节点 Top 5", renderRankingTable(nodes, 5)) + "\n" +
		wrapTable("用户 Top 5", renderRankingTable(users, 5))
}

func formatUserRanking(title string, summary domain.TrafficSummary, users []domain.TrafficRank) string {
	return wrapTable(title, formatTrafficSummaryTable(summary)) + "\n" +
		wrapTable("用户 Top 10", renderRankingTable(users, 10))
}

func formatTrafficSummaryTable(summary domain.TrafficSummary) string {
	return renderTable([]tableColumn{
		{Title: "方向", Width: 10},
		{Title: "流量", Width: 16, Right: true},
	}, [][]string{
		{"上传", formatBytes(summary.UploadBytes)},
		{"下载", formatBytes(summary.DownloadBytes)},
		{"合计", formatBytes(summary.TotalBytes)},
	})
}

func renderRankingTable(items []domain.TrafficRank, maximum int) string {
	columns := []tableColumn{
		{Title: "#", Width: 3, Right: true},
		{Title: "名称", Width: 18},
		{Title: "上传", Width: 10, Right: true},
		{Title: "下载", Width: 10, Right: true},
		{Title: "合计", Width: 10, Right: true},
	}
	limit := len(items)
	if limit > maximum {
		limit = maximum
	}
	rows := make([][]string, 0, limit)
	for index, item := range items[:limit] {
		rows = append(rows, []string{
			fmt.Sprint(index + 1), cleanLabel(item.Name), formatBytes(item.UploadBytes),
			formatBytes(item.DownloadBytes), formatBytes(item.TotalBytes),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "暂无流量数据", "-", "-", "-"})
	}
	return renderTable(columns, rows)
}

func formatHelp() string {
	return wrapTable("CPanel TG Bot 命令", renderTable([]tableColumn{
		{Title: "命令", Width: 12},
		{Title: "功能", Width: 28},
	}, [][]string{
		{"/status", "面板运行状态"},
		{"/traffic", "今日上传与下载流量"},
		{"/today", "今日节点和用户流量排行"},
		{"/ranking", "昨日节点和用户流量排行"},
		{"/month", "本月用户使用量排行"},
		{"/machines", "服务器在线状态"},
		{"/id", "查看 Telegram 数字 ID"},
		{"/help", "查看命令说明"},
	}))
}

func formatTelegramID(id int64) string {
	return wrapTable("Telegram ID", renderTable([]tableColumn{
		{Title: "项目", Width: 12},
		{Title: "数值", Width: 20, Right: true},
	}, [][]string{{"数字 ID", fmt.Sprint(id)}}))
}

func formatTestMessage(platform string, now time.Time, adminID int64) string {
	return wrapTable(cleanLabel(platform)+" TG Bot 测试成功", renderTable([]tableColumn{
		{Title: "项目", Width: 12},
		{Title: "数值", Width: 22, Right: true},
	}, [][]string{
		{"连接时间", now.Format("2006-01-02 15:04:05")},
		{"管理员 ID", fmt.Sprint(adminID)},
	}))
}

func formatNotice(title string, lines ...string) string {
	rows := make([][]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, []string{line})
	}
	return wrapTable(title, renderTable([]tableColumn{{Title: "说明", Width: 38}}, rows))
}

func wrapTable(title, table string) string {
	return "<b>" + html.EscapeString(cleanLabel(title)) + "</b>\n<pre>" + html.EscapeString(table) + "</pre>"
}

func renderTable(columns []tableColumn, rows [][]string) string {
	header := make([]string, len(columns))
	for index, column := range columns {
		header[index] = column.Title
	}
	lines := []string{renderTableRow(columns, header)}
	totalWidth := 0
	for _, column := range columns {
		totalWidth += column.Width
	}
	if len(columns) > 1 {
		totalWidth += len(columns) - 1
	}
	lines = append(lines, strings.Repeat("-", totalWidth))
	for _, row := range rows {
		lines = append(lines, renderTableRow(columns, row))
	}
	return strings.Join(lines, "\n")
}

func renderTableRow(columns []tableColumn, values []string) string {
	cells := make([]string, len(columns))
	for index, column := range columns {
		value := ""
		if index < len(values) {
			value = strings.TrimSpace(strings.Join(strings.Fields(values[index]), " "))
		}
		value = truncateDisplay(value, column.Width)
		padding := strings.Repeat(" ", column.Width-displayWidth(value))
		if column.Right {
			cells[index] = padding + value
		} else {
			cells[index] = value + padding
		}
	}
	return strings.Join(cells, " ")
}

func truncateDisplay(value string, maximum int) string {
	if displayWidth(value) <= maximum {
		return value
	}
	if maximum <= 1 {
		return "…"
	}
	var result strings.Builder
	width := 0
	for _, char := range value {
		charWidth := runeDisplayWidth(char)
		if width+charWidth > maximum-1 {
			break
		}
		result.WriteRune(char)
		width += charWidth
	}
	result.WriteRune('…')
	return result.String()
}

func displayWidth(value string) int {
	width := 0
	for _, char := range value {
		width += runeDisplayWidth(char)
	}
	return width
}

func runeDisplayWidth(char rune) int {
	if unicode.Is(unicode.Mn, char) || unicode.Is(unicode.Me, char) || char == '\u200d' {
		return 0
	}
	if char >= 0x1100 && (char <= 0x115f || char == 0x2329 || char == 0x232a ||
		(char >= 0x2e80 && char <= 0xa4cf && char != 0x303f) ||
		(char >= 0xac00 && char <= 0xd7a3) || (char >= 0xf900 && char <= 0xfaff) ||
		(char >= 0xfe10 && char <= 0xfe19) || (char >= 0xfe30 && char <= 0xfe6f) ||
		(char >= 0xff00 && char <= 0xff60) || (char >= 0xffe0 && char <= 0xffe6) ||
		(char >= 0x1f300 && char <= 0x1faff) || (char >= 0x20000 && char <= 0x3fffd)) {
		return 2
	}
	return 1
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
