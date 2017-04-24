package trackers

import (
	"fmt"
	"os"

	"github.com/shaseley/phonelab-go"
)

type SuspendState int

const (
	SUSPEND_STATE_UNKNOWN   SuspendState = 1 << iota
	SUSPEND_STATE_SUSPENDED SuspendState = 1 << iota
	SUSPEND_STATE_AWAKE     SuspendState = 1 << iota
)

type SleepTracker struct {
	*Tracker
	Exclusive            bool
	CurrentState         SuspendState
	TrackerState         SuspendState
	lastSuspendEntry     *phonelab.PowerManagementPrintk
	SuspendEntryCallback func(logline *phonelab.Logline)
	SuspendExitCallback  func(logline *phonelab.Logline)
	TrackerFunc          LoglineTrackerFunc
	Log                  bool
}

func NewSleepTracker(tracker *Tracker) (sleepTracker *SleepTracker) {
	if tracker == nil {
		tracker = New()
	}
	sleepTracker = new(SleepTracker)
	sleepTracker.Tracker = tracker
	sleepTracker.CurrentState = SUSPEND_STATE_UNKNOWN
	sleepTracker.TrackerState = SUSPEND_STATE_AWAKE
	sleepTracker.Exclusive = false
	sleepTracker.SuspendEntryCallback = nil
	sleepTracker.SuspendExitCallback = nil
	sleepTracker.Log = false

	trackerFunc := func(logline *phonelab.Logline) bool {
		result := false

		log := func(message string) {
			if sleepTracker.Log {
				fmt.Fprintln(os.Stderr, message)
			}
		}

		result = true

		switch t := logline.Payload.(type) {
		case *phonelab.PowerManagementPrintk:
			pmp := t
			switch pmp.State {
			case phonelab.PM_SUSPEND_ENTRY:
				if sleepTracker.CurrentState == SUSPEND_STATE_SUSPENDED {
					log("Suspend when suspended?")
				}
				sleepTracker.CurrentState = SUSPEND_STATE_SUSPENDED
				if sleepTracker.SuspendEntryCallback != nil {
					sleepTracker.SuspendEntryCallback(logline)
				}
				sleepTracker.lastSuspendEntry = pmp
			case phonelab.PM_SUSPEND_EXIT:
				if sleepTracker.CurrentState != SUSPEND_STATE_SUSPENDED {
					log("Suspend exit when not suspended??")
				} else {
					if sleepTracker.lastSuspendEntry == nil {
						log("Suspend exit when lastSuspendEntry is nil??")
						os.Exit(-1)
					}
				}
				sleepTracker.CurrentState = SUSPEND_STATE_AWAKE
				if sleepTracker.SuspendExitCallback != nil {
					sleepTracker.SuspendExitCallback(logline)
				}
			}
			if sleepTracker.Exclusive {
				return result
			}

			if sleepTracker.CurrentState&sleepTracker.TrackerState != 0 {
				return true
			}
		}
		return false
	}
	tracker.AddTracker(trackerFunc)
	return sleepTracker
}
