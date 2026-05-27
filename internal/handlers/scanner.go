package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aashorefp-droid/stockpulsego/internal/db"
	"github.com/aashorefp-droid/stockpulsego/internal/models"
	"github.com/aashorefp-droid/stockpulsego/internal/scanner"
	"github.com/aashorefp-droid/stockpulsego/internal/snapshot"
)

// Dynamic universe presets — resolved at scan time.
var dynamicUniverses = map[string]scanner.UniverseFilter{
	"nyse_swing":   {Exchange: "NYSE", MinPrice: 10, MinVol: 500_000, MaxCount: 200},
	"nasdaq_swing": {Exchange: "NASDAQ", MinPrice: 10, MinVol: 500_000, MaxCount: 200},
	"all_swing":    {Exchange: "", MinPrice: 10, MinVol: 1_000_000, MaxCount: 250},
}

// handleScannerSnapshotRun manually triggers the daily snapshot job.
// Useful for first-time use, on-demand refresh, or scheduling via Windows Task Scheduler.
//
// POST /api/scanner/snapshot/run            → runs all default universes (NYSE + NASDAQ)
// POST /api/scanner/snapshot/run?watchlist=nyse_swing  → runs only that universe
func (s *Server) handleScannerSnapshotRun(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil || s.Snapshot == nil {
		writeError(w, http.StatusServiceUnavailable, "snapshot service unavailable")
		return
	}
	requested := r.URL.Query().Get("watchlist")

	// Run async so the HTTP request returns quickly — actual scan takes 30s+
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if requested == "" {
			s.Snapshot.RunAll(ctx)
			return
		}
		// Find matching dynamic universe and run just that one
		filter, ok := dynamicUniverses[requested]
		if !ok {
			fmt.Printf("snapshot run: unknown watchlist %q\n", requested)
			return
		}
		_ = s.Snapshot.RunOne(ctx, snapshot.Universe{Key: requested, Filter: filter})
	}()

	resp := map[string]any{"status": "started", "watchlist": requested}
	if requested == "" {
		resp["watchlist"] = "all"
	}
	writeJSON(w, http.StatusAccepted, resp)
}

// handleScannerSnapshot returns the most recent saved snapshot for a watchlist.
// Used by the frontend to instantly load NYSE/NASDAQ swing results without rescanning.
func (s *Server) handleScannerSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "snapshot DB unavailable")
		return
	}
	watchlist := r.URL.Query().Get("watchlist")
	if watchlist == "" {
		writeError(w, http.StatusBadRequest, "watchlist required")
		return
	}
	snap, err := s.DB.GetLatestSnapshot(watchlist)
	if err != nil {
		if err == db.ErrNotFound {
			writeJSON(w, http.StatusOK, map[string]any{
				"watchlist": watchlist, "available": false, "results": []any{},
			})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Pass-through: results is already valid JSON
	w.Header().Set("Content-Type", "application/json")
	body := fmt.Sprintf(
		`{"watchlist":%q,"date":%q,"count":%d,"created_at":%q,"available":true,"results":%s}`,
		snap.Watchlist, snap.Date, snap.Count, snap.CreatedAt, snap.Results,
	)
	w.Write([]byte(body))
}

func (s *Server) handleWatchlists(w http.ResponseWriter, r *http.Request) {
	out := map[string]int{}
	for k, v := range scanner.Watchlists {
		out[k] = len(v)
	}
	for k, f := range dynamicUniverses {
		out[k] = f.MaxCount // approximate; actual size resolved at scan time
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleScannerStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	q := r.URL.Query()
	watchlist := q.Get("watchlist")
	if watchlist == "" {
		watchlist = "default"
	}
	customTickers := q.Get("tickers")
	asOfStr := q.Get("as_of")
	scanMode := scanner.ParseMode(q.Get("mode"))

	var tickers []string
	if customTickers != "" {
		for _, t := range strings.Split(customTickers, ",") {
			t = strings.ToUpper(strings.TrimSpace(t))
			if t != "" {
				tickers = append(tickers, t)
			}
		}
	} else if filter, ok := dynamicUniverses[watchlist]; ok {
		// Dynamic universe (NYSE swing, etc.) — build at scan time, cached 1h
		built, err := s.Universe.Build(filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "universe build failed: "+err.Error())
			return
		}
		tickers = built
	} else if list, ok := scanner.Watchlists[watchlist]; ok {
		tickers = list
	} else {
		tickers = scanner.Watchlists["default"]
	}

	var asOf *time.Time
	if asOfStr != "" {
		if t, err := time.Parse("2006-01-02", asOfStr); err == nil {
			asOf = &t
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	out := make(chan models.ScanResult, len(tickers))

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go s.Scanner.StreamMode(ctx, tickers, asOf, scanMode, out)

	count := 0
	for res := range out {
		data, err := json.Marshal(res)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		count++
	}

	doneMsg := models.ScanResult{Done: true, Total: count}
	data, _ := json.Marshal(doneMsg)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
