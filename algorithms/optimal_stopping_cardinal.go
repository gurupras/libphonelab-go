package algorithms

import (
	"math"

	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
)

type OptimalStoppingTheoryCardinal struct {
	OptimalStoppingTheory
}

func (o *OptimalStoppingTheoryCardinal) Process(alarm *alarms.DeliverAlarmsLocked, triggerTemp int32, temps []int32, timestamps []int64, distribution *trackers.Distribution) int32 {
	limit := float64(math.Ceil(math.Sqrt(float64(len(temps)))))
	return o.process(alarm, triggerTemp, temps, timestamps, distribution, limit)
}

func (a *OptimalStoppingTheoryCardinal) Name() string {
	return "optimal-stopping-theory-cardinal"
}
