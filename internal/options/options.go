package options

import (
	"math"
	"sort"
	"time"

	"github.com/aashorefp-droid/stockpulsego/internal/marketdata"
	"github.com/aashorefp-droid/stockpulsego/internal/models"
)

const (
	maxQuoteAgeHours = 72.0
	maxSpreadPct     = 0.50
	minOI            = 10
	minMid           = 0.05
)

type Service struct {
	MD *marketdata.Client
}

func NewService(md *marketdata.Client) *Service {
	return &Service{MD: md}
}

// Strategy is the recommended options play, mirroring Python output.
type Strategy struct {
	Ticker    string             `json:"ticker"`
	Strategy  string             `json:"strategy"`
	Legs      []models.OptionLeg `json:"legs"`
	NetDebit  float64            `json:"net_debit"`
	MaxProfit *float64           `json:"max_profit,omitempty"`
	Width     *float64           `json:"width,omitempty"`
	ExpShort  string             `json:"exp_short,omitempty"`
	ExpLong   string             `json:"exp_long,omitempty"`
	QuoteTS   string             `json:"quote_ts,omitempty"`
	Source    string             `json:"source"`
	Summary   string             `json:"summary"`
	Alt       string             `json:"alt,omitempty"`
}

// Bias is the options-flow read for a ticker.
type Bias struct {
	CallOI       int                   `json:"call_oi"`
	PutOI        int                   `json:"put_oi"`
	CallVol      int                   `json:"call_vol"`
	PutVol       int                   `json:"put_vol"`
	OIPCRatio    float64               `json:"oi_pc_ratio"`
	VolPCRatio   float64               `json:"vol_pc_ratio"`
	OISentiment  string                `json:"oi_sentiment"`
	UnusualCount int                   `json:"unusual_count"`
	OTMLiquid    []models.OTMLiquidRow `json:"otm_liquid"`
	Error        string                `json:"error,omitempty"`
}

// BuildStrategy returns a strategy recommendation for the given direction + zone.
// direction: "LONG" | "SHORT"
// zone: "HIGH" | "MID" | "LOW"
func (s *Service) BuildStrategy(ticker string, currentPrice float64, direction, zone string) (*Strategy, error) {
	snaps, err := s.MD.GetOptionSnapshots(ticker)
	if err != nil || len(snaps) == 0 {
		return nil, err
	}

	today := time.Now().UTC()
	expMin := today.AddDate(0, 0, 5).Format("2006-01-02")
	expMax := today.AddDate(0, 0, 60).Format("2006-01-02")
	sLo := currentPrice * 0.85
	sHi := currentPrice * 1.15

	var contracts []marketdata.OptionSnapshot
	for _, sn := range snaps {
		if sn.Expiration < expMin || sn.Expiration > expMax {
			continue
		}
		if currentPrice > 0 && (sn.Strike < sLo || sn.Strike > sHi) {
			continue
		}
		if !isLiquid(sn) {
			continue
		}
		contracts = append(contracts, sn)
	}
	if len(contracts) == 0 {
		return nil, nil
	}

	// Find nearest expirations to today+7d and today+21d
	expSet := map[string]bool{}
	for _, c := range contracts {
		expSet[c.Expiration] = true
	}
	var exps []string
	for e := range expSet {
		exps = append(exps, e)
	}
	sort.Strings(exps)

	expShort := nearestExp(exps, today.AddDate(0, 0, 7))
	expLong := nearestExp(exps, today.AddDate(0, 0, 21))

	calls := func(exp string) []marketdata.OptionSnapshot {
		var out []marketdata.OptionSnapshot
		for _, c := range contracts {
			if c.Expiration == exp && c.IsCall {
				out = append(out, c)
			}
		}
		return out
	}
	puts := func(exp string) []marketdata.OptionSnapshot {
		var out []marketdata.OptionSnapshot
		for _, c := range contracts {
			if c.Expiration == exp && !c.IsCall {
				out = append(out, c)
			}
		}
		return out
	}

	isBullish := direction == "LONG" || (zone == "LOW" && direction != "SHORT")
	isBearish := direction == "SHORT" || (zone == "HIGH" && direction != "LONG")

	if isBullish {
		return s.buildBullish(ticker, currentPrice, expShort, expLong, calls(expShort), calls(expLong))
	}
	if isBearish {
		return s.buildBearish(ticker, currentPrice, expShort, expLong, puts(expShort), puts(expLong))
	}
	return s.buildNeutral(ticker, currentPrice, expLong, calls(expLong), puts(expLong), expShort)
}

