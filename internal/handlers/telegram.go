package handlers

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/aashorefp-droid/stockpulsego/internal/models"
	scanpkg "github.com/aashorefp-droid/stockpulsego/internal/scanner"
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

func (srv *Server) handleTelegramLightningScan(w http.ResponseWriter, r *http.Request) {
	if srv.Telegram == nil || srv.Cfg == nil || srv.Cfg.TelegramBotToken == "" || srv.Cfg.TelegramChatID == "" {
		writeError(w, http.StatusServiceUnavailable, "Telegram is not configured")
		return
	}
	if srv.Scanner == nil {
		writeError(w, http.StatusServiceUnavailable, "Scanner is not configured")
		return
	}

	tickers := scanpkg.Watchlists["default"]
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()

	out := make(chan models.ScanResult, len(tickers))
	go srv.Scanner.Stream(ctx, tickers, nil, out)

	var hits []models.ScanResult
	scanned := 0
	for res := range out {
		scanned++
		if res.Error == "" && res.VolSurge {
			hits = append(hits, res)
		}
	}

	if len(hits) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"count":   0,
			"scanned": scanned,
			"message": fmt.Sprintf("Scanned %d default tickers; no lightning found", scanned),
		})
		return
	}

	msg := formatLightningScanSummary(hits, scanned)
	if !srv.Telegram.Send(msg) {
		writeError(w, http.StatusBadGateway, "Telegram send failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"count":   len(hits),
		"scanned": scanned,
		"message": fmt.Sprintf("Telegram sent: %d lightning ticker(s)", len(hits)),
	})
}

func formatLightningScanSummary(hits []models.ScanResult, scanned int) string {
	now := time.Now().In(cstLocation()).Format("2006-01-02 15:04 CT")
	lines := []string{
		fmt.Sprintf("⚡ <b>Default 50 lightning scan</b> · %d found / %d scanned", len(hits), scanned),
		html.EscapeString(now),
	}
	for i, r := range hits {
		if i >= 12 {
			lines = append(lines, fmt.Sprintf("+%d more", len(hits)-i))
			break
		}
		line := fmt.Sprintf("<b>%s</b> $%.2f · %s · %s",
			html.EscapeString(strings.ToUpper(r.Ticker)),
			r.Price,
			html.EscapeString(r.Verdict),
			html.EscapeString(r.Direction),
		)
		if r.OptStrategy != "" {
			line += "\n   Options: " + html.EscapeString(r.OptStrategy)
			if r.OptSummary != "" {
				line += "\n   " + html.EscapeString(r.OptSummary)
			}
			if r.OptAlt != "" {
				line += "\n   " + html.EscapeString(r.OptAlt)
			}
		} else {
			line += "\n   Options: unavailable"
		}
		if len(r.OptLiquid) > 0 {
			top := r.OptLiquid[0]
			line += fmt.Sprintf("\n   OTM: %s $%.2f %s · vol %d · OI %d · IV %.0f%%",
				html.EscapeString(top.Type), top.Strike, html.EscapeString(top.Expiry), top.Volume, top.OI, top.IV)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n\n")
}

func cstLocation() *time.Location {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		return time.UTC
	}
	return loc
}
