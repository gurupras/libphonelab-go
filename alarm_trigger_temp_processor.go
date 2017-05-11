package libphonelab

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/set"
	"github.com/gurupras/gocommons/gsync"
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	"github.com/shaseley/phonelab-go/serialize"
	log "github.com/sirupsen/logrus"
)

type AlarmTriggerTempProcGenerator struct{}

func (t *AlarmTriggerTempProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &AlarmTriggerTempProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type AlarmTriggerTempProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

var att_distributionMap = make(map[string]*trackers.Distribution)
var att_distributionMapLock = sync.Mutex{}

type AlarmTriggerTempData struct {
	*alarms.DeliverAlarmsLocked
	TriggerTemp int32
}

func NewAlarmTriggerTempData() *AlarmTriggerTempData {
	atd := &AlarmTriggerTempData{}
	return atd
}

func (p *AlarmTriggerTempProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	uid := fmt.Sprintf("%v->%v", p.Info.Context())

	whenTempSkipped := uint32(0)
	triggerTempSkipped := uint32(0)

	go func() {
		defer close(outChan)

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
					att_distributionMapLock.Lock()
					att_distributionMap[uid] = trackers.NewDistribution(nil, 24*time.Hour)
					distribution = att_distributionMap[uid]
					att_distributionMapLock.Unlock()
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
						alarm := obj.(*AlarmTriggerTempData)
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
					alarmTempData := NewAlarmTriggerTempData()
					alarmTempData.DeliverAlarmsLocked = deliverAlarm
					alarmSet.Add(alarmTempData)
				}
			default:
				log.Warnf("Unknown line: %v", ll.Line)
			}
		}
		att_distributionMapLock.Lock()
		delete(att_distributionMap, uid)
		att_distributionMapLock.Unlock()
	}()
	return outChan
}

type AlarmTriggerTempCollector struct {
	sync.Mutex
	outPath    string
	Serializer serialize.Serializer
	wg         sync.WaitGroup
	deviceMap  map[string]map[string]int32
	*gsync.Semaphore
}

func (c *AlarmTriggerTempCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	c.wg.Add(1)
	c.P()
	go func() {
		defer c.wg.Done()
		defer c.V()
		r := data.(*AlarmTriggerTempData)
		sourceInfo := info.(*phonelab.PhonelabSourceInfo)
		deviceId := sourceInfo.DeviceId

		h := md5.New()
		io.WriteString(h, r.DeliverAlarmsLocked.Logline.Line)
		checksum := fmt.Sprintf("%x", h.Sum(nil))

		c.Lock()
		if c.deviceMap[deviceId] == nil {
			c.deviceMap[deviceId] = make(map[string]int32)
		}
		c.deviceMap[deviceId][checksum] = r.TriggerTemp
		c.Unlock()
	}()
}

func (c *AlarmTriggerTempCollector) Finish() {
	// Nothing to do here
	c.wg.Wait()

	for deviceId, data := range c.deviceMap {
		u, err := url.Parse(c.outPath)
		if err != nil {
			log.Fatalf("Failed to parse URL from string: %v: %v", c.outPath, err)
		}
		u.Path = filepath.Join(u.Path, deviceId, "analysis", "alarm_trigger_temp.gz")
		filename := u.String()

		log.Infof("Serializing filename=%v", filename)
		err = c.Serializer.Serialize(data, filename)
		if err != nil {
			log.Fatalf("Failed to serialize: %v", err)
		}
	}
}
