package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aashorefp-droid/stockpulsego/internal/analysis"
	"github.com/aashorefp-droid/stockpulsego/internal/config"
	"github.com/aashorefp-droid/stockpulsego/internal/db"
	"github.com/aashorefp-droid/stockpulsego/internal/fundamentals"
	"github.com/aashorefp-droid/stockpulsego/internal/macro"
	"github.com/aashorefp-droid/stockpulsego/internal/marketdata"
	"github.com/aashorefp-droid/stockpulsego/internal/options"
	"github.com/aashorefp-droid/stockpulsego/internal/scanner"
	"github.com/aashorefp-droid/stockpulsego/internal/snapshot"
	"github.com/aashorefp-droid/stockpulsego/internal/telegram"
)

// Server holds shared dependencies for HTTP handlers.
type Server struct {
	Cfg          *config.Config
	MD           *marketdata.Client
	MacroService *macro.Service
	Options      *options.Service
	Analysis     *analysis.Service
	Scanner      *scanner.Service
	Universe     *scanner.UniverseService
	Snapshot     *snapshot.Service
	Fundamentals *fundamentals.Service
	DB           *db.Store
	Telegram     *telegram.Client
}

// NewServer constructs a Server with all dependencies wired up.
// Returned Server is also used by the scheduler/snapshot service.
func NewServer(cfg *config.Config) *Server {
	md := marketdata.NewClient(cfg.AlpacaAPIKey, cfg.AlpacaAPISecret, cfg.AlpacaDataBase)
	an := analysis.NewService(md)
	opts := options.NewService(md)
	fund := fundamentals.NewService()
	srv := &Server{
		Cfg:          cfg,
		MD:           md,
		MacroService: macro.NewService(md),
		Options:      opts,
		Analysis:     an,
		Scanner:      scanner.NewService(an, opts, fund),
		Universe:     scanner.NewUniverseService(md),
		Fundamentals: fund,
	}
	if store, err := db.Open(cfg.DBPath); err == nil {
		srv.DB = store
	} else {
		log.Printf("tracker DB unavailable (%s): %v", cfg.DBPath, err)
	}
	srv.Telegram = telegram.NewClient(cfg.TelegramBotToken, cfg.TelegramChatID)
	srv.Snapshot = snapshot.New(srv.Universe, srv.Scanner, srv.DB, srv.Telegram)
	return srv
}

func (srv *Server) Register(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.Get("/macro/snapshot", srv.handleMacroSnapshot)
		r.Post("/telegram/test", srv.handleTelegramTest)
		r.Post("/telegram/lightning-scan", srv.handleTelegramLightningScan)
		r.Get("/scanner/stream", srv.handleScannerStream)
		r.Get("/scanner/snapshot", srv.handleScannerSnapshot)
		r.Post("/scanner/snapshot/run", srv.handleScannerSnapshotRun)
		r.Get("/scanner/watchlists", srv.handleWatchlists)
		r.Get("/analysis/{ticker}", srv.handleAnalysis)
		r.Get("/options/{ticker}", srv.handleOptionsBias)
		r.Get("/fundamentals/{ticker}", srv.handleFundamentals)
		r.Get("/tracker/today", srv.handleTrackerToday)
		r.Get("/watchlist", srv.handleWatchlistGet)
		r.Post("/watchlist", srv.handleWatchlistAdd)
		r.Delete("/watchlist", srv.handleWatchlistDelete)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
