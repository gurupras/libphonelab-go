package libphonelab

import (
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/fatih/set"
	"github.com/gurupras/gocommons/gsync"
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/parsers"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type AlarmWakelockCpuProcGenerator struct{}

func (t *AlarmWakelockCpuProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &AlarmWakelockCpuProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type AlarmWakelockCpuProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type AlarmWakelockSet struct {
	Acquire *phonelab.Logline
	Release *phonelab.Logline
}

func (p *AlarmWakelockCpuProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	// XXX: This is expected to be phonelab type
	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	pidAlarmMap := make(map[int]set.Interface)
	pidWakelockCount := make(map[int]int)
	pidWakelockMap := make(map[int]*AlarmWakelockSet)
	lockAddrSet := set.NewNonTS()
	wakelockMap := make(map[int64]*AlarmWakelockSet)
	processSet := set.NewNonTS()

	clearData := func() {
		pidAlarmMap = make(map[int]set.Interface)
		pidWakelockCount = make(map[int]int)
		pidWakelockMap = make(map[int]*AlarmWakelockSet)
		lockAddrSet = set.NewNonTS()
		wakelockMap = make(map[int64]*AlarmWakelockSet)
		processSet = set.NewNonTS()
	}

	loglineDistribution := NewAbstractDistribution(nil, 6*time.Hour)

	tracker := trackers.New()
	missingLoglinesTracker := trackers.NewMissingLoglinesTracker(tracker)
	missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
		clearData()
	})
	chargingStateTracker := trackers.NewChargingStateTracker(tracker)
	chargingStateTracker.Callback = func(state trackers.ChargingState, logline *phonelab.Logline) {
		if state == trackers.CHARGE_STATE_CHARGING {
			clearData()
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
			clearData()
		}
	}

	trackers.NewCpuTracker(tracker)

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

			for _, obj := range processSet.List() {
				entry := obj.(*AlarmWakelockSet)
				releaseDatetime := entry.Release.Datetime
				if ll.Datetime.Sub(releaseDatetime).Seconds() > 10 {
					pid := (entry.Acquire.Payload.(*parsers.ThermaPlanWakelock)).Pid

					// Remove one alarm from the set
					if pidAlarmMap[pid] == nil || pidAlarmMap[pid].Size() == 0 {
						log.Warnf("Wakelock release but no alarm")
						processSet.Remove(obj)
						continue
					}
					obj := pidAlarmMap[pid].Pop()
					alarm := obj.(*alarms.DeliverAlarmsLocked)

					// Now find all loglines between this acquire and release
					idx, _ := loglineDistribution.FindIdxByTimestampBinarySearch(entry.Acquire.Datetime.UnixNano(), 5*time.Second)
					if idx == -1 {
						log.Errorf("Failed to find nearest timestamp to wakelock acquire")
						// TODO: Do some cleanup
					} else {
						loglines := loglineDistribution.Data[idx:]
						data := processWakelockBusyness(deviceId, alarm, entry, loglines)
						if data != nil {
							//log.Infof("Got alarm busyness data")
							outChan <- data
						}
					}
					processSet.Remove(obj)
				}
			}
			switch t := ll.Payload.(type) {
			case *phonelab.CpuFrequency:
				_ = t
				break
			case *phonelab.PhonelabNumOnlineCpus:
				break
			case *phonelab.PowerManagementPrintk:
				break
			case *alarms.DeliverAlarmsLocked:
				dal := t
				if dal.WindowLength == 0 {
					//continue
				}
				dal.Logline = ll
				if chargingStateTracker.CurrentState == trackers.CHARGE_STATE_UNPLUGGED && screenStateTracker.CurrentState == trackers.SCREEN_STATE_OFF {
					if pidAlarmMap[dal.AppPid] == nil {
						pidAlarmMap[dal.AppPid] = set.New()
					}
					pidAlarmMap[dal.AppPid].Add(dal)
				}
			case *parsers.ThermaPlanWakelock:
				switch t.Type() {
				case parsers.WAKELOCK_ACQUIRE:
					pid := t.Pid
					if pidAlarmMap[pid] != nil && pidAlarmMap[pid].Size() > 0 {
						lockAddrSet.Add(t.Lock)
						wakelockMap[t.Lock] = &AlarmWakelockSet{ll, nil}
						if pidWakelockCount[pid] == 0 {
							pidWakelockMap[pid] = &AlarmWakelockSet{ll, nil}
						}
						pidWakelockCount[pid]++

					}
				case parsers.WAKELOCK_RELEASE:
					// Find loglines from acquire time to release time and track busyness, frequency, periods
					if !lockAddrSet.Has(t.Lock) {
						break
					}
					lockAddrSet.Remove(t.Lock)
					wakelockMap[t.Lock].Release = ll
					pid := wakelockMap[t.Lock].Acquire.Payload.(*parsers.ThermaPlanWakelock).Pid
					if pidWakelockCount[pid] > 0 {
						pidWakelockCount[pid]--
					}
					if pidWakelockCount[pid] == 0 && pidWakelockMap[pid] != nil {
						pidWakelockMap[pid].Release = ll
						processSet.Add(pidWakelockMap[pid])
						delete(pidWakelockMap, pid)
					}
					// Clear stuff up
					delete(wakelockMap, t.Lock)
				}
			}
		}
		log.Errorf("Unused alarms")
		for k, v := range pidAlarmMap {
			log.Errorf("%v: %v", k, v.Size())
		}
	}()
	return outChan
}

