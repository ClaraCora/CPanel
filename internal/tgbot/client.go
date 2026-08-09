package tgbot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrTelegramAPI = errors.New("telegram bot api request failed")

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	Text string        `json:"text"`
	Chat telegramChat  `json:"chat"`
	From *telegramUser `json:"from"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

type telegramUser struct {
	ID int64 `json:"id"`
}

type telegramCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type botAPI interface {
	GetUpdates(context.Context, string, int64) ([]telegramUpdate, error)
	SendMessage(context.Context, string, int64, string) error
	SetCommands(context.Context, string, []telegramCommand) error
}

type telegramClient struct {
	httpClient *http.Client
	baseURL    string
}

func newTelegramClient(httpClient *http.Client) *telegramClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 35 * time.Second}
	}
	return &telegramClient{httpClient: httpClient, baseURL: "https://api.telegram.org"}
}

func (c *telegramClient) GetUpdates(ctx context.Context, token string, offset int64) ([]telegramUpdate, error) {
	var updates []telegramUpdate
	err := c.call(ctx, token, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         25,
		"allowed_updates": []string{"message"},
	}, &updates)
	return updates, err
}

func (c *telegramClient) SendMessage(ctx context.Context, token string, chatID int64, text string) error {
	return c.call(ctx, token, "sendMessage", map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}, nil)
}

func (c *telegramClient) SetCommands(ctx context.Context, token string, commands []telegramCommand) error {
	return c.call(ctx, token, "setMyCommands", map[string]any{"commands": commands}, nil)
}

func (c *telegramClient) call(ctx context.Context, token, method string, input, output any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("%w: encode %s request", ErrTelegramAPI, method)
	}
	endpoint := strings.TrimRight(c.baseURL, "/") + "/bot" + token + "/" + method
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%w: create %s request", ErrTelegramAPI, method)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		// Network errors can contain the request URL, which includes the Bot token.
		return fmt.Errorf("%w: %s transport unavailable", ErrTelegramAPI, method)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: read %s response", ErrTelegramAPI, method)
	}
	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return fmt.Errorf("%w: invalid %s response", ErrTelegramAPI, method)
	}
	if response.StatusCode != http.StatusOK || !envelope.OK {
		description := sanitizeTelegramDescription(envelope.Description, token)
		if description == "" {
			description = "request rejected"
		}
		return fmt.Errorf("%w: %s", ErrTelegramAPI, description)
	}
	if output != nil && json.Unmarshal(envelope.Result, output) != nil {
		return fmt.Errorf("%w: invalid %s result", ErrTelegramAPI, method)
	}
	return nil
}

func sanitizeTelegramDescription(value, token string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, token, "[redacted]"))
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = value[:240]
	}
	return value
}
