// Package scheduler runs the cron jobs (pre-earnings discovery, EPS polling, momentum scan).
//
// Schedule (CST / America/Chicago):
//
//	08:30  pre-earnings job — discover today's reporters, send Telegram, store in DB
//	08:45  momentum scan    — scan momentum watchlist, send weekly targets
//	15:00  start EPS polling (1-minute interval until 18:00)
//	15:35  after-close swing setup scan
//	18:00  stop EPS polling
package scheduler

import (
	"context"
	"fmt"
	"html"
	"log"
	"sort"
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
	if _, err := s.cron.AddFunc("35 15 * * 1-5", s.afterCloseSwingSetupJob); err != nil {
		return err
	}
	s.cron.Start()
	log.Println("scheduler: started - pre_earnings@8:30, momentum@8:45, lightning watcher@5m, snapshot+EPS poll@15:00, swing setups@15:35, stop@18:00 CST")
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
	earningsToday := s.sameDayEarningsTickers(alertDate)
	alerted := 0
	for res := range out {
		if res.Error != "" || !res.VolSurge || res.Ticker == "" {
			continue
		}
		if s.lightningAlreadyAlerted(res.Ticker, alertDate) {
			continue
		}
		ticker := strings.ToUpper(strings.TrimSpace(res.Ticker))
		if s.tg.Send(formatLightningOptionsAlert(res, earningsToday[ticker], alertDate)) {
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

func (s *Scheduler) sameDayEarningsTickers(date string) map[string]bool {
	out := map[string]bool{}
	if s == nil || s.snap == nil || s.snap.DB == nil || date == "" {
		return out
	}
	rows, err := s.snap.DB.GetAllForDate(date)
	if err != nil {
		log.Printf("scheduler: same-day earnings lookup failed for %s: %v", date, err)
		return out
	}
	for _, row := range rows {
		ticker := strings.ToUpper(strings.TrimSpace(row.Ticker))
		if ticker != "" {
			out[ticker] = true
		}
	}
	return out
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

func formatLightningOptionsAlert(r models.ScanResult, earningsToday bool, earningsDate string) string {
	dir := "LONG"
	if r.Direction != "" {
		dir = r.Direction
	}
	lines := []string{
		fmt.Sprintf("⚡ <b>%s lightning volume</b>", html.EscapeString(strings.ToUpper(r.Ticker))),
		fmt.Sprintf("$%.2f · %s · %s · score %+d", r.Price, html.EscapeString(r.Verdict), html.EscapeString(dir), r.Score),
	}
	if earningsToday {
		lines = append(lines, fmt.Sprintf("<b>!! EARNINGS TODAY (%s) !!</b> same-day earnings; treat volume/options as event-driven.", html.EscapeString(earningsDate)))
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

func (s *Scheduler) afterCloseSwingSetupJob() {
	if s.scanner == nil || s.tg == nil || s.tg.BotToken == "" || s.tg.ChatID == "" {
		return
	}
	now := time.Now().In(s.loc)
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return
	}
	tickers := s.afterCloseSwingUniverse()
	if len(tickers) == 0 {
		return
	}

	log.Printf("scheduler: after-close swing setup scan started (%d tickers)", len(tickers))
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	out := make(chan models.ScanResult, len(tickers))
	go s.scanner.StreamCore(ctx, tickers, nil, out)

	scanned := 0
	var actionable, exceptional, btd, preBreakout []models.ScanResult
	for res := range out {
		if res.Error != "" || res.Ticker == "" {
			continue
		}
		scanned++
		if isActionableSwing(res) {
			actionable = append(actionable, res)
		}
		if res.IsExceptional() {
			exceptional = append(exceptional, res)
		}
		if res.BTDTrigger {
			btd = append(btd, res)
		}
		if res.SwingPreBreakout {
			preBreakout = append(preBreakout, res)
		}
	}

	sortSwingLists(actionable, exceptional, btd, preBreakout)
	msg := formatAfterCloseSwingSetupAlert(scanned, actionable, exceptional, btd, preBreakout)
	if msg != "" {
		s.tg.Send(msg)
	}
	log.Printf("scheduler: after-close swing setup scan done scanned=%d actionable=%d exceptional=%d btd=%d prebreakout=%d",
		scanned, len(actionable), len(exceptional), len(btd), len(preBreakout))
}

func (s *Scheduler) afterCloseSwingUniverse() []string {
	seen := map[string]bool{}
	var tickers []string
	add := func(list []string) {
		for _, t := range list {
			t = strings.ToUpper(strings.TrimSpace(t))
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			tickers = append(tickers, t)
		}
	}
	if s.snap != nil && s.snap.Universe != nil {
		filters := []scanpkg.UniverseFilter{
			{Exchange: "NYSE", MinPrice: 10, MinVol: 500_000},
			{Exchange: "NASDAQ", MinPrice: 10, MinVol: 500_000},
		}
		for _, f := range filters {
			list, err := s.snap.Universe.Build(f)
			if err != nil {
				log.Printf("scheduler: after-close universe %s failed: %v", f.Exchange, err)
				continue
			}
			add(list)
		}
	}
	if len(tickers) == 0 {
		add(scanpkg.Watchlists["default"])
		add(scanpkg.Watchlists["momentum"])
	}
	return tickers
}

func isActionableSwing(r models.ScanResult) bool {
	if r.Entry == nil || r.StopLoss == nil || r.Target1 == nil {
		return false
	}
	if r.LREScore >= 3 && (r.LREStatus == "ACTIVE" || r.LREStatus == "DISCOUNT") {
		return true
	}
	if (r.EntryGrade == "S" || r.EntryGrade == "A") && r.MTFRank > 0 && r.MTFRank <= 2 && r.VolTrend == "ACCUMULATING" {
		return true
	}
	return false
}

func sortSwingLists(actionable, exceptional, btd, preBreakout []models.ScanResult) {
	byScore := func(list []models.ScanResult) {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Score != list[j].Score {
				return list[i].Score > list[j].Score
			}
			return list[i].Ticker < list[j].Ticker
		})
	}
	byScore(actionable)
	byScore(exceptional)
	byScore(btd)
	sort.Slice(preBreakout, func(i, j int) bool {
		if preBreakout[i].SwingPreBreakoutScore != preBreakout[j].SwingPreBreakoutScore {
			return preBreakout[i].SwingPreBreakoutScore > preBreakout[j].SwingPreBreakoutScore
		}
		di, dj := 999.0, 999.0
		if preBreakout[i].SwingPreBreakoutDistPct != nil {
			di = *preBreakout[i].SwingPreBreakoutDistPct
		}
		if preBreakout[j].SwingPreBreakoutDistPct != nil {
			dj = *preBreakout[j].SwingPreBreakoutDistPct
		}
		if di != dj {
			return di < dj
		}
		return preBreakout[i].Ticker < preBreakout[j].Ticker
	})
}

func formatAfterCloseSwingSetupAlert(scanned int, actionable, exceptional, btd, preBreakout []models.ScanResult) string {
	today := time.Now().Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("<b>After-close swing scan</b> %s", html.EscapeString(today)),
		fmt.Sprintf("Scanned %d stocks. Actionable %d | Exceptional %d | BTD Trigger %d | Pre-breakout %d",
			scanned, len(actionable), len(exceptional), len(btd), len(preBreakout)),
	}
	if len(actionable)+len(exceptional)+len(btd)+len(preBreakout) == 0 {
		lines = append(lines, "No swing setups matched today.")
		return strings.Join(lines, "\n")
	}
	appendSwingSection(&lines, "Pre-breakout watch", preBreakout, 10, formatPreBreakoutLine)
	appendSwingSection(&lines, "Actionable swing", actionable, 8, formatSwingLine)
	appendSwingSection(&lines, "Exceptional", exceptional, 8, formatSwingLine)
	appendSwingSection(&lines, "BTD Trigger", btd, 8, formatBTDLine)
	return strings.Join(lines, "\n\n")
}

func appendSwingSection(lines *[]string, title string, rows []models.ScanResult, limit int, format func(models.ScanResult) string) {
	if len(rows) == 0 {
		return
	}
	*lines = append(*lines, fmt.Sprintf("<b>%s</b>", html.EscapeString(title)))
	n := len(rows)
	if n > limit {
		n = limit
	}
	for i := 0; i < n; i++ {
		*lines = append(*lines, format(rows[i]))
	}
	if len(rows) > limit {
		*lines = append(*lines, fmt.Sprintf("+%d more", len(rows)-limit))
	}
}

func formatPreBreakoutLine(r models.ScanResult) string {
	level := ptrMoney(r.SwingPreBreakoutLevel)
	dist := ptrPct(r.SwingPreBreakoutDistPct)
	line := fmt.Sprintf("<b>%s</b> $%.2f - Pre-B/O %s under %s (score %d)",
		html.EscapeString(strings.ToUpper(r.Ticker)), r.Price, dist, level, r.SwingPreBreakoutScore)
	if r.SwingPreBreakoutTrigger != "" {
		line += "\n   Trigger: " + html.EscapeString(r.SwingPreBreakoutTrigger)
	}
	if swing := swingEntryLine(r); swing != "" {
		line += "\n   " + swing
	}
	if r.SwingPreBreakoutReason != "" {
		line += "\n   " + html.EscapeString(r.SwingPreBreakoutReason)
	}
	return line
}

func formatSwingLine(r models.ScanResult) string {
	line := fmt.Sprintf("<b>%s</b> $%.2f - %s grade %s score %+d",
		html.EscapeString(strings.ToUpper(r.Ticker)), r.Price,
		html.EscapeString(r.Verdict), html.EscapeString(r.EntryGrade), r.Score)
	if r.LREStatus != "" {
		line += fmt.Sprintf(" | LRE %d %s", r.LREScore, html.EscapeString(r.LREStatus))
	}
	if swing := swingEntryLine(r); swing != "" {
		line += "\n   " + swing
	}
	return line
}

func formatBTDLine(r models.ScanResult) string {
	line := formatSwingLine(r)
	if r.BTDTriggerText != "" {
		line += "\n   " + html.EscapeString(r.BTDTriggerText)
	}
	return line
}

func swingEntryLine(r models.ScanResult) string {
	if r.Entry == nil || r.StopLoss == nil || r.Target1 == nil {
		return ""
	}
	return fmt.Sprintf("Entry $%.2f / Stop $%.2f / T1 $%.2f", *r.Entry, *r.StopLoss, *r.Target1)
}

func ptrMoney(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("$%.2f", *v)
}

func ptrPct(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", *v)
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
