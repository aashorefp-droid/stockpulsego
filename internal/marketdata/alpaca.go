package marketdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aashorefp-droid/stockpulsego/internal/models"
)

type Client struct {
	APIKey    string
	APISecret string
	BaseURL   string
	HTTP      *http.Client
}

func NewClient(apiKey, apiSecret, baseURL string) *Client {
	return &Client{
		APIKey:    apiKey,
		APISecret: apiSecret,
		BaseURL:   baseURL,
		HTTP:      &http.Client{Timeout: 20 * time.Second},
	}
}

type alpacaBar struct {
	T string  `json:"t"`
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
	V int64   `json:"v"`
}

type alpacaBarsResp struct {
	Bars          []alpacaBar `json:"bars"`
	NextPageToken string      `json:"next_page_token"`
}

// OptionSnapshot is a single contract's snapshot from Alpaca.
type OptionSnapshot struct {
	Symbol       string
	Strike       float64
	Expiration   string
	IsCall       bool
	Bid          float64
	Ask          float64
	Last         float64
	Volume       int     // from dailyBar.v
	IV           float64 // impliedVolatility
	OpenInterest int     // from /v1beta1/options/contracts (separate fetch)
	QuoteTS      string
}

type alpacaOptSnapshot struct {
	LatestQuote *struct {
		Bp float64 `json:"bp"`
		Ap float64 `json:"ap"`
		T  string  `json:"t"`
	} `json:"latestQuote"`
	LatestTrade *struct {
		P float64 `json:"p"`
	} `json:"latestTrade"`
	DailyBar *struct {
		V int64 `json:"v"`
	} `json:"dailyBar"`
	ImpliedVolatility float64 `json:"impliedVolatility"`
}

type alpacaOptSnapshotsResp struct {
	Snapshots     map[string]alpacaOptSnapshot `json:"snapshots"`
	NextPageToken string                       `json:"next_page_token"`
}

