package trackers

import (
	"strings"

	"github.com/shaseley/phonelab-go"
)

type CpuState int

const (
	FREQUENCY_STATE_UNKNOWN int = 0
)

const (
	CPU_STATE_UNKNOWN CpuState = -1
	CPU_OFFLINE       CpuState = 0
	CPU_ONLINE        CpuState = 1
)

type CpuLineType int

const (
	FREQUENCY_LINE_TYPE CpuLineType = iota
	HOTPLUG_LINE_TYPE   CpuLineType = iota
)

type CpuTrackerData struct {
	Cpu              int
	CpuState         CpuState
	Frequency        int
	FrequencyLogline *phonelab.Logline
	CpuStateLogline  *phonelab.Logline
}

type CpuTracker struct {
	*Tracker
	Exclusive    bool
	CurrentState map[int]*CpuTrackerData
	Callback     func(cpu int, lineType CpuLineType, logline *phonelab.Logline)
	TrackerFunc  LoglineTrackerFunc
}

func (ct *CpuTracker) LastLogline(cpu int) *phonelab.Logline {
	fLogline := ct.CurrentState[cpu].FrequencyLogline
	csLogline := ct.CurrentState[cpu].CpuStateLogline

	if fLogline != nil {
		if csLogline != nil {
			if fLogline.Datetime.After(csLogline.Datetime) {
				return fLogline
			} else {
				return csLogline
			}
		}
		return fLogline
	}
	return csLogline
}

func NewCpuTracker(tracker *Tracker) (cpuTracker *CpuTracker) {
	if tracker == nil {
		tracker = New()
	}
	cpuTracker = new(CpuTracker)
	cpuTracker.Tracker = tracker
	cpuTracker.CurrentState = make(map[int]*CpuTrackerData)
	cpuTracker.Callback = nil

	trackerFunc := func(logline *phonelab.Logline) bool {

		switch t := logline.Payload.(type) {
		case phonelab.TraceInterface:
			var cpu int
			var ctd *CpuTrackerData
			switch t.TraceTag() {
			case "cpu_frequency":
				cf := t.(*phonelab.CpuFrequency)
				cpu = cf.CpuId
				if _, ok := cpuTracker.CurrentState[cpu]; !ok {
					cpuTracker.CurrentState[cpu] = new(CpuTrackerData)
					cpuTracker.CurrentState[cpu].Cpu = cpu
					cpuTracker.CurrentState[cpu].CpuState = CPU_STATE_UNKNOWN
					cpuTracker.CurrentState[cpu].Frequency = FREQUENCY_STATE_UNKNOWN
				}

				if cpuTracker.Callback != nil {
					cpuTracker.Callback(cpu, FREQUENCY_LINE_TYPE, logline)
				}

				ctd = cpuTracker.CurrentState[cpu]
				if ctd.CpuState != CPU_ONLINE {
					// This CPU is clearly up
					ctd.CpuStateLogline = logline
					ctd.CpuState = CPU_ONLINE
				}

				ctd.Frequency = cf.State
				ctd.FrequencyLogline = logline
			case "sched_cpu_hotplug":
				sch := t.(*phonelab.SchedCpuHotplug)
				cpu = sch.Cpu
				if _, ok := cpuTracker.CurrentState[cpu]; !ok {
					cpuTracker.CurrentState[cpu] = new(CpuTrackerData)
					cpuTracker.CurrentState[cpu].Cpu = cpu
					cpuTracker.CurrentState[cpu].CpuState = CPU_STATE_UNKNOWN
					cpuTracker.CurrentState[cpu].Frequency = FREQUENCY_STATE_UNKNOWN
				}
				if cpuTracker.Callback != nil {
					cpuTracker.Callback(cpu, HOTPLUG_LINE_TYPE, logline)
				}
				ctd = cpuTracker.CurrentState[cpu]
				if strings.Compare(sch.State, "offline") == 0 && sch.Error == 0 {
					// This core just went offline
					ctd.CpuState = CPU_OFFLINE
					ctd.CpuStateLogline = logline
				} else if strings.Compare(sch.State, "online") == 0 && sch.Error == 0 {
					ctd.CpuState = CPU_ONLINE
					ctd.CpuStateLogline = logline
				}
			}
		}
		return true
	}
	cpuTracker.TrackerFunc = trackerFunc
	tracker.AddTracker(trackerFunc)
	return cpuTracker
}
