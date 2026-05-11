package macro

import (
	"fmt"
	"sync"
	"time"

	"github.com/aashorefp-droid/stockpulsego/internal/marketdata"
)

type Item struct {
	Ticker   string  `json:"ticker"`
	Label    string  `json:"label"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Chg1d    float64 `json:"chg_1d"`
	Chg5d    float64 `json:"chg_5d"`
	Chg20d   float64 `json:"chg_20d"`
}

type Risk struct {
	Score int      `json:"score"`
	Label string   `json:"label"`
	Notes []string `json:"notes"`
}

type EconEvent struct {
	Day    string `json:"day"`
	Date   string `json:"date"`
	Title  string `json:"title"`
	Time   string `json:"time"`
	Impact string `json:"impact"`
	Note   string `json:"note"`
}

type Snapshot struct {
	Items           []Item      `json:"items"`
	Risk            Risk        `json:"risk"`
	EconomicEvents  []EconEvent `json:"economic_events"`
	EconRefreshDate string      `json:"econ_refresh_date"`
}

// VIXY is used as a proxy for ^VIX since Alpaca doesn't expose index symbols.
var instruments = []struct {
	Ticker, Label, Category string
}{
	{"SPY", "S&P 500", "index"},
	{"QQQ", "Nasdaq", "index"},
	{"DIA", "Dow Jones", "index"},
	{"IWM", "Russell 2K", "index"},
	{"VIXY", "VIX (proxy)", "fear"},
	{"GLD", "Gold", "commodity"},
	{"SLV", "Silver", "commodity"},
	{"USO", "Oil", "commodity"},
	{"TLT", "Bonds 20Y", "bonds"},
	{"XLB", "Materials", "sector"},
	{"XLC", "Comm", "sector"},
	{"XLE", "Energy", "sector"},
	{"XLF", "Financials", "sector"},
	{"XLI", "Industrials", "sector"},
	{"XLK", "Tech", "sector"},
	{"XLP", "Staples", "sector"},
	{"XLRE", "Real Estate", "sector"},
	{"XLU", "Utilities", "sector"},
	{"XLV", "Health", "sector"},
	{"XLY", "Discretionary", "sector"},
}

var econEvents = []struct {
	Date, Title, Time, Impact, Note string
	Priority                        int
}{
	{"2026-05-12", "CPI", "08:30 AM ET", "High", "Consumer Price Index for Apr 2026", 100},
	{"2026-06-10", "CPI", "08:30 AM ET", "High", "Consumer Price Index for May 2026", 100},
	{"2026-07-14", "CPI", "08:30 AM ET", "High", "Consumer Price Index for Jun 2026", 100},
	{"2026-08-12", "CPI", "08:30 AM ET", "High", "Consumer Price Index for Jul 2026", 100},
	{"2026-09-11", "CPI", "08:30 AM ET", "High", "Consumer Price Index for Aug 2026", 100},
	{"2026-10-14", "CPI", "08:30 AM ET", "High", "Consumer Price Index for Sep 2026", 100},
	{"2026-11-10", "CPI", "08:30 AM ET", "High", "Consumer Price Index for Oct 2026", 100},
	{"2026-12-10", "CPI", "08:30 AM ET", "High", "Consumer Price Index for Nov 2026", 100},
	{"2026-05-13", "PPI", "08:30 AM ET", "High", "Producer Price Index for Apr 2026", 80},
	{"2026-06-11", "PPI", "08:30 AM ET", "High", "Producer Price Index for May 2026", 80},
	{"2026-07-15", "PPI", "08:30 AM ET", "High", "Producer Price Index for Jun 2026", 80},
	{"2026-08-13", "PPI", "08:30 AM ET", "High", "Producer Price Index for Jul 2026", 80},
	{"2026-09-10", "PPI", "08:30 AM ET", "High", "Producer Price Index for Aug 2026", 80},
	{"2026-10-15", "PPI", "08:30 AM ET", "High", "Producer Price Index for Sep 2026", 80},
	{"2026-11-13", "PPI", "08:30 AM ET", "High", "Producer Price Index for Oct 2026", 80},
	{"2026-12-15", "PPI", "08:30 AM ET", "High", "Producer Price Index for Nov 2026", 80},
	{"2026-06-05", "Jobs", "08:30 AM ET", "High", "Employment Situation for May 2026", 90},
	{"2026-07-02", "Jobs", "08:30 AM ET", "High", "Employment Situation for Jun 2026", 90},
	{"2026-08-07", "Jobs", "08:30 AM ET", "High", "Employment Situation for Jul 2026", 90},
	{"2026-09-04", "Jobs", "08:30 AM ET", "High", "Employment Situation for Aug 2026", 90},
	{"2026-10-02", "Jobs", "08:30 AM ET", "High", "Employment Situation for Sep 2026", 90},
	{"2026-11-06", "Jobs", "08:30 AM ET", "High", "Employment Situation for Oct 2026", 90},
	{"2026-12-04", "Jobs", "08:30 AM ET", "High", "Employment Situation for Nov 2026", 90},
	{"2026-06-17", "FOMC", "02:00 PM ET", "High", "Fed statement / rate decision", 95},
	{"2026-07-29", "FOMC", "02:00 PM ET", "High", "Fed statement / rate decision", 95},
	{"2026-09-16", "FOMC", "02:00 PM ET", "High", "Fed statement / rate decision", 95},
	{"2026-10-28", "FOMC", "02:00 PM ET", "High", "Fed statement / rate decision", 95},
	{"2026-12-09", "FOMC", "02:00 PM ET", "High", "Fed statement / rate decision", 95},
}

type Service struct {
	md       *marketdata.Client
	mu       sync.Mutex
	cache    *Snapshot
	cacheAt  time.Time
	cacheTTL time.Duration
}

func NewService(md *marketdata.Client) *Service {
	return &Service{md: md, cacheTTL: 5 * time.Minute}
}

func (s *Service) Snapshot() (*Snapshot, error) {
	s.mu.Lock()
	if s.cache != nil && time.Since(s.cacheAt) < s.cacheTTL {
		out := s.cache
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -45)
	startStr := start.Format("2006-01-02")
	endStr := end.Format("2006-01-02")

	type result struct {
		idx  int
		item Item
		err  error
	}
	out := make([]Item, 0, len(instruments))
	resCh := make(chan result, len(instruments))

	for i, ins := range instruments {
		go func(i int, t, l, c string) {
			bars, err := s.md.GetDailyBars(t, startStr, endStr)
			if err != nil || len(bars) < 2 {
				resCh <- result{idx: i, err: fmt.Errorf("no data for %s", t)}
				return
			}
			price := bars[len(bars)-1].Close
			chg := func(n int) float64 {
				if len(bars) <= n {
					return 0
				}
				prev := bars[len(bars)-1-n].Close
				if prev == 0 {
					return 0
				}
				return (price - prev) / prev * 100
			}
			resCh <- result{idx: i, item: Item{
				Ticker: t, Label: l, Category: c,
				Price:  round2(price),
				Chg1d:  round2(chg(1)),
				Chg5d:  round2(chg(5)),
				Chg20d: round2(chg(20)),
			}}
		}(i, ins.Ticker, ins.Label, ins.Category)
	}
	for i := 0; i < len(instruments); i++ {
		r := <-resCh
		if r.err == nil {
			out = append(out, r.item)
		}
	}

	risk := computeRisk(out)
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.Local
	}
	today := time.Now().In(loc)
	snap := &Snapshot{
		Items:           out,
		Risk:            risk,
		EconomicEvents:  economicEvents(today),
		EconRefreshDate: today.Format("2006-01-02"),
	}

	s.mu.Lock()
	s.cache = snap
	s.cacheAt = time.Now()
	s.mu.Unlock()
	return snap, nil
}

func economicEvents(today time.Time) []EconEvent {
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	targets := []struct {
		label string
		date  time.Time
	}{
		{"Today", day},
		{"Tomorrow", day.AddDate(0, 0, 1)},
	}
	out := make([]EconEvent, 0, len(targets))
	for _, target := range targets {
		dateStr := target.date.Format("2006-01-02")
		bestIdx := -1
		bestPriority := -1
		for i, event := range econEvents {
			if event.Date == dateStr && event.Priority > bestPriority {
				bestIdx = i
				bestPriority = event.Priority
			}
		}
		if bestIdx >= 0 {
			event := econEvents[bestIdx]
			out = append(out, EconEvent{
				Day:    target.label,
				Date:   event.Date,
				Title:  event.Title,
				Time:   event.Time,
				Impact: event.Impact,
				Note:   event.Note,
			})
			continue
		}
		out = append(out, EconEvent{
			Day:    target.label,
			Date:   dateStr,
			Title:  "No major scheduled",
			Impact: "Low",
			Note:   "No CPI, PPI, Employment Situation, or FOMC event scheduled.",
		})
	}
	return out
}

func computeRisk(items []Item) Risk {
	score := 0
	notes := []string{}
	find := func(t string) *Item {
		for i := range items {
			if items[i].Ticker == t {
				return &items[i]
			}
		}
		return nil
	}
	vix := find("VIXY")
	spy := find("SPY")
	tlt := find("TLT")
	gld := find("GLD")

	// VIXY proxy: use 5d change as fear gauge instead of absolute level
	if vix != nil {
		if vix.Chg5d > 10 {
			score += 2
			notes = append(notes, fmt.Sprintf("VIXY +%.1f%% 5d — elevated fear", vix.Chg5d))
		} else if vix.Chg5d > 5 {
			score += 1
			notes = append(notes, fmt.Sprintf("VIXY +%.1f%% 5d — mild caution", vix.Chg5d))
		} else {
			notes = append(notes, fmt.Sprintf("VIXY %.1f%% 5d — calm", vix.Chg5d))
		}
	}
	if spy != nil && spy.Chg5d < -2 {
		score++
		notes = append(notes, fmt.Sprintf("SPY 5d: %+.1f%% — under pressure", spy.Chg5d))
	}
	if tlt != nil && tlt.Chg5d > 1 {
		score++
		notes = append(notes, "Bonds rallying — flight to safety")
	}
	if gld != nil && gld.Chg5d > 2 {
		score++
		notes = append(notes, fmt.Sprintf("Gold 5d: %+.1f%% — safe haven demand", gld.Chg5d))
	}

	label := "LOW RISK"
	if score >= 3 {
		label = "HIGH RISK"
	} else if score >= 1 {
		label = "MODERATE RISK"
	}
	return Risk{Score: score, Label: label, Notes: notes}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
