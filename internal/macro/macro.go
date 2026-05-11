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

type Snapshot struct {
	Items []Item `json:"items"`
	Risk  Risk   `json:"risk"`
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
	snap := &Snapshot{Items: out, Risk: risk}

	s.mu.Lock()
	s.cache = snap
	s.cacheAt = time.Now()
	s.mu.Unlock()
	return snap, nil
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
