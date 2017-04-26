package libphonelab

import (
	"math"
	"time"

	"github.com/gurupras/go_cpuprof/post_processing"
	"github.com/shaseley/phonelab-go"
)

type AbstractDistribution struct {
	Data        []interface{}
	Timestamps  []int64 // Ensure these are in nanoseconds
	StartTime   *int64
	LastTime    *int64
	LastData    interface{}
	LastLogline *phonelab.Logline
	Period      time.Duration
	isFull      bool
}

func NewAbstractDistribution(boot *post_processing.Boot, Period time.Duration) *AbstractDistribution {
	distribution := new(AbstractDistribution)
	distribution.Period = Period
	distribution.Data = make([]interface{}, 0)
	distribution.Timestamps = make([]int64, 0)
	return distribution
}

func (d *AbstractDistribution) Update(data interface{}, t interface{}) {
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
		for idx = 0; idx < len(d.Data); idx++ {
			oldTimestamp := d.Timestamps[idx]

			if time.Duration(timestamp-oldTimestamp) <= d.Period {
				break
			}
		}
		// Shorten arrays
		d.Data = d.Data[idx:]
		d.Timestamps = d.Timestamps[idx:]

	}
	// Now add the new data
	d.Data = append(d.Data, data)
	// And new timestamp
	d.Timestamps = append(d.Timestamps, timestamp)

	if d.StartTime == nil {
		d.StartTime = &d.Timestamps[0]
	}
	d.LastTime = &d.Timestamps[len(d.Timestamps)-1]
	d.LastData = &d.Data[len(d.Data)-1]
}

func (d *AbstractDistribution) FindIdxByTime(tm time.Time, thresholds ...time.Duration) (int, int64) {
	return d.FindIdxByTimeLinear(tm, thresholds...)
}

func (d *AbstractDistribution) FindIdxByTimeLinear(tm time.Time, thresholds ...time.Duration) (int, int64) {
	timestamp := tm.UnixNano()
	return d.FindIdxByTimestampLinear(timestamp, thresholds...)
}

func (d *AbstractDistribution) FindIdxByTimeBinarySearch(tm time.Time, thresholds ...time.Duration) (int, int64) {
	timestamp := tm.UnixNano()
	return d.FindIdxByTimestampBinarySearch(timestamp, thresholds...)
}

func (d *AbstractDistribution) FindIdxByTimestamp(timestamp int64, thresholds ...time.Duration) (int, int64) {
	return d.FindIdxByTimestampLinear(timestamp, thresholds...)
}

func (d *AbstractDistribution) FindIdxByTimestampLinear(timestamp int64, thresholds ...time.Duration) (int, int64) {
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

func (d *AbstractDistribution) FindIdxByTimestampBinarySearch(timestamp int64, thresholds ...time.Duration) (int, int64) {
	var threshold time.Duration = 10 * time.Second
	if len(thresholds) > 0 {
		threshold = thresholds[0]
	}
	// Get within 40minutes of the timestamp
	// This is an optimization. the duration of 40 minutes is arbitrary
	startIdx, endIdx := d.findIdxWithin(timestamp, 40*time.Minute)
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

func (d *AbstractDistribution) findIdxWithin(timestamp int64, duration time.Duration) (int, int) {
	return d.__findIdxWithin(timestamp, duration, 0, len(d.Timestamps))
}

func (d *AbstractDistribution) __findIdxWithin(timestamp int64, duration time.Duration, startIdx int, endIdx int) (int, int) {
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
