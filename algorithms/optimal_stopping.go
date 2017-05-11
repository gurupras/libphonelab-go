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

	totalTime := alarm.WindowLength * 1000000
	best := int32(100000)
	state := false
	for idx := 0; idx < len(timestamps); idx++ {
		percentTimeElapsed := float64((timestamps[idx]-(alarm.WhenRtc*1000000))*100) / float64(totalTime)
		if state == false {
			state = percentTimeElapsed >= ostLimit
		}
		switch state {
		case true:
			// Stop at the first value that is better than the one
			// that was found when we were tracking the
			// 'best-so-far'
			if temps[idx] < best {
				best = temps[idx]
				// Stop
				break
			}
		case false:
			// We're less than 1/e. Track the best-so-far
			if temps[idx] < best {
				best = temps[idx]
			}
		}
	}
	if best == 100000 {
		best = temps[len(temps)-1]
	}
	return best
}

func (a *OptimalStoppingTheory) Name() string {
	return "optimal-stopping-theory"
}
