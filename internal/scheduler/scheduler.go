// Package scheduler runs the cron jobs (pre-earnings discovery, EPS polling, momentum scan).
//
// Schedule (CST / America/Chicago):
//
//	08:30  pre-earnings job — discover today's reporters, send Telegram, store in DB
//	08:45  momentum scan    — scan momentum watchlist, send weekly targets
//	15:00  start EPS polling (1-minute interval until 18:00)
//	18:00  stop EPS polling
package scheduler

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/aashorefp-droid/stockpulsego/internal/models"
	scanpkg "github.com/aashorefp-droid/stockpulsego/internal/scanner"
	"github.com/aashorefp-droid/stockpulsego/internal/snapshot"
	"github.com/aashorefp-droid/stockpulsego/internal/telegram"
)

type Scheduler struct {
	cron             *cron.Cron
	loc              *time.Location
	tg               *telegram.Client
	snap             *snapshot.Service
	scanner          *scanpkg.Service
	pollID           cron.EntryID
	hasPoll          bool
	lightningMu      sync.Mutex
	lightningRunning bool
	lightningAlerted map[string]string
}

// New creates a scheduler bound to America/Chicago.
// Caller must Start() and Stop() to manage lifecycle.
func New(tg *telegram.Client, snap *snapshot.Service, scannerSvc *scanpkg.Service) *Scheduler {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		log.Printf("scheduler: failed to load America/Chicago, using UTC: %v", err)
		loc = time.UTC
	}
	return &Scheduler{
		cron:             cron.New(cron.WithLocation(loc)),
		loc:              loc,
		tg:               tg,
		snap:             snap,
		scanner:          scannerSvc,
		lightningAlerted: map[string]string{},
	}
}

// Start registers all jobs and begins the cron loop.
func (s *Scheduler) Start() error {
	if _, err := s.cron.AddFunc("30 8 * * *", s.preEarningsJob); err != nil {
		return err
	}
	if _, err := s.cron.AddFunc("45 8 * * *", s.momentumScanJob); err != nil {
		return err
	}
	if _, err := s.cron.AddFunc("0 15 * * *", s.startEPSPolling); err != nil {
		return err
	}
	if _, err := s.cron.AddFunc("0 15 * * *", s.dailySnapshotJob); err != nil {
		return err
	}
	if _, err := s.cron.AddFunc("0 18 * * *", s.stopEPSPolling); err != nil {
		return err
	}
	if _, err := s.cron.AddFunc("@every 5m", s.lightningOptionsWatcher); err != nil {
		return err
	}
	s.cron.Start()
	log.Println("scheduler: started - pre_earnings@8:30, momentum@8:45, lightning watcher@5m, snapshot+EPS poll@15:00, stop@18:00 CST")
	return nil
}

// Stop halts the cron loop and waits for running jobs.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

// IsPolling reports whether the EPS poll job is currently registered.
func (s *Scheduler) IsPolling() bool {
	return s.hasPoll
}

// ── Job stubs — real implementation needs earnings service which is in Phase 2.5 ──

func (s *Scheduler) preEarningsJob() {
	log.Println("scheduler: pre-earnings job triggered")
	if s.tg != nil {
		s.tg.Send("📅 <b>Pre-earnings discovery</b> — implementation pending earnings service port")
	}
}

func (s *Scheduler) momentumScanJob() {
	log.Println("scheduler: momentum scan triggered")
}

func (s *Scheduler) lightningOptionsWatcher() {
	if s.scanner == nil || s.tg == nil || s.tg.BotToken == "" || s.tg.ChatID == "" {
		return
	}
	now := time.Now().In(s.loc)
	if !isLightningWatcherWindow(now) {
		return
	}

	s.lightningMu.Lock()
	if s.lightningRunning {
		s.lightningMu.Unlock()
		return
	}
	s.lightningRunning = true
	s.lightningMu.Unlock()
	defer func() {
		s.lightningMu.Lock()
		s.lightningRunning = false
		s.lightningMu.Unlock()
	}()

	tickers := scanpkg.Watchlists["default"]
	if len(tickers) == 0 {
		return
	}

	log.Printf("scheduler: lightning options watcher scanning %d default tickers", len(tickers))
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	out := make(chan models.ScanResult, len(tickers))
	go s.scanner.Stream(ctx, tickers, nil, out)

	alertDate := marketDate(now)
	alerted := 0
	for res := range out {
		if res.Error != "" || !res.VolSurge || res.Ticker == "" {
			continue
		}
		if s.lightningAlreadyAlerted(res.Ticker, alertDate) {
			continue
		}
		if s.tg.Send(formatLightningOptionsAlert(res)) {
			s.markLightningAlerted(res.Ticker, alertDate)
			alerted++
		}
	}
	if alerted > 0 {
		log.Printf("scheduler: lightning options watcher sent %d alert(s)", alerted)
	}
}

