package trackers

import "github.com/shaseley/phonelab-go"

type MissingLoglinesTracker struct {
	*Tracker
	CurrentToken int64
	Callbacks    []MissingLoglinesCallback
	TrackerFunc  LoglineTrackerFunc
}

type MissingLoglinesCallback func(logline *phonelab.Logline)

func (mlt *MissingLoglinesTracker) AddCallback(cb MissingLoglinesCallback) {
	mlt.Callbacks = append(mlt.Callbacks, cb)
}

func NewMissingLoglinesTracker(tracker *Tracker) *MissingLoglinesTracker {
	mlt := new(MissingLoglinesTracker)
	mlt.Callbacks = make([]MissingLoglinesCallback, 0)
	mlt.Tracker = tracker
	// By default only unplugged

	trackerFunc := func(logline *phonelab.Logline) bool {
		if mlt.CurrentToken != 0 && logline.LogcatToken > mlt.CurrentToken+1 {
			for _, cb := range mlt.Callbacks {
				cb(logline)
			}
		}
		mlt.CurrentToken = logline.LogcatToken
		return true
	}
	mlt.TrackerFunc = trackerFunc
	tracker.AddTracker(trackerFunc)
	return mlt
}
