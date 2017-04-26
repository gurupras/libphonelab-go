package trackers

import (
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type LineFilter func(line string) bool
type LoglineTrackerFunc func(logline *phonelab.Logline) bool

type Tracker struct {
	trackerFuncs []LoglineTrackerFunc
}

func New() *Tracker {
	f := new(Tracker)
	f.trackerFuncs = make([]LoglineTrackerFunc, 0)
	return f
}

func (f *Tracker) AddTracker(filter LoglineTrackerFunc) {
	f.trackerFuncs = append(f.trackerFuncs, filter)
}

func (f *Tracker) AsLineTrackerArray() []LoglineTrackerFunc {
	return f.trackerFuncs
}

func (f *Tracker) Apply(line string) {
	logline, err := phonelab.ParseLogline(line)
	if err != nil {
		log.Fatalf("Failed to parse line: %v: %v", line, err)
		return
	}
	f.ApplyLogline(logline)
}

func (f *Tracker) ApplyLogline(logline *phonelab.Logline) {
	if logline == nil || logline.Payload == nil {
		return
	}
	for _, ffunc := range f.trackerFuncs {
		ffunc(logline)
	}
}
