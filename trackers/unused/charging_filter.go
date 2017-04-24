package trackers

import (
	"fmt"
	"os"
	"strings"
)

type ChargingState int

const (
	HEALTHD_CHARGE_STATE_UNKNOWN   ChargingState = 1 << iota
	HEALTHD_CHARGE_STATE_CHARGING  ChargingState = 1 << iota
	HEALTHD_CHARGE_STATE_UNPLUGGED ChargingState = 1 << iota
	HEALTHD_CHARGE_STATE_ALL       ChargingState = HEALTHD_CHARGE_STATE_UNKNOWN | HEALTHD_CHARGE_STATE_CHARGING | HEALTHD_CHARGE_STATE_UNPLUGGED
)

type ChargingStateFilter struct {
	*Filter
	CurrentState        ChargingState
	FilterState         ChargingState
	Exclusive           bool
	StateChangeCallback func(line string)
	FilterFunc          LoglineFilter
}

func NewChargingStateFilter(filter *Filter) *ChargingStateFilter {
	csf := new(ChargingStateFilter)
	csf.Filter = filter
	csf.CurrentState = HEALTHD_CHARGE_STATE_UNKNOWN
	// By default only unplugged
	csf.FilterState = HEALTHD_CHARGE_STATE_UNPLUGGED
	csf.Exclusive = false
	csf.StateChangeCallback = nil

	filterFunc := func(logline *phonelab.Logline) bool {
		result := false
		if strings.Contains(logline.Line, "healthd:") {
			result = true

			healthd := phonelab.ParseHealthdPrintk(logline)
			if healthd == nil {
				fmt.Fprintln(os.Stderr, "Failed to parse line to healthd:", logline.Line)
				return true
			}

			if strings.Compare(healthd.Chg, "") == 0 {
				csf.CurrentState = HEALTHD_CHARGE_STATE_CHARGING
			} else {
				csf.CurrentState = HEALTHD_CHARGE_STATE_UNPLUGGED
			}
			if csf.StateChangeCallback != nil {
				csf.StateChangeCallback(logline.Line)
			}
		}

		if csf.Exclusive {
			return result
		}

		if csf.CurrentState&csf.FilterState != 0 {
			return true
		} else {
			return false
		}
	}
	csf.FilterFunc = filterFunc
	filter.AddFilter(filterFunc)
	return csf
}