// ── Bias + OTM Liquid ────────────────────────────────────────────────────────

// AnalyzeBias scans the full chain and returns sentiment + OTM-liquid + unusual flow.
// Fetches both snapshots (volume, IV, quotes) and contracts metadata (OI) and merges them.
func (s *Service) AnalyzeBias(ticker string, currentPrice float64) Bias {
	snaps, err := s.MD.GetOptionSnapshots(ticker)
	if err != nil {
		return Bias{Error: err.Error()}
	}
	// Fetch OI separately — Alpaca snapshots don't include it.
	oiMap, _ := s.MD.GetOptionOpenInterest(ticker)
	for i := range snaps {
		if oi, ok := oiMap[snaps[i].Symbol]; ok {
			snaps[i].OpenInterest = oi
		}
	}

	today := time.Now().UTC().Format("2006-01-02")
	expSet := map[string]bool{}
	for _, sn := range snaps {
		if sn.Expiration > today {
			expSet[sn.Expiration] = true
		}
	}
	var validExps []string
	for e := range expSet {
		validExps = append(validExps, e)
	}
	sort.Strings(validExps)
	if len(validExps) > 6 {
		validExps = validExps[:6]
	}
	expFilter := map[string]bool{}
	for _, e := range validExps {
		expFilter[e] = true
	}

	type item struct {
		strike, otmPct, mid, iv float64
		oi, volume              int
		exp                     string
		isCall, itm             bool
		notional                float64
	}
	var items []item
	for _, sn := range snaps {
		if !expFilter[sn.Expiration] {
			continue
		}
		mid := 0.0
		if sn.Bid > 0 && sn.Ask > 0 {
			mid = (sn.Bid + sn.Ask) / 2
		} else if sn.Last > 0 {
			mid = sn.Last
		}
		otmPct := 0.0
		var itm bool
		if currentPrice > 0 {
			if sn.IsCall {
				itm = sn.Strike < currentPrice
				otmPct = math.Max(0, (sn.Strike-currentPrice)/currentPrice*100)
			} else {
				itm = sn.Strike > currentPrice
				otmPct = math.Max(0, (currentPrice-sn.Strike)/currentPrice*100)
			}
		}
		notional := mid * float64(sn.Volume) * 100
		items = append(items, item{
			strike: sn.Strike, otmPct: otmPct, mid: mid, iv: sn.IV,
			oi: sn.OpenInterest, volume: sn.Volume, exp: sn.Expiration,
			isCall: sn.IsCall, itm: itm, notional: notional,
		})
	}

	callOI, putOI, callVol, putVol := 0, 0, 0, 0
	for _, i := range items {
		if i.isCall {
			callOI += i.oi
			callVol += i.volume
		} else {
			putOI += i.oi
			putVol += i.volume
		}
	}
	oiPC, volPC := 0.0, 0.0
	if callOI > 0 {
		oiPC = float64(putOI) / float64(callOI)
	}
	if callVol > 0 {
		volPC = float64(putVol) / float64(callVol)
	}

	oiSent := "NEUTRAL"
	switch {
	case oiPC < 0.7:
		oiSent = "BULLISH"
	case oiPC > 1.0:
		oiSent = "BEARISH"
	}

	// OTM-liquid: filter by volume ≥ 50 + OI ≥ 100 (OI may be 0 if contracts call failed)
	// Falls back to volume-only when OI data isn't available.
	hasOI := false
	for _, i := range items {
		if i.oi > 0 {
			hasOI = true
			break
		}
	}

	var otmRows []models.OTMLiquidRow
	unusual := 0
	for _, i := range items {
		if i.itm {
			continue
		}
		if i.volume < 50 {
			continue
		}
		if hasOI && i.oi < 100 {
			continue
		}
		typ := "PUT"
		if i.isCall {
			typ = "CALL"
		}
		volOI := 0.0
		if i.oi > 0 {
			volOI = float64(i.volume) / float64(i.oi)
		}
		// "Unusual" requires positioning signal beyond just block size:
		// SWEEP (vol>oi), HIGH_RATIO (vol/oi>2 with vol≥100), FAR_OTM (≥30% with vol≥50)
		isSweep := i.oi > 0 && i.volume > i.oi
		isHighRatio := i.oi > 0 && volOI > 2 && i.volume >= 100
		isFarOTM := i.otmPct >= 30 && i.volume >= 50
		isUnusual := isSweep || isHighRatio || isFarOTM
		if isUnusual {
			unusual++
		}
		otmRows = append(otmRows, models.OTMLiquidRow{
			Strike: i.strike, Type: typ, Expiry: i.exp,
			Volume: i.volume, OI: i.oi,
			IV:         round1(i.iv * 100),
			OTMPct:     round1(i.otmPct),
			VolOIRatio: round2(volOI),
			Unusual:    isUnusual,
		})
	}
	sort.Slice(otmRows, func(a, b int) bool {
		if otmRows[a].Unusual != otmRows[b].Unusual {
			return otmRows[a].Unusual
		}
		return otmRows[a].Volume > otmRows[b].Volume
	})
	if len(otmRows) > 10 {
		otmRows = otmRows[:10]
	}

	return Bias{
		CallOI: callOI, PutOI: putOI,
		CallVol: callVol, PutVol: putVol,
		OIPCRatio:    round2(oiPC),
		VolPCRatio:   round2(volPC),
		OISentiment:  oiSent,
		UnusualCount: unusual,
		OTMLiquid:    otmRows,
	}
}

