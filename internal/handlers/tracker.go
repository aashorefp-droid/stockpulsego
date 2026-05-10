package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aashorefp-droid/stockpulsego/internal/db"
)

func (s *Server) handleTrackerToday(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "tracker DB not available")
		return
	}
	rows, err := s.DB.GetAllForDate(db.TodayStr())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []db.EarningsRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleWatchlistGet(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "tracker DB not available")
		return
	}
	tickers, err := s.DB.GetWatchlist()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tickers == nil {
		tickers = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"watchlist": tickers})
}

func (s *Server) handleWatchlistAdd(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "tracker DB not available")
		return
	}
	var body struct {
		Tickers []string `json:"tickers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	clean := make([]string, 0, len(body.Tickers))
	for _, t := range body.Tickers {
		t = strings.ToUpper(strings.TrimSpace(t))
		if t != "" {
			clean = append(clean, t)
		}
	}
	if err := s.DB.AddWatchlistTickers(clean); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"added": clean})
}

func (s *Server) handleWatchlistDelete(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "tracker DB not available")
		return
	}
	ticker := strings.ToUpper(r.URL.Query().Get("ticker"))
	if ticker == "" {
		writeError(w, http.StatusBadRequest, "ticker query param required")
		return
	}
	if err := s.DB.RemoveWatchlistTicker(ticker); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"removed": ticker})
}
