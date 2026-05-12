package analysis

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/aashorefp-droid/stockpulsego/internal/marketdata"
	"github.com/aashorefp-droid/stockpulsego/internal/models"
	"github.com/aashorefp-droid/stockpulsego/internal/ta"
)

type Service struct {
	MD *marketdata.Client
}

func NewService(md *marketdata.Client) *Service {
	return &Service{MD: md}
}

// Result is a compact analysis output. The Python version returns ~40 fields;
// this is a working subset that the scanner uses.
type Result struct {
	Ticker             string      `json:"ticker"`
	Bars               models.Bars `json:"-"`
	CurrentPrice       float64     `json:"current_price"`
	Verdict            string      `json:"verdict"`
	Confidence         string      `json:"confidence"`
	Score              int         `json:"score"`
	Direction          string      `json:"direction"`
	WeeklyBias         string      `json:"weekly_bias"`
	DailyBias          string      `json:"daily_bias"`
	H4Bias             string      `json:"h4_bias"`
	LongTermSpring     bool        `json:"long_term_spring,omitempty"`
	LongTermSpringText string      `json:"long_term_spring_text,omitempty"`
	SwingSpring        bool        `json:"swing_spring,omitempty"`
	SwingSpringText    string      `json:"swing_spring_text,omitempty"`
	DaySpring          bool        `json:"day_spring,omitempty"`
	DaySpringText      string      `json:"day_spring_text,omitempty"`
	VolTrend           string      `json:"vol_trend"`
	VolSurge           bool        `json:"vol_surge"`
	Opening15VolText   string      `json:"cpr_day_15m_volume_text,omitempty"`
	Opening15VolRatio  float64     `json:"cpr_day_15m_volume_ratio,omitempty"`
	Opening15VolSurge  bool        `json:"cpr_day_15m_volume_surge,omitempty"`
	// Zones (ported from stock_pulse.py earnzone/weekzone)
	WeekHi         float64 `json:"week_hi"`
	WeekLo         float64 `json:"week_lo"`
	WkPosPct       float64 `json:"wk_pos_pct"`
	WeeklyZoneCls  string  `json:"weekly_zone"` // HIGH / MID / LOW
	EarnHi         float64 `json:"earn_hi"`
	EarnLo         float64 `json:"earn_lo"`
	EarnPosPct     float64 `json:"earn_pos_pct"`
	EarnZone       string  `json:"earn_zone"` // HIGH / MID / LOW
	NearFibName    string  `json:"near_fib_name"`
	NearFibPrice   float64 `json:"near_fib_price"`
	FibCompression bool    `json:"fib_compression"`
	// CPR (Central Pivot Range) — ported from stock_pulse.py
	CPRType        string             `json:"cpr_type,omitempty"`
	CPRTC          float64            `json:"cpr_tc,omitempty"`
	CPRBC          float64            `json:"cpr_bc,omitempty"`
	CPRP           float64            `json:"cpr_p,omitempty"`
	CPRPosition    string             `json:"cpr_position,omitempty"`
	CPRInterp      string             `json:"cpr_interpretation,omitempty"`
	ExpMoveUp      float64            `json:"exp_move_up,omitempty"` // from last close
	ExpMoveDown    float64            `json:"exp_move_down,omitempty"`
	ExpMovePct     float64            `json:"exp_move_pct,omitempty"`
	ExpMoveOpenUp  float64            `json:"exp_move_open_up,omitempty"` // from day open
	ExpMoveOpenDn  float64            `json:"exp_move_open_dn,omitempty"`
	ExpMoveOpenPct float64            `json:"exp_move_open_pct,omitempty"`
	DayOpen        float64            `json:"day_open,omitempty"`
	VolumeProfile  *ta.VolumeProfile  `json:"volume_profile,omitempty"`
	NearestFib     string             `json:"nearest_fib"`
	FibLevels      map[string]float64 `json:"fib_levels"`
	Support        []float64          `json:"support"`
	Resistance     []float64          `json:"resistance"`
	RSI4H          float64            `json:"rsi_4h"`
	WeeklyFibZone  string             `json:"weekly_fib"`
	BreakoutScore  int                `json:"breakout_score"`
	DistFromHigh   float64            `json:"dist_from_high"`
	IsUptrend      bool               `json:"is_uptrend"`
	IsDowntrend    bool               `json:"is_downtrend"`
	MA10BelowMA30  bool               `json:"ma10_below_ma30"`
	PricePos52W    float64            `json:"price_position"`
	Entry          float64            `json:"entry"`
	StopLoss       float64            `json:"stop_loss"`
	Target1        float64            `json:"target1"`
	Target2        float64            `json:"target2"`
	RiskPct        float64            `json:"risk_pct"`
	RRT1           float64            `json:"rr_t1"`
	RRT2           float64            `json:"rr_t2"`
	ATR            float64            `json:"atr"`
}

