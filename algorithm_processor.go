package libphonelab

import (
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/set"
	"github.com/gurupras/gocommons/gsync"
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/algorithms"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type AlgorithmProcGenerator struct{}

func (t *AlgorithmProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &AlgorithmProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type AlgorithmProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type AlgorithmData struct {
	dal          *alarms.DeliverAlarmsLocked `json:"-"`
	WindowLength int64                       `json:"window_length"`
	Data         map[string]int              `json:"data"`
}

func (p *AlgorithmProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{}, 100)

	uid := fmt.Sprintf("%v", p.Info.Context())
	//log.Infof("Processing: %v", uid)
	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId
	bootId := sourceInfo.BootId
	total := len(sourceInfo.BootIds())

	algorithmSet := set.NewNonTS()
	algorithmSet.Add(algorithms.NewSimpleLinearThreshold(25.0))
	algorithmSet.Add(algorithms.NewSimpleLinearThreshold(10.0))
	algorithmSet.Add(&algorithms.Oracle{})
	algorithmSet.Add(&algorithms.OptimalStoppingTheory{})
	algorithmSet.Add(&algorithms.OptimalStoppingTheoryCardinal{})
	algorithmSet.Add(&algorithms.Android{})

	atDeviceMapLock.Lock()
	if _, ok := atDeviceMap[deviceId]; !ok {
		atDeviceMap[deviceId] = make(map[string]interface{})
		atDeviceMap[deviceId]["total"] = total
		finished := uint32(0)
		atDeviceMap[deviceId]["finished"] = &finished
	}
	atDeviceMapLock.Unlock()

	atMaxConcurrentSem.P()
	alarmWg := sync.WaitGroup{}
	maxConcurrentAlarms := gsync.NewSem(100)
	go func() {
		defer close(outChan)
		defer atMaxConcurrentSem.V()

		alarmSet := set.New()
		var distribution *trackers.Distribution

		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			switch ll.Payload.(type) {
			case *phonelab.ThermalTemp:
				tt := ll.Payload.(*phonelab.ThermalTemp)
				temp := int32(tt.Temp)
				timestamp := ll.Datetime.UnixNano()
				if distribution == nil {
					distributionMapLock.Lock()
					distributionMap[uid] = trackers.NewDistribution(nil, 24*time.Hour)
					distribution = distributionMap[uid]
					distributionMapLock.Unlock()
				}

				// Update the distribution
				distribution.Update(temp, timestamp)
				// If the timestamp on this line is > one of
				// the alarms in alarmSet, then finalize that
				// alarm with its set of temperatures and pass
				// it along
				wg := sync.WaitGroup{}
				for _, obj := range alarmSet.List() {
					wg.Add(1)
					go func(obj interface{}) {
						defer wg.Done()
						alarm := obj.(*AlarmTempData)
						if timestamp > alarm.MaxWhenRtc {
							alarmSet.Remove(obj)
							// Add all the temperatures for this alarm and ship it out
							// Our max threshold is 80% the window length of the alarm
							var (
								startIdx         int
								closestTimestamp int64
							)

							/*
								for threshold := 1; threshold < 80; threshold += 5 {
									thresholdDuration := time.Duration((int64(threshold) * (alarm.WindowLength * 1000000)) / 100)
									startIdx, closestTimestamp = distribution.FindIdxByTimestampBinarySearch(alarm.DeliverAlarmsLocked.WhenRtc*1000000, thresholdDuration)
									_ = closestTimestamp
									if startIdx != -1 {
										break
									}
								}
							*/
							startIdx, closestTimestamp = distribution.FindIdxByTimestampBinarySearch(alarm.DeliverAlarmsLocked.WhenRtc*1000000, 3*time.Second)

							if startIdx == -1 {
								log.Debugf("Failed to find nearest timestamp. When=%v nearest=%v", alarm.DeliverAlarmsLocked.WhenRtc*1000000, closestTimestamp)
								return
							}
							// Trigger temperature was not set earlier.

							// Find if we have a temperature close to when the alarm fired.
							// XXX: The Rtc field is never fixed up and this may be changed in the future
							// So instead, rely on backtracking from WhenELAPSED and NowELAPSED and WhenRtc
							// now = when + time_since_when_until_now ('now' refers to when the alarm was triggered. not __NOW__
							rtc := int64(time.Duration(alarm.WhenRtc+(alarm.NowElapsed-alarm.WhenElapsed)) * 1000000)
							if idx, closestTimestamp := distribution.FindIdxByTimestampBinarySearch(rtc, 2*time.Second); idx == -1 {
								// We don't have a trigger temperature. Skip this alarm.
								log.Debugf("Trigger skip when=%v nearest=%v", rtc, closestTimestamp)
								return
							} else {
								alarm.TriggerTemp = distribution.Temps[idx]
							}
							for idx := startIdx; idx < len(distribution.Temps); idx++ {
								if distribution.Timestamps[idx] < alarm.MaxWhenRtc*1000000 {
									alarm.Temps = append(alarm.Temps, distribution.Temps[idx])
									alarm.Timestamps = append(alarm.Timestamps, distribution.Timestamps[idx])
								} else {
									break
								}
							}
							// TODO: Add algorithm analysis for this alarm
							maxConcurrentAlarms.P()
							alarmWg.Add(1)
							go func(alarm *AlarmTempData, distribution *trackers.Distribution) {
								defer maxConcurrentAlarms.V()
								defer alarmWg.Done()
								if len(alarm.Temps) > 100 {
									log.Debugf("Running algorithms")
									algorithmResults := make(map[string]int)
									algWg := sync.WaitGroup{}
									algLock := sync.Mutex{}
									algWg.Add(algorithmSet.Size())
									for _, obj := range algorithmSet.List() {
										alg := obj.(algorithms.Algorithm)
										go func(alg algorithms.Algorithm) {
											defer algWg.Done()
											temp := alg.Process(alarm.DeliverAlarmsLocked, alarm.TriggerTemp, alarm.Temps, alarm.Timestamps, distribution)
											algLock.Lock()
											defer algLock.Unlock()
											algorithmResults[alg.Name()] = int(temp)
										}(alg)
									}
									algWg.Wait()
									log.Debugf("Shipping out alarm")
									outChan <- &AlgorithmData{
										alarm.DeliverAlarmsLocked,
										alarm.WindowLength,
										algorithmResults,
									}
								} else {
									log.Warnf("Temps < 100")
								}
							}(alarm, distribution.Clone())
						}
					}(obj)
				}
				wg.Wait()
			case *alarms.DeliverAlarmsLocked:
				deliverAlarm := ll.Payload.(*alarms.DeliverAlarmsLocked)
				deliverAlarm.Logline = ll

				if time.Duration(deliverAlarm.WindowLength)*time.Millisecond < 1*time.Hour {
					// Nothing to do
					continue
				}
				if distribution != nil && distribution.IsFull() {
					log.Infof("Found alarm")
					// Add this alarm to the alarm set
					alarmTempData := NewAlarmTempData()
					alarmTempData.DeliverAlarmsLocked = deliverAlarm
					alarmSet.Add(alarmTempData)
				}
			default:
				log.Warnf("Unknown line: %v", ll.Line)
			}
		}
		distributionMapLock.Lock()
		delete(distributionMap, uid)
		distributionMapLock.Unlock()
		alarmWg.Wait()
		log.Infof("%v->%v: %d/%d", deviceId, bootId, atomic.AddUint32(atDeviceMap[deviceId]["finished"].(*uint32), 1), total)
	}()
	return outChan
}

