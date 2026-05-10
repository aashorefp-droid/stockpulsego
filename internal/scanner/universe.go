package scanner

import (
	"sort"
	"sync"
	"time"

	"github.com/aashorefp-droid/stockpulsego/internal/marketdata"
)

// UniverseFilter describes a dynamic universe selection.
type UniverseFilter struct {
	Exchange string  // "NYSE", "NASDAQ", "" for all
	MinPrice float64 // e.g. 10.0
	MinVol   int64   // e.g. 500_000 daily shares
	MaxCount int     // hard cap on returned tickers (sorted by volume desc)
}

// UniverseService builds dynamic ticker lists with caching.
type UniverseService struct {
	md       *marketdata.Client
	mu       sync.Mutex
	cache    map[string]universeEntry
	cacheTTL time.Duration
}

type universeEntry struct {
	tickers []string
	at      time.Time
}

func NewUniverseService(md *marketdata.Client) *UniverseService {
	return &UniverseService{
		md:       md,
		cache:    map[string]universeEntry{},
		cacheTTL: time.Hour,
	}
}

// Build returns the filtered, sorted ticker list. Cached by filter signature for 1 hour.
func (u *UniverseService) Build(f UniverseFilter) ([]string, error) {
	key := cacheKey(f)
	u.mu.Lock()
	if e, ok := u.cache[key]; ok && time.Since(e.at) < u.cacheTTL {
		out := append([]string(nil), e.tickers...)
		u.mu.Unlock()
		return out, nil
	}
	u.mu.Unlock()

	assets, err := u.md.GetAssets(f.Exchange)
	if err != nil {
		return nil, err
	}
	symbols := make([]string, 0, len(assets))
	for _, a := range assets {
		if !a.Tradable || a.Status != "active" {
			continue
		}
		// skip warrants, units, preferreds — heuristic: avoid dots/dashes
		if hasNonAlpha(a.Symbol) {
			continue
		}
		symbols = append(symbols, a.Symbol)
	}

	snaps, err := u.md.GetStockSnapshots(symbols)
	if err != nil {
		return nil, err
	}

	type ranked struct {
		symbol string
		volume int64
	}
	var passing []ranked
	for sym, s := range snaps {
		if s.Price < f.MinPrice {
			continue
		}
		if s.Volume < f.MinVol {
			continue
		}
		passing = append(passing, ranked{symbol: sym, volume: s.Volume})
	}
	sort.Slice(passing, func(i, j int) bool { return passing[i].volume > passing[j].volume })
	if f.MaxCount > 0 && len(passing) > f.MaxCount {
		passing = passing[:f.MaxCount]
	}
	out := make([]string, len(passing))
	for i, p := range passing {
		out[i] = p.symbol
	}

	u.mu.Lock()
	u.cache[key] = universeEntry{tickers: out, at: time.Now()}
	u.mu.Unlock()
	return out, nil
}

func cacheKey(f UniverseFilter) string {
	return f.Exchange + "|" + itoa(int(f.MinPrice*100)) + "|" + itoa(int(f.MinVol)) + "|" + itoa(f.MaxCount)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func hasNonAlpha(s string) bool {
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			return true
		}
	}
	return false
}
