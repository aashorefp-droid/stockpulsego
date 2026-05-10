package models

import "time"

// Bar represents a single OHLCV bar.
type Bar struct {
	Time   time.Time `json:"time"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume int64     `json:"volume"`
}

// Bars is a slice of bars sorted ascending by time.
type Bars []Bar

func (b Bars) Closes() []float64 {
	out := make([]float64, len(b))
	for i, x := range b {
		out[i] = x.Close
	}
	return out
}

func (b Bars) Highs() []float64 {
	out := make([]float64, len(b))
	for i, x := range b {
		out[i] = x.High
	}
	return out
}

func (b Bars) Lows() []float64 {
	out := make([]float64, len(b))
	for i, x := range b {
		out[i] = x.Low
	}
	return out
}

func (b Bars) Volumes() []float64 {
	out := make([]float64, len(b))
	for i, x := range b {
		out[i] = float64(x.Volume)
	}
	return out
}

// ScanResult mirrors the Python scan output.
type ScanResult struct {
	Ticker          string             `json:"ticker"`
	Sector          string             `json:"sector,omitempty"`
	Price           float64            `json:"price,omitempty"`
	Verdict         string             `json:"verdict,omitempty"`
	VerdictFlipDate string             `json:"verdict_flip_date,omitempty"`
	VerdictFlipFrom string             `json:"verdict_flip_from,omitempty"`
	VerdictFlipDays *int               `json:"verdict_flip_days,omitempty"`
	VerdictFlipText string             `json:"verdict_flip_text,omitempty"`
	Confidence      string             `json:"confidence,omitempty"`
	Score           int                `json:"score"`
	Direction       string             `json:"direction,omitempty"`
	EntryGrade      string             `json:"entry_grade,omitempty"`
	EntryLabel      string             `json:"entry_label,omitempty"`
	GradeColor      string             `json:"grade_color,omitempty"`
	ExpectedWR      float64            `json:"expected_wr,omitempty"`
	MTFRank         int                `json:"mtf_rank,omitempty"`
	MTFSignal       string             `json:"mtf_signal,omitempty"`
	MTFAction       string             `json:"mtf_action,omitempty"`
	MTFKey          string             `json:"mtf_key,omitempty"`
	WeeklyBias      string             `json:"weekly_bias,omitempty"`
	DailyBias       string             `json:"daily_bias,omitempty"`
	EarnZone        string             `json:"earn_zone,omitempty"`
	WeeklyZone      string             `json:"weekly_zone,omitempty"`
	NearFibName     string             `json:"near_fib_name,omitempty"`
	NearFibPrice    float64            `json:"near_fib_price,omitempty"`
	WeekHi          float64            `json:"week_hi,omitempty"`
	WeekLo          float64            `json:"week_lo,omitempty"`
	WkPosPct        float64            `json:"wk_pos_pct,omitempty"`
	EarnHi          float64            `json:"earn_hi,omitempty"`
	EarnLo          float64            `json:"earn_lo,omitempty"`
	EarnPosPct      float64            `json:"earn_pos_pct,omitempty"`
	FibLevels       map[string]float64 `json:"fib_levels,omitempty"`
	FibCompression  bool               `json:"fib_compression,omitempty"`
	Signals         string             `json:"signals,omitempty"`
	// CPR (Central Pivot Range)
	CPRType            string   `json:"cpr_type,omitempty"`
	CPRTC              float64  `json:"cpr_tc,omitempty"`
	CPRBC              float64  `json:"cpr_bc,omitempty"`
	CPRP               float64  `json:"cpr_p,omitempty"`
	CPRPosition        string   `json:"cpr_position,omitempty"`
	CPRInterp          string   `json:"cpr_interpretation,omitempty"`
	CPRDayResult       string   `json:"cpr_day_result,omitempty"`
	CPRDayEntry        *float64 `json:"cpr_day_entry,omitempty"`
	CPRDayStop         *float64 `json:"cpr_day_stop,omitempty"`
	CPRDayT1           *float64 `json:"cpr_day_t1,omitempty"`
	NextDayDate        string   `json:"next_day_date,omitempty"`
	NextDayOutcome     string   `json:"next_day_outcome,omitempty"`
	NextDayBias        string   `json:"next_day_bias,omitempty"`
	NextDaySummary     string   `json:"next_day_summary,omitempty"`
	NextDayPrediction  string   `json:"next_day_prediction,omitempty"`
	NextDayRef         string   `json:"next_day_ref,omitempty"`
	NextDayTarget      *float64 `json:"next_day_target,omitempty"`
	NextDayATR         *float64 `json:"next_day_atr,omitempty"`
	NextDayATRPct      *float64 `json:"next_day_atr_pct,omitempty"`
	NextDayTriggerUp   *float64 `json:"next_day_trigger_up,omitempty"`
	NextDayTriggerDown *float64 `json:"next_day_trigger_down,omitempty"`
	NextDayPivot       *float64 `json:"next_day_pivot,omitempty"`
	PrevDayHigh        *float64 `json:"prev_day_high,omitempty"`
	PrevDayLow         *float64 `json:"prev_day_low,omitempty"`
	// Expected move based on ATR
	ExpMoveUp      float64 `json:"exp_move_up,omitempty"` // from last close
	ExpMoveDown    float64 `json:"exp_move_down,omitempty"`
	ExpMovePct     float64 `json:"exp_move_pct,omitempty"`
	ExpMoveOpenUp  float64 `json:"exp_move_open_up,omitempty"` // from day open
	ExpMoveOpenDn  float64 `json:"exp_move_open_dn,omitempty"`
	ExpMoveOpenPct float64 `json:"exp_move_open_pct,omitempty"`
	DayOpen        float64 `json:"day_open,omitempty"`
	// LRE = Low Risk Entry: 0–5 quality score with rationale + suggested entry zone
	LREScore        int            `json:"lre_score,omitempty"`
	LRELabel        string         `json:"lre_label,omitempty"`
	LREDirection    string         `json:"lre_direction,omitempty"`
	LREReason       string         `json:"lre_reason,omitempty"`
	LREEntry        float64        `json:"lre_entry,omitempty"`    // suggested entry price
	LREStop         float64        `json:"lre_stop,omitempty"`     // entry-based stop
	LRERisk         float64        `json:"lre_risk_pct,omitempty"` // risk % from LRE entry
	LREStatus       string         `json:"lre_status,omitempty"`   // ACTIVE / DISCOUNT / STALE / INVALIDATED
	LRETakeaway     string         `json:"lre_takeaway,omitempty"`
	ValuationLabel  string         `json:"valuation_label,omitempty"`
	ValuationScore  int            `json:"valuation_score,omitempty"`
	ValuationReason string         `json:"valuation_reason,omitempty"`
	VolTrend        string         `json:"vol_trend,omitempty"`
	VolSurge        bool           `json:"vol_surge,omitempty"`
	BreakoutScore   int            `json:"breakout_score,omitempty"`
	DistFromHigh    *float64       `json:"dist_from_high,omitempty"`
	ShortPct        *float64       `json:"short_pct,omitempty"`
	Entry           *float64       `json:"entry,omitempty"`
	StopLoss        *float64       `json:"stop_loss,omitempty"`
	Target1         *float64       `json:"target1,omitempty"`
	RiskPct         *float64       `json:"risk_pct,omitempty"`
	RRT1            *float64       `json:"rr_t1,omitempty"`
	ATR             *float64       `json:"atr,omitempty"`
	OptStrategy     string         `json:"opt_strategy,omitempty"`
	OptSummary      string         `json:"opt_summary,omitempty"`
	OptDebit        *float64       `json:"opt_debit,omitempty"`
	OptProfit       *float64       `json:"opt_profit,omitempty"`
	OptSource       string         `json:"opt_source,omitempty"`
	OptQuoteTS      string         `json:"opt_quote_ts,omitempty"`
	OptLegs         []OptionLeg    `json:"opt_legs,omitempty"`
	OptWidth        *float64       `json:"opt_width,omitempty"`
	OptExpShort     string         `json:"opt_exp_short,omitempty"`
	OptExpLong      string         `json:"opt_exp_long,omitempty"`
	OptAlt          string         `json:"opt_alt,omitempty"`
	OptLiquid       []OTMLiquidRow `json:"opt_liquid,omitempty"`
	Done            bool           `json:"done,omitempty"`
	Total           int            `json:"total,omitempty"`
	Error           string         `json:"error,omitempty"`
}

type OptionLeg struct {
	Action    string   `json:"action"`
	Type      string   `json:"type"`
	Strike    float64  `json:"strike"`
	Exp       string   `json:"exp"`
	Bid       float64  `json:"bid"`
	Ask       float64  `json:"ask"`
	Mid       float64  `json:"mid"`
	SpreadPct *float64 `json:"spread_pct,omitempty"`
}

type OTMLiquidRow struct {
	Strike     float64 `json:"strike"`
	Type       string  `json:"type"`
	Expiry     string  `json:"expiry"`
	Volume     int     `json:"volume"`
	OI         int     `json:"oi"`
	IV         float64 `json:"iv"`
	OTMPct     float64 `json:"otm_pct"`
	VolOIRatio float64 `json:"vol_oi_ratio"`
	Unusual    bool    `json:"unusual"`
}

// IsExceptional matches the frontend "exceptional" filter:
// grade S/A + MTF rank 1 + ACCUMULATING volume.
func (r ScanResult) IsExceptional() bool {
	if r.Error != "" {
		return false
	}
	if r.EntryGrade != "S" && r.EntryGrade != "A" {
		return false
	}
	if r.MTFRank != 1 {
		return false
	}
	if r.VolTrend != "ACCUMULATING" {
		return false
	}
	return true
}
