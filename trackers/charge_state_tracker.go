package trackers

import "github.com/shaseley/phonelab-go"

type ChargingState int

const (
	CHARGE_STATE_UNKNOWN   ChargingState = 1 << iota
	CHARGE_STATE_CHARGING  ChargingState = 1 << iota
	CHARGE_STATE_UNPLUGGED ChargingState = 1 << iota
	CHARGE_STATE_ALL       ChargingState = CHARGE_STATE_UNKNOWN | CHARGE_STATE_CHARGING | CHARGE_STATE_UNPLUGGED
)

type ChargingStateTracker struct {
	*Tracker
	CurrentState ChargingState
	Callback     func(state ChargingState, logline *phonelab.Logline)
	TrackerFunc  LoglineTrackerFunc
}

func NewChargingStateTracker(tracker *Tracker) *ChargingStateTracker {
	csf := new(ChargingStateTracker)
	csf.Tracker = tracker
	csf.CurrentState = CHARGE_STATE_UNKNOWN
	// By default only unplugged
	csf.Callback = nil

	trackerFunc := func(logline *phonelab.Logline) bool {
		switch t := logline.Payload.(type) {
		case *phonelab.PLPowerBatteryLog:
			var newState ChargingState
			if t.BatteryProperties.ChargerAcOnline || t.BatteryProperties.ChargerUsbOnline || t.BatteryProperties.ChargerWirelessOnline {
				newState = CHARGE_STATE_CHARGING
			} else {
				newState = CHARGE_STATE_UNPLUGGED
			}
			if newState != csf.CurrentState {
				csf.CurrentState = newState
				if csf.Callback != nil {
					csf.Callback(newState, logline)
				}
			}
		}
		return true
	}
	csf.TrackerFunc = trackerFunc
	tracker.AddTracker(trackerFunc)
	return csf
}
