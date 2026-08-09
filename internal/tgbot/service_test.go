package tgbot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"cpanel/internal/domain"
	"cpanel/internal/store"
)

type fakeRepository struct {
	overview domain.Overview
	summary  domain.TrafficSummary
	machines []domain.Machine
	marked   string
}

func (f *fakeRepository) RawSetting(context.Context, string, string) (json.RawMessage, error) {
	return nil, store.ErrNotFound
}
func (f *fakeRepository) SettingBool(context.Context, string, string, bool) bool { return false }
func (f *fakeRepository) SettingString(_ context.Context, _, _, fallback string) string {
	return fallback
}
func (f *fakeRepository) PrepareTelegramBotState(context.Context, string) (store.TelegramBotState, error) {
	return store.TelegramBotState{}, nil
}
func (f *fakeRepository) SetTelegramUpdateOffset(context.Context, string, int64) error { return nil }
func (f *fakeRepository) MarkTelegramReportSent(_ context.Context, _, day string) error {
	f.marked = day
	return nil
}
func (f *fakeRepository) Overview(context.Context, string) (domain.Overview, error) {
	return f.overview, nil
}
func (f *fakeRepository) TrafficSummary(context.Context, string) (domain.TrafficSummary, error) {
	return f.summary, nil
}
func (f *fakeRepository) ListMachines(context.Context) ([]domain.Machine, error) {
	return f.machines, nil
}

type sentMessage struct {
	chatID int64
	text   string
}

type fakeBotAPI struct {
	messages []sentMessage
}

func (f *fakeBotAPI) GetUpdates(context.Context, string, int64) ([]telegramUpdate, error) {
	return nil, nil
}
func (f *fakeBotAPI) SendMessage(_ context.Context, _ string, chatID int64, text string) error {
	f.messages = append(f.messages, sentMessage{chatID: chatID, text: text})
	return nil
}
func (f *fakeBotAPI) SetCommands(context.Context, string, []telegramCommand) error { return nil }

func TestProcessUpdateEnforcesAdministratorAndAllowsID(t *testing.T) {
	repository := &fakeRepository{summary: domain.TrafficSummary{UploadBytes: 1024, DownloadBytes: 2048, TotalBytes: 3072}}
	api := &fakeBotAPI{}
	service := &Service{repository: repository, api: api, now: time.Now}
	config := botConfig{Token: "token", AdminID: 100, PlatformName: "CPanel", Location: time.UTC}

	admin := telegramUser{ID: 100}
	if err := service.processUpdate(context.Background(), config, telegramUpdate{Message: &telegramMessage{Text: "/traffic", Chat: telegramChat{ID: 100}, From: &admin}}); err != nil {
		t.Fatal(err)
	}
	if len(api.messages) != 1 || api.messages[0].chatID != 100 || !strings.Contains(api.messages[0].text, "3.00 KB") {
		t.Fatalf("administrator response = %#v", api.messages)
	}

	other := telegramUser{ID: 200}
	if err := service.processUpdate(context.Background(), config, telegramUpdate{Message: &telegramMessage{Text: "/status", Chat: telegramChat{ID: 200}, From: &other}}); err != nil {
		t.Fatal(err)
	}
	if len(api.messages) != 2 || !strings.Contains(api.messages[1].text, "仅限已绑定") {
		t.Fatalf("unauthorized response = %#v", api.messages)
	}

	if err := service.processUpdate(context.Background(), config, telegramUpdate{Message: &telegramMessage{Text: "/id", Chat: telegramChat{ID: 200}, From: &other}}); err != nil {
		t.Fatal(err)
	}
	if len(api.messages) != 3 || !strings.Contains(api.messages[2].text, "200") {
		t.Fatalf("ID response = %#v", api.messages)
	}
}

func TestDailyReportIsSentOncePerDay(t *testing.T) {
	repository := &fakeRepository{
		summary: domain.TrafficSummary{UploadBytes: 1024, DownloadBytes: 2048, TotalBytes: 3072},
		overview: domain.Overview{
			NodeTrafficRanking: []domain.TrafficRank{{ID: "n1", Name: "节点一", TotalBytes: 2048}},
			UserTrafficRanking: []domain.TrafficRank{{ID: "u1", Name: "用户一", TotalBytes: 1024}},
		},
	}
	api := &fakeBotAPI{}
	now := time.Date(2026, 8, 9, 9, 5, 0, 0, time.FixedZone("CST", 8*60*60))
	service := &Service{repository: repository, api: api, now: func() time.Time { return now }}
	config := botConfig{Token: "token", AdminID: 100, PlatformName: "CPanel", Location: now.Location(), DailyReportEnabled: true, DailyReportTime: "09:00"}
	state := store.TelegramBotState{}

	if err := service.maybeSendDailyReport(context.Background(), config, "fingerprint", &state); err != nil {
		t.Fatal(err)
	}
	if len(api.messages) != 1 || repository.marked != "2026-08-09" || !strings.Contains(api.messages[0].text, "2026-08-08") {
		t.Fatalf("daily report = %#v, marked = %q", api.messages, repository.marked)
	}
	if err := service.maybeSendDailyReport(context.Background(), config, "fingerprint", &state); err != nil {
		t.Fatal(err)
	}
	if len(api.messages) != 1 {
		t.Fatalf("daily report sent %d times, want 1", len(api.messages))
	}
}

func TestParseCommandRemovesBotUsername(t *testing.T) {
	if command := parseCommand(" /RANKING@cpanel_bot extra "); command != "/ranking" {
		t.Fatalf("command = %q, want /ranking", command)
	}
}

func TestDailyReportWaitsForAdministratorBinding(t *testing.T) {
	repository := &fakeRepository{}
	api := &fakeBotAPI{}
	now := time.Date(2026, 8, 9, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	service := &Service{repository: repository, api: api, now: func() time.Time { return now }}
	config := botConfig{Token: "token", Location: now.Location(), DailyReportEnabled: true, DailyReportTime: "09:00"}

	if err := service.maybeSendDailyReport(context.Background(), config, "fingerprint", &store.TelegramBotState{}); err != nil {
		t.Fatal(err)
	}
	if len(api.messages) != 0 || repository.marked != "" {
		t.Fatalf("report sent before administrator binding: %#v", api.messages)
	}
}
