package libphonelab

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/set"
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	"github.com/shaseley/phonelab-go/serialize"
	log "github.com/sirupsen/logrus"
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

type SuspendData struct {
	lastSuspendExit  *phonelab.Logline
	lastSuspendEntry *phonelab.Logline
}

func (p *AlarmCpuProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	// XXX: This is expected to be phonelab type
	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	loglineDistribution := NewAbstractDistribution(nil, 2*time.Hour)

	var (
		suspendDataMap  = make(map[*SuspendData][]*phonelab.Logline)
		mapLock         sync.Mutex
		lastSuspendExit *phonelab.Logline
	)

	tracker := trackers.New()
	trackers.NewCpuTracker(tracker)
	sleepTracker := trackers.NewSleepTracker(tracker)
	sleepTracker.SuspendEntryCallback = func(logline *phonelab.Logline) {
		// We're entering suspend. Process all
		// existing suspendDataMap entries
		deleteSet := set.New()
		wg := sync.WaitGroup{}
		for sData, _ := range suspendDataMap {
			sData.lastSuspendEntry = logline
			_ = mapLock
			// Find the proper subset of lines
			wg.Add(1)
			go func(sData *SuspendData) {
				defer wg.Done()
				startIdx, _ := loglineDistribution.FindIdxByTimeBinarySearch(sData.lastSuspendExit.Datetime, 10*time.Second)
				if startIdx != -1 {
					deleteSet.Add(sData)
					processSuspend(deviceId, sData, loglineDistribution.Data[startIdx:], outChan)
				} else {
					log.Warnf("Did not find logline near suspend exit")
				}
			}(sData)
		}
		wg.Wait()
		for _, obj := range deleteSet.List() {
			sData := obj.(*SuspendData)
			delete(suspendDataMap, sData)
		}

	}

	sleepTracker.SuspendExitCallback = func(logline *phonelab.Logline) {
		lastSuspendExit = logline
		sData := &SuspendData{lastSuspendExit, nil}
		suspendDataMap[sData] = make([]*phonelab.Logline, 0)
		//log.Infof("Added new suspend entry")
	}

	go func() {
		defer close(outChan)

		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}

			// Update all trackers
			tracker.ApplyLogline(ll)

			// Update any suspend
			loglineDistribution.Update(ll, ll.Datetime.UnixNano())
			for sData, _ := range suspendDataMap {
				diff := ll.Datetime.Sub(sData.lastSuspendExit.Datetime)
				if diff > 2*time.Hour {
					// Too long. Phone may have been put on charge
					//log.Warnf("Too long. Dropping suspend data")
					delete(suspendDataMap, sData)
				}
				// The valid removal of an entry from suspendDataMap happens below
				// in case *phonelab.PowerManagementPrintk
			}

			switch t := ll.Payload.(type) {
			case *phonelab.CpuFrequency:
				_ = t
				break
			case *phonelab.PhonelabNumOnlineCpus:
				break
			case *phonelab.PowerManagementPrintk:
				break
			}
		}
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

func processSuspend(deviceId string, sData *SuspendData, loglines []interface{}, outChannel chan interface{}) {
	// Find all alarms that occured within the first 5 seconds of suspend exit
	alarmList := make([]*alarms.DeliverAlarmsLocked, 0)
	startTime := sData.lastSuspendExit.Datetime
	for _, obj := range loglines {
		logline := obj.(*phonelab.Logline)
		if logline.Datetime.Sub(startTime) > 5*time.Second {
			break
		}
		if !strings.Contains(logline.Line, "deliverAlarmsLocked()") {
			continue
		}
		dal, _ := alarms.ParseDeliverAlarmsLocked(logline)
		if dal != nil {
			alarmList = append(alarmList, dal)
		}
		dal.Logline = logline
	}

	if len(alarmList) == 0 {
		// No alarms within the first 5 seconds
		// just return
		return
	}

	log.Debugf("Found suspend entry with alarms: %v", len(alarmList))

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
						/*
							data[alarm].Lines = make([]string, len(loglines))
							for idx, logline := range loglines {
								data[alarm].Lines[idx] = logline.Line
							}
						*/
					}
					busyness := float64(info.Rtime) / float64(ctxSwitchInfo.TotalTime())
					cpu := fmt.Sprintf("%d", info.Cpu)
					if _, ok := data[alarm].Busyness[cpu]; !ok {
						data[alarm].Busyness[cpu] = make([]float64, 0)
						data[alarm].Periods[cpu] = make([]int64, 0)
						data[alarm].Frequency[cpu] = make([]int, 0)
					}
					if _, ok := cpuTracker.CurrentState[info.Cpu]; !ok {
						// We don't have data yet for this CPU
						continue
					}
					data[alarm].Busyness[cpu] = append(data[alarm].Busyness[cpu], busyness)
					data[alarm].Periods[cpu] = append(data[alarm].Periods[cpu], ctxSwitchInfo.TotalTime())
					data[alarm].Frequency[cpu] = append(data[alarm].Frequency[cpu], cpuTracker.CurrentState[info.Cpu].Frequency)
					lastInfo[alarm] = line.Datetime
				}
			}
		}
	}

	for _, obj := range loglines {
		logline := obj.(*phonelab.Logline)
		tracker.ApplyLogline(logline)
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
	outPath    string
	Serializer serialize.Serializer
}

type fileContext struct {
	filename string
}

func (c *AlarmCpuCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	go func() {
		r := data.(*AlarmSuspendData)

		sourceInfo := info.(*phonelab.PhonelabSourceInfo)
		deviceId := sourceInfo.DeviceId

		h := md5.New()
		io.WriteString(h, r.DeliverAlarmsLocked.Logline.Line)
		checksum := fmt.Sprintf("%x", h.Sum(nil))

		u, err := url.Parse(c.outPath)
		if err != nil {
			log.Fatalf("Failed to parse URL from string: %v: %v", c.outPath, err)
		}
		u.Path = filepath.Join(u.Path, deviceId, "analysis", "alarm_cpu", fmt.Sprintf("%v.gz", checksum))
		filename := u.String()

		// XXX: Hack. Set Logline.Payload = nil
		// Otherwise, Logline.Payload refers to
		// deliverAlarmsLocked -> Logline -> deliverAlarmsLocked ->
		// you see where this is going
		r.DeliverAlarmsLocked.Logline.Payload = nil

		c.Serializer.Serialize(r, filename)
	}()
}

func (c *AlarmCpuCollector) Finish() {
	// Nothing to do here
}
