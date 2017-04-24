package trackers

import (
	"github.com/gurupras/libphonelab-go/parsers"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type ScreenState int

const (
	SCREEN_STATE_UNKNOWN ScreenState = -1
	SCREEN_STATE_OFF     ScreenState = 0
	SCREEN_STATE_ON      ScreenState = 2
)

type ScreenStateTracker struct {
	*Tracker
	CurrentState         ScreenState
	lastScreenStateEntry *phonelab.Logline
	Callback             func(state ScreenState, logline *phonelab.Logline)
	TrackerFunc          LoglineTrackerFunc
	Log                  bool
}

func NewScreenStateTracker(tracker *Tracker) (screenStateTracker *ScreenStateTracker) {
	if tracker == nil {
		tracker = New()
	}
	screenStateTracker = new(ScreenStateTracker)
	screenStateTracker.Tracker = tracker
	screenStateTracker.CurrentState = SCREEN_STATE_UNKNOWN
	screenStateTracker.Callback = nil

	trackerFunc := func(logline *phonelab.Logline) bool {
		switch t := logline.Payload.(type) {
		case *parsers.ScreenState:
			switch t.Mode {
			case 0:
				screenStateTracker.CurrentState = SCREEN_STATE_OFF
			case 2:
				screenStateTracker.CurrentState = SCREEN_STATE_ON
			default:
				log.Errorf("Received unknown screen state value: %v", t.Mode)
				return false
			}
			if screenStateTracker.Callback != nil {
				screenStateTracker.Callback(screenStateTracker.CurrentState, logline)
			}
		}
		return true
	}
	tracker.AddTracker(trackerFunc)
	return screenStateTracker
}
