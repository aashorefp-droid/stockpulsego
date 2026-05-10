package ta

import "math"

// SMA returns the simple moving average over `period`. Output has same length as input;
// values before the period are set to NaN.
func SMA(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	for i := range out {
		out[i] = math.NaN()
	}
	if period <= 0 || len(values) < period {
		return out
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	out[period-1] = sum / float64(period)
	for i := period; i < len(values); i++ {
		sum += values[i] - values[i-period]
		out[i] = sum / float64(period)
	}
	return out
}

// EMA returns exponential moving average over `period`.
func EMA(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	for i := range out {
		out[i] = math.NaN()
	}
	if period <= 0 || len(values) < period {
		return out
	}
	k := 2.0 / float64(period+1)
	// Seed with SMA of first `period` values
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	prev := sum / float64(period)
	out[period-1] = prev
	for i := period; i < len(values); i++ {
		prev = values[i]*k + prev*(1-k)
		out[i] = prev
	}
	return out
}

// RSI returns the Wilder-smoothed RSI over `period`.
func RSI(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	for i := range out {
		out[i] = math.NaN()
	}
	if period <= 0 || len(values) <= period {
		return out
	}
	gains, losses := 0.0, 0.0
	for i := 1; i <= period; i++ {
		diff := values[i] - values[i-1]
		if diff >= 0 {
			gains += diff
		} else {
			losses += -diff
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	out[period] = rsiFromAvg(avgGain, avgLoss)
	for i := period + 1; i < len(values); i++ {
		diff := values[i] - values[i-1]
		gain, loss := 0.0, 0.0
		if diff >= 0 {
			gain = diff
		} else {
			loss = -diff
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
		out[i] = rsiFromAvg(avgGain, avgLoss)
	}
	return out
}

func rsiFromAvg(g, l float64) float64 {
	if l == 0 {
		return 100
	}
	rs := g / l
	return 100 - (100 / (1 + rs))
}

// MACD returns macd line, signal line, histogram (each same length as input).
func MACD(values []float64, fast, slow, signal int) (macd, sig, hist []float64) {
	emaFast := EMA(values, fast)
	emaSlow := EMA(values, slow)
	macd = make([]float64, len(values))
	for i := range macd {
		if math.IsNaN(emaFast[i]) || math.IsNaN(emaSlow[i]) {
			macd[i] = math.NaN()
		} else {
			macd[i] = emaFast[i] - emaSlow[i]
		}
	}
	// Signal line is EMA of macd, but we need to handle NaNs by stripping them
	sig = emaIgnoreNaN(macd, signal)
	hist = make([]float64, len(values))
	for i := range hist {
		if math.IsNaN(macd[i]) || math.IsNaN(sig[i]) {
			hist[i] = math.NaN()
		} else {
			hist[i] = macd[i] - sig[i]
		}
	}
	return
}

func emaIgnoreNaN(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	for i := range out {
		out[i] = math.NaN()
	}
	// Find first non-NaN index
	start := -1
	for i, v := range values {
		if !math.IsNaN(v) {
			start = i
			break
		}
	}
	if start < 0 || len(values)-start < period {
		return out
	}
	k := 2.0 / float64(period+1)
	sum := 0.0
	for i := start; i < start+period; i++ {
		sum += values[i]
	}
	prev := sum / float64(period)
	out[start+period-1] = prev
	for i := start + period; i < len(values); i++ {
		prev = values[i]*k + prev*(1-k)
		out[i] = prev
	}
	return out
}

// ATR returns Average True Range using Wilder's smoothing.
func ATR(highs, lows, closes []float64, period int) []float64 {
	n := len(closes)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if n <= period || period <= 0 {
		return out
	}
	tr := make([]float64, n)
	for i := 0; i < n; i++ {
		if i == 0 {
			tr[i] = highs[i] - lows[i]
		} else {
			a := highs[i] - lows[i]
			b := math.Abs(highs[i] - closes[i-1])
			c := math.Abs(lows[i] - closes[i-1])
			tr[i] = math.Max(a, math.Max(b, c))
		}
	}
	// Wilder's smoothing: first ATR is simple average of first `period` TRs
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += tr[i]
	}
	prev := sum / float64(period)
	out[period] = prev
	for i := period + 1; i < n; i++ {
		prev = (prev*float64(period-1) + tr[i]) / float64(period)
		out[i] = prev
	}
	return out
}

// BollingerBands returns upper, middle, lower bands.
func BollingerBands(values []float64, period int, stdDev float64) (upper, middle, lower []float64) {
	middle = SMA(values, period)
	upper = make([]float64, len(values))
	lower = make([]float64, len(values))
	for i := range upper {
		upper[i] = math.NaN()
		lower[i] = math.NaN()
	}
	for i := period - 1; i < len(values); i++ {
		mean := middle[i]
		variance := 0.0
		for j := i - period + 1; j <= i; j++ {
			d := values[j] - mean
			variance += d * d
		}
		sd := math.Sqrt(variance / float64(period))
		upper[i] = mean + stdDev*sd
		lower[i] = mean - stdDev*sd
	}
	return
}

// LastValid returns the last non-NaN value in the slice (and true), or 0 + false if none.
func LastValid(values []float64) (float64, bool) {
	for i := len(values) - 1; i >= 0; i-- {
		if !math.IsNaN(values[i]) {
			return values[i], true
		}
	}
	return 0, false
}
