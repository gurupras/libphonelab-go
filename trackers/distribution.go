package trackers

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/gurupras/go_cpuprof/post_processing"
	"github.com/shaseley/phonelab-go"
)

type Distribution struct {
	Boot         *post_processing.Boot
	Temps        []int32
	Timestamps   []int64 // Ensure these are in nanoseconds
	StartTime    *int64
	LastTime     *int64
	LastTemp     *int32
	LastLogline  *phonelab.Logline
	Period       time.Duration
	Distribution map[int32]int64
	isFull       bool
}

func NewDistribution(boot *post_processing.Boot, Period time.Duration) *Distribution {
	distribution := new(Distribution)
	distribution.Boot = boot
	distribution.Period = Period
	distribution.Distribution = make(map[int32]int64)
	distribution.Temps = make([]int32, 0)
	distribution.Timestamps = make([]int64, 0)
	return distribution
}

func (d *Distribution) Clone() *Distribution {
	dist := NewDistribution(d.Boot, d.Period)

	for k, v := range d.Distribution {
		dist.Distribution[k] = v
	}
	for _, v := range d.Temps {
		dist.Temps = append(dist.Temps, v)
	}
	for _, v := range d.Timestamps {
		dist.Timestamps = append(dist.Timestamps, v)
	}

	if len(dist.Temps) > 0 {
		dist.StartTime = &dist.Timestamps[0]
		dist.LastTime = &dist.Timestamps[len(dist.Timestamps)-1]
		dist.LastTemp = &dist.Temps[len(dist.Temps)-1]
	}
	dist.isFull = d.isFull
	dist.LastLogline = d.LastLogline
	return dist
}

func (d *Distribution) Update(temp int32, t interface{}) {
	var timestamp int64
	switch t.(type) {
	case time.Time:
		timestamp = (t.(time.Time)).UnixNano()
	case int64:
		timestamp = t.(int64)
	}
	if d.StartTime != nil && time.Duration(timestamp-*d.StartTime) > d.Period {
		// We've exceeded the period of this distribution..
		// Remove the first element and append the current temp at the end
		// so that we simulate a queue
		d.isFull = true

		// Remove all elements until 24 hours
		idx := 0
		for idx = 0; idx < len(d.Temps); idx++ {
			oldTemp := d.Temps[idx]
			oldTimestamp := d.Timestamps[idx]

			if time.Duration(timestamp-oldTimestamp) <= d.Period {
				break
			}

			// We're removing this temp..so remove it from the distribution
			d.Distribution[oldTemp]--
		}
		// Shorten arrays
		d.Temps = d.Temps[idx:]
		d.Timestamps = d.Timestamps[idx:]

	}
	// Now add the new temp
	d.Temps = append(d.Temps, temp)
	// And new timestamp
	d.Timestamps = append(d.Timestamps, timestamp)

	if d.StartTime == nil {
		d.StartTime = &d.Timestamps[0]
	}

	// Update the distribution
	if _, ok := d.Distribution[temp]; !ok {
		d.Distribution[temp] = 0
	}
	d.Distribution[temp]++

	d.LastTime = &d.Timestamps[len(d.Timestamps)-1]
	d.LastTemp = &d.Temps[len(d.Temps)-1]
}

func (d *Distribution) MeasuredDurationSec() float64 {
	if len(d.Timestamps) < 2 {
		return 0.0
	}
	return time.Duration(d.Timestamps[len(d.Timestamps)-1] - d.Timestamps[0]).Seconds()
}

func (d *Distribution) NthPercentileTemp(percentile int) int32 {
	totalPoints := len(d.Temps)
	nthPercentilePoints := int64(totalPoints / percentile)

	if percentile < 0 {
		percentile = 0
	}
	if percentile > 100 {
		percentile = 100
	}

	counted := int64(0)
	for k := range d.Distribution {
		counted += d.Distribution[k]
		if counted >= nthPercentilePoints {
			return k
		}
	}
	return -1
}

type ComparisonOperator int

