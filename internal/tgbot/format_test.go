package tgbot

import (
	"html"
	"math"
	"strings"
	"testing"

	"cpanel/internal/domain"
)

func TestRenderTableAlignsChineseAndASCIIColumns(t *testing.T) {
	table := renderTable([]tableColumn{
		{Title: "名称", Width: 12},
		{Title: "上传", Width: 10, Right: true},
		{Title: "下载", Width: 10, Right: true},
	}, [][]string{
		{"香港节点", "1.00 GB", "12.00 GB"},
		{"long-ascii-node-name", "512 B", "2.00 MB"},
	})
	lines := strings.Split(table, "\n")
	wantWidth := displayWidth(lines[0])
	for index, line := range lines {
		if width := displayWidth(line); width != wantWidth {
			t.Fatalf("line %d width = %d, want %d: %q", index, width, wantWidth, line)
		}
	}
	if !strings.Contains(table, "long-ascii-…") {
		t.Fatalf("long name was not truncated predictably:\n%s", table)
	}
}

func TestFormatRankingEscapesNamesAndUsesPreformattedTables(t *testing.T) {
	message := formatRanking("今日 <排行>", domain.TrafficSummary{},
		[]domain.TrafficRank{{Name: "节点<&", TotalBytes: 1024}}, nil)
	if strings.Contains(message, "节点<&") || !strings.Contains(message, html.EscapeString("节点<&")) {
		t.Fatalf("message did not escape dynamic name: %s", message)
	}
	if count := strings.Count(message, "<pre>"); count != 3 {
		t.Fatalf("preformatted table count = %d, want 3", count)
	}
	for _, label := range []string{"上传", "下载", "合计", "节点 Top 5", "用户 Top 5"} {
		if !strings.Contains(message, label) {
			t.Fatalf("message missing %q: %s", label, message)
		}
	}
}

func TestFormatUserRankingOmitsNodeTable(t *testing.T) {
	message := formatUserRanking("本月用户使用量", domain.TrafficSummary{}, []domain.TrafficRank{{Name: "用户一"}})
	if count := strings.Count(message, "<pre>"); count != 2 {
		t.Fatalf("preformatted table count = %d, want 2", count)
	}
	for _, label := range []string{"本月用户使用量", "用户 Top 10", "用户一"} {
		if !strings.Contains(message, label) {
			t.Fatalf("message missing %q: %s", label, message)
		}
	}
	if strings.Contains(message, "节点 Top") {
		t.Fatalf("message unexpectedly contains node table: %s", message)
	}
}

func TestRankingTableFitsTelegramMobileWidth(t *testing.T) {
	table := renderRankingTable([]domain.TrafficRank{{
		Name:          "很长的中英文 mixed user name",
		UploadBytes:   1023 * 1024 * 1024,
		DownloadBytes: 12 * 1024 * 1024 * 1024,
		TotalBytes:    math.MaxInt64,
	}}, 5)
	lines := strings.Split(table, "\n")
	for index, line := range lines {
		if width := displayWidth(line); width != 36 {
			t.Fatalf("line %d width = %d, want 36: %q", index, width, line)
		}
	}
	for _, value := range []string{"1023M", "12.0G", "8.00E"} {
		if !strings.Contains(table, value) {
			t.Fatalf("compact ranking table missing %q:\n%s", value, table)
		}
	}
}

func TestFormatBytesCompact(t *testing.T) {
	tests := map[int64]string{
		-1:               "0B",
		0:                "0B",
		1024:             "1.00K",
		10 * 1024 * 1024: "10.0M",
	}
	for value, want := range tests {
		if got := formatBytesCompact(value); got != want {
			t.Errorf("formatBytesCompact(%d) = %q, want %q", value, got, want)
		}
	}
}
