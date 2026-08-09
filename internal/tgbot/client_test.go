package tgbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelegramClientRedactsTokenFromAPIError(t *testing.T) {
	token := "123456:secret-token-value-used-for-testing"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"ok":false,"description":"invalid token %s"}`, token)
	}))
	defer server.Close()

	client := newTelegramClient(server.Client())
	client.baseURL = server.URL
	err := client.SendMessage(context.Background(), token, 123, "test")
	if !errors.Is(err, ErrTelegramAPI) {
		t.Fatalf("error = %v, want ErrTelegramAPI", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaked Bot token: %v", err)
	}
}

func TestTelegramClientSendsHTMLMessages(t *testing.T) {
	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":{}}`)
	}))
	defer server.Close()

	client := newTelegramClient(server.Client())
	client.baseURL = server.URL
	if err := client.SendMessage(context.Background(), "token", 7, "<b>标题</b>\n<pre>表格</pre>"); err != nil {
		t.Fatal(err)
	}
	if request["parse_mode"] != "HTML" {
		t.Fatalf("parse_mode = %#v, want HTML", request["parse_mode"])
	}
}

func TestTelegramClientParsesUpdates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/getUpdates") {
			t.Fatalf("path = %q, want getUpdates", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":[{"update_id":42,"message":{"text":"/status","chat":{"id":7},"from":{"id":7}}}]}`)
	}))
	defer server.Close()

	client := newTelegramClient(server.Client())
	client.baseURL = server.URL
	updates, err := client.GetUpdates(context.Background(), "token", 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 42 || updates[0].Message == nil || updates[0].Message.From.ID != 7 {
		t.Fatalf("updates = %#v", updates)
	}
}
