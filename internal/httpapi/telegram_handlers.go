package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"cpanel/internal/tgbot"
)

func (s *Server) handleTestTelegramBot(w http.ResponseWriter, r *http.Request) {
	if s.telegramBot == nil {
		writeError(w, r, http.StatusServiceUnavailable, "TG_BOT_UNAVAILABLE", "TG Bot 服务尚未启动", nil)
		return
	}
	if err := s.telegramBot.SendTest(r.Context()); err != nil {
		switch {
		case errors.Is(err, tgbot.ErrNotConfigured):
			writeError(w, r, http.StatusConflict, "TG_BOT_NOT_CONFIGURED", "请先保存 Bot 密钥和管理员 Telegram ID", nil)
		case errors.Is(err, tgbot.ErrTelegramAPI):
			writeError(w, r, http.StatusBadGateway, "TG_BOT_SEND_FAILED", "测试消息发送失败，请检查 Bot 密钥、管理员 ID 和 Telegram 网络连接", nil)
		default:
			slog.Error("send telegram bot test message failed", "request_id", requestID(r), "error", err)
			writeError(w, r, http.StatusServiceUnavailable, "TG_BOT_SEND_FAILED", "测试消息发送失败，请检查面板日志", nil)
		}
		return
	}
	admin := currentAdmin(r)
	_ = s.store.WriteAudit(r.Context(), admin.ID, "tgbot.test", "settings", "tgbot", nil, clientIP(r), requestID(r))
	writeData(w, r, http.StatusOK, map[string]bool{"sent": true})
}
