package scanner

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/aashorefp-droid/stockpulsego/internal/analysis"
	"github.com/aashorefp-droid/stockpulsego/internal/fundamentals"
	"github.com/aashorefp-droid/stockpulsego/internal/models"
	"github.com/aashorefp-droid/stockpulsego/internal/options"
)

type Service struct {
	Analysis     *analysis.Service
	Options      *options.Service
	Fundamentals *fundamentals.Service
	MaxWorkers   int
}

func NewService(a *analysis.Service, o *options.Service, f *fundamentals.Service) *Service {
	return &Service{Analysis: a, Options: o, Fundamentals: f, MaxWorkers: 12}
}

// Stream runs the scan in parallel and pushes each ScanResult to `out` as it completes.
// The channel is closed when all tickers are processed or ctx is cancelled.
func (s *Service) Stream(ctx context.Context, tickers []string, asOf *time.Time, out chan<- models.ScanResult) {
	defer close(out)

	sem := make(chan struct{}, s.MaxWorkers)
	var wg sync.WaitGroup

	for _, t := range tickers {
		select {
		case <-ctx.Done():
			return
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(ticker string) {
			defer wg.Done()
			defer func() { <-sem }()
			res := s.scanOne(ticker, asOf)
			select {
			case <-ctx.Done():
			case out <- res:
			}
		}(t)
	}
	wg.Wait()
}

func (s *Service) scanOne(ticker string, asOf *time.Time) models.ScanResult {
	a, err := s.Analysis.Analyze(ticker, asOf)
	if err != nil {
		return models.ScanResult{Ticker: ticker, Error: truncateErr(err.Error()), Score: 0}
	}
	if a == nil || a.Verdict == "INSUFFICIENT DATA" {
		return models.ScanResult{Ticker: ticker, Error: "Insufficient data", Score: 0}
	}

	rank := mtfRank(a.WeeklyBias, a.DailyBias, a.H4Bias)
	signal, action, key := mtfSignalAction(a.WeeklyBias, a.DailyBias, a.H4Bias)

	entry := a.Entry
	stop := a.StopLoss
	t1 := a.Target1
	risk := a.RiskPct
	rr := a.RRT1
	atr := a.ATR
	dfh := a.DistFromHigh

	res := models.ScanResult{
		Ticker:             ticker,
		Sector:             sectorForTicker(ticker, nil),
		Price:              a.CurrentPrice,
		Verdict:            a.Verdict,
		Confidence:         a.Confidence,
		Score:              a.Score,
		Direction:          a.Direction,
		EntryGrade:         grade(a.Score, a.Confidence),
		MTFRank:            rank,
		MTFSignal:          signal,
		MTFAction:          action,
		MTFKey:             key,
		WeeklyBias:         a.WeeklyBias,
		DailyBias:          a.DailyBias,
		LongTermSpring:     a.LongTermSpring,
		LongTermSpringText: a.LongTermSpringText,
		SwingSpring:        a.SwingSpring,
		SwingSpringText:    a.SwingSpringText,
		DaySpring:          a.DaySpring,
		DaySpringText:      a.DaySpringText,
		VolTrend:           a.VolTrend,
		VolSurge:           a.VolSurge,
		BreakoutScore:      a.BreakoutScore,
		// Zones + fib levels (ported from stock_pulse.py)
		EarnZone:          a.EarnZone,
		WeeklyZone:        a.WeeklyZoneCls,
		NearFibName:       a.NearFibName,
		NearFibPrice:      a.NearFibPrice,
		WeekHi:            a.WeekHi,
		WeekLo:            a.WeekLo,
		WkPosPct:          a.WkPosPct,
		EarnHi:            a.EarnHi,
		EarnLo:            a.EarnLo,
		EarnPosPct:        a.EarnPosPct,
		FibLevels:         a.FibLevels,
		FibCompression:    a.FibCompression,
		CPRType:           a.CPRType,
		CPRTC:             a.CPRTC,
		CPRBC:             a.CPRBC,
		CPRP:              a.CPRP,
		CPRPosition:       a.CPRPosition,
		CPRInterp:         a.CPRInterp,
		CPRDay15mVolText:  a.Opening15VolText,
		CPRDay15mVolRatio: a.Opening15VolRatio,
		CPRDay15mVolSurge: a.Opening15VolSurge,
		ExpMoveUp:         a.ExpMoveUp,
		ExpMoveDown:       a.ExpMoveDown,
		ExpMovePct:        a.ExpMovePct,
		ExpMoveOpenUp:     a.ExpMoveOpenUp,
		ExpMoveOpenDn:     a.ExpMoveOpenDn,
		ExpMoveOpenPct:    a.ExpMoveOpenPct,
		DayOpen:           a.DayOpen,
		DistFromHigh:      &dfh,
		Entry:             &entry,
		StopLoss:          &stop,
		Target1:           &t1,
		RiskPct:           &risk,
		RRT1:              &rr,
		ATR:               &atr,
	}
	attachCPRDay(&res, a)
	attachNextDay(&res, a)

	// Fundamentals signals (best-effort, cached 6h, won't block on Yahoo failures)
	if s.Fundamentals != nil {
		if f, err := s.Fundamentals.Get(ticker); err == nil && f != nil {
			labels := make([]string, 0, len(f.Flags))
			for _, fl := range f.Flags {
				labels = append(labels, fl.Label)
			}
			res.Signals = strings.Join(labels, " | ")
			res.Sector = sectorForTicker(ticker, f)
			label, score, reason, fairValue, upsidePct, source := valuationEstimate(f, a.CurrentPrice)
			res.ValuationLabel = label
			res.ValuationScore = score
			res.ValuationReason = reason
			res.ValuationFair = fairValue
			res.ValuationUpside = upsidePct
			res.ValuationSource = source
			// Yahoo returns short_pct_of_float as a fraction (e.g., 0.054 = 5.4%)
			if f.ShortPctOfFloat != nil {
				// round to 1 decimal to avoid float-precision noise like 0.91999999%
				pct := float64(int(*f.ShortPctOfFloat*1000+0.5)) / 10
				res.ShortPct = &pct
			}
		}
	}

	// Options strategy + OTM liquid (best-effort)
	if s.Options != nil && a.CurrentPrice > 0 {
		if strat, err := s.Options.BuildStrategy(ticker, a.CurrentPrice, a.Direction, a.WeeklyFibZone); err == nil && strat != nil {
			res.OptStrategy = strat.Strategy
			res.OptSummary = strat.Summary
			res.OptDebit = &strat.NetDebit
			res.OptProfit = strat.MaxProfit
			res.OptSource = strat.Source
			res.OptQuoteTS = strat.QuoteTS
			res.OptLegs = strat.Legs
			res.OptWidth = strat.Width
			res.OptExpShort = strat.ExpShort
			res.OptExpLong = strat.ExpLong
			res.OptAlt = strat.Alt
		}
		bias := s.Options.AnalyzeBias(ticker, a.CurrentPrice)
		if len(bias.OTMLiquid) > 5 {
			res.OptLiquid = bias.OTMLiquid[:5]
		} else {
			res.OptLiquid = bias.OTMLiquid
		}
	}

	// LRE = Low Risk Entry — combines volume profile, MTF, vol trend, risk %, CPR, confidence
	scoreLRE(&res, a)

	return res
}

// scoreLRE assigns a 0–5 quality score for low-risk entry with a directional bias.
// Mirrors the "exceptional setup" idea but extends to short side and weights signals.
var etfSectors = map[string]string{
	"XLB": "Materials", "VAW": "Materials", "RTM": "Materials",
	"XLC": "Communication", "VOX": "Communication",
	"XLE": "Energy", "VDE": "Energy", "IXE": "Energy",
	"XLF": "Financials", "VFH": "Financials", "KBE": "Banks",
	"XLI": "Industrials", "VIS": "Industrials", "ITA": "Aerospace",
	"XLK": "Technology", "VGT": "Technology", "IYW": "Technology", "QQQ": "Nasdaq 100",
	"XLP": "Staples", "VDC": "Staples", "KXI": "Staples",
	"XLRE": "Real Estate", "VNQ": "Real Estate", "REZ": "Real Estate",
	"XLU": "Utilities", "VPU": "Utilities", "IDU": "Utilities",
	"XLV": "Health Care", "VHT": "Health Care", "IXJ": "Health Care",
	"XLY": "Discretionary", "VCR": "Discretionary", "RXI": "Discretionary",
	"GLD": "Gold", "GLDM": "Gold", "SLV": "Silver",
	"DBMF": "Managed Futures", "CTA": "Managed Futures", "HFGM": "Alternatives",
	"PPI": "Inflation", "RINF": "Inflation", "UUP": "US Dollar",
	"TLT": "Treasury", "JPST": "Short Bond", "BIL": "T-Bills", "SHY": "Treasury",
	"TIP": "TIPS", "VTIP": "Short TIPS", "SCHP": "TIPS",
	"SCHD": "Dividend", "VXUS": "International", "IEMG": "Emerging Markets",
	"VTI": "US Total Market", "SPY": "S&P 500", "USMV": "Low Volatility", "SPLV": "Low Volatility",
}

func sectorForTicker(ticker string, f *fundamentals.Fundamentals) string {
	symbol := strings.ToUpper(ticker)
	if sector, ok := etfSectors[symbol]; ok {
		return sector
	}
	if f != nil {
		if f.Sector != "" {
			return f.Sector
		}
		if f.Industry != "" {
			return f.Industry
		}
	}
	return "Unknown"
}

func valuationEstimate(f *fundamentals.Fundamentals, fallbackPrice float64) (string, int, string, *float64, *float64, string) {
	if f == nil {
		return "", 0, "", nil, nil, ""
	}
	score := 0
	reasons := []string{}
	price := f.Price
	if price <= 0 {
		price = fallbackPrice
	}
	if price > 0 && f.TargetPrice != nil {
		upside := (*f.TargetPrice - price) / price * 100
		reasons = append(reasons, fmt.Sprintf("Analyst target %+0.f%%", upside))
		switch {
		case upside >= 20:
			score += 2
		case upside >= 8:
			score++
		case upside <= -20:
			score -= 2
		case upside <= -8:
			score--
		}
	}
	pe := f.PERatio
	peName := "P/E"
	if f.ForwardPE != nil {
		pe = f.ForwardPE
		peName = "Fwd P/E"
	}
	if pe != nil {
		reasons = append(reasons, fmt.Sprintf("%s %.1f", peName, *pe))
		switch {
		case *pe <= 15:
			score++
		case *pe >= 60:
			score -= 2
		case *pe >= 35:
			score--
		}
	}
	if v := f.EarningsGrowth; v != nil {
		reasons = append(reasons, fmt.Sprintf("Growth %+0.f%%", *v*100))
		if *v >= 0.20 {
			score++
		} else if *v < -0.05 {
			score--
		}
	}
	if v := f.ProfitMargin; v != nil {
		reasons = append(reasons, fmt.Sprintf("Margin %.0f%%", *v*100))
		if *v >= 0.20 {
			score++
		} else if *v < 0 {
			score -= 2
		} else if *v < 0.05 {
			score--
		}
	}
	cashflow := f.FreeCashflow
	if cashflow == nil {
		cashflow = f.OperatingCashflow
	}
	if cashflow != nil {
		reasons = append(reasons, fmt.Sprintf("Cash flow %s", moneyShort(*cashflow)))
		if *cashflow > 0 {
			score++
		} else if *cashflow < 0 {
			score--
		}
	}
	if v := f.DebtToEquity; v != nil {
		reasons = append(reasons, fmt.Sprintf("D/E %.0f%%", *v))
		if *v <= 50 {
			score++
		} else if *v >= 250 {
			score -= 2
		} else if *v >= 150 {
			score--
		}
	}
	if v := f.PriceToBook; v != nil {
		reasons = append(reasons, fmt.Sprintf("P/B %.1f", *v))
		if *v <= 3 {
			score++
		} else if *v >= 10 {
			score--
		}
	}
	label := "Fair Value"
	switch {
	case score >= 4:
		label = "Undervalued"
	case score >= 2:
		label = "Attractive"
	case score >= -1:
		label = "Fair Value"
	case score >= -3:
		label = "Expensive"
	default:
		label = "Overvalued"
	}

	fairValue, upsidePct, source := valuationFairValue(f, price, pe, score)
	if len(reasons) == 0 {
		return label, score, "Insufficient valuation fundamentals", fairValue, upsidePct, source
	}
	if len(reasons) > 7 {
		reasons = reasons[:7]
	}
	return label, score, strings.Join(reasons, " | "), fairValue, upsidePct, source
}

type valuationCandidate struct {
	value  float64
	weight float64
	source string
}

func valuationFairValue(f *fundamentals.Fundamentals, price float64, pe *float64, score int) (*float64, *float64, string) {
	candidates := make([]valuationCandidate, 0, 2)
	if f.TargetPrice != nil && *f.TargetPrice > 0 {
		candidates = append(candidates, valuationCandidate{value: *f.TargetPrice, weight: 0.60, source: "Analyst target"})
	}
	if price > 0 && pe != nil && *pe > 0 {
		impliedEPS := price / *pe
		fairPE := valuationTargetPE(f)
		peValue := impliedEPS * fairPE
		if peValue > 0 {
			candidates = append(candidates, valuationCandidate{value: peValue, weight: 0.40, source: fmt.Sprintf("P/E fair value (%.0fx)", fairPE)})
		}
	}
	if len(candidates) == 0 && price > 0 {
		impliedPct := math.Max(-0.30, math.Min(0.30, float64(score)*0.06))
		candidates = append(candidates, valuationCandidate{value: price * (1 + impliedPct), weight: 1.0, source: "Score-implied fair value"})
	}
	if len(candidates) == 0 {
		return nil, nil, ""
	}
	totalWeight := 0.0
	totalValue := 0.0
	sources := make([]string, 0, len(candidates))
	for _, c := range candidates {
		totalWeight += c.weight
		totalValue += c.value * c.weight
		sources = append(sources, c.source)
	}
	if totalWeight <= 0 {
		return nil, nil, ""
	}
	fair := round2(totalValue / totalWeight)
	var upsidePtr *float64
	if price > 0 {
		upside := round2((fair - price) / price * 100)
		upsidePtr = &upside
	}
	source := strings.Join(sources, " + ")
	return &fair, upsidePtr, source
}

func valuationTargetPE(f *fundamentals.Fundamentals) float64 {
	multiple := 18.0
	if f.EarningsGrowth != nil {
		switch {
		case *f.EarningsGrowth >= 0.20:
			multiple = 28.0
		case *f.EarningsGrowth >= 0.10:
			multiple = 24.0
		case *f.EarningsGrowth >= 0.05:
			multiple = 21.0
		case *f.EarningsGrowth < 0:
			multiple = 12.0
		}
	}
	if f.ProfitMargin != nil {
		marginPct := *f.ProfitMargin * 100
		switch {
		case marginPct >= 25:
			multiple += 2
		case marginPct < 0:
			multiple -= 4
		case marginPct < 5:
			multiple -= 2
		}
	}
	if f.DebtToEquity != nil {
		switch {
		case *f.DebtToEquity <= 50:
			multiple += 1
		case *f.DebtToEquity >= 250:
			multiple -= 3
		case *f.DebtToEquity >= 150:
			multiple -= 1.5
		}
	}
	return math.Max(8, math.Min(35, multiple))
}

func moneyShort(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	switch {
	case v >= 1e12:
		return fmt.Sprintf("%s$%.2fT", sign, v/1e12)
	case v >= 1e9:
		return fmt.Sprintf("%s$%.2fB", sign, v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%s$%.2fM", sign, v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%s$%.1fK", sign, v/1e3)
	default:
		return fmt.Sprintf("%s$%.0f", sign, v)
	}
}

func attachCPRDay(res *models.ScanResult, a *analysis.Result) {
	if res == nil || a == nil || a.ATR <= 0 || a.CurrentPrice <= 0 || a.CPRP <= 0 {
		return
	}
	entry := round2(a.CurrentPrice)
	stop := round2(a.CPRP)
	openContext := fmt.Sprintf("Open inside CPR ($%.2f-$%.2f)", res.CPRBC, res.CPRTC)
	if a.DayOpen > 0 && a.DayOpen > res.CPRTC {
		openContext = fmt.Sprintf("Open above TC ($%.2f)", res.CPRTC)
	} else if a.DayOpen > 0 && a.DayOpen < res.CPRBC {
		openContext = fmt.Sprintf("Open below BC ($%.2f)", res.CPRBC)
	}
	// REVERT_DAY_TRIGGER_V2: additive trigger text; legacy CPR Entry/Stop/T1 stay unchanged.
	res.CPRDayTriggerText = fmt.Sprintf("Break TC $%.2f / BC $%.2f", res.CPRTC, res.CPRBC)
	res.CPRDayInvalidText = "Failed break back inside CPR"
	res.CPRDayTargetText = fmt.Sprintf("Breakout side + $%.2f", a.ATR*0.5)
	volRatio := 0.0
	if a.VolumeProfile != nil {
		volRatio = a.VolumeProfile.VolRatio
	}
	res.CPRDayVolumeText = dayVolumeConfirmText(res.CPRPosition, res.VolTrend, res.VolSurge, volRatio)
	res.CPRDayRef = openContext + "; wait for TC/BC break."
	var t1 float64
	switch res.CPRPosition {
	case "Above":
		if res.CPRType == "Narrow" {
			res.CPRDayResult = "Trend up"
		} else if res.CPRType == "Wide" {
			res.CPRDayResult = "Above CPR; pullback risk"
		} else {
			res.CPRDayResult = "Bullish above TC"
		}
		t1 = a.DayOpen + a.ATR
		if t1 <= a.CurrentPrice {
			t1 = a.CurrentPrice + a.ATR*0.5
		}
		if a.DayOpen >= res.CPRTC {
			res.CPRDayTriggerText = fmt.Sprintf("Hold > TC $%.2f", res.CPRTC)
		} else {
			res.CPRDayTriggerText = fmt.Sprintf("Reclaim TC $%.2f", res.CPRTC)
		}
		res.CPRDayInvalidText = fmt.Sprintf("Back < P $%.2f", res.CPRP)
		res.CPRDayRef = openContext + "; long only while holding above TC/P."
	case "Below":
		if res.CPRType == "Narrow" {
			res.CPRDayResult = "Trend down"
		} else if res.CPRType == "Wide" {
			res.CPRDayResult = "Below CPR; bounce risk"
		} else {
			res.CPRDayResult = "Bearish below BC"
		}
		t1 = a.DayOpen - a.ATR
		if t1 >= a.CurrentPrice {
			t1 = a.CurrentPrice - a.ATR*0.5
		}
		if a.DayOpen <= res.CPRBC {
			res.CPRDayTriggerText = fmt.Sprintf("Hold < BC $%.2f", res.CPRBC)
		} else {
			res.CPRDayTriggerText = fmt.Sprintf("Lose BC $%.2f", res.CPRBC)
		}
		res.CPRDayInvalidText = fmt.Sprintf("Back > P $%.2f", res.CPRP)
		res.CPRDayRef = openContext + "; short only while staying below BC/P."
	default:
		res.CPRDayResult = "Inside CPR; wait"
		return
	}
	t1 = round2(t1)
	res.CPRDayTargetText = fmt.Sprintf("$%.2f", t1)
	res.CPRDayEntry = &entry
	res.CPRDayStop = &stop
	res.CPRDayT1 = &t1
}

func dayVolumeConfirmText(position, trend string, surge bool, ratio float64) string {
	trend = strings.ToUpper(strings.TrimSpace(trend))
	ratioText := ""
	if ratio > 0 {
		ratioText = fmt.Sprintf(" (%.1fx avg)", ratio)
	}
	switch position {
	case "Above":
		switch {
		case trend == "ACCUMULATING" && surge:
			return "Confirmed: accumulating + volume surge" + ratioText
		case trend == "ACCUMULATING":
			return "Supportive: accumulating volume" + ratioText
		case trend == "DISTRIBUTING":
			return "Caution: distribution against long" + ratioText
		case surge:
			return "Watch: volume surge; confirm direction" + ratioText
		default:
			return "Needs volume: no clear accumulation"
		}
	case "Below":
		switch {
		case trend == "DISTRIBUTING" && surge:
			return "Confirmed: distributing + volume surge" + ratioText
		case trend == "DISTRIBUTING":
			return "Supportive: distributing volume" + ratioText
		case trend == "ACCUMULATING":
			return "Caution: accumulation against short" + ratioText
		case surge:
			return "Watch: volume surge; confirm direction" + ratioText
		default:
			return "Needs volume: no clear distribution"
		}
	default:
		if surge {
			return "Watch: volume surge; wait for CPR break" + ratioText
		}
		return "Needs volume on CPR break"
	}
}

func attachNextDay(res *models.ScanResult, a *analysis.Result) {
	if res == nil || a == nil || len(a.Bars) == 0 || a.CurrentPrice <= 0 {
		return
	}
	cur := a.Bars[len(a.Bars)-1]
	curRange := cur.High - cur.Low
	if curRange < 0 {
		curRange = 0
	}
	closePos := 0.5
	if curRange > 0 {
		closePos = (cur.Close - cur.Low) / curRange
	}
	atr := a.ATR
	if atr <= 0 {
		atr = curRange
	}
	rangeATR := 0.0
	if atr > 0 {
		rangeATR = curRange / atr
	}
	atrPct := 0.0
	if atr > 0 && cur.Close > 0 {
		atrPct = atr / cur.Close * 100
	}
	p := (cur.High + cur.Low + cur.Close) / 3
	bc := (cur.High + cur.Low) / 2
	tc := 2*p - bc
	top, bot := tc, bc
	if bc > tc {
		top, bot = bc, tc
	}
	widthPct := 0.0
	if p > 0 {
		widthPct = (top - bot) / p * 100
	}
	cprType := "Normal"
	if widthPct < 0.15 {
		cprType = "Narrow"
	} else if widthPct > 0.5 {
		cprType = "Wide"
	}
	nextDate := nextTradingDay(cur.Time)
	upTrigger := round2(cur.High)
	downTrigger := round2(cur.Low)
	upTarget := round2(cur.High + atr*0.5)
	downTarget := round2(cur.Low - atr*0.5)
	target := round2(p)
	setup := "Close mid-range"
	action := "inside open is balanced; wait for prior high/low break."
	outcome := "Neutral"
	bias := "Neutral Watch"
	switch {
	case rangeATR >= 1.25 && closePos >= 0.70:
		outcome = "Trending Bullish"
		bias = "Extended Bullish"
		target = upTarget
		setup = fmt.Sprintf("Close near high after %.1f ATR day", rangeATR)
		action = "needs open/hold above high for continuation; losing pivot favors pullback."
	case rangeATR >= 1.25 && closePos <= 0.30:
		outcome = "Trending Bearish"
		bias = "Extended Bearish"
		target = downTarget
		setup = fmt.Sprintf("Close near low after %.1f ATR day", rangeATR)
		action = "needs open/hold below low for continuation; reclaiming pivot favors bounce."
	case closePos >= 0.65 && cur.Close >= p:
		outcome = "Bullish"
		bias = "Bullish Watch"
		target = upTarget
		setup = fmt.Sprintf("Close strong (%.0f%% of range)", closePos*100)
		action = "open above high favors continuation; inside open uses pivot as support test."
	case closePos <= 0.35 && cur.Close <= p:
		outcome = "Bearish"
		bias = "Bearish Watch"
		target = downTarget
		setup = fmt.Sprintf("Close weak (%.0f%% of range)", closePos*100)
		action = "open below low favors breakdown; inside open uses pivot as resistance test."
	case cprType == "Wide":
		outcome = "Range"
		bias = "Range Watch"
		target = round2(p)
		setup = "Wide next CPR"
		action = "expect chop or mean reversion unless price clears prior high/low."
	}
	summary := fmt.Sprintf(
		"%s: %s. %s. ATR $%.2f (%.1f%%). Open > H $%.2f points to $%.2f; open < L $%.2f points to $%.2f; %s",
		nextDate.Format("2006-01-02"), outcome, setup, atr, atrPct, upTrigger, upTarget, downTrigger, downTarget, action,
	)
	ref := fmt.Sprintf("P %.2f / ATR %.2f", p, atr)
	atrRounded := round2(atr)
	atrPctRounded := round2(atrPct)
	pivot := round2(p)
	prevHigh := upTrigger
	prevLow := downTrigger
	res.NextDayDate = nextDate.Format("2006-01-02")
	res.NextDayOutcome = outcome
	res.NextDayBias = bias
	res.NextDaySummary = summary
	res.NextDayPrediction = summary
	res.NextDayRef = ref
	res.NextDayTarget = &target
	res.NextDayATR = &atrRounded
	res.NextDayATRPct = &atrPctRounded
	res.NextDayTriggerUp = &upTrigger
	res.NextDayTriggerDown = &downTrigger
	res.NextDayPivot = &pivot
	res.PrevDayHigh = &prevHigh
	res.PrevDayLow = &prevLow
}

func nextTradingDay(t time.Time) time.Time {
	d := t.AddDate(0, 0, 1)
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func scoreLRE(res *models.ScanResult, a *analysis.Result) {
	if a == nil || res.Price <= 0 || res.ATR == nil || *res.ATR <= 0 {
		return
	}
	atr := *res.ATR
	price := res.Price

	// Determine candidate direction from verdict + biases
	bullishish := res.Verdict == "BULLISH" || res.Verdict == "LEAN BULLISH"
	bearishish := res.Verdict == "BEARISH" || res.Verdict == "LEAN BEARISH"
	if !bullishish && !bearishish {
		return
	}

	score := 0
	reasons := make([]string, 0, 5)

	// 1) Price position vs Volume Profile VAH/VAL — best long entries near VAL, short near VAH
	if a.VolumeProfile != nil {
		vp := a.VolumeProfile
		nearVAL := price <= vp.VAL+atr*0.5 && price >= vp.VAL-atr*0.5
		nearVAH := price <= vp.VAH+atr*0.5 && price >= vp.VAH-atr*0.5
		if bullishish && nearVAL {
			score++
			reasons = append(reasons, "near VAL")
		} else if bearishish && nearVAH {
			score++
			reasons = append(reasons, "near VAH")
		}
		// Also reward inside-VA with vol bias matching direction
		if bullishish && vp.VolBias == "BULLISH" {
			score++
			reasons = append(reasons, "VA bias bullish")
		} else if bearishish && vp.VolBias == "BEARISH" {
			score++
			reasons = append(reasons, "VA bias bearish")
		}
	}

	// 2) MTF alignment (rank 1 = all 3 timeframes confirmed)
	if res.MTFRank == 1 {
		score++
		reasons = append(reasons, "MTF aligned")
	}

	// 3) Vol trend matches direction
	if bullishish && res.VolTrend == "ACCUMULATING" {
		score++
		reasons = append(reasons, "accumulating")
	} else if bearishish && res.VolTrend == "DISTRIBUTING" {
		score++
		reasons = append(reasons, "distributing")
	}

	// 4) Tight risk (entry-to-stop < 3%)
	if res.RiskPct != nil && *res.RiskPct > 0 && *res.RiskPct < 3 {
		score++
		reasons = append(reasons, "tight stop <3%")
	}

	// 5) Confidence boost
	if res.Confidence == "HIGH" {
		score++
		reasons = append(reasons, "high confidence")
	}

	// Cap at 5
	if score > 5 {
		score = 5
	}
	if score == 0 {
		return
	}

	res.LREScore = score
	res.LREReason = strings.Join(reasons, " · ")
	if bullishish {
		res.LREDirection = "LONG"
	} else {
		res.LREDirection = "SHORT"
	}
	switch {
	case score >= 5:
		res.LRELabel = "🎯 PRIME"
	case score >= 4:
		res.LRELabel = "✅ STRONG"
	case score >= 3:
		res.LRELabel = "👍 GOOD"
	case score >= 2:
		res.LRELabel = "•• Decent"
	default:
		res.LRELabel = "Weak"
	}

	// Suggest LRE entry zone: VAL for long (or current if already at/below VAL),
	// VAH for short (or current if already at/above VAH). Stop = entry ∓ 1× ATR.
	var entry, stop float64
	if bullishish {
		entry = price
		if a.VolumeProfile != nil && a.VolumeProfile.VAL > 0 && a.VolumeProfile.VAL < price {
			entry = a.VolumeProfile.VAL
		}
		stop = entry - atr
	} else {
		entry = price
		if a.VolumeProfile != nil && a.VolumeProfile.VAH > price {
			entry = a.VolumeProfile.VAH
		}
		stop = entry + atr
	}
	res.LREEntry = round2(entry)
	res.LREStop = round2(stop)
	if entry > 0 {
		risk := math.Abs(entry-stop) / entry * 100
		res.LRERisk = round2(risk)
	}

	// LRE status — summarizes "where is price right now relative to the setup"
	//   ACTIVE       — price within 5% of entry (actionable now)
	//   DISCOUNT     — price moved past entry in your favor, still above (long) / below (short) the stop
	//   INVALIDATED  — price has crossed the stop; the setup broke down
	//   STALE        — price has run far past entry (long: way above, short: way below) — wait for pullback
	if entry > 0 {
		near := func(p, e float64) bool {
			return math.Abs(p-e)/e <= 0.05
		}
		switch {
		case bullishish:
			switch {
			case near(price, entry):
				res.LREStatus = "ACTIVE"
			case price < stop:
				res.LREStatus = "INVALIDATED"
			case price < entry:
				res.LREStatus = "DISCOUNT"
			default:
				res.LREStatus = "STALE"
			}
		case bearishish:
			switch {
			case near(price, entry):
				res.LREStatus = "ACTIVE"
			case price > stop:
				res.LREStatus = "INVALIDATED"
			case price > entry:
				res.LREStatus = "DISCOUNT"
			default:
				res.LREStatus = "STALE"
			}
		}
	}
	res.LRETakeaway = lreTakeaway(res.Verdict, res.LRELabel, res.LREStatus, res.LREDirection)
}

func lreTakeaway(verdict, label, status, direction string) string {
	isLean := verdict == "LEAN BULLISH" || verdict == "LEAN BEARISH"
	isWeak := strings.Contains(label, "Weak")
	isDecent := strings.Contains(label, "Decent")
	switch status {
	case "ACTIVE":
		if isLean || isWeak || isDecent {
			return "Active setup; wait confirmation"
		}
		if direction == "SHORT" {
			return "Active short setup; extended"
		}
		return "Active long setup; extended"
	case "DISCOUNT":
		if direction == "LONG" {
			return "Discount to entry; watch bounce"
		}
		return "Discount to entry; watch fade"
	case "STALE":
		if direction == "LONG" {
			return "Stale; wait pullback"
		}
		return "Stale; wait bounce"
	case "INVALIDATED":
		return "Invalidated"
	default:
		return ""
	}
}

func round2(v float64) float64 {
	if v == 0 {
		return 0
	}
	return float64(int(v*100+0.5)) / 100
}

func mtfRank(w, d, h string) int {
	score := 0
	for _, b := range []string{w, d, h} {
		if b == "BULLISH" || b == "BEARISH" {
			score++
		}
	}
	switch score {
	case 3:
		return 1
	case 2:
		return 2
	case 1:
		return 3
	default:
		return 4
	}
}

func mtfSignalAction(w, d, h string) (signal, action, key string) {
	key = string(w[0]) + "/" + string(d[0]) + "/" + string(h[0])
	bull := count(w, d, h, "BULLISH")
	bear := count(w, d, h, "BEARISH")
	switch {
	case bull == 3:
		return "STRONG BUY", "ENTER LONG", key
	case bear == 3:
		return "STRONG SELL", "ENTER SHORT", key
	case bull == 2:
		return "BUY", "WATCH LONG", key
	case bear == 2:
		return "SELL", "WATCH SHORT", key
	default:
		return "NEUTRAL", "WAIT", key
	}
}

func count(a, b, c, target string) int {
	n := 0
	for _, x := range []string{a, b, c} {
		if x == target {
			n++
		}
	}
	return n
}

func grade(score int, confidence string) string {
	switch {
	case score >= 8 && confidence == "HIGH":
		return "S"
	case score >= 6:
		return "A"
	case score >= 3:
		return "B"
	case score >= 1:
		return "B-"
	case score >= -2:
		return "C"
	default:
		return "D"
	}
}

func truncateErr(s string) string {
	if len(s) > 120 {
		return s[:120]
	}
	return s
}
