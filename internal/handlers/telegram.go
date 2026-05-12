package handlers

import (
	"fmt"
	"net/http"
	"time"
)

func (srv *Server) handleTelegramTest(w http.ResponseWriter, r *http.Request) {
	if srv.Telegram == nil || srv.Cfg == nil || srv.Cfg.TelegramBotToken == "" || srv.Cfg.TelegramChatID == "" {
		writeError(w, http.StatusServiceUnavailable, "Telegram is not configured")
		return
	}

	now := time.Now().In(cstLocation())
	msg := fmt.Sprintf(
		"⚡ <b>StockPulse Telegram test alert</b>\nSent from Scanner UI at %s CT.\nDefault-50 lightning watcher path is reachable.",
		now.Format("2006-01-02 15:04:05"),
	)
	if !srv.Telegram.Send(msg) {
		writeError(w, http.StatusBadGateway, "Telegram send failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Telegram test alert sent",
	})
}

func cstLocation() *time.Location {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		return time.UTC
	}
	return loc
}
