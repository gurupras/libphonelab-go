package algorithms

import (
	"math"

	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
)

type OptimalStoppingTheory struct {
}

func (o *OptimalStoppingTheory) Process(alarm *alarms.DeliverAlarmsLocked, triggerTemp int32, temps []int32, timestamps []int64, distribution *trackers.Distribution) int32 {
	ostLimit := float64(1.0*100) / math.E
	return o.process(alarm, triggerTemp, temps, timestamps, distribution, ostLimit)
}

func (o *OptimalStoppingTheory) process(alarm *alarms.DeliverAlarmsLocked, triggerTemp int32, temps []int32, timestamps []int64, distribution *trackers.Distribution, limit float64) int32 {
	totalTime := alarm.WindowLength * 1000000
	best := int32(100000)
	state := false
	for idx := 0; idx < len(timestamps); idx++ {
		percentTimeElapsed := float64((timestamps[idx]-(alarm.WhenRtc*1000000))*100) / float64(totalTime)
		if state == false {
			state = percentTimeElapsed >= limit
		}
		switch state {
		case true:
			// Stop at the first value that is better than the one
			// that was found when we were tracking the
			// 'best-so-far'
			if temps[idx] < best {
				return temps[idx]
			}
		case false:
			// We're less than 1/e. Track the best-so-far
			if temps[idx] < best {
				best = temps[idx]
			}
		}
	}
	return temps[len(temps)-1]
}

func (a *OptimalStoppingTheory) Name() string {
	return "optimal-stopping-theory"
}
