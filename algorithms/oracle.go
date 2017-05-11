package algorithms

import (
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
)

type Oracle struct {
}

func (o *Oracle) Process(alarm *alarms.DeliverAlarmsLocked, triggerTemp int32, temps []int32, timestamps []int64, distribution *trackers.Distribution) int32 {
	min := int32(100000)

	for idx := 0; idx < len(temps); idx++ {
		if temps[idx] < min {
			min = temps[idx]
		}
	}
	return min
}

func (a *Oracle) Name() string {
	return "oracle"
}