const (
	LT ComparisonOperator = iota
	LE ComparisonOperator = iota
	EQ ComparisonOperator = iota
	GE ComparisonOperator = iota
	GT ComparisonOperator = iota
	NE ComparisonOperator = iota
)

func (d *Distribution) FindIdxByTime(tm time.Time, thresholds ...time.Duration) (int, int64) {
	return d.FindIdxByTimeLinear(tm, thresholds...)
}

func (d *Distribution) FindIdxByTimeLinear(tm time.Time, thresholds ...time.Duration) (int, int64) {
	timestamp := tm.UnixNano()
	return d.FindIdxByTimestampLinear(timestamp, thresholds...)
}

func (d *Distribution) FindIdxByTimeBinarySearch(tm time.Time, thresholds ...time.Duration) (int, int64) {
	timestamp := tm.UnixNano()
	return d.FindIdxByTimestampBinarySearch(timestamp, thresholds...)
}

func (d *Distribution) FindIdxByTimestamp(timestamp int64, thresholds ...time.Duration) (int, int64) {
	return d.FindIdxByTimestampLinear(timestamp, thresholds...)
}

func (d *Distribution) FindIdxByTimestampLinear(timestamp int64, thresholds ...time.Duration) (int, int64) {
	var threshold time.Duration = 10 * time.Second
	if len(thresholds) > 0 {
		threshold = thresholds[0]
	}

	closestDiff := float64(math.MaxUint64)
	closestIdx := -1
	var closestTime int64
	for idx := 0; idx < len(d.Timestamps); idx++ {
		diff := math.Abs(float64(time.Duration(timestamp - d.Timestamps[idx])))
		if diff < closestDiff {
			closestDiff = diff
			closestIdx = idx
			closestTime = d.Timestamps[idx]
		}
	}
	if closestDiff > float64(threshold) {
		//fmt.Fprintf(os.Stderr, "Closest index is off by more than 5 seconds: %d [%d-%d) : %.2f\n", closestIdx, 0, len(d.Timestamps), closestDiff.Seconds())
		return -1, closestTime
	} else {
		//fmt.Fprintln(os.Stderr, "Success")
	}
	return closestIdx, closestTime
}

func (d *Distribution) FindIdxByTimestampBinarySearch(timestamp int64, thresholds ...time.Duration) (int, int64) {
	var threshold time.Duration = 10 * time.Second
	if len(thresholds) > 0 {
		threshold = thresholds[0]
	}
	// Get within 4hours of the timestamp
	// This is an optimization. the duration of 4hrs is arbitrary
	startIdx, endIdx := d.findIdxWithin(timestamp, 4*time.Hour)
	closestDiff := float64(math.MaxUint64)
	closestIdx := -1
	var closestTime int64
	for idx := startIdx; idx < endIdx; idx++ {
		diff := math.Abs(float64(timestamp - d.Timestamps[idx]))
		if diff < closestDiff {
			closestDiff = diff
			closestIdx = idx
			closestTime = d.Timestamps[idx]
		}
	}
	if closestDiff > float64(threshold) {
		//fmt.Fprintln(os.Stderr, "Closest index is off by more than 2 seconds:", startIdx, endIdx)
		return -1, closestTime
	} else {
		//fmt.Fprintln(os.Stderr, "Success")
	}
	return closestIdx, closestTime
}

func (d *Distribution) findIdxWithin(timestamp int64, duration time.Duration) (int, int) {
	return d.__findIdxWithin(timestamp, duration, 0, len(d.Timestamps))
}

func (d *Distribution) __findIdxWithin(timestamp int64, duration time.Duration, startIdx int, endIdx int) (int, int) {
	midPt := (startIdx + endIdx) / 2
	if midPt == startIdx || midPt == endIdx {
		return startIdx, endIdx
	}

	midTime := d.Timestamps[midPt]
	diff := time.Duration(timestamp - midTime)
	switch {
	case diff > duration:
		// midTime is too far before timestamp
		return d.__findIdxWithin(timestamp, duration, midPt, endIdx)
	case diff < 0 && math.Abs(float64(diff)) > float64(duration):
		// midTime is too far after timestamp
		return d.__findIdxWithin(timestamp, duration, startIdx, midPt)
	default:
		return startIdx, endIdx
	}
}

