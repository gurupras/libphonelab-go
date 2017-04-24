package libphonelab

import (
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/fatih/set"
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
)

type AlarmCpuProcGenerator struct{}

func (t *AlarmCpuProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &AlarmCpuProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type AlarmCpuProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type AlarmCpuData struct {
	*alarms.DeliverAlarmsLocked
	Temps       []int32
	Timestamps  []int64
	TriggerTemp int32
}

type AlarmCpuResult struct {
	*AlarmCpuData
	phonelab.PipelineSourceInfo
}

func NewAlarmCpuData() *AlarmCpuData {
	atd := &AlarmCpuData{}
	atd.Temps = make([]int32, 0)
	atd.Timestamps = make([]int64, 0)
	return atd
}

type SuspendData struct {
	lastSuspendEntry *phonelab.Logline
	lastSuspendExit  *phonelab.Logline
}

func (p *AlarmCpuProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	// XXX: This is expected to be phonelab type
	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	var (
		suspendDataMap   map[*SuspendData][]*phonelab.Logline
		mapLock          sync.Mutex
		lastSuspendEntry *phonelab.Logline
		lastSuspendExit  *phonelab.Logline
	)
	tracker := trackers.New()
	trackers.NewCpuTracker(tracker)
	sleepTracker := trackers.NewSleepTracker(tracker)
	sleepTracker.SuspendEntryCallback = func(logline *phonelab.Logline) {
		lastSuspendEntry = logline
	}

	sleepTracker.SuspendExitCallback = func(logline *phonelab.Logline) {
		lastSuspendExit = logline
		sData := &SuspendData{lastSuspendEntry, lastSuspendExit}
		suspendDataMap[sData] = make([]*phonelab.Logline, 0)
	}

	go func() {
		defer close(outChan)
		wg := sync.WaitGroup{}

		alarmSet := set.NewNonTS()
		var distribution *Distribution

		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}

			// Update all trackers
			tracker.ApplyLogline(ll)

			// Update any suspend
			for sData, lines := range suspendDataMap {
				diff := ll.Datetime.Sub(sData.lastSuspendExit.Datetime)
				if diff > 2*time.Hour {
					// Too long. Phone may have been put on charge
					delete(suspendDataMap, sData)
				} else {
					suspendDataMap[sData] = append(lines, ll)
				}
				// The valid removal of an entry from suspendDataMap happens below
				// in case *phonelab.PowerManagementPrintk
			}

			switch t := ll.Payload.(type) {
			case *phonelab.CpuFrequency:
				break
			case *phonelab.PhonelabNumOnlineCpus:
				break
			case *phonelab.PowerManagementPrintk:
				pmp := t
				if pmp.State == phonelab.PM_SUSPEND_ENTRY {
					// We're entering suspend. Process all
					// existing suspendDataMap entries
					for sData, lines := range suspendDataMap {
						wg.Add(1)
						go func() {
							defer wg.Done()
							processSuspend(deviceId, sData, lines, outChan)
							mapLock.Lock()
							defer mapLock.Unlock()
							delete(suspendDataMap, sData)
						}()
					}
				}

			case *alarms.DeliverAlarmsLocked:
				deliverAlarm := t
				deliverAlarm.Logline = ll
				if deliverAlarm.WindowLength == 0 {
					// Nothing to do
					continue
				}
				if distribution != nil && distribution.IsFull() {
					// Add this alarm to the alarm set
					alarmTempData := NewAlarmCpuData()
					alarmTempData.DeliverAlarmsLocked = deliverAlarm
					alarmSet.Add(alarmTempData)
				}
			}
		}
		wg.Wait()
	}()
	return outChan
}

type AlarmSuspendData struct {
	DeviceId string
	*alarms.DeliverAlarmsLocked
	Busyness  map[string][]float64
	Periods   map[string][]int64
	Frequency map[string][]int
	Duration  int64
	Lines     []string
}