func (c *Client) get(endpoint string, params url.Values) ([]byte, error) {
	full := c.BaseURL + endpoint
	if params != nil {
		full += "?" + params.Encode()
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest("GET", full, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("APCA-API-KEY-ID", c.APIKey)
		req.Header.Set("APCA-API-SECRET-KEY", c.APISecret)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch resp.StatusCode {
		case 200:
			return body, nil
		case 429:
			time.Sleep(5 * time.Second)
			continue
		case 401, 403:
			return nil, fmt.Errorf("alpaca auth error %d", resp.StatusCode)
		case 404:
			return nil, fmt.Errorf("alpaca 404: not found")
		default:
			lastErr = fmt.Errorf("alpaca status %d: %s", resp.StatusCode, string(body))
			time.Sleep(time.Second)
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("alpaca request failed after retries")
}

// GetDailyBars fetches daily OHLCV bars in [start, end] (YYYY-MM-DD).
func (c *Client) GetDailyBars(ticker, start, end string) (models.Bars, error) {
	return c.fetchBars(ticker, "1Day", start, end)
}

// GetHourlyBars fetches hourly OHLCV bars.
func (c *Client) GetHourlyBars(ticker, start, end string) (models.Bars, error) {
	return c.fetchBars(ticker, "1Hour", start, end)
}

// GetFiveMinuteBars fetches 5-minute OHLCV bars for opening-volume checks.
func (c *Client) GetFiveMinuteBars(ticker, start, end string) (models.Bars, error) {
	return c.fetchBars(ticker, "5Min", start, end)
}

func (c *Client) fetchBars(ticker, timeframe, start, end string) (models.Bars, error) {
	var all []alpacaBar
	pageToken := ""
	for {
		params := url.Values{}
		params.Set("timeframe", timeframe)
		params.Set("start", start+"T00:00:00Z")
		params.Set("end", end+"T23:59:59Z")
		params.Set("adjustment", "all")
		params.Set("limit", "10000")
		params.Set("sort", "asc")
		params.Set("feed", "iex")
		if pageToken != "" {
			params.Set("page_token", pageToken)
		}
		body, err := c.get(fmt.Sprintf("/v2/stocks/%s/bars", ticker), params)
		if err != nil {
			return nil, err
		}
		var resp alpacaBarsResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Bars...)
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	bars := make(models.Bars, 0, len(all))
	for _, b := range all {
		t, err := time.Parse(time.RFC3339, b.T)
		if err != nil {
			continue
		}
		bars = append(bars, models.Bar{
			Time: t, Open: b.O, High: b.H, Low: b.L, Close: b.C, Volume: b.V,
		})
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Time.Before(bars[j].Time) })
	return bars, nil
}

// GetOptionSnapshots fetches all options snapshots for a ticker, paginated.
// Caps at 4 pages = 4000 contracts (matches Python behavior).
func (c *Client) GetOptionSnapshots(ticker string) ([]OptionSnapshot, error) {
	var all []OptionSnapshot
	pageToken := ""
	for page := 0; page < 4; page++ {
		params := url.Values{}
		params.Set("limit", "1000")
		params.Set("feed", "indicative")
		if pageToken != "" {
			params.Set("page_token", pageToken)
		}
		body, err := c.get(fmt.Sprintf("/v1beta1/options/snapshots/%s", ticker), params)
		if err != nil {
			return nil, err
		}
		var resp alpacaOptSnapshotsResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		for sym, snap := range resp.Snapshots {
			parsed, ok := parseOptionSymbol(sym, ticker)
			if !ok {
				continue
			}
			os := OptionSnapshot{
				Symbol:     sym,
				Strike:     parsed.strike,
				Expiration: parsed.exp,
				IsCall:     parsed.isCall,
				IV:         snap.ImpliedVolatility,
			}
			if snap.LatestQuote != nil {
				os.Bid = snap.LatestQuote.Bp
				os.Ask = snap.LatestQuote.Ap
				os.QuoteTS = snap.LatestQuote.T
			}
			if snap.LatestTrade != nil {
				os.Last = snap.LatestTrade.P
			}
			if snap.DailyBar != nil {
				os.Volume = int(snap.DailyBar.V)
			}
			all = append(all, os)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return all, nil
}

// alpacaContract is one option contract from /v1beta1/options/contracts.
type alpacaContract struct {
	Symbol           string `json:"symbol"`
	OpenInterest     string `json:"open_interest"`
	OpenInterestDate string `json:"open_interest_date"`
}

type alpacaContractsResp struct {
	OptionContracts []alpacaContract `json:"option_contracts"`
	NextPageToken   string           `json:"next_page_token"`
}

// Asset is a tradable stock from Alpaca's /v2/assets endpoint.
type Asset struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	Exchange  string `json:"exchange"`
	Class     string `json:"class"`
	Status    string `json:"status"`
	Tradable  bool   `json:"tradable"`
	Shortable bool   `json:"shortable"`
}

// GetAssets returns all active US-equity assets, optionally filtered by exchange.
// exchange: "NYSE", "NASDAQ", "ARCA", "AMEX", "BATS", or "" for all.
func (c *Client) GetAssets(exchange string) ([]Asset, error) {
	// Note: assets endpoint lives on the trading API host, not data.alpaca.markets
	tradingURL := "https://api.alpaca.markets"
	params := url.Values{}
	params.Set("status", "active")
	params.Set("asset_class", "us_equity")
	if exchange != "" {
		params.Set("exchange", exchange)
	}
	req, err := http.NewRequest("GET", tradingURL+"/v2/assets?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", c.APIKey)
	req.Header.Set("APCA-API-SECRET-KEY", c.APISecret)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("assets %d: %s", resp.StatusCode, string(body))
	}
	var assets []Asset
	if err := json.Unmarshal(body, &assets); err != nil {
		return nil, err
	}
	return assets, nil
}

// StockSnapshot is a bulk-snapshot entry for one ticker.
type StockSnapshot struct {
	Symbol    string
	Price     float64
	Volume    int64
	PrevClose float64
	ChangePct float64
}

type alpacaStockSnap struct {
	LatestTrade *struct {
		P float64 `json:"p"`
	} `json:"latestTrade"`
	DailyBar *struct {
		C float64 `json:"c"`
		V int64   `json:"v"`
	} `json:"dailyBar"`
	PrevDailyBar *struct {
		C float64 `json:"c"`
	} `json:"prevDailyBar"`
}

// GetStockSnapshots fetches snapshots for many symbols in batches of 100.
// Returns a map keyed by symbol.
func (c *Client) GetStockSnapshots(symbols []string) (map[string]StockSnapshot, error) {
	out := make(map[string]StockSnapshot, len(symbols))
	const batchSize = 100
	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[i:end]
		params := url.Values{}
		params.Set("symbols", strings.Join(batch, ","))
		params.Set("feed", "iex")
		body, err := c.get("/v2/stocks/snapshots", params)
		if err != nil {
			continue // skip failed batch, keep going
		}
		var resp map[string]alpacaStockSnap
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		for sym, snap := range resp {
			s := StockSnapshot{Symbol: sym}
			if snap.LatestTrade != nil {
				s.Price = snap.LatestTrade.P
			}
			if snap.DailyBar != nil {
				if s.Price == 0 {
					s.Price = snap.DailyBar.C
				}
				s.Volume = snap.DailyBar.V
			}
			if snap.PrevDailyBar != nil {
				s.PrevClose = snap.PrevDailyBar.C
				if s.PrevClose > 0 && s.Price > 0 {
					s.ChangePct = (s.Price - s.PrevClose) / s.PrevClose * 100
				}
			}
			out[sym] = s
		}
	}
	return out, nil
}

// GetOptionOpenInterest returns a map of contract symbol → open interest
// by paginating through /v1beta1/options/contracts.
func (c *Client) GetOptionOpenInterest(ticker string) (map[string]int, error) {
	out := make(map[string]int)
	pageToken := ""
	for page := 0; page < 4; page++ {
		params := url.Values{}
		params.Set("underlying_symbols", ticker)
		params.Set("status", "active")
		params.Set("limit", "10000")
		if pageToken != "" {
			params.Set("page_token", pageToken)
		}
		body, err := c.get("/v1beta1/options/contracts", params)
		if err != nil {
			return out, err
		}
		var resp alpacaContractsResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return out, err
		}
		for _, oc := range resp.OptionContracts {
			oi := 0
			fmt.Sscanf(oc.OpenInterest, "%d", &oi)
			out[oc.Symbol] = oi
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return out, nil
}

type parsedOption struct {
	strike float64
	exp    string
	isCall bool
}

// parseOptionSymbol parses OCC symbols like "AAPL250516C00190000".
func parseOptionSymbol(sym, ticker string) (parsedOption, bool) {
	tail := sym
	if len(tail) > len(ticker) && tail[:len(ticker)] == ticker {
		tail = tail[len(ticker):]
	}
	if len(tail) < 15 {
		return parsedOption{}, false
	}
	expRaw := tail[:6]
	if len(expRaw) != 6 {
		return parsedOption{}, false
	}
	cp := tail[6]
	strikeRaw := tail[7:15]
	var strikeInt int
	for _, ch := range strikeRaw {
		if ch < '0' || ch > '9' {
			return parsedOption{}, false
		}
		strikeInt = strikeInt*10 + int(ch-'0')
	}
	exp := "20" + expRaw[:2] + "-" + expRaw[2:4] + "-" + expRaw[4:6]
	return parsedOption{
		strike: float64(strikeInt) / 1000,
		exp:    exp,
		isCall: cp == 'C',
	}, true
}
