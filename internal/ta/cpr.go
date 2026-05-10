package ta

import "fmt"

// CPR is the Central Pivot Range computed from prior day's H/L/C.
// Mirrors stock_pulse.py:_calc_cpr exactly.
type CPR struct {
	P        float64 `json:"p"`  // pivot
	BC       float64 `json:"bc"` // bottom central
	TC       float64 `json:"tc"` // top central
	Width    float64 `json:"width"`
	WidthPct float64 `json:"width_pct"`
	Type     string  `json:"type"`     // Narrow / Normal / Wide
	Position string  `json:"position"` // Above / Inside / Below
	Interp   string  `json:"interpretation"`
}

// ComputeCPR builds the CPR from yesterday's bar and classifies the current price.
// Returns nil if there's insufficient data.
func ComputeCPR(prevHigh, prevLow, prevClose, currentPrice float64) *CPR {
	if prevHigh <= 0 || prevLow <= 0 {
		return nil
	}
	p := (prevHigh + prevLow + prevClose) / 3
	bc := (prevHigh + prevLow) / 2
	tc := 2*p - bc

	top := tc
	bot := bc
	if bc > tc {
		top, bot = bc, tc
	}

	width := top - bot
	widthPct := 0.0
	if p > 0 {
		widthPct = width / p * 100
	}

	cprType := "Normal"
	switch {
	case widthPct < 0.15:
		cprType = "Narrow"
	case widthPct > 0.5:
		cprType = "Wide"
	}

	pos := "Inside"
	switch {
	case currentPrice > top:
		pos = "Above"
	case currentPrice < bot:
		pos = "Below"
	}

	return &CPR{
		P:        round2(p),
		BC:       round2(bot),
		TC:       round2(top),
		Width:    round4(width),
		WidthPct: round3(widthPct),
		Type:     cprType,
		Position: pos,
		Interp:   cprInterp(cprType, pos, top, p, bot),
	}
}

// cprInterp returns the human-readable interpretation with embedded TC/P/BC prices.
// Mirrors stock_pulse.py:cpr_interpretation but includes actual numbers in the text.
func cprInterp(cprType, position string, tc, p, bc float64) string {
	switch cprType + "/" + position {
	case "Narrow/Above":
		return fmt.Sprintf("🚀 Trending day ↑ — price above TC ($%.2f), strong bull momentum", tc)
	case "Narrow/Inside":
		return fmt.Sprintf("⏳ Trending day — inside CPR ($%.2f–$%.2f), pivot $%.2f, wait for TC/BC breakout", bc, tc, p)
	case "Narrow/Below":
		return fmt.Sprintf("📉 Trending day ↓ — price below BC ($%.2f), strong bear momentum", bc)
	case "Wide/Above":
		return fmt.Sprintf("↩️ Range day — above TC ($%.2f), may pull back to CPR ($%.2f)", tc, p)
	case "Wide/Inside":
		return fmt.Sprintf("〰️ Range day — chop between TC ($%.2f) and BC ($%.2f), pivot $%.2f, trade bounces", tc, bc, p)
	case "Wide/Below":
		return fmt.Sprintf("↪️ Range day — below BC ($%.2f), may bounce back to CPR ($%.2f)", bc, p)
	case "Normal/Above":
		return fmt.Sprintf("🟢 Bullish bias — above TC ($%.2f), P ($%.2f) acts as support", tc, p)
	case "Normal/Inside":
		return fmt.Sprintf("⚖️ Neutral — inside CPR ($%.2f–$%.2f), pivot $%.2f, watch for breakout", bc, tc, p)
	case "Normal/Below":
		return fmt.Sprintf("🔴 Bearish bias — below BC ($%.2f), P ($%.2f) acts as resistance", bc, p)
	}
	return "—"
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
func round3(v float64) float64 { return float64(int(v*1000+0.5)) / 1000 }
func round4(v float64) float64 { return float64(int(v*10000+0.5)) / 10000 }
