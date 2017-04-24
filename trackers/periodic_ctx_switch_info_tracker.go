package trackers

import "github.com/shaseley/phonelab-go"

type PeriodicCtxSwitchInfoTracker struct {
	*Tracker
	CtxSwitchInfo map[int]*PeriodicCtxSwitchInfo
	Callback      func(ctxSwitchInfo *PeriodicCtxSwitchInfo)
	TrackerFunc   LoglineTrackerFunc
}

func NewPeriodicCtxSwitchInfoTracker(tracker *Tracker) (pcsiTracker *PeriodicCtxSwitchInfoTracker) {
	if tracker == nil {
		tracker = New()
	}
	pcsiTracker = new(PeriodicCtxSwitchInfoTracker)
	pcsiTracker.Tracker = tracker
	pcsiTracker.CtxSwitchInfo = make(map[int]*PeriodicCtxSwitchInfo)
	pcsiTracker.Callback = nil

	trackerFunc := func(logline *phonelab.Logline) bool {
		switch t := logline.Payload.(type) {
		case phonelab.TraceInterface:
			var cpu int
			switch t.TraceTag() {
			case "phonelab_periodic_ctx_switch_marker":
				ppcsm := t.(*phonelab.PhonelabPeriodicCtxSwitchMarker)
				cpu = ppcsm.Cpu
				switch ppcsm.State {
				case phonelab.PPCSMBegin:
					if v, ok := pcsiTracker.CtxSwitchInfo[cpu]; v == nil || !ok {
						pcsiTracker.CtxSwitchInfo[cpu] = &PeriodicCtxSwitchInfo{}
						pcsiTracker.CtxSwitchInfo[cpu].Info = make([]*phonelab.Logline, 0)
					} else if v != nil {
						//fmt.Fprintln(os.Stderr, "Start logline when already started")
						//fmt.Fprintln(os.Stderr, logline.Line)
						//os.Exit(-1)
						return true
					}
					pcsiTracker.CtxSwitchInfo[cpu].Start = ppcsm
				case phonelab.PPCSMEnd:
					if v, ok := pcsiTracker.CtxSwitchInfo[cpu]; !ok || v == nil {
						// End marker without begin..ignore
						return true
					}
					pcsi := pcsiTracker.CtxSwitchInfo[cpu]
					pcsi.End = ppcsm
					if pcsiTracker.Callback != nil {
						pcsiTracker.Callback(pcsi)
					}
					pcsiTracker.CtxSwitchInfo[cpu] = nil
				}
			case "phonelab_periodic_ctx_switch_info":
				ppcsi := t.(*phonelab.PhonelabPeriodicCtxSwitchInfo)
				cpu = ppcsi.Cpu
				if v, ok := pcsiTracker.CtxSwitchInfo[cpu]; v == nil || !ok {
					// Info line without begin marker.. ignore
					return true
				}
				pcsiTracker.CtxSwitchInfo[cpu].Info = append(pcsiTracker.CtxSwitchInfo[cpu].Info, logline)
			}
		}
		return true
	}
	pcsiTracker.TrackerFunc = trackerFunc
	tracker.AddTracker(trackerFunc)
	return pcsiTracker
}
