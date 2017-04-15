package libphonelab

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/gurupras/go_cpuprof/post_processing"
)

type TemperatureCurve int

const (
	CURVE_UNKNOWN TemperatureCurve = iota
	HEATING_UP    TemperatureCurve = iota
	COOLING_DOWN  TemperatureCurve = iota
)

type ProbabilityDistribution struct {
	Total   int64
	Success int64
}

type TemperatureMap map[int32]PeriodProbabilityDistribution
type PeriodProbabilityDistribution map[int]*ProbabilityDistribution

type CurveMap struct {
	Map       TemperatureMap
	Periods   []int
	Temps     []int32
	MaxPeriod int
}

func NewCurveMap(periods []int) *CurveMap {
	sort.Ints(periods)

	cm := new(CurveMap)
	cm.Map = make(TemperatureMap)
	cm.Periods = periods
	cm.Temps = make([]int32, 0)

	for i := int32(0); i < 120; i += 5 {
		cm.Map[i] = make(PeriodProbabilityDistribution)
		cm.Temps = append(cm.Temps, i)
		for _, p := range periods {
			cm.Map[i][p] = new(ProbabilityDistribution)
		}
	}
	cm.MaxPeriod = periods[len(periods)-1]
	return cm
}

type HeatingCoolingDistribution struct {
	*Distribution
	Heating        *CurveMap
	Cooling        *CurveMap
	Unknown        *CurveMap
	CurveThreshold int32
}

func NewHeatingCoolingDistribution(boot *post_processing.Boot, period time.Duration, periods []time.Duration, curveThreshold int32) *HeatingCoolingDistribution {
	hcd := new(HeatingCoolingDistribution)
	hcd.Distribution = NewDistribution(boot, period)
	timePeriods := make([]int, 0)
	for _, p := range periods {
		pSec := int(p.Seconds())
		timePeriods = append(timePeriods, pSec)
	}
	hcd.Heating = NewCurveMap(timePeriods)
	hcd.Cooling = NewCurveMap(timePeriods)
	hcd.Unknown = NewCurveMap(timePeriods)
	hcd.CurveThreshold = curveThreshold
	return hcd
}

func (hcd *HeatingCoolingDistribution) BuildProbabilities(curvePeriod time.Duration, nthPercentile int) {
	for idx := 0; idx < len(hcd.Distribution.Temps); idx++ {
		var curveMap *CurveMap
		temp := hcd.Distribution.Temps[idx] - (hcd.Distribution.Temps[idx] % 5)
		timestamp := hcd.Distribution.Timestamps[idx]
		_ = temp

		switch hcd.FindCurveByIdx(idx, curvePeriod, hcd.CurveThreshold) {
		case CURVE_UNKNOWN:
			curveMap = hcd.Unknown
		case HEATING_UP:
			curveMap = hcd.Heating
		case COOLING_DOWN:
			curveMap = hcd.Cooling
		}

		// Get nth-percentile temperature
		thresholdTemp := hcd.Distribution.NthPercentileTemp(nthPercentile)

		// Now check all temperatures up to the maximum period and assign probabilities
		maxPeriod := time.Duration(curveMap.MaxPeriod) * time.Second
		for nidx := idx + 1; nidx < len(hcd.Distribution.Temps); nidx++ {
			nTemp := hcd.Distribution.Temps[nidx]
			nTimestamp := hcd.Distribution.Timestamps[nidx]

			diff := time.Duration(nTimestamp - timestamp)
			// Break condition
			if diff > maxPeriod {
				break
			}

			for _, period := range curveMap.Periods {
				interval := time.Duration(period) * time.Second
				pd := curveMap.Map[temp][period]
				if pd == nil {
					fmt.Printf("Failed to find pd for temp=%v period=%v\n", temp, period)
					os.Exit(-1)
				}
				if diff < interval {
					// First, increment seen
					pd.Success += 1
					// Now check if less than nth percentile temp
					if nTemp < thresholdTemp {
						pd.Total += 1
					}
				}
			}
		}
	}
}

func (hcd *HeatingCoolingDistribution) GetCurveMap(curve TemperatureCurve) (curveMap *CurveMap) {
	switch curve {
	case CURVE_UNKNOWN:
		curveMap = hcd.Unknown
	case HEATING_UP:
		curveMap = hcd.Heating
	case COOLING_DOWN:
		curveMap = hcd.Cooling
	}
	return
}

func (hcd *HeatingCoolingDistribution) UpdateAccuracy(curvePeriod time.Duration, nthPercentile int, dist *Distribution) {
	// Find which curve the current distribution is on
	curveEndIdx, curve := dist.FindFirstCurve(curvePeriod, hcd.CurveThreshold)
	curveMap := hcd.GetCurveMap(curve)
	if curveMap == nil {
		return
	}
	startTemp := dist.Temps[curveEndIdx] - (dist.Temps[curveEndIdx] % 5)
	startTimestamp := dist.Timestamps[curveEndIdx]

	nthPercentileTemp := hcd.NthPercentileTemp(nthPercentile)

	for _, period := range curveMap.Periods {
		interval := time.Duration(period) * time.Second
		markedSeen := false
		pd := curveMap.Map[startTemp][period]

		for idx := curveEndIdx + 1; idx < len(dist.Timestamps); idx++ {
			temp := dist.Temps[idx]
			timestamp := dist.Timestamps[idx]

			diff := time.Duration(timestamp - startTimestamp)
			if diff >= interval {
				break
			}
			if !markedSeen {
				pd.Success++
				markedSeen = true
			}

			if temp < nthPercentileTemp {
				pd.Total++
				break
			}
		}
	}
}
