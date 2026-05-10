// Package fundamentals fetches fundamentals data from Yahoo Finance's quoteSummary
// endpoint (no API key required) and derives flag-based "signals" matching
// stock_pulse.py's _build_fundamentals output.
package fundamentals

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

const (
	yahooURL   = "https://query2.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=summaryDetail,financialData,defaultKeyStatistics,assetProfile,price&crumb=%s"
	crumbURL   = "https://query2.finance.yahoo.com/v1/test/getcrumb"
	consentURL = "https://fc.yahoo.com"
	cacheTTL   = 6 * time.Hour
	uaHeader   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// Flag is one signal label with a color hint matching stock_pulse.py.
type Flag struct {
	Label string `json:"label"`
	Color string `json:"color"`
}

// Fundamentals matches the subset of fields used by stock_pulse.py's flags + display.
type Fundamentals struct {
	Ticker              string   `json:"ticker"`
	Name                string   `json:"name,omitempty"`
	Sector              string   `json:"sector,omitempty"`
	Industry            string   `json:"industry,omitempty"`
	MarketCap           int64    `json:"market_cap,omitempty"`
	MarketCapStr        string   `json:"market_cap_str,omitempty"`
	Price               float64  `json:"price,omitempty"`
	Valuation           string   `json:"valuation,omitempty"`
	PERatio             *float64 `json:"pe_ratio,omitempty"`
	ForwardPE           *float64 `json:"forward_pe,omitempty"`
	PriceToBook         *float64 `json:"price_to_book,omitempty"`
	EnterpriseToRevenue *float64 `json:"enterprise_to_revenue,omitempty"`
	EnterpriseToEBITDA  *float64 `json:"enterprise_to_ebitda,omitempty"`
	PEGRatio            *float64 `json:"peg_ratio,omitempty"`
	RevenueGrowth       *float64 `json:"revenue_growth,omitempty"`
	EarningsGrowth      *float64 `json:"earnings_growth,omitempty"`
	ProfitMargin        *float64 `json:"profit_margin,omitempty"`
	GrossMargin         *float64 `json:"gross_margin,omitempty"`
	DebtToEquity        *float64 `json:"debt_to_equity,omitempty"`
	ROE                 *float64 `json:"roe,omitempty"`
	ROA                 *float64 `json:"roa,omitempty"`
	Beta                *float64 `json:"beta,omitempty"`
	DividendYield       *float64 `json:"dividend_yield,omitempty"`
	TotalDebt           *float64 `json:"total_debt,omitempty"`
	FreeCashflow        *float64 `json:"free_cashflow,omitempty"`
	OperatingCashflow   *float64 `json:"operating_cashflow,omitempty"`
	ShortPctOfFloat     *float64 `json:"short_pct,omitempty"`
	TargetPrice         *float64 `json:"target_price,omitempty"`
	TargetUpside        *float64 `json:"target_upside,omitempty"`
	NumAnalysts         int      `json:"num_analysts,omitempty"`
	RecMean             *float64 `json:"rec_mean,omitempty"`
	Week52High          *float64 `json:"week52_high,omitempty"`
	Week52Low           *float64 `json:"week52_low,omitempty"`
	Week52Position      *float64 `json:"week52_position,omitempty"`
	TrailingEPS         *float64 `json:"trailing_eps,omitempty"`
	ForwardEPS          *float64 `json:"forward_eps,omitempty"`
	Flags               []Flag   `json:"flags"`
}

// Service caches fundamentals per ticker for 6 hours and shares a Yahoo crumb session.
type Service struct {
	http    *http.Client
	mu      sync.Mutex
	cache   map[string]cacheEntry
	crumb   string
	crumbAt time.Time
}

type cacheEntry struct {
	data *Fundamentals
	at   time.Time
}

func NewService() *Service {
	jar, _ := cookiejar.New(nil)
	return &Service{
		http: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
		},
		cache: map[string]cacheEntry{},
	}
}

