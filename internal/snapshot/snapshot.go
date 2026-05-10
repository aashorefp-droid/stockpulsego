// Package snapshot runs scheduled scans and persists results to the DB.
package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aashorefp-droid/stockpulsego/internal/db"
	"github.com/aashorefp-droid/stockpulsego/internal/models"
	"github.com/aashorefp-droid/stockpulsego/internal/scanner"
	"github.com/aashorefp-droid/stockpulsego/internal/telegram"
)

// Universe describes one preset to snapshot daily.
type Universe struct {
	Key    string
	Filter scanner.UniverseFilter
}

// DefaultUniverses is the list snapshotted each day at the cron time.
var DefaultUniverses = []Universe{
	{Key: "nyse_swing", Filter: scanner.UniverseFilter{Exchange: "NYSE", MinPrice: 10, MinVol: 500_000, MaxCount: 200}},
	{Key: "nasdaq_swing", Filter: scanner.UniverseFilter{Exchange: "NASDAQ", MinPrice: 10, MinVol: 500_000, MaxCount: 200}},
}

type Service struct {
	Universe *scanner.UniverseService
	Scanner  *scanner.Service
	DB       *db.Store
	Telegram *telegram.Client
}

func New(u *scanner.UniverseService, s *scanner.Service, store *db.Store, tg *telegram.Client) *Service {
	return &Service{Universe: u, Scanner: s, DB: store, Telegram: tg}
}

// RunAll runs every default universe and saves the result.
// Logs (does not return) errors for individual universes so a single failure
// doesn't abort the rest.
func (s *Service) RunAll(ctx context.Context) {
	if s.DB == nil {
		log.Println("snapshot: DB unavailable — skipping run")
		return
	}
	for _, u := range DefaultUniverses {
		if err := s.RunOne(ctx, u); err != nil {
			log.Printf("snapshot: %s failed: %v", u.Key, err)
		}
	}
}

// RunOne builds the universe, runs the scan, filters to Exceptional setups only,
// persists them, and sends a Telegram alert with the ticker list.
func (s *Service) RunOne(ctx context.Context, u Universe) error {
	start := time.Now()
	tickers, err := s.Universe.Build(u.Filter)
	if err != nil {
		return err
	}
	log.Printf("snapshot: %s — built universe (%d tickers) in %s", u.Key, len(tickers), time.Since(start))

	out := make(chan models.ScanResult, len(tickers))
	scanCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	go s.Scanner.Stream(scanCtx, tickers, nil, out)

	scannedTotal := 0
	var exceptional []models.ScanResult
	for r := range out {
		if r.Error == "" {
			scannedTotal++
			if r.IsExceptional() {
				exceptional = append(exceptional, r)
			}
		}
	}

	body, err := json.Marshal(exceptional)
	if err != nil {
		return err
	}
	today := time.Now().In(cstLocation()).Format("2006-01-02")
	if err := s.DB.SaveSnapshot(u.Key, today, len(exceptional), string(body)); err != nil {
		return err
	}
	log.Printf("snapshot: %s — scanned %d, saved %d exceptional, total %s",
		u.Key, scannedTotal, len(exceptional), time.Since(start))

	if s.Telegram != nil {
		s.sendTelegramSummary(u.Key, scannedTotal, exceptional)
	}
	return nil
}

func (s *Service) sendTelegramSummary(key string, scanned int, exceptional []models.ScanResult) {
	if len(exceptional) == 0 {
		s.Telegram.Send(fmt.Sprintf(
			"📊 <b>%s snapshot</b> — scanned %d, no exceptional setups today.",
			strings.ToUpper(key), scanned,
		))
		return
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("🎯 <b>%s — %d exceptional setups</b> (of %d scanned)",
		strings.ToUpper(key), len(exceptional), scanned))
	for _, r := range exceptional {
		dir := "🟢"
		if r.Direction == "SHORT" {
			dir = "🔴"
		}
		line := fmt.Sprintf("%s <b>%s</b> $%.2f · %s · grade %s · score %+d",
			dir, r.Ticker, r.Price, r.Verdict, r.EntryGrade, r.Score)
		if r.Entry != nil && r.StopLoss != nil && r.Target1 != nil {
			line += fmt.Sprintf("\n   Entry $%.2f / Stop $%.2f / T1 $%.2f", *r.Entry, *r.StopLoss, *r.Target1)
		}
		if r.OptStrategy != "" {
			line += "\n   " + r.OptStrategy
		}
		lines = append(lines, line)
	}
	s.Telegram.Send(strings.Join(lines, "\n"))
}

func cstLocation() *time.Location {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		return time.UTC
	}
	return loc
}
