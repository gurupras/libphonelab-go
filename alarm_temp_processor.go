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
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type AlarmTempProcGenerator struct{}

func (t *AlarmTempProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &AlarmTempProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type AlarmTempProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

var distributionMap = make(map[string]*Distribution)
var distributionMapLock = sync.Mutex{}

type AlarmTempData struct {
	*alarms.DeliverAlarmsLocked
	Temps       []int32
	Timestamps  []int64
	TriggerTemp int32
}

func NewAlarmTempData() *AlarmTempData {
	atd := &AlarmTempData{}
	atd.Temps = make([]int32, 0)
	atd.Timestamps = make([]int64, 0)
	return atd
}

var atMaxConcurrentSem = gsync.NewSem(8)

func (p *AlarmTempProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{}, 100)

	uid := fmt.Sprintf("%v", p.Info.Context())
	//log.Infof("Processing: %v", uid)
	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId
	total := len(sourceInfo.BootIds())
	finished := uint32(0)

	whenTempSkipped := uint32(0)
	triggerTempSkipped := uint32(0)

	atMaxConcurrentSem.P()
	go func() {
		defer close(outChan)
		defer atMaxConcurrentSem.V()
		defer log.Infof("%v: %d/%d", deviceId, atomic.AddUint32(&finished, 1), total)

		alarmSet := set.New()
		var distribution *Distribution

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
					distributionMap[uid] = NewDistribution(nil, 24*time.Hour)
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
							threshold := time.Duration((80 * (alarm.WindowLength * 1000000)) / 100)
							startIdx, closestTimestamp := distribution.FindIdxByTimestampBinarySearch(alarm.DeliverAlarmsLocked.WhenRtc*1000000, threshold)
							_ = closestTimestamp
							if startIdx == -1 {
								log.Debugf("Failed to find nearest timestamp. When=%v nearest=%v", alarm.DeliverAlarmsLocked.WhenRtc*1000000, closestTimestamp)
								atomic.AddUint32(&whenTempSkipped, 1)
								return
							}
							// Trigger temperature was not set earlier.

							// Find if we have a temperature close to when the alarm fired.
							// XXX: The Rtc field is never fixed up and this may be changed in the future
							// So instead, rely on backtracking from WhenELAPSED and NowELAPSED and WhenRtc
							// now = when + time_since_when_until_now ('now' refers to when the alarm was triggered. not __NOW__
							rtc := int64(time.Duration(alarm.WhenRtc+(alarm.NowElapsed-alarm.WhenElapsed)) * time.Millisecond)
							if idx, closestTimestamp := distribution.FindIdxByTimestampBinarySearch(rtc, 2*time.Second); idx == -1 {
								// We don't have a trigger temperature. Skip this alarm.
								log.Debugf("Trigger skip when=%v nearest=%v", rtc, closestTimestamp)
								atomic.AddUint32(&triggerTempSkipped, 1)
								return
							} else {
								alarm.TriggerTemp = distribution.Temps[idx]
							}
							alarm.Temps = append(alarm.Temps, distribution.Temps[startIdx:]...)
							alarm.Timestamps = append(alarm.Timestamps, distribution.Timestamps[startIdx:]...)
							outChan <- alarm
						}
					}(obj)
				}
				wg.Wait()
			case *alarms.DeliverAlarmsLocked:
				deliverAlarm := ll.Payload.(*alarms.DeliverAlarmsLocked)
				deliverAlarm.Logline = ll

				if deliverAlarm.WindowLength == 0 {
					// Nothing to do
					continue
				}
				if distribution != nil && distribution.IsFull() {
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
	}()
	return outChan
}

type AlarmTempCollector struct {
	sync.Mutex
	outPath string
	wg      sync.WaitGroup
	*phonelab.DefaultCollector
	*gsync.Semaphore
}

func (c *AlarmTempCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	c.wg.Add(1)
	c.P()
	go func() {
		defer c.wg.Done()
		defer c.V()
		r := data.(*AlarmTempData)
		sourceInfo := info.(*phonelab.PhonelabSourceInfo)
		deviceId := sourceInfo.DeviceId

		h := md5.New()
		io.WriteString(h, r.DeliverAlarmsLocked.Logline.Line)
		checksum := fmt.Sprintf("%x", h.Sum(nil))

		// XXX: Hack. Set Logline.Payload = nil
		// Otherwise, Logline.Payload refers to
		// deliverAlarmsLocked -> Logline -> deliverAlarmsLocked ->
		// you see where this is going
		r.DeliverAlarmsLocked.Logline.Payload = nil

		path := filepath.Join(deviceId, "analysis", "alarm_temp", fmt.Sprintf("%v.gz", checksum))
		info := &CustomInfo{path, "custom"}
		c.DefaultCollector.OnData(data, info)
	}()
}

func (c *AlarmTempCollector) Finish() {
	// Nothing to do here
	c.wg.Wait()
}
