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
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/aashorefp-droid/stockpulsego/internal/snapshot"
	"github.com/aashorefp-droid/stockpulsego/internal/telegram"
)

type Scheduler struct {
	cron    *cron.Cron
	tg      *telegram.Client
	snap    *snapshot.Service
	pollID  cron.EntryID
	hasPoll bool
}

// New creates a scheduler bound to America/Chicago.
// Caller must Start() and Stop() to manage lifecycle.
func New(tg *telegram.Client, snap *snapshot.Service) *Scheduler {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		log.Printf("scheduler: failed to load America/Chicago, using UTC: %v", err)
		loc = time.UTC
	}
	return &Scheduler{
		cron: cron.New(cron.WithLocation(loc)),
		tg:   tg,
		snap: snap,
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
	s.cron.Start()
	log.Println("scheduler: started — pre_earnings@8:30, momentum@8:45, snapshot+EPS poll@15:00, stop@18:00 CST")
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