type AlgorithmCollector struct {
	sync.Mutex
	wg sync.WaitGroup
	*phonelab.DefaultCollector
	*gsync.Semaphore
	deviceDataMap map[string]map[string][]int
}

func (c *AlgorithmCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	c.wg.Add(1)
	c.P()
	go func() {
		defer c.wg.Done()
		defer c.V()
		r := data.(*AlgorithmData)
		sourceInfo := info.(*phonelab.PhonelabSourceInfo)
		deviceId := sourceInfo.DeviceId

		h := md5.New()
		io.WriteString(h, r.dal.Logline.Line)
		checksum := fmt.Sprintf("%x", h.Sum(nil))

		path := filepath.Join(deviceId, "analysis", "algorithms", fmt.Sprintf("%v", checksum))
		info := &CustomInfo{path, "custom"}
		c.DefaultCollector.OnData(r, info)
		if _, ok := c.deviceDataMap[deviceId]; !ok {
			c.Lock()
			c.deviceDataMap[deviceId] = make(map[string][]int)
			c.Unlock()
		}
		dMap := c.deviceDataMap[deviceId]
		c.Lock()
		for k, v := range r.Data {
			if _, ok := dMap[k]; !ok {
				dMap[k] = make([]int, 0)
			}
			dMap[k] = append(dMap[k], v)
		}
		c.Unlock()
	}()
}

func (c *AlgorithmCollector) Finish() {
	// Nothing to do here
	c.wg.Wait()

	for deviceId, data := range c.deviceDataMap {
		path := filepath.Join(deviceId, "analysis", "algorithm_results")
		info := &CustomInfo{path, "custom"}
		c.DefaultCollector.OnData(data, info)
	}
}