// ── Strategy builders ────────────────────────────────────────────────────────

func (s *Service) buildBullish(ticker string, price float64, expShort, expLong string, callsShort, callsLong []marketdata.OptionSnapshot) (*Strategy, error) {
	buyC := nearestStrike(callsLong, price)
	if buyC == nil {
		buyC = nearestStrike(callsShort, price)
	}
	if buyC == nil {
		return nil, nil
	}
	sellC := nearestStrike(callsLong, price*1.03)
	if sellC != nil && sellC.Strike == buyC.Strike {
		var above []marketdata.OptionSnapshot
		for _, c := range callsLong {
			if c.Strike > buyC.Strike {
				above = append(above, c)
			}
		}
		sellC = nearestStrike(above, buyC.Strike+1)
	}

	if sellC != nil && sellC.Strike != buyC.Strike {
		netDebit := round2(midPrice(*buyC) - midPrice(*sellC))
		width := round2(sellC.Strike - buyC.Strike)
		if validSpread(netDebit, width) {
			maxProfit := round2(width - netDebit)
			return &Strategy{
				Ticker:    ticker,
				Strategy:  "Bull Call Spread",
				Legs:      []models.OptionLeg{leg("BUY", "CALL", *buyC, expLong), leg("SELL", "CALL", *sellC, expLong)},
				NetDebit:  netDebit,
				MaxProfit: &maxProfit,
				Width:     &width,
				ExpShort:  expShort,
				ExpLong:   expLong,
				QuoteTS:   buyC.QuoteTS,
				Source:    "alpaca",
				Summary:   formatBCS(ticker, *buyC, *sellC, width, netDebit, maxProfit, expLong),
				Alt:       formatLongCallAlt(*buyC, expShort),
			}, nil
		}
	}
	mid := midPrice(*buyC)
	return &Strategy{
		Ticker:   ticker,
		Strategy: "Long Call",
		Legs:     []models.OptionLeg{leg("BUY", "CALL", *buyC, expShort)},
		NetDebit: round2(mid),
		ExpShort: expShort,
		ExpLong:  expLong,
		QuoteTS:  buyC.QuoteTS,
		Source:   "alpaca",
		Summary:  formatLongCall(ticker, *buyC, mid, expShort),
	}, nil
}

func (s *Service) buildBearish(ticker string, price float64, expShort, expLong string, putsShort, putsLong []marketdata.OptionSnapshot) (*Strategy, error) {
	buyP := nearestStrike(putsLong, price)
	if buyP == nil {
		buyP = nearestStrike(putsShort, price)
	}
	if buyP == nil {
		return nil, nil
	}
	sellP := nearestStrike(putsLong, price*0.97)
	if sellP != nil && sellP.Strike == buyP.Strike {
		var below []marketdata.OptionSnapshot
		for _, c := range putsLong {
			if c.Strike < buyP.Strike {
				below = append(below, c)
			}
		}
		sellP = nearestStrike(below, buyP.Strike-1)
	}

	if sellP != nil && sellP.Strike != buyP.Strike {
		netDebit := round2(midPrice(*buyP) - midPrice(*sellP))
		width := round2(buyP.Strike - sellP.Strike)
		if validSpread(netDebit, width) {
			maxProfit := round2(width - netDebit)
			return &Strategy{
				Ticker:    ticker,
				Strategy:  "Bear Put Spread",
				Legs:      []models.OptionLeg{leg("BUY", "PUT", *buyP, expLong), leg("SELL", "PUT", *sellP, expLong)},
				NetDebit:  netDebit,
				MaxProfit: &maxProfit,
				Width:     &width,
				ExpShort:  expShort,
				ExpLong:   expLong,
				QuoteTS:   buyP.QuoteTS,
				Source:    "alpaca",
				Summary:   formatBPS(ticker, *buyP, *sellP, width, netDebit, maxProfit, expLong),
				Alt:       formatLongPutAlt(*buyP, expShort),
			}, nil
		}
	}
	mid := midPrice(*buyP)
	return &Strategy{
		Ticker:   ticker,
		Strategy: "Long Put",
		Legs:     []models.OptionLeg{leg("BUY", "PUT", *buyP, expShort)},
		NetDebit: round2(mid),
		ExpShort: expShort,
		ExpLong:  expLong,
		QuoteTS:  buyP.QuoteTS,
		Source:   "alpaca",
		Summary:  formatLongPut(ticker, *buyP, mid, expShort),
	}, nil
}

