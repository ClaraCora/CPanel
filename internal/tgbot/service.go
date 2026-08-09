package tgbot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"cpanel/internal/domain"
	"cpanel/internal/securebox"
	"cpanel/internal/store"
)

var ErrNotConfigured = errors.New("telegram bot is not configured")

type repository interface {
	RawSetting(context.Context, string, string) (json.RawMessage, error)
	SettingBool(context.Context, string, string, bool) bool
	SettingString(context.Context, string, string, string) string
	PrepareTelegramBotState(context.Context, string) (store.TelegramBotState, error)
	SetTelegramUpdateOffset(context.Context, string, int64) error
	MarkTelegramReportSent(context.Context, string, string) error
	Overview(context.Context, string) (domain.Overview, error)
	TrafficSummary(context.Context, string) (domain.TrafficSummary, error)
	ListMachines(context.Context) ([]domain.Machine, error)
}

type Service struct {
	repository repository
	box        *securebox.Box
	api        botAPI
	now        func() time.Time
}

type botConfig struct {
	Enabled            bool
	Token              string
	AdminID            int64
	DailyReportEnabled bool
	DailyReportTime    string
	PlatformName       string
	Location           *time.Location
}

func New(dataStore *store.Store, box *securebox.Box) *Service {
	return &Service{repository: dataStore, box: box, api: newTelegramClient(nil), now: time.Now}
}

func (s *Service) Run(ctx context.Context) {
	var fingerprint string
	var state store.TelegramBotState
	commandsConfiguredFor := ""
	for {
		if ctx.Err() != nil {
			return
		}
		config, err := s.loadConfig(ctx)
		if err != nil {
			slog.Error("load telegram bot settings failed", "error", err)
			if !waitContext(ctx, 15*time.Second) {
				return
			}
			continue
		}
		if !config.Enabled || config.Token == "" {
			fingerprint = ""
			commandsConfiguredFor = ""
			if !waitContext(ctx, 10*time.Second) {
				return
			}
			continue
		}

		currentFingerprint := tokenFingerprint(config.Token)
		if currentFingerprint != fingerprint {
			state, err = s.repository.PrepareTelegramBotState(ctx, currentFingerprint)
			if err != nil {
				slog.Error("prepare telegram bot state failed", "error", err)
				if !waitContext(ctx, 10*time.Second) {
					return
				}
				continue
			}
			fingerprint = currentFingerprint
			commandsConfiguredFor = ""
		}

		if commandsConfiguredFor != fingerprint {
			if err := s.api.SetCommands(ctx, config.Token, supportedCommands()); err != nil {
				slog.Warn("configure telegram bot commands failed", "error", err)
			} else {
				commandsConfiguredFor = fingerprint
			}
		}
		if err := s.maybeSendDailyReport(ctx, config, fingerprint, &state); err != nil {
			slog.Warn("send telegram daily report failed", "error", err)
		}

		updates, err := s.api.GetUpdates(ctx, config.Token, state.UpdateOffset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("poll telegram bot updates failed", "error", err)
			if !waitContext(ctx, 5*time.Second) {
				return
			}
			continue
		}
		for _, update := range updates {
			if err := s.processUpdate(ctx, config, update); err != nil {
				slog.Warn("process telegram bot command failed", "error", err)
			}
			nextOffset := update.UpdateID + 1
			if nextOffset <= state.UpdateOffset {
				continue
			}
			if err := s.repository.SetTelegramUpdateOffset(ctx, fingerprint, nextOffset); err != nil {
				slog.Error("persist telegram bot offset failed", "error", err)
				break
			}
			state.UpdateOffset = nextOffset
		}
	}
}

func (s *Service) SendTest(ctx context.Context) error {
	config, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	if config.Token == "" || config.AdminID == 0 {
		return ErrNotConfigured
	}
	message := formatTestMessage(config.PlatformName, s.now().In(config.Location), config.AdminID)
	return s.api.SendMessage(ctx, config.Token, config.AdminID, message)
}

func (s *Service) loadConfig(ctx context.Context) (botConfig, error) {
	config := botConfig{
		Enabled:            s.repository.SettingBool(ctx, "tgbot", "enabled", false),
		DailyReportEnabled: s.repository.SettingBool(ctx, "tgbot", "daily_report_enabled", true),
		DailyReportTime:    s.repository.SettingString(ctx, "tgbot", "daily_report_time", "09:00"),
		PlatformName:       s.repository.SettingString(ctx, "site", "platform_name", "CPanel"),
	}
	config.Location = loadLocation(s.repository.SettingString(ctx, "site", "timezone", "Asia/Shanghai"))
	adminID := strings.TrimSpace(s.repository.SettingString(ctx, "tgbot", "admin_telegram_id", ""))
	if adminID != "" {
		parsed, err := strconv.ParseInt(adminID, 10, 64)
		if err != nil || parsed <= 0 {
			return botConfig{}, fmt.Errorf("invalid telegram administrator ID")
		}
		config.AdminID = parsed
	}

	sealed, err := s.repository.RawSetting(ctx, "tgbot", "bot_token")
	if errors.Is(err, store.ErrNotFound) {
		return config, nil
	}
	if err != nil {
		return botConfig{}, err
	}
	plain, err := s.box.Open(sealed)
	if err != nil {
		return botConfig{}, fmt.Errorf("decrypt telegram bot token: %w", err)
	}
	if err := json.Unmarshal(plain, &config.Token); err != nil {
		return botConfig{}, fmt.Errorf("decode telegram bot token")
	}
	config.Token = strings.TrimSpace(config.Token)
	return config, nil
}