func isLightningWatcherWindow(t time.Time) bool {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false
	}
	mins := t.Hour()*60 + t.Minute()
	return mins >= 8*60+30 && mins <= 15*60+15
}

func marketDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func (s *Scheduler) lightningAlreadyAlerted(ticker, date string) bool {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	s.lightningMu.Lock()
	defer s.lightningMu.Unlock()
	return s.lightningAlerted[ticker] == date
}

func (s *Scheduler) markLightningAlerted(ticker, date string) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	s.lightningMu.Lock()
	defer s.lightningMu.Unlock()
	s.lightningAlerted[ticker] = date
}

func formatLightningOptionsAlert(r models.ScanResult) string {
	dir := "LONG"
	if r.Direction != "" {
		dir = r.Direction
	}
	lines := []string{
		fmt.Sprintf("⚡ <b>%s lightning volume</b>", html.EscapeString(strings.ToUpper(r.Ticker))),
		fmt.Sprintf("$%.2f · %s · %s · score %+d", r.Price, html.EscapeString(r.Verdict), html.EscapeString(dir), r.Score),
	}
	if r.CPRDay15mVolText != "" {
		lines = append(lines, html.EscapeString(r.CPRDay15mVolText))
	}
	if r.OptStrategy != "" {
		lines = append(lines, fmt.Sprintf("Options: <b>%s</b>", html.EscapeString(r.OptStrategy)))
		if r.OptSummary != "" {
			lines = append(lines, html.EscapeString(r.OptSummary))
		}
		if r.OptAlt != "" {
			lines = append(lines, "Alt: "+html.EscapeString(r.OptAlt))
		}
		if r.OptDebit != nil {
			lines = append(lines, fmt.Sprintf("Debit: $%.2f", *r.OptDebit))
		}
		if r.OptProfit != nil {
			lines = append(lines, fmt.Sprintf("Max profit: $%.2f", *r.OptProfit))
		}
	} else {
		lines = append(lines, "Options: unavailable from current chain data")
	}
	if len(r.OptLiquid) > 0 {
		top := r.OptLiquid[0]
		lines = append(lines, fmt.Sprintf("Top OTM: %s $%.2f %s · vol %d · OI %d · IV %.0f%%",
			html.EscapeString(top.Type), top.Strike, html.EscapeString(top.Expiry), top.Volume, top.OI, top.IV*100))
	}
	return strings.Join(lines, "\n")
}

func (s *Scheduler) startEPSPolling() {
	if s.hasPoll {
		return
	}
	id, err := s.cron.AddFunc("@every 1m", s.pollEPS)
	if err != nil {
		log.Printf("scheduler: failed to register EPS poll: %v", err)
		return
	}
	s.pollID = id
	s.hasPoll = true
	log.Println("scheduler: EPS polling started")
	if s.tg != nil {
		s.tg.Send("⏱ EPS polling started (every 1 min)")
	}
}

func (s *Scheduler) stopEPSPolling() {
	if !s.hasPoll {
		return
	}
	s.cron.Remove(s.pollID)
	s.hasPoll = false
	log.Println("scheduler: EPS polling stopped")
	if s.tg != nil {
		s.tg.Send("🛑 EPS polling stopped")
	}
}

func (s *Scheduler) pollEPS() {
	// Real EPS poll requires earnings tracker DB — stubbed for now.
}

// dailySnapshotJob runs at 3:00 PM CST and saves NYSE/NASDAQ swing scan results.
func (s *Scheduler) dailySnapshotJob() {
	if s.snap == nil {
		return
	}
	log.Println("scheduler: daily snapshot job triggered")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		s.snap.RunAll(ctx)
		if s.tg != nil {
			s.tg.Send("📸 <b>Daily snapshot saved</b> — NYSE & NASDAQ swing universes")
		}
	}()
}
