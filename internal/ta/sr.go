package ta

import "sort"

// SupportResistance returns N support and resistance levels using simple pivot detection.
// A pivot high is a bar where the high exceeds `lookback` bars on either side.
func SupportResistance(highs, lows []float64, lookback, maxLevels int) (support, resistance []float64) {
	n := len(highs)
	if n < lookback*2+1 {
		return
	}
	for i := lookback; i < n-lookback; i++ {
		isPivotHigh, isPivotLow := true, true
		for j := 1; j <= lookback; j++ {
			if highs[i-j] >= highs[i] || highs[i+j] >= highs[i] {
				isPivotHigh = false
			}
			if lows[i-j] <= lows[i] || lows[i+j] <= lows[i] {
				isPivotLow = false
			}
			if !isPivotHigh && !isPivotLow {
				break
			}
		}
		if isPivotHigh {
			resistance = append(resistance, highs[i])
		}
		if isPivotLow {
			support = append(support, lows[i])
		}
	}
	// Cluster nearby levels: dedupe within 0.5% of each other, keep latest
	resistance = clusterLevels(resistance, 0.005)
	support = clusterLevels(support, 0.005)

	sort.Sort(sort.Reverse(sort.Float64Slice(resistance)))
	sort.Float64s(support)

	if maxLevels > 0 {
		if len(resistance) > maxLevels {
			resistance = resistance[:maxLevels]
		}
		if len(support) > maxLevels {
			support = support[len(support)-maxLevels:]
		}
	}
	return
}

func clusterLevels(levels []float64, tolerance float64) []float64 {
	if len(levels) == 0 {
		return levels
	}
	sort.Float64s(levels)
	out := []float64{levels[0]}
	for i := 1; i < len(levels); i++ {
		last := out[len(out)-1]
		if last == 0 {
			out = append(out, levels[i])
			continue
		}
		if (levels[i]-last)/last > tolerance {
			out = append(out, levels[i])
		}
	}
	return out
}