func (s *Service) buildNeutral(ticker string, price float64, expLong string, callsLong, putsLong []marketdata.OptionSnapshot, expShort string) (*Strategy, error) {
	atmC := nearestStrike(callsLong, price)
	atmP := nearestStrike(putsLong, price)
	var wingCalls, wingPuts []marketdata.OptionSnapshot
	for _, c := range callsLong {
		if c.Strike > price*1.02 {
			wingCalls = append(wingCalls, c)
		}
	}
	for _, c := range putsLong {
		if c.Strike < price*0.98 {
			wingPuts = append(wingPuts, c)
		}
	}
	wingC := nearestStrike(wingCalls, price*1.03)
	wingP := nearestStrike(wingPuts, price*0.97)

	if atmC != nil && atmP != nil && wingC != nil && wingP != nil {
		credit := round2(midPrice(*atmC) + midPrice(*atmP) - midPrice(*wingC) - midPrice(*wingP))
		return &Strategy{
			Ticker:   ticker,
			Strategy: "Iron Butterfly",
			Legs: []models.OptionLeg{
				leg("SELL", "CALL", *atmC, expLong),
				leg("SELL", "PUT", *atmP, expLong),
				leg("BUY", "CALL", *wingC, expLong),
				leg("BUY", "PUT", *wingP, expLong),
			},
			NetDebit:  -credit,
			MaxProfit: &credit,
			ExpShort:  expShort,
			ExpLong:   expLong,
			QuoteTS:   atmC.QuoteTS,
			Source:    "alpaca",
			Summary: "🦋 " + ticker + " Iron Butterfly: " +
				"Sell $" + f0(atmC.Strike) + "C+$" + f0(atmP.Strike) + "P / " +
				"Buy $" + f0(wingC.Strike) + "C+$" + f0(wingP.Strike) + "P " +
				"Exp " + expLong + " | Credit ~$" + f2(credit),
		}, nil
	}
	if atmC != nil && atmP != nil {
		netDebit := round2(midPrice(*atmC) + midPrice(*atmP))
		return &Strategy{
			Ticker:   ticker,
			Strategy: "Straddle",
			Legs:     []models.OptionLeg{leg("BUY", "CALL", *atmC, expLong), leg("BUY", "PUT", *atmP, expLong)},
			NetDebit: netDebit,
			ExpShort: expShort,
			ExpLong:  expLong,
			QuoteTS:  atmC.QuoteTS,
			Source:   "alpaca",
			Summary: "🦋 " + ticker + " Straddle: $" + f0(atmC.Strike) + "C @ ~$" + f2(midPrice(*atmC)) +
				" + $" + f0(atmP.Strike) + "P @ ~$" + f2(midPrice(*atmP)) + " Exp " + expLong,
		}, nil
	}
	return nil, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func isLiquid(c marketdata.OptionSnapshot) bool {
	if c.QuoteTS != "" {
		if t, err := time.Parse(time.RFC3339Nano, c.QuoteTS); err == nil {
			ageH := time.Since(t).Hours()
			if ageH > maxQuoteAgeHours {
				return false
			}
		}
	}
	hasTwoSided := c.Bid > 0 && c.Ask > 0
	var mid float64
	if hasTwoSided {
		mid = (c.Bid + c.Ask) / 2
	} else if c.Last > 0 {
		mid = c.Last
	} else {
		return false
	}
	if mid < minMid || c.Bid < 0 || c.Ask < 0 {
		return false
	}
	if hasTwoSided {
		spread := (c.Ask - c.Bid) / mid
		if spread > maxSpreadPct {
			return false
		}
	} else if c.OpenInterest < minOI {
		return false
	}
	return true
}

func validSpread(netDebit, width float64) bool {
	return netDebit > 0 && netDebit < width
}

func midPrice(c marketdata.OptionSnapshot) float64 {
	if c.Bid > 0 && c.Ask > 0 {
		return round2((c.Bid + c.Ask) / 2)
	}
	return round2(c.Last)
}

func nearestExp(exps []string, target time.Time) string {
	if len(exps) == 0 {
		return ""
	}
	best := exps[0]
	bestDiff := math.MaxFloat64
	for _, e := range exps {
		t, err := time.Parse("2006-01-02", e)
		if err != nil {
			continue
		}
		d := math.Abs(t.Sub(target).Hours())
		if d < bestDiff {
			bestDiff = d
			best = e
		}
	}
	return best
}

func nearestStrike(contracts []marketdata.OptionSnapshot, target float64) *marketdata.OptionSnapshot {
	if len(contracts) == 0 {
		return nil
	}
	best := &contracts[0]
	bestDiff := math.Abs(contracts[0].Strike - target)
	for i := 1; i < len(contracts); i++ {
		d := math.Abs(contracts[i].Strike - target)
		if d < bestDiff {
			bestDiff = d
			best = &contracts[i]
		}
	}
	return best
}

func leg(action, typ string, c marketdata.OptionSnapshot, exp string) models.OptionLeg {
	mid := midPrice(c)
	var sp *float64
	if c.Bid > 0 && c.Ask > 0 && mid > 0 {
		v := round1((c.Ask - c.Bid) / mid * 100)
		sp = &v
	}
	return models.OptionLeg{
		Action: action, Type: typ,
		Strike: c.Strike, Exp: exp,
		Bid: c.Bid, Ask: c.Ask, Mid: mid,
		SpreadPct: sp,
	}
}

func formatBCS(ticker string, b, s marketdata.OptionSnapshot, width, debit, maxProfit float64, exp string) string {
	return "📈 " + ticker + " Bull Call Spread: Width: $" + f0(width) +
		"  Buy $" + f0(b.Strike) + "C / Sell $" + f0(s.Strike) + "C " +
		"Exp " + exp + " | Debit ~$" + f2(debit) + " | Max Profit ~$" + f2(maxProfit)
}

func formatBPS(ticker string, b, s marketdata.OptionSnapshot, width, debit, maxProfit float64, exp string) string {
	return "📉 " + ticker + " Bear Put Spread: Width: $" + f0(width) +
		"  Buy $" + f0(b.Strike) + "P / Sell $" + f0(s.Strike) + "P " +
		"Exp " + exp + " | Debit ~$" + f2(debit) + " | Max Profit ~$" + f2(maxProfit)
}

func formatLongCall(ticker string, c marketdata.OptionSnapshot, mid float64, exp string) string {
	return "📈 " + ticker + " Long $" + f0(c.Strike) + " Call @ ~$" + f2(mid) + " Exp " + exp
}

func formatLongPut(ticker string, c marketdata.OptionSnapshot, mid float64, exp string) string {
	return "📉 " + ticker + " Long $" + f0(c.Strike) + " Put @ ~$" + f2(mid) + " Exp " + exp
}

func formatLongCallAlt(c marketdata.OptionSnapshot, exp string) string {
	return "Alt: Long $" + f0(c.Strike) + " Call @ ~$" + f2(midPrice(c)) + " Exp " + exp
}

func formatLongPutAlt(c marketdata.OptionSnapshot, exp string) string {
	return "Alt: Long $" + f0(c.Strike) + " Put @ ~$" + f2(midPrice(c)) + " Exp " + exp
}

func round1(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return float64(int(v*10+0.5)) / 10
}

func round2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return float64(int(v*100+0.5)) / 100
}

func f0(v float64) string {
	return formatFloat(v, 0)
}

func f2(v float64) string {
	return formatFloat(v, 2)
}

func formatFloat(v float64, decimals int) string {
	mult := math.Pow(10, float64(decimals))
	rounded := math.Round(v*mult) / mult
	return fmtFloat(rounded, decimals)
}

func fmtFloat(v float64, decimals int) string {
	if decimals == 0 {
		return itoa64(int64(v))
	}
	whole := int64(v)
	frac := int64(math.Round((v - float64(whole)) * math.Pow(10, float64(decimals))))
	if frac < 0 {
		frac = -frac
	}
	return itoa64(whole) + "." + padLeftZero(itoa64(frac), decimals)
}

func itoa64(v int64) string {
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
	out := string(buf[i:])
	if neg {
		out = "-" + out
	}
	return out
}

func padLeftZero(s string, width int) string {
	for len(s) < width {
		s = "0" + s
	}
	return s
}