func (s *Service) processUpdate(ctx context.Context, config botConfig, update telegramUpdate) error {
	message := update.Message
	if message == nil || message.From == nil {
		return nil
	}
	command := parseCommand(message.Text)
	if command == "" {
		return nil
	}
	if command == "/id" {
		return s.api.SendMessage(ctx, config.Token, message.Chat.ID, formatTelegramID(message.From.ID))
	}
	if message.From.ID != config.AdminID || message.Chat.ID != config.AdminID {
		return s.api.SendMessage(ctx, config.Token, message.Chat.ID, formatNotice("访问受限", "此 Bot 仅限已绑定的面板管理员使用", "发送 /id 可查看你的 Telegram 数字 ID"))
	}

	var text string
	switch command {
	case "/status":
		overview, err := s.repository.Overview(ctx, "today")
		if err != nil {
			return s.sendQueryFailure(ctx, config, err)
		}
		text = formatStatus(config.PlatformName, overview)
	case "/traffic":
		summary, err := s.repository.TrafficSummary(ctx, "today")
		if err != nil {
			return s.sendQueryFailure(ctx, config, err)
		}
		text = formatTraffic("今日流量", summary)
	case "/machines":
		machines, err := s.repository.ListMachines(ctx)
		if err != nil {
			return s.sendQueryFailure(ctx, config, err)
		}
		text = formatMachines(machines, config.Location)
	case "/today":
		summary, overview, err := s.rankingData(ctx, "today")
		if err != nil {
			return s.sendQueryFailure(ctx, config, err)
		}
		text = formatRanking("今日流量排行", summary, overview.NodeTrafficRanking, overview.UserTrafficRanking)
	case "/ranking":
		summary, overview, err := s.rankingData(ctx, "yesterday")
		if err != nil {
			return s.sendQueryFailure(ctx, config, err)
		}
		text = formatRanking("昨日流量排行", summary, overview.NodeTrafficRanking, overview.UserTrafficRanking)
	case "/help", "/start":
		text = formatHelp()
	default:
		text = formatNotice("未知命令", "无法识别该命令") + "\n" + formatHelp()
	}
	return s.api.SendMessage(ctx, config.Token, config.AdminID, text)
}

func (s *Service) sendQueryFailure(ctx context.Context, config botConfig, queryErr error) error {
	if err := s.api.SendMessage(ctx, config.Token, config.AdminID, formatNotice("查询失败", "请稍后重试或检查面板日志")); err != nil {
		return err
	}
	return queryErr
}

func (s *Service) rankingData(ctx context.Context, period string) (domain.TrafficSummary, domain.Overview, error) {
	summary, err := s.repository.TrafficSummary(ctx, period)
	if err != nil {
		return domain.TrafficSummary{}, domain.Overview{}, err
	}
	overview, err := s.repository.Overview(ctx, period)
	return summary, overview, err
}

func (s *Service) maybeSendDailyReport(ctx context.Context, config botConfig, fingerprint string, state *store.TelegramBotState) error {
	if !config.DailyReportEnabled || config.AdminID == 0 {
		return nil
	}
	now := s.now().In(config.Location)
	scheduled, err := time.ParseInLocation("15:04", config.DailyReportTime, config.Location)
	if err != nil {
		return fmt.Errorf("invalid telegram daily report time")
	}
	scheduledToday := time.Date(now.Year(), now.Month(), now.Day(), scheduled.Hour(), scheduled.Minute(), 0, 0, config.Location)
	today := now.Format(time.DateOnly)
	if now.Before(scheduledToday) || (state.LastReportDay != nil && state.LastReportDay.Format(time.DateOnly) == today) {
		return nil
	}
	summary, overview, err := s.rankingData(ctx, "yesterday")
	if err != nil {
		return err
	}
	reportDay := now.AddDate(0, 0, -1).Format("2006-01-02")
	text := formatRanking(config.PlatformName+" 昨日报表（"+reportDay+"）", summary, overview.NodeTrafficRanking, overview.UserTrafficRanking)
	if err := s.api.SendMessage(ctx, config.Token, config.AdminID, text); err != nil {
		return err
	}
	marked := now
	state.LastReportDay = &marked
	if err := s.repository.MarkTelegramReportSent(ctx, fingerprint, today); err != nil {
		return fmt.Errorf("persist telegram report date: %w", err)
	}
	return nil
}

func supportedCommands() []telegramCommand {
	return []telegramCommand{
		{Command: "status", Description: "查看面板运行状态"},
		{Command: "traffic", Description: "查看今日流量"},
		{Command: "today", Description: "查看今日流量排行"},
		{Command: "machines", Description: "查看服务器状态"},
		{Command: "ranking", Description: "查看昨日流量排行"},
		{Command: "id", Description: "查看 Telegram 数字 ID"},
		{Command: "help", Description: "查看命令说明"},
	}
}

func parseCommand(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return ""
	}
	return strings.ToLower(strings.SplitN(fields[0], "@", 2)[0])
}

func tokenFingerprint(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:8])
}

func loadLocation(name string) *time.Location {
	if location, err := time.LoadLocation(strings.TrimSpace(name)); err == nil {
		return location
	}
	if location, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return location
	}
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
