package ta

import "math"

// FibLevels computes Fibonacci retracement levels from a swing high/low window.
// Standard levels: 0.0, 0.236, 0.382, 0.5, 0.618, 0.786, 1.0
func FibLevels(high, low float64) map[string]float64 {
	if high <= low {
		return nil
	}
	rng := high - low
	return map[string]float64{
		"0":     low,
		"0.236": low + 0.236*rng,
		"0.382": low + 0.382*rng,
		"0.5":   low + 0.5*rng,
		"0.618": low + 0.618*rng,
		"0.786": low + 0.786*rng,
		"1":     high,
	}
}

// SwingHighLow returns max high and min low over the last `lookback` bars.
func SwingHighLow(highs, lows []float64, lookback int) (h, l float64) {
	if lookback > len(highs) {
		lookback = len(highs)
	}
	if lookback == 0 {
		return 0, 0
	}
	start := len(highs) - lookback
	h = highs[start]
	l = lows[start]
	for i := start + 1; i < len(highs); i++ {
		if highs[i] > h {
			h = highs[i]
		}
		if lows[i] < l {
			l = lows[i]
		}
	}
	return
}

// NearestFib returns the level label closest to `price`.
func NearestFib(levels map[string]float64, price float64) string {
	bestKey := ""
	bestDist := math.MaxFloat64
	for k, v := range levels {
		d := math.Abs(price - v)
		if d < bestDist {
			bestDist = d
			bestKey = k
		}
	}
	return bestKey
}

// FibZone classifies price position within the fib range as HIGH/MID/LOW.
func FibZone(levels map[string]float64, price float64) string {
	if levels == nil {
		return "MID"
	}
	if price >= levels["0.618"] {
		return "HIGH"
	}
	if price <= levels["0.382"] {
		return "LOW"
	}
	return "MID"
}
