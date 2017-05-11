package algorithms

import (
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
)

type Android struct {
}

func (a *Android) Process(alarm *alarms.DeliverAlarmsLocked, triggerTemp int32, temps []int32, timestamps []int64, distribution *trackers.Distribution) int32 {
	return triggerTemp
}

func (a *Android) Name() string {
	return "android"
}
