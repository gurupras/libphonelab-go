package algorithms

import (
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
)

type Algorithm interface {
	Process(alarm *alarms.DeliverAlarmsLocked, triggerTemp int32, temps []int32, timestamps []int64, distribution *trackers.Distribution) int32
	Name() string
}
