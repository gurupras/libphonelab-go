package trackers

import (
	"strings"

	"github.com/shaseley/phonelab-go"
)

//////////////////////////////////////////////////////////////////////////////
// TODO: Should this move?

type PeriodicCtxSwitchInfo struct {
	Start *phonelab.PhonelabPeriodicCtxSwitchMarker
	Info  []*phonelab.Logline
	End   *phonelab.PhonelabPeriodicCtxSwitchMarker
}

func (pcsi *PeriodicCtxSwitchInfo) TotalTime() int64 {
	total_time := int64(0)
	for _, line := range pcsi.Info {
		info := line.Payload.(*phonelab.PhonelabPeriodicCtxSwitchInfo)
		total_time += info.Rtime
	}
	return total_time
}

func (pcsi *PeriodicCtxSwitchInfo) Busyness() float64 {
	total_time := pcsi.TotalTime()
	busy_time := int64(0)

	if total_time == 0 {
		return 0.0
	}

	for _, line := range pcsi.Info {
		info := line.Payload.(*phonelab.PhonelabPeriodicCtxSwitchInfo)
		if !strings.Contains(info.Comm, "swapper") {
			busy_time += info.Rtime
		}
	}
	return float64(busy_time) / float64(total_time)
}

func (pcsi *PeriodicCtxSwitchInfo) FgBusyness() float64 {
	total_time := pcsi.TotalTime()
	busy_time := int64(0)

	if total_time == 0 {
		return 0.0
	}

	for _, line := range pcsi.Info {
		info := line.Payload.(*phonelab.PhonelabPeriodicCtxSwitchInfo)
		if !strings.Contains(info.Comm, "swapper") {
			busy_time += info.Rtime
			busy_time -= info.BgRtime
		}
	}
	return float64(busy_time) / float64(total_time)
}

func (pcsi *PeriodicCtxSwitchInfo) BgBusyness() float64 {
	total_time := pcsi.TotalTime()
	busy_time := int64(0)

	if total_time == 0 {
		return 0.0
	}

	for _, line := range pcsi.Info {
		info := line.Payload.(*phonelab.PhonelabPeriodicCtxSwitchInfo)
		if !strings.Contains(info.Comm, "swapper") {
			busy_time += info.BgRtime
		}
	}
	return float64(busy_time) / float64(total_time)
}