func (d *Distribution) Probability(temp int32, op ComparisonOperator) float64 {
	ltSum := int64(0)
	eqSum := int64(0)
	gtSum := int64(0)
	for k, v := range d.Distribution {
		if k < temp {
			ltSum += v
		} else if k == temp {
			eqSum += v
		} else if k > temp {
			gtSum += v
		}
	}
	var numerator float64
	var denominator = float64(ltSum + gtSum + eqSum)

	switch op {
	case LT:
		numerator = float64(ltSum)
	case LE:
		numerator = float64(ltSum + eqSum)
	case EQ:
		numerator = float64(eqSum)
	case GE:
		numerator = float64(eqSum + gtSum)
	case GT:
		numerator = float64(gtSum)
	case NE:
		numerator = float64(ltSum + gtSum)
	default:
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Unimplemented comparison operator: %v", op))
		os.Exit(-1)
	}
	return numerator / denominator
}

func (d *Distribution) IsFull() bool {
	return d.isFull
}

func (d *Distribution) IdxForDuration(duration time.Duration) int {
	if duration > d.Period {
		return -1
	}

	start := d.Timestamps[0]
	for idx := 0; idx < len(d.Timestamps); idx++ {
		if time.Duration(d.Timestamps[idx]-start) >= duration {
			return idx
		}
	}
	return -1
}

func (dist *Distribution) FindCurve(period time.Duration, curveThreshold int32) TemperatureCurve {
	idx := len(dist.Timestamps) - 1

	return dist.FindCurveByIdx(idx, period, curveThreshold)
}

func (dist *Distribution) FindCurveByIdx(idx int, period time.Duration, curveThreshold int32) TemperatureCurve {
	var startTimestamp, endTimestamp int64
	var startTemp, endTemp int32

	endTimestamp = dist.Timestamps[idx]
	endTemp = dist.Temps[idx]

	valid := false
	for idx >= 0 {
		startTimestamp = dist.Timestamps[idx]
		if time.Duration(endTimestamp-startTimestamp) >= period {
			valid = true
			break
		} else {
			idx--
		}
	}

	if !valid {
		// Not enough periods
		return CURVE_UNKNOWN
	}

	startTemp = dist.Temps[idx]

	if startTemp > endTemp+curveThreshold {
		// Was startTemp higher than endTemp?
		// If yes, we were cooling down
		return COOLING_DOWN
	} else if startTemp < endTemp-curveThreshold {
		// Was startTemp lower than endTemp?
		// If yes, we were heating up
		return HEATING_UP
	} else {
		return CURVE_UNKNOWN
	}
}

func (dist *Distribution) FindFirstCurve(period time.Duration, curveThreshold int32) (curveEndIdx int, curve TemperatureCurve) {
	curveEndIdx = -1
	curve = CURVE_UNKNOWN

	for i := 0; i < len(dist.Timestamps); i++ {
		if curve = dist.FindCurveByIdx(i, period, curveThreshold); curve != CURVE_UNKNOWN {
			curveEndIdx = i
			return
		}
	}
	return
}

func (d *Distribution) AsMsg() *DistributionMsg {

	p := int64(d.Period)
	n := int64(len(d.Temps))

	dm := new(DistributionMsg)

	dm.DeviceId = &d.Boot.DeviceId
	dm.BootId = &d.Boot.BootId
	dm.Temps = d.Temps
	dm.Timestamps = make([]int64, 0)
	dm.Period = &p
	dm.IsFull = &d.isFull
	dm.NumEntries = &n

	for _, t := range d.Timestamps {
		dm.Timestamps = append(dm.Timestamps, t)
	}
	for _, t := range d.Temps {
		dm.Temps = append(dm.Temps, t)
	}
	return dm
}