func processWakelockBusyness(deviceId string, alarm *alarms.DeliverAlarmsLocked, wakelock *AlarmWakelockSet, loglines []interface{}) *AlarmBusynessData {
	data := make(map[*alarms.DeliverAlarmsLocked]*AlarmBusynessData)
	lastInfo := make(map[*alarms.DeliverAlarmsLocked]time.Time)
	temps := make([]int, 0)

	acquire := wakelock.Acquire.Payload.(*parsers.ThermaPlanWakelock)

	tracker := trackers.New()
	cpuTracker := trackers.NewCpuTracker(tracker)
	pcsiTracker := trackers.NewPeriodicCtxSwitchInfoTracker(tracker)
	pcsiTracker.Callback = func(ctxSwitchInfo *trackers.PeriodicCtxSwitchInfo) {
		for _, line := range ctxSwitchInfo.Info {
			info := line.Payload.(*phonelab.PhonelabPeriodicCtxSwitchInfo)
			if alarm.AppPid == info.Tgid || alarm.AppPid == info.Pid || acquire.Pid == info.Tgid || acquire.Pid == info.Pid {
				if _, ok := data[alarm]; !ok {
					data[alarm] = &AlarmBusynessData{}
					data[alarm].BusynessData = NewBusynessData()
					//data[alarm].DeviceId = deviceId
					data[alarm].DeliverAlarmsLocked = alarm
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

	for _, obj := range loglines {
		logline := obj.(*phonelab.Logline)
		tracker.ApplyLogline(logline)
		switch t := logline.Payload.(type) {
		case *phonelab.ThermalTemp:
			temps = append(temps, t.Temp)
		}
	}

	if _, ok := data[alarm]; ok {
		data[alarm].Duration = wakelock.Release.Datetime.Sub(wakelock.Acquire.Datetime).Nanoseconds()
		data[alarm].Temps = temps
		return data[alarm]
	} else {
		//log.Errorf("Did not find context switch info for alarm. appPid=%v", alarm.AppPid)
		/*
			for _, obj := range loglines {
				logline := obj.(*phonelab.Logline)
				log.Errorf("%v", logline.Line)
			}
			log.Errorf("\n")
		*/
		return nil
	}
}

type AlarmWakelockCpuCollector struct {
	sync.Mutex
	wg sync.WaitGroup
	*gsync.Semaphore
	*phonelab.DefaultCollector
	durations []int64
}

func __avg(slice []int64) float64 {
	sum := int64(0)
	for _, v := range slice {
		sum += v
	}
	return float64(sum) / float64(len(slice))
}

func (c *AlarmWakelockCpuCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	c.wg.Add(1)
	c.P()
	go func() {
		defer c.wg.Done()
		defer c.V()
		r := data.(*AlarmBusynessData)

		c.Lock()
		if c.durations == nil {
			c.durations = make([]int64, 0)
		}
		c.durations = append(c.durations, r.Duration)
		log.Infof("Mean duration: %2.2fs", (__avg(c.durations) / 1e9))
		c.Unlock()
		sourceInfo := info.(*phonelab.PhonelabSourceInfo)
		deviceId := sourceInfo.DeviceId

		h := md5.New()
		io.WriteString(h, r.DeliverAlarmsLocked.Logline.Line)
		checksum := fmt.Sprintf("%x", h.Sum(nil))

		path := filepath.Join(deviceId, "analysis", "alarm_wakelock_cpu", fmt.Sprintf("%v", checksum))

		// XXX: Hack. Set Logline.Payload = nil
		// Otherwise, Logline.Payload refers to
		// deliverAlarmsLocked -> Logline -> deliverAlarmsLocked -> ...
		// you see where this is going
		r.DeliverAlarmsLocked.Logline.Payload = nil

		info := &CustomInfo{path, "custom"}
		c.DefaultCollector.OnData(r, info)
	}()
}

func (c *AlarmWakelockCpuCollector) Finish() {
	// Nothing to do here
	c.wg.Wait()
}