// ensureCrumb fetches a session cookie + crumb token. Refreshes if older than 1 hour.
// Yahoo requires both for the quoteSummary endpoint as of 2024.
func (s *Service) ensureCrumb() (string, error) {
	s.mu.Lock()
	if s.crumb != "" && time.Since(s.crumbAt) < time.Hour {
		c := s.crumb
		s.mu.Unlock()
		return c, nil
	}
	s.mu.Unlock()

	// Step 1: hit consent URL to seed cookies
	req, _ := http.NewRequest("GET", consentURL, nil)
	req.Header.Set("User-Agent", uaHeader)
	resp, err := s.http.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	// Some regions get a redirect to a consent form; we ignore it — the cookies
	// already attached via the jar are usually enough.

	// Step 2: get the crumb
	req2, _ := http.NewRequest("GET", crumbURL, nil)
	req2.Header.Set("User-Agent", uaHeader)
	req2.Header.Set("Accept", "text/plain")
	resp2, err := s.http.Do(req2)
	if err != nil {
		return "", fmt.Errorf("crumb fetch: %w", err)
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode >= 300 || len(body) == 0 {
		return "", fmt.Errorf("crumb fetch: status %d body=%q", resp2.StatusCode, string(body))
	}
	crumb := string(body)
	s.mu.Lock()
	s.crumb = crumb
	s.crumbAt = time.Now()
	s.mu.Unlock()
	return crumb, nil
}

// Get returns fundamentals for a ticker. Caches 6h to avoid hammering Yahoo.
func (s *Service) Get(ticker string) (*Fundamentals, error) {
	s.mu.Lock()
	if e, ok := s.cache[ticker]; ok && time.Since(e.at) < cacheTTL {
		s.mu.Unlock()
		return e.data, nil
	}
	s.mu.Unlock()

	data, err := s.fetchYahoo(ticker)
	if err != nil {
		return nil, err
	}

	data.Flags = buildFlags(data)
	data.Valuation = classifyValuation(data.PERatio)
	data.MarketCapStr = formatMarketCap(data.MarketCap)
	if data.Week52High != nil && data.Week52Low != nil && data.Price > 0 {
		hi, lo := *data.Week52High, *data.Week52Low
		if hi > lo {
			pos := (data.Price - lo) / (hi - lo) * 100
			data.Week52Position = &pos
		}
	}

	s.mu.Lock()
	s.cache[ticker] = cacheEntry{data: data, at: time.Now()}
	s.mu.Unlock()
	return data, nil
}

// ── Yahoo quoteSummary parsing ───────────────────────────────────────────────

type yRaw struct {
	Raw float64 `json:"raw"`
	Fmt string  `json:"fmt"`
}

type ySummaryDetail struct {
	TrailingPE    *yRaw `json:"trailingPE"`
	ForwardPE     *yRaw `json:"forwardPE"`
	DividendYield *yRaw `json:"dividendYield"`
	MarketCap     *yRaw `json:"marketCap"`
	FiftyTwoHigh  *yRaw `json:"fiftyTwoWeekHigh"`
	FiftyTwoLow   *yRaw `json:"fiftyTwoWeekLow"`
	Beta          *yRaw `json:"beta"`
}

type yFinancialData struct {
	CurrentPrice            *yRaw `json:"currentPrice"`
	TargetMeanPrice         *yRaw `json:"targetMeanPrice"`
	RecommendationMean      *yRaw `json:"recommendationMean"`
	NumberOfAnalystOpinions *yRaw `json:"numberOfAnalystOpinions"`
	ProfitMargins           *yRaw `json:"profitMargins"`
	GrossMargins            *yRaw `json:"grossMargins"`
	RevenueGrowth           *yRaw `json:"revenueGrowth"`
	EarningsGrowth          *yRaw `json:"earningsGrowth"`
	DebtToEquity            *yRaw `json:"debtToEquity"`
	TotalDebt               *yRaw `json:"totalDebt"`
	FreeCashflow            *yRaw `json:"freeCashflow"`
	OperatingCashflow       *yRaw `json:"operatingCashflow"`
	ReturnOnAssets          *yRaw `json:"returnOnAssets"`
	ReturnOnEquity          *yRaw `json:"returnOnEquity"`
}

type yKeyStats struct {
	PegRatio            *yRaw `json:"pegRatio"`
	PriceToBook         *yRaw `json:"priceToBook"`
	EnterpriseToRevenue *yRaw `json:"enterpriseToRevenue"`
	EnterpriseToEbitda  *yRaw `json:"enterpriseToEbitda"`
	ShortPercentOfFloat *yRaw `json:"shortPercentOfFloat"`
	TrailingEps         *yRaw `json:"trailingEps"`
	ForwardEps          *yRaw `json:"forwardEps"`
}

type yAssetProfile struct {
	Sector   string `json:"sector"`
	Industry string `json:"industry"`
}

type yPrice struct {
	LongName           string `json:"longName"`
	ShortName          string `json:"shortName"`
	RegularMarketPrice *yRaw  `json:"regularMarketPrice"`
}

type yResult struct {
	SummaryDetail        *ySummaryDetail `json:"summaryDetail"`
	FinancialData        *yFinancialData `json:"financialData"`
	DefaultKeyStatistics *yKeyStats      `json:"defaultKeyStatistics"`
	AssetProfile         *yAssetProfile  `json:"assetProfile"`
	Price                *yPrice         `json:"price"`
}

type yResp struct {
	QuoteSummary struct {
		Result []yResult `json:"result"`
		Error  *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"quoteSummary"`
}

// doQuoteSummary fetches the JSON body from quoteSummary, handling crumb refresh.
// retried=true means we already retried once after a 401 — don't loop forever.
func (s *Service) doQuoteSummary(ticker string, retried bool) ([]byte, error) {
	crumb, err := s.ensureCrumb()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(yahooURL, ticker, crumb)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", uaHeader)
	req.Header.Set("Accept", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 && !retried {
		// crumb expired — invalidate and retry once
		s.mu.Lock()
		s.crumb = ""
		s.mu.Unlock()
		return s.doQuoteSummary(ticker, true)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("yahoo %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return body, nil
}

func (s *Service) fetchYahoo(ticker string) (*Fundamentals, error) {
	body, err := s.doQuoteSummary(ticker, false)
	if err != nil {
		return nil, err
	}

	var parsed yResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if parsed.QuoteSummary.Error != nil {
		return nil, fmt.Errorf("yahoo: %s", parsed.QuoteSummary.Error.Description)
	}
	if len(parsed.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("yahoo: no result for %s", ticker)
	}
	r := parsed.QuoteSummary.Result[0]

	out := &Fundamentals{Ticker: ticker}

	if r.Price != nil {
		out.Name = firstNonEmpty(r.Price.LongName, r.Price.ShortName)
		if r.Price.RegularMarketPrice != nil {
			out.Price = r.Price.RegularMarketPrice.Raw
		}
	}
	if r.AssetProfile != nil {
		out.Sector = r.AssetProfile.Sector
		out.Industry = r.AssetProfile.Industry
	}
	if r.SummaryDetail != nil {
		sd := r.SummaryDetail
		out.PERatio = rawPtr(sd.TrailingPE)
		out.ForwardPE = rawPtr(sd.ForwardPE)
		out.DividendYield = rawPtr(sd.DividendYield)
		out.Beta = rawPtr(sd.Beta)
		out.Week52High = rawPtr(sd.FiftyTwoHigh)
		out.Week52Low = rawPtr(sd.FiftyTwoLow)
		if sd.MarketCap != nil {
			out.MarketCap = int64(sd.MarketCap.Raw)
		}
	}
	if r.FinancialData != nil {
		fd := r.FinancialData
		out.RevenueGrowth = rawPtr(fd.RevenueGrowth)
		out.EarningsGrowth = rawPtr(fd.EarningsGrowth)
		out.ProfitMargin = rawPtr(fd.ProfitMargins)
		out.GrossMargin = rawPtr(fd.GrossMargins)
		out.DebtToEquity = rawPtr(fd.DebtToEquity)
		out.TotalDebt = rawPtr(fd.TotalDebt)
		out.FreeCashflow = rawPtr(fd.FreeCashflow)
		out.OperatingCashflow = rawPtr(fd.OperatingCashflow)
		out.ROE = rawPtr(fd.ReturnOnEquity)
		out.ROA = rawPtr(fd.ReturnOnAssets)
		out.TargetPrice = rawPtr(fd.TargetMeanPrice)
		out.RecMean = rawPtr(fd.RecommendationMean)
		if fd.NumberOfAnalystOpinions != nil {
			out.NumAnalysts = int(fd.NumberOfAnalystOpinions.Raw)
		}
		// upside %
		if out.TargetPrice != nil && out.Price > 0 {
			up := (*out.TargetPrice - out.Price) / out.Price * 100
			out.TargetUpside = &up
		}
	}
	if r.DefaultKeyStatistics != nil {
		ks := r.DefaultKeyStatistics
		out.PEGRatio = rawPtr(ks.PegRatio)
		out.PriceToBook = rawPtr(ks.PriceToBook)
		out.EnterpriseToRevenue = rawPtr(ks.EnterpriseToRevenue)
		out.EnterpriseToEBITDA = rawPtr(ks.EnterpriseToEbitda)
		out.ShortPctOfFloat = rawPtr(ks.ShortPercentOfFloat)
		out.TrailingEPS = rawPtr(ks.TrailingEps)
		out.ForwardEPS = rawPtr(ks.ForwardEps)
	}
	return out, nil
}

// ── Flag rules — mirrors stock_pulse.py:3966-3996 ────────────────────────────

func buildFlags(f *Fundamentals) []Flag {
	const (
		green  = "#00e5a0"
		red    = "#ff4d6a"
		yellow = "#f5c842"
		orange = "#ff8c42"
	)
	var flags []Flag
	add := func(label, color string) { flags = append(flags, Flag{Label: label, Color: color}) }

	if v := f.RevenueGrowth; v != nil {
		if *v > 0.20 {
			add("🚀 High Revenue Growth", green)
		} else if *v < -0.05 {
			add("📉 Revenue Declining", red)
		}
	}
	if v := f.EarningsGrowth; v != nil {
		if *v > 0.25 {
			add("💰 Strong Earnings Growth", green)
		} else if *v < -0.10 {
			add("⚠️ Earnings Declining", red)
		}
	}
	if v := f.ProfitMargin; v != nil {
		if *v > 0.20 {
			add("✅ High Margins", green)
		} else if *v < 0 {
			add("🔴 Unprofitable", red)
		}
	}
	if v := f.DebtToEquity; v != nil {
		if *v > 200 {
			add("⚠️ High Debt", red)
		} else if *v < 30 {
			add("✅ Low Debt", green)
		}
	}
	if v := f.FreeCashflow; v != nil {
		if *v > 0 {
			add(fmt.Sprintf("Positive Cash Flow %s", formatMoneyShort(*v)), green)
		} else if *v < 0 {
			add(fmt.Sprintf("Negative Cash Flow %s", formatMoneyShort(*v)), red)
		}
	} else if v := f.OperatingCashflow; v != nil {
		if *v > 0 {
			add(fmt.Sprintf("Positive Op Cash Flow %s", formatMoneyShort(*v)), green)
		} else if *v < 0 {
			add(fmt.Sprintf("Negative Op Cash Flow %s", formatMoneyShort(*v)), red)
		}
	}
	if v := f.ShortPctOfFloat; v != nil && *v > 0.10 {
		add("🔥 High Short Interest", orange)
	}
	if v := f.DividendYield; v != nil && *v > 0.03 {
		add("💵 Good Dividend", green)
	}
	if v := f.PEGRatio; v != nil && *v > 0 && *v < 1 {
		add("🎯 PEG < 1 (Growth Bargain)", green)
	}
	if v := f.TargetUpside; v != nil {
		if *v > 20 {
			add("📈 Analyst Upside >20%", green)
		} else if *v < -15 {
			add("📉 Analyst Downside >15%", red)
		}
	}
	if v := f.Week52Position; v != nil {
		if *v > 90 {
			add("⚡ Near 52W High", yellow)
		} else if *v < 15 {
			add("📉 Near 52W Low", orange)
		}
	}
	return flags
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func classifyValuation(pe *float64) string {
	if pe == nil || *pe <= 0 {
		return "N/A"
	}
	switch {
	case *pe <= 15:
		return "Undervalued"
	case *pe <= 25:
		return "Fair Value"
	case *pe <= 40:
		return "Overvalued"
	default:
		return "Very Expensive"
	}
}

func formatMarketCap(v int64) string {
	if v <= 0 {
		return ""
	}
	switch {
	case v >= 1_000_000_000_000:
		return fmt.Sprintf("$%.2fT", float64(v)/1e12)
	case v >= 1_000_000_000:
		return fmt.Sprintf("$%.2fB", float64(v)/1e9)
	case v >= 1_000_000:
		return fmt.Sprintf("$%.0fM", float64(v)/1e6)
	default:
		return fmt.Sprintf("$%d", v)
	}
}

func formatMoneyShort(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	switch {
	case v >= 1_000_000_000_000:
		return fmt.Sprintf("%s$%.2fT", sign, v/1e12)
	case v >= 1_000_000_000:
		return fmt.Sprintf("%s$%.2fB", sign, v/1e9)
	case v >= 1_000_000:
		return fmt.Sprintf("%s$%.2fM", sign, v/1e6)
	case v >= 1_000:
		return fmt.Sprintf("%s$%.1fK", sign, v/1e3)
	default:
		return fmt.Sprintf("%s$%.0f", sign, v)
	}
}

func rawPtr(r *yRaw) *float64 {
	if r == nil || r.Raw == 0 {
		return nil
	}
	v := r.Raw
	return &v
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