func processSuspend(deviceId string, sData *SuspendData, lines []*phonelab.Logline, outChannel chan interface{}) {
	// Find all alarms that occured within the first 5 seconds of suspend exit
	alarmList := make([]*alarms.DeliverAlarmsLocked, 0)
	startTime := sData.lastSuspendExit.Datetime
	for _, line := range lines {
		if line.Datetime.Sub(startTime) > 5*time.Second {
			break
		}
		dal, _ := alarms.ParseDeliverAlarmsLocked(line)
		if dal != nil {
			alarmList = append(alarmList, dal)
		}
		dal.Logline = line
	}

	if len(alarmList) == 0 {
		// No alarms within the first 5 seconds
		// just return
		return
	}

	// Find all relevant appPids
	pids := set.NewNonTS()
	alarmMap := make(map[int][]*alarms.DeliverAlarmsLocked)

	for _, alarm := range alarmList {
		pids.Add(alarm.AppPid)
		if _, ok := alarmMap[alarm.AppPid]; !ok {
			alarmMap[alarm.AppPid] = make([]*alarms.DeliverAlarmsLocked, 0)
		}
		alarmMap[alarm.AppPid] = append(alarmMap[alarm.AppPid], alarm)
	}

	data := make(map[*alarms.DeliverAlarmsLocked]*AlarmSuspendData)
	lastInfo := make(map[*alarms.DeliverAlarmsLocked]time.Time)

	tracker := trackers.New()
	cpuTracker := trackers.NewCpuTracker(tracker)
	pcsiTracker := trackers.NewPeriodicCtxSwitchInfoTracker(tracker)
	pcsiTracker.Callback = func(ctxSwitchInfo *trackers.PeriodicCtxSwitchInfo) {
		for _, line := range ctxSwitchInfo.Info {
			info := line.Payload.(*phonelab.PhonelabPeriodicCtxSwitchInfo)
			if pids.Has(info.Tgid) {
				for _, alarm := range alarmMap[info.Tgid] {
					if _, ok := data[alarm]; !ok {
						data[alarm] = &AlarmSuspendData{}
						data[alarm].DeviceId = deviceId
						data[alarm].DeliverAlarmsLocked = alarm
						data[alarm].Busyness = make(map[string][]float64)
						data[alarm].Periods = make(map[string][]int64)
						data[alarm].Frequency = make(map[string][]int)
						data[alarm].Lines = make([]string, len(lines))
						for idx, logline := range lines {
							data[alarm].Lines[idx] = logline.Line
						}
					}
					busyness := float64(info.Rtime) / float64(ctxSwitchInfo.TotalTime())
					cpu := fmt.Sprintf("%d", info.Cpu)
					if _, ok := data[alarm].Busyness[cpu]; !ok {
						data[alarm].Busyness[cpu] = make([]float64, 0)
						data[alarm].Periods[cpu] = make([]int64, 0)
						data[alarm].Frequency[cpu] = make([]int, 0)
					}
					data[alarm].Busyness[cpu] = append(data[alarm].Busyness[cpu], busyness)
					data[alarm].Periods[cpu] = append(data[alarm].Periods[cpu], ctxSwitchInfo.TotalTime())
					data[alarm].Frequency[cpu] = append(data[alarm].Frequency[cpu], cpuTracker.CurrentState[info.Cpu].Frequency)
					lastInfo[alarm] = line.Datetime
				}
			}
		}
	}

	for _, line := range lines {
		tracker.ApplyLogline(line)
	}

	for alarm, datetime := range lastInfo {
		data[alarm].Duration = int64(datetime.Sub(alarm.Logline.Datetime))
	}

	for _, suspendData := range data {
		outChannel <- suspendData
	}
}

type AlarmCpuCollector struct {
	sync.Mutex
	outPath          string
	DefaultCollector phonelab.DataCollector
}

type fileContext struct {
	filename string
}

func (c *fileContext) Context() string {
	return c.filename
}

func (c *fileContext) Type() string {
	return "file-context"
}

func (c *AlarmCpuCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	r := data.(*AlarmCpuResult)

	sourceInfo := r.PipelineSourceInfo.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	outdir := filepath.Join(c.outPath, deviceId, "analysis", "alarm_cpu")
	h := md5.New()
	io.WriteString(h, r.DeliverAlarmsLocked.Logline.Line)
	checksum := fmt.Sprintf("%x", h.Sum(nil))
	filename := filepath.Join(outdir, fmt.Sprintf("%v.gz", checksum))

	cc := &fileContext{filename}

	r.PipelineSourceInfo = nil
	c.DefaultCollector.OnData(r, cc)
}

func (c *AlarmCpuCollector) Finish() {
	// Nothing to do here
	c.DefaultCollector.Finish()
}