// Analyze fetches daily bars and runs the scoring pipeline.
func (s *Service) Analyze(ticker string, asOf *time.Time) (*Result, error) {
	end := time.Now().UTC()
	if asOf != nil {
		end = *asOf
	}
	// roll back to Friday on weekends
	switch end.Weekday() {
	case time.Saturday:
		end = end.AddDate(0, 0, -1)
	case time.Sunday:
		end = end.AddDate(0, 0, -2)
	}
	start := end.AddDate(0, 0, -400)

	bars, err := s.MD.GetDailyBars(ticker, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	if len(bars) < 30 {
		return &Result{Ticker: ticker, Verdict: "INSUFFICIENT DATA"}, nil
	}

	closes := bars.Closes()
	highs := bars.Highs()
	lows := bars.Lows()
	volumes := bars.Volumes()
	last := closes[len(closes)-1]

	// Volume trend: weight last 5 days' volume by direction; compare to 20d average.
	volTrend, volSurge := classifyVolume(closes, volumes)

	// Indicators
	ma10 := ta.SMA(closes, 10)
	ma30 := ta.SMA(closes, 30)
	rsi14 := ta.RSI(closes, 14)
	atr14 := ta.ATR(highs, lows, closes, 14)

	// Multi-timeframe biases (simplified — using daily bars for all)
	weeklyBias := classifyBias(closes, 50, 20)
	dailyBias := classifyBias(closes, 20, 5)
	h4Bias := classifyBias(closes, 10, 3)

	// Fibonacci on last 60-day swing — used for "earnings window" zone (no earnings cal yet)
	swingHigh, swingLow := ta.SwingHighLow(highs, lows, 60)
	fibs := ta.FibLevels(swingHigh, swingLow)
	nearest := ta.NearestFib(fibs, last)
	weeklyFib := ta.FibZone(fibs, last)

	// Earn window zone (HIGH/MID/LOW) — % position within 60-day swing range
	earnRange := swingHigh - swingLow
	earnPos := 50.0
	if earnRange > 0 {
		earnPos = (last - swingLow) / earnRange * 100
	}
	earnZoneCls := classifyZonePct(earnPos)

	// Week window zone — last 5 trading days hi/lo
	weekHi, weekLo := ta.SwingHighLow(highs, lows, 5)
	weekRange := weekHi - weekLo
	wkPos := 50.0
	if weekRange > 0 {
		wkPos = (last - weekLo) / weekRange * 100
	}
	weeklyZoneCls := classifyZonePct(wkPos)
	weeklySpring, weeklySpringText := springAction(aggregateWeeks(bars), 20, "Weekly")
	dailySpring, dailySpringText := springAction(bars, 20, "Daily")

	// Nearest fib name + price
	var nearFibPrice float64
	if v, ok := fibs[nearest]; ok {
		nearFibPrice = v
	}

	// Fib Compression: 3+ levels clustered within 3% of swing range
	fibCompression := detectFibCompression(fibs, earnRange)

	// CPR (Central Pivot Range) — needs prior day's bar
	var cprResult *ta.CPR
	if len(bars) >= 2 {
		prev := bars[len(bars)-2]
		cprResult = ta.ComputeCPR(prev.High, prev.Low, prev.Close, last)
	}

	// Support/Resistance
	support, resistance := ta.SupportResistance(highs, lows, 5, 3)

	// 52-week stats
	high52, low52 := ta.SwingHighLow(highs, lows, 252)
	pricePos := 0.0
	if high52 > low52 {
		pricePos = (last - low52) / (high52 - low52) * 100
	}
	distFromHigh := 0.0
	if high52 > 0 {
		distFromHigh = (high52 - last) / high52 * 100
	}

	// Trend flags
	ma10Last, _ := ta.LastValid(ma10)
	ma30Last, _ := ta.LastValid(ma30)
	ma10BelowMA30 := ma10Last < ma30Last
	isUptrend := !ma10BelowMA30 && pricePos > 50
	isDowntrend := ma10BelowMA30 && pricePos < 50

	// Breakout score: count of (price > 20d high), (vol surge), (RSI > 50)
	breakoutScore := 0
	if len(closes) >= 20 {
		max20, _ := maxN(highs, 20)
		if last > max20*0.99 {
			breakoutScore++
		}
	}
	rsiLast, _ := ta.LastValid(rsi14)
	if rsiLast > 50 {
		breakoutScore++
	}
	if isUptrend {
		breakoutScore++
	}

	// Score the setup (mirrors Python full_score_pipeline at high level)
	score := scoreSetup(weeklyBias, dailyBias, h4Bias, isUptrend, breakoutScore, rsiLast)
	verdict, confidence := classifyVerdict(score)
	direction := "LONG"
	if verdict == "BEARISH" || verdict == "LEAN BEARISH" {
		direction = "SHORT"
	}

	// Trade levels
	atrLast, _ := ta.LastValid(atr14)
	entry := last
	var stop, t1, t2 float64
	if direction == "LONG" {
		stop = entry - 1.5*atrLast
		t1 = entry + 2.0*atrLast
		t2 = entry + 3.5*atrLast
	} else {
		stop = entry + 1.5*atrLast
		t1 = entry - 2.0*atrLast
		t2 = entry - 3.5*atrLast
	}
	risk := math.Abs(entry-stop) / entry * 100
	rr1 := math.Abs(t1-entry) / math.Abs(entry-stop)
	rr2 := math.Abs(t2-entry) / math.Abs(entry-stop)

	result := &Result{
		Ticker:             ticker,
		Bars:               bars,
		CurrentPrice:       round2(last),
		Verdict:            verdict,
		Confidence:         confidence,
		Score:              score,
		Direction:          direction,
		WeeklyBias:         weeklyBias,
		DailyBias:          dailyBias,
		H4Bias:             h4Bias,
		LongTermSpring:     weeklySpring,
		LongTermSpringText: weeklySpringText,
		SwingSpring:        dailySpring,
		SwingSpringText:    dailySpringText,
		VolTrend:           volTrend,
		VolSurge:           volSurge,
		WeekHi:             round2(weekHi),
		WeekLo:             round2(weekLo),
		WkPosPct:           round2(wkPos),
		WeeklyZoneCls:      weeklyZoneCls,
		EarnHi:             round2(swingHigh),
		EarnLo:             round2(swingLow),
		EarnPosPct:         round2(earnPos),
		EarnZone:           earnZoneCls,
		NearFibName:        nearest,
		NearFibPrice:       round2(nearFibPrice),
		FibCompression:     fibCompression,
		NearestFib:         nearest,
		FibLevels:          fibs,
		Support:            support,
		Resistance:         resistance,
		RSI4H:              round2(rsiLast),
		WeeklyFibZone:      weeklyFib,
		BreakoutScore:      breakoutScore,
		DistFromHigh:       round2(distFromHigh),
		IsUptrend:          isUptrend,
		IsDowntrend:        isDowntrend,
		MA10BelowMA30:      ma10BelowMA30,
		PricePos52W:        round2(pricePos),
		Entry:              round2(entry),
		StopLoss:           round2(stop),
		Target1:            round2(t1),
		Target2:            round2(t2),
		RiskPct:            round2(risk),
		RRT1:               round2(rr1),
		RRT2:               round2(rr2),
		ATR:                round2(atrLast),
	}
	if cprResult != nil {
		result.CPRType = cprResult.Type
		result.CPRTC = cprResult.TC
		result.CPRBC = cprResult.BC
		result.CPRP = cprResult.P
		result.CPRPosition = cprResult.Position
		result.CPRInterp = cprResult.Interp
	}
	// Expected move based on ATR — compute both from-close (where could price go now)
	// and from-open (today's expected range from market open).
	if atrLast > 0 && last > 0 {
		result.ExpMoveUp = round2(last + atrLast)
		result.ExpMoveDown = round2(last - atrLast)
		result.ExpMovePct = round2(atrLast / last * 100)
	}
	if len(bars) > 0 {
		dayOpen := bars[len(bars)-1].Open
		if atrLast > 0 && dayOpen > 0 {
			result.DayOpen = round2(dayOpen)
			result.ExpMoveOpenUp = round2(dayOpen + atrLast)
			result.ExpMoveOpenDn = round2(dayOpen - atrLast)
			result.ExpMoveOpenPct = round2(atrLast / dayOpen * 100)
		}
	}
	// Volume profile (last 60 daily bars, 50 buckets)
	result.VolumeProfile = ta.ComputeVolumeProfile(highs, lows, closes, volumes, 60, 50)
	targetDate := end.Format("2006-01-02")
	if asOf == nil {
		if loc, err := time.LoadLocation("America/New_York"); err == nil {
			targetDate = end.In(loc).Format("2006-01-02")
		}
	}
	if intraday, err := s.MD.GetFiveMinuteBars(ticker, end.AddDate(0, 0, -35).Format("2006-01-02"), end.Format("2006-01-02")); err == nil {
		result.Opening15VolText, result.Opening15VolRatio, result.Opening15VolSurge = opening15VolumeSignal(intraday, targetDate)
		result.DaySpring, result.DaySpringText = springAction(aggregateFourHour(intraday), 12, "4H")
	}
	if result.Opening15VolText == "" {
		result.Opening15VolText = "15m pending: waiting for opening bars"
	}

	return result, nil
}

func aggregateWeeks(bars models.Bars) models.Bars {
	if len(bars) == 0 {
		return nil
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.UTC
	}
	var out models.Bars
	var cur models.Bar
	curKey := ""
	for _, bar := range bars {
		t := bar.Time.In(loc)
		year, week := t.ISOWeek()
		key := fmt.Sprintf("%04d-%02d", year, week)
		if curKey == "" {
			curKey = key
			cur = bar
			continue
		}
		if key != curKey {
			out = append(out, cur)
			curKey = key
			cur = bar
			continue
		}
		cur = mergeBar(cur, bar)
	}
	out = append(out, cur)
	return out
}

func aggregateFourHour(bars models.Bars) models.Bars {
	if len(bars) == 0 {
		return nil
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.UTC
	}
	var out models.Bars
	var cur models.Bar
	curKey := ""
	for _, bar := range bars {
		t := bar.Time.In(loc)
		mins := t.Hour()*60 + t.Minute()
		if mins < 9*60+30 || mins >= 16*60 {
			continue
		}
		bucket := (mins - (9*60 + 30)) / 240
		key := fmt.Sprintf("%s-%d", t.Format("2006-01-02"), bucket)
		if curKey == "" {
			curKey = key
			cur = bar
			continue
		}
		if key != curKey {
			out = append(out, cur)
			curKey = key
			cur = bar
			continue
		}
		cur = mergeBar(cur, bar)
	}
	if curKey != "" {
		out = append(out, cur)
	}
	return out
}

func mergeBar(a, b models.Bar) models.Bar {
	if b.High > a.High {
		a.High = b.High
	}
	if b.Low < a.Low {
		a.Low = b.Low
	}
	a.Close = b.Close
	a.Volume += b.Volume
	a.Time = b.Time
	return a
}

func springAction(bars models.Bars, lookback int, timeframe string) (bool, string) {
	if lookback < 5 {
		lookback = 5
	}
	if len(bars) < lookback+2 {
		return false, ""
	}
	last := bars[len(bars)-1]
	if last.Close <= 0 || last.High <= last.Low {
		return false, ""
	}
	start := len(bars) - 1 - lookback
	if start < 0 {
		start = 0
	}
	support := math.MaxFloat64
	volSum := 0.0
	volCount := 0
	for i := start; i < len(bars)-1; i++ {
		if bars[i].Low > 0 && bars[i].Low < support {
			support = bars[i].Low
		}
		if bars[i].Volume > 0 {
			volSum += float64(bars[i].Volume)
			volCount++
		}
	}
	if support == math.MaxFloat64 || support <= 0 {
		return false, ""
	}
	closePos := (last.Close - last.Low) / (last.High - last.Low)
	testedSupport := last.Low <= support*1.01
	reclaimedSupport := last.Close > support
	greenClose := last.Close >= last.Open
	if !(testedSupport && reclaimedSupport && greenClose && closePos >= 0.55) {
		return false, ""
	}
	volRatio := 0.0
	if volCount > 0 && volSum > 0 {
		volRatio = float64(last.Volume) / (volSum / float64(volCount))
		if volRatio < 0.65 {
			return false, ""
		}
	}
	text := fmt.Sprintf("%s spring: tested/reclaimed $%.2f support; close %.0f%% of range", timeframe, support, closePos*100)
	if volRatio > 0 {
		text += fmt.Sprintf("; volume %.1fx avg", volRatio)
	}
	return true, text
}

// classifyZonePct returns HIGH/MID/LOW based on % position within a swing range.
// Mirrors the earnzone/weekzone logic from stock_pulse.py.
func classifyZonePct(pct float64) string {
	switch {
	case pct >= 70:
		return "HIGH"
	case pct <= 30:
		return "LOW"
	default:
		return "MID"
	}
}

// detectFibCompression returns true if 3+ Fib levels cluster within 3% of swing range.
func detectFibCompression(fibs map[string]float64, swingRange float64) bool {
	if swingRange <= 0 || len(fibs) < 3 {
		return false
	}
	threshold := swingRange * 0.03
	values := make([]float64, 0, len(fibs))
	for _, v := range fibs {
		values = append(values, v)
	}
	// sort ascending
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
	for i := 0; i < len(values)-2; i++ {
		if values[i+2]-values[i] <= threshold {
			return true
		}
	}
	return false
}

// classifyVolume returns ACCUMULATING / DISTRIBUTING / NEUTRAL and a vol-surge flag.
// ACCUMULATING: last 5d up-volume > down-volume by ≥30% AND average vol > 20d average.
// DISTRIBUTING: opposite. NEUTRAL otherwise.
// VolSurge: latest day volume > 1.5× 20d average.
func classifyVolume(closes, volumes []float64) (trend string, surge bool) {
	n := len(closes)
	if n < 21 {
		return "NEUTRAL", false
	}

	// 20-day average volume (excluding most recent day)
	sum20 := 0.0
	for i := n - 21; i < n-1; i++ {
		sum20 += volumes[i]
	}
	avg20 := sum20 / 20

	if avg20 > 0 && volumes[n-1] > 1.5*avg20 {
		surge = true
	}

	// Last 5 days: up vs down weighted by volume
	upVol, downVol := 0.0, 0.0
	for i := n - 5; i < n; i++ {
		if i <= 0 {
			continue
		}
		if closes[i] > closes[i-1] {
			upVol += volumes[i]
		} else if closes[i] < closes[i-1] {
			downVol += volumes[i]
		}
	}
	avg5 := (upVol + downVol) / 5

	if avg20 > 0 && avg5 < 0.8*avg20 {
		// 5d activity is below avg — no clear trend
		return "NEUTRAL", surge
	}
	switch {
	case upVol > downVol*1.3:
		return "ACCUMULATING", surge
	case downVol > upVol*1.3:
		return "DISTRIBUTING", surge
	default:
		return "NEUTRAL", surge
	}
}

// classifyBias returns BULLISH/BEARISH/NEUTRAL based on slope of MA over `slope` lookback,
// and price position vs MA over `period`.
func classifyBias(closes []float64, period, slope int) string {
	if len(closes) < period+slope {
		return "NEUTRAL"
	}
	ma := ta.EMA(closes, period)
	last := closes[len(closes)-1]
	maNow, ok := ta.LastValid(ma)
	if !ok {
		return "NEUTRAL"
	}
	maPast := ma[len(ma)-1-slope]
	if math.IsNaN(maPast) {
		return "NEUTRAL"
	}
	above := last > maNow
	rising := maNow > maPast
	switch {
	case above && rising:
		return "BULLISH"
	case !above && !rising:
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

// scoreSetup combines biases + breakout + RSI into a -10..+10 score.
func scoreSetup(w, d, h string, isUp bool, breakout int, rsi float64) int {
	s := 0
	for _, b := range []string{w, d, h} {
		switch b {
		case "BULLISH":
			s += 2
		case "BEARISH":
			s -= 2
		}
	}
	if isUp {
		s++
	}
	s += breakout
	switch {
	case rsi > 70:
		s -= 1 // overbought
	case rsi > 55:
		s += 1
	case rsi < 30:
		s += 1 // oversold bounce setup
	case rsi < 45:
		s -= 1
	}
	return s
}

func classifyVerdict(score int) (verdict, confidence string) {
	switch {
	case score >= 6:
		return "BULLISH", "HIGH"
	case score >= 3:
		return "LEAN BULLISH", "MEDIUM"
	case score <= -6:
		return "BEARISH", "HIGH"
	case score <= -3:
		return "LEAN BEARISH", "MEDIUM"
	default:
		return "NEUTRAL", "LOW"
	}
}

func maxN(values []float64, n int) (max float64, ok bool) {
	if len(values) < n {
		return 0, false
	}
	max = values[len(values)-n]
	for i := len(values) - n + 1; i < len(values); i++ {
		if values[i] > max {
			max = values[i]
		}
	}
	return max, true
}

func round2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return float64(int(v*100+0.5)) / 100
}

type openingVolumeDay struct {
	Volume float64
	Count  int
}

func opening15VolumeSignal(bars models.Bars, targetKey string) (string, float64, bool) {
	if len(bars) == 0 {
		return "15m pending: waiting for opening bars", 0, false
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.UTC
	}
	byDate := map[string]*openingVolumeDay{}
	for _, bar := range bars {
		t := bar.Time.In(loc)
		mins := t.Hour()*60 + t.Minute()
		if mins < 9*60+30 || mins >= 9*60+45 {
			continue
		}
		key := t.Format("2006-01-02")
		day := byDate[key]
		if day == nil {
			day = &openingVolumeDay{}
			byDate[key] = day
		}
		day.Volume += float64(bar.Volume)
		day.Count++
	}
	targetDay := byDate[targetKey]
	if targetDay == nil || targetDay.Volume <= 0 {
		return "15m pending: waiting for opening bars", 0, false
	}
	if targetDay.Count < 3 {
		return fmt.Sprintf("15m pending: %d/3 bars", targetDay.Count), 0, false
	}
	keys := make([]string, 0, len(byDate))
	for key, day := range byDate {
		if key < targetKey && day.Count >= 3 && day.Volume > 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) > 10 {
		keys = keys[len(keys)-10:]
	}
	if len(keys) == 0 {
		return "15m baseline pending", 0, false
	}
	sum := 0.0
	for _, key := range keys {
		sum += byDate[key].Volume
	}
	avg := sum / float64(len(keys))
	if avg <= 0 {
		return "15m baseline pending", 0, false
	}
	ratio := round2(targetDay.Volume / avg)
	switch {
	case ratio >= 1.5:
		return fmt.Sprintf("15m Surge %.1fx avg", ratio), ratio, true
	case ratio >= 1.1:
		return fmt.Sprintf("15m Active %.1fx avg", ratio), ratio, false
	case ratio >= 0.8:
		return fmt.Sprintf("15m Normal %.1fx avg", ratio), ratio, false
	default:
		return fmt.Sprintf("15m Light %.1fx avg", ratio), ratio, false
	}
}
