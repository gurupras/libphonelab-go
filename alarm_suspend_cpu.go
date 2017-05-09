package libphonelab

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fatih/set"
	"github.com/gurupras/gocommons/gsync"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type SuspendCpuProcGenerator struct{}

func (t *SuspendCpuProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &SuspendCpuProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type SuspendCpuProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

var scpSem = gsync.NewSem(8) // Max 8 concurrent bootIDs
func (p *SuspendCpuProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	// XXX: This is expected to be phonelab type
	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	loglineDistribution := NewAbstractDistribution(nil, 2*time.Hour)

	var (
		suspendDataMap  = make(map[*SuspendData][]*phonelab.Logline)
		mapLock         sync.Mutex
		lastSuspendExit *phonelab.Logline
		data            = NewDateBusynessData()
	)

	tracker := trackers.New()
	cpuTracker := trackers.NewCpuTracker(tracker)

	missingLoglinesTracker := trackers.NewMissingLoglinesTracker(tracker)
	missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
		// We cannot be sure if we missed a suspend logline
		for k := range suspendDataMap {
			delete(suspendDataMap, k)
		}
	})

	dayTracker := trackers.NewDayTracker(tracker)
	dayTracker.Callback = func(logline *phonelab.Logline) {
		//log.Infof("%v: dayTrack=%v", sourceInfo.BootId, logline.Datetime)
		date := time.Unix(0, dayTracker.DayStartLogline.Datetime.UnixNano())
		dateStr := fmt.Sprintf("%04d%02d%02d", date.Year(), int(date.Month()), date.Day())
		data.Date = dateStr
		outChan <- data
		data = NewDateBusynessData()
	}

	chargingStateTracker := trackers.NewChargingStateTracker(tracker)
	chargingStateTracker.Callback = func(state trackers.ChargingState, logline *phonelab.Logline) {
		if state == trackers.CHARGE_STATE_CHARGING {
			// Clear map. Device began charging
			for k := range suspendDataMap {
				delete(suspendDataMap, k)
			}
		}
	}
	missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
		chargingStateTracker.CurrentState = trackers.CHARGE_STATE_UNKNOWN
	})

	screenStateTracker := trackers.NewScreenStateTracker(tracker)
	missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
		screenStateTracker.CurrentState = trackers.SCREEN_STATE_UNKNOWN
	})
	screenStateTracker.Callback = func(state trackers.ScreenState, logline *phonelab.Logline) {
		if state == trackers.SCREEN_STATE_ON {
			// Clear map. The device screen turned on
			for k := range suspendDataMap {
				delete(suspendDataMap, k)
			}
		}
	}
	missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
		screenStateTracker.CurrentState = trackers.SCREEN_STATE_UNKNOWN
	})

	trackers.NewCpuTracker(tracker)
	sleepTracker := trackers.NewSleepTracker(tracker)
	missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
		//log.Infof("Resetting sleep state")
		sleepTracker.CurrentState = trackers.SUSPEND_STATE_UNKNOWN
	})
	sleepTracker.SuspendEntryCallback = func(logline *phonelab.Logline) {
		if sleepTracker.CurrentState != trackers.SUSPEND_STATE_AWAKE {
			// Going to sleep when not awake??
			// Delete existing entries
			for k := range suspendDataMap {
				delete(suspendDataMap, k)
			}
			return
		}
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
					bd := processSuspend1(deviceId, sData, loglineDistribution.Data[startIdx:])
					data.Update(bd)
				} else {
					log.Warnf("Did not find logline near suspend exit")
				}
				deleteSet.Add(sData)
			}(sData)
		}
		wg.Wait()
		for _, obj := range deleteSet.List() {
			sData := obj.(*SuspendData)
			delete(suspendDataMap, sData)
		}

	}

	sleepTracker.SuspendExitCallback = func(logline *phonelab.Logline) {
		if screenStateTracker.CurrentState == trackers.SCREEN_STATE_OFF && chargingStateTracker.CurrentState == trackers.CHARGE_STATE_UNPLUGGED {
			lastSuspendExit = logline
			sData := &SuspendData{lastSuspendExit, cpuTracker.CloneCurrentState(), nil}
			suspendDataMap[sData] = make([]*phonelab.Logline, 0)
			//log.Infof("Added new suspend entry")
		}
	}

	scpSem.P()
	go func() {
		defer close(outChan)
		defer scpSem.V()
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

func processSuspend1(deviceId string, sData *SuspendData, loglines []interface{}) *BusynessData {
	data := NewBusynessData()

	tracker := trackers.New()
	cpuTracker := trackers.NewCpuTracker(tracker)
	cpuTracker.CurrentState = sData.WakeupCpuState
	pcsiTracker := trackers.NewPeriodicCtxSwitchInfoTracker(tracker)
	pcsiTracker.Callback = func(ctxSwitchInfo *trackers.PeriodicCtxSwitchInfo) {
		cpu := ctxSwitchInfo.Start.Cpu
		cpuStr := fmt.Sprintf("%d", cpu)
		if _, ok := data.Busyness[cpuStr]; !ok {
			data.Busyness[cpuStr] = make([]float64, 0)
			data.Periods[cpuStr] = make([]int64, 0)
			data.Frequency[cpuStr] = make([]int, 0)
		}
		data.Busyness[cpuStr] = append(data.Busyness[cpuStr], ctxSwitchInfo.Busyness())
		data.Periods[cpuStr] = append(data.Periods[cpuStr], ctxSwitchInfo.TotalTime())
		data.Frequency[cpuStr] = append(data.Frequency[cpuStr], cpuTracker.CurrentState[cpu].Frequency)
	}

	var lastLogline *phonelab.Logline
	for _, obj := range loglines {
		logline := obj.(*phonelab.Logline)
		lastLogline = logline
		tracker.ApplyLogline(logline)
	}

	data.Duration = loglines[0].(*phonelab.Logline).Datetime.Sub(lastLogline.Datetime).Nanoseconds()
	return data
}

type DateBusynessData struct {
	Date string
	*BusynessData
}

func NewDateBusynessData() *DateBusynessData {
	d := &DateBusynessData{}
	d.BusynessData = NewBusynessData()
	return d
}

type SuspendCpuCollector struct {
	*phonelab.DefaultCollector
	sync.Mutex
	wg sync.WaitGroup
	*gsync.Semaphore
	deviceDateMap map[string]map[string][]*BusynessData
	idx           uint64
}

func (c *SuspendCpuCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	c.wg.Add(1)
	c.P()
	go func() {
		defer c.wg.Done()
		defer c.V()

		sourceInfo := info.(*phonelab.PhonelabSourceInfo)
		deviceId := sourceInfo.DeviceId

		if _, ok := c.deviceDateMap[deviceId]; !ok {
			c.Lock()
			c.deviceDateMap[deviceId] = make(map[string][]*BusynessData)
			c.Unlock()
		}

		deviceData := c.deviceDateMap[deviceId]
		r := data.(*DateBusynessData)
		if _, ok := deviceData[r.Date]; !ok {
			deviceData[r.Date] = make([]*BusynessData, 0)
		}
		deviceData[r.Date] = append(deviceData[r.Date], r.BusynessData)
	}()
}

func (c *SuspendCpuCollector) Finish() {
	// Nothing to do here
	c.wg.Wait()

	for deviceId, deviceData := range c.deviceDateMap {
		for date, data := range deviceData {
			path := filepath.Join(deviceId, "analysis", "suspend_cpu", fmt.Sprintf("%v", date))
			info := &CustomInfo{path, "custom"}
			c.DefaultCollector.OnData(data, info)
		}
	}
}
