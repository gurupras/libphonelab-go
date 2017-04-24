package trackers

import "github.com/shaseley/phonelab-go"

// Tracker to provide callback every 24 hours
// Can be tweaked to provide callback after arbitrary durations of time
type DayTracker struct {
	*Tracker
	Callback        func(logline *phonelab.Logline)
	TrackerFunc     LoglineTrackerFunc
	DayStartLogline *phonelab.Logline
}

func NewDayTracker(tracker *Tracker) *DayTracker {
	if tracker == nil {
		tracker = New()
	}
	df := new(DayTracker)
	df.Tracker = tracker
	df.Callback = nil

	trackerFunc := func(logline *phonelab.Logline) bool {
		// Always returns true. This is only used for the callback
		if df.DayStartLogline == nil {
			df.DayStartLogline = logline
			goto done
		} else {
			if logline.Datetime.YearDay() != df.DayStartLogline.Datetime.YearDay() && df.Callback != nil {
				df.Callback(logline)
				df.DayStartLogline = logline
			}
		}
	done:
		return true
	}
	df.TrackerFunc = trackerFunc
	tracker.AddTracker(trackerFunc)
	return df
}
