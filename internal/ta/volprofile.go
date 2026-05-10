package ta

// VolumeProfile is the standard market-profile output:
//
//	POC  — price level with the most volume traded
//	VAH  — upper edge of the 70% value area
//	VAL  — lower edge of the 70% value area
type VolumeProfile struct {
	POC      float64 `json:"poc"`
	VAH      float64 `json:"vah"`
	VAL      float64 `json:"val"`
	VolBias  string  `json:"vol_bias"`  // BULLISH / BEARISH / NEUTRAL — price vs VA
	VolTrend string  `json:"vol_trend"` // ACCUMULATING / DISTRIBUTING / FLAT
	VolRatio float64 `json:"vol_ratio"` // latest vs 20d average
	VolSurge bool    `json:"vol_surge"`
	Detail   string  `json:"detail"`
}

// ComputeVolumeProfile builds a market profile over the last `lookback` daily bars.
// Distributes each bar's volume across price buckets it spans (high–low), then finds
// POC and expands outward to capture ~70% of total volume.
func ComputeVolumeProfile(highs, lows, closes, volumes []float64, lookback, buckets int) *VolumeProfile {
	n := len(closes)
	if n < 5 {
		return nil
	}
	if lookback > n {
		lookback = n
	}
	if buckets <= 0 {
		buckets = 50
	}
	start := n - lookback

	// Find price range across window
	pMin, pMax := lows[start], highs[start]
	for i := start; i < n; i++ {
		if lows[i] < pMin {
			pMin = lows[i]
		}
		if highs[i] > pMax {
			pMax = highs[i]
		}
	}
	if pMax <= pMin {
		return nil
	}
	bucketSize := (pMax - pMin) / float64(buckets)
	if bucketSize <= 0 {
		return nil
	}

	// Distribute each bar's volume evenly across the buckets it spans
	hist := make([]float64, buckets)
	totalVol := 0.0
	for i := start; i < n; i++ {
		lo, hi := lows[i], highs[i]
		loIdx := int((lo - pMin) / bucketSize)
		hiIdx := int((hi - pMin) / bucketSize)
		if loIdx < 0 {
			loIdx = 0
		}
		if hiIdx >= buckets {
			hiIdx = buckets - 1
		}
		span := hiIdx - loIdx + 1
		if span <= 0 {
			span = 1
		}
		share := volumes[i] / float64(span)
		for b := loIdx; b <= hiIdx; b++ {
			hist[b] += share
		}
		totalVol += volumes[i]
	}
	if totalVol == 0 {
		return nil
	}

	// POC = bucket with highest volume
	pocIdx := 0
	pocVol := hist[0]
	for i := 1; i < buckets; i++ {
		if hist[i] > pocVol {
			pocVol = hist[i]
			pocIdx = i
		}
	}
	poc := pMin + (float64(pocIdx)+0.5)*bucketSize

	// Value Area: expand outward from POC to capture 70% of total volume
	target := totalVol * 0.7
	covered := hist[pocIdx]
	lo, hi := pocIdx, pocIdx
	for covered < target && (lo > 0 || hi < buckets-1) {
		var addVol float64
		nextLo, nextHi := lo-1, hi+1
		loVol, hiVol := -1.0, -1.0
		if nextLo >= 0 {
			loVol = hist[nextLo]
		}
		if nextHi < buckets {
			hiVol = hist[nextHi]
		}
		if hiVol >= loVol {
			if nextHi < buckets {
				addVol = hist[nextHi]
				hi = nextHi
			} else {
				addVol = hist[nextLo]
				lo = nextLo
			}
		} else {
			if nextLo >= 0 {
				addVol = hist[nextLo]
				lo = nextLo
			} else {
				addVol = hist[nextHi]
				hi = nextHi
			}
		}
		covered += addVol
	}
	val := pMin + float64(lo)*bucketSize
	vah := pMin + float64(hi+1)*bucketSize

	// Vol ratio + surge
	avg20 := 0.0
	count := 0
	for i := n - 21; i < n-1 && i >= 0; i++ {
		avg20 += volumes[i]
		count++
	}
	volRatio := 0.0
	if count > 0 && avg20 > 0 {
		avg20 /= float64(count)
		volRatio = volumes[n-1] / avg20
	}
	volRatio = round2(volRatio)
	surge := volRatio > 1.5

	// Vol bias from price vs VA
	last := closes[n-1]
	bias := "NEUTRAL"
	switch {
	case last > vah:
		bias = "BULLISH"
	case last < val:
		bias = "BEARISH"
	}

	// Vol trend (same logic as analysis classifyVolume — keep here for reuse)
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
	trend := "FLAT"
	switch {
	case upVol > downVol*1.3:
		trend = "ACCUMULATING"
	case downVol > upVol*1.3:
		trend = "DISTRIBUTING"
	}

	detail := buildVPDetail(bias, trend, last, val, vah, poc)

	return &VolumeProfile{
		POC:      round2(poc),
		VAH:      round2(vah),
		VAL:      round2(val),
		VolBias:  bias,
		VolTrend: trend,
		VolRatio: volRatio,
		VolSurge: surge,
		Detail:   detail,
	}
}

func buildVPDetail(bias, trend string, last, val, vah, poc float64) string {
	pos := "inside VA"
	switch {
	case last > vah:
		pos = "above VA — breakout territory"
	case last < val:
		pos = "below VA — breakdown territory"
	}
	return bias + " · " + trend + " · " + pos
}
