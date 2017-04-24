package trackers

import (
	"fmt"
	"strings"
)

type FgBgState int

const (
	FgBgUnknown FgBgState = 1 << iota
	Foreground  FgBgState = 1 << iota
	Background  FgBgState = 1 << iota
)

type FgBgTracker struct {
	*Filter
	CurrentState          FgBgState
	TrackerState          FgBgState
	LastForegroundLogline *phonelab.Logline
	LastBackgroundLogline *phonelab.Logline
	LastStateLogline      *phonelab.Logline
	lastBgTime            float64
	Exclusive             bool
	BgDelaySec            float64
	Callback              func(line string, oldState FgBgState, newState FgBgState)
	TrackerFunc           LoglineFilter
	ForegroundTime        float64
	BackgroundTime        float64
}

func NewFgBgTracker(filter *Filter) (fgbgTracker *FgBgTracker) {
	fgbgTracker = new(FgBgTracker)
	fgbgTracker.Filter = filter
	fgbgTracker.Exclusive = false
	fgbgTracker.CurrentState = FgBgUnknown
	fgbgTracker.lastBgTime = 0.0
	fgbgTracker.BgDelaySec = 1.0
	shouldSwitchToBg := false

	fgbgTracker.ForegroundTime = 0.0
	fgbgTracker.BackgroundTime = 0.0

	filterFunc := func(logline *phonelab.Logline) bool {
		result := false

		oldState := fgbgTracker.CurrentState
		if strings.Contains(logline.Line, "phonelab_proc_foreground:") {
			trace := phonelab.ParseTraceFromLoglinePayload(logline)
			if trace == nil {
				panic(fmt.Sprintf("Trace is nil: %v", logline.Line))
			}

			switch trace.Tag() {
			case "phonelab_proc_foreground":
				ppf := trace.(*phonelab.PhonelabProcForeground)
				if ppf.Pid != 0 {
					oldState = fgbgTracker.CurrentState
					fgbgTracker.CurrentState = Foreground
					shouldSwitchToBg = false
					if fgbgTracker.Callback != nil {
						fgbgTracker.Callback(logline.Line, oldState, fgbgTracker.CurrentState)
					}
					// Update last state only after callback
					fgbgTracker.LastForegroundLogline = logline
					fgbgTracker.LastStateLogline = logline
				} else {
					// Just store the time. Once enough time has elapsed,
					// we will change state to background
					fgbgTracker.lastBgTime = logline.TraceTime
					shouldSwitchToBg = true
				}
			}
			result = true
		}
		// If current state is background and time elapsed is > BgDelaySec, then set background
		if shouldSwitchToBg {
			if logline.TraceTime-fgbgTracker.lastBgTime > fgbgTracker.BgDelaySec {
				oldState = fgbgTracker.CurrentState
				fgbgTracker.CurrentState = Background
				shouldSwitchToBg = false
				if fgbgTracker.Callback != nil {
					fgbgTracker.Callback(logline.Line, oldState, fgbgTracker.CurrentState)
				}
				fgbgTracker.LastBackgroundLogline = logline
				fgbgTracker.LastStateLogline = logline
			}
		}

		if fgbgTracker.Exclusive {
			return result
		}

		if fgbgTracker.TrackerState&fgbgTracker.CurrentState != 0 {
			return true
		}
		return false
	}
	fgbgTracker.TrackerFunc = filterFunc
	filter.AddFilter(filterFunc)
	return fgbgTracker
}
