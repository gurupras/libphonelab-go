package libphonelab

import (
	"fmt"
	"sync"
	"time"

	"github.com/fatih/set"
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

type AlarmTempResult struct {
	*AlarmTempData
	phonelab.PipelineSourceInfo
}

func NewAlarmTempData() *AlarmTempData {
	atd := &AlarmTempData{}
	atd.Temps = make([]int32, 0)
	atd.Timestamps = make([]int64, 0)
	return atd
}

func (p *AlarmTempProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	uid := fmt.Sprintf("%v->%v", p.Info.Context())

	whenTempSkipped := 0
	triggerTempSkipped := 0

	go func() {
		defer close(outChan)

		alarmSet := set.NewNonTS()
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
				for _, obj := range alarmSet.List() {
					alarm := obj.(*AlarmTempData)
					if timestamp > alarm.MaxWhenRtc {
						alarmSet.Remove(obj)
						// Add all the temperatures for this alarm and ship it out
						// Our max threshold is 80% the window length of the alarm
						threshold := time.Duration((80 * (alarm.WindowLength * 1000000)) / 100)
						startIdx, closestTimestamp := distribution.FindIdxByTimestamp(alarm.DeliverAlarmsLocked.WhenRtc*1000000, threshold)
						_ = closestTimestamp
						if startIdx == -1 {
							whenTempSkipped++
							continue
						}
						// Trigger temperature was not set earlier.

						// Find if we have a temperature close to when the alarm fired.
						// XXX: The Rtc field is never fixed up and this may be changed in the future
						// So instead, rely on backtracking from WhenELAPSED and NowELAPSED and WhenRtc
						// now = when + time_since_when_until_now ('now' refers to when the alarm was triggered. not __NOW__
						rtc := int64(time.Duration(alarm.WhenRtc+(alarm.NowElapsed-alarm.WhenElapsed)) * time.Millisecond)
						if idx, _ := distribution.FindIdxByTimestamp(rtc, 2*time.Second); idx == -1 {
							// We don't have a trigger temperature. Skip this alarm.
							triggerTempSkipped++
							continue
						} else {
							alarm.TriggerTemp = distribution.Temps[idx]
						}
						alarm.Temps = append(alarm.Temps, distribution.Temps[startIdx:]...)
						alarm.Timestamps = append(alarm.Timestamps, distribution.Timestamps[startIdx:]...)
						result := &AlarmTempResult{alarm, p.Info}
						outChan <- result
					}
				}
			default:
				deliverAlarm, err := alarms.ParseDeliverAlarmsLocked(ll)
				if err != nil {
					log.Errorf("Failed to parse deliverAlarm: %v", err)
				}
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
			}
		}
	}()
	return outChan
}

type AlarmTempCollector struct {
	sync.Mutex
	phonelab.PipelineSourceInfo
	Idx int32
}

func (c *AlarmTempCollector) OnData(data interface{}) {
	r := data.(*AlarmTempResult)
	c.Lock()
	if c.PipelineSourceInfo == nil {
		c.PipelineSourceInfo = r.PipelineSourceInfo
	}
	idx := c.Idx
	_ = idx
	c.Idx++
	c.Unlock()

	// We have the file index in which we're going to dump the data
	// Output directory is always path/deviceid/analysis/alarm_temp
	// FIXME: Pipeline source info only exposes context.
	// Update this to reflect that
	/*
		outdir := filepath.Join(c.Path, c.DeviceId, "analysis", "alarm_temp")
		filename := fmt.Sprintf("%08d.gz", idx)

		client, err := hdfs.NewHdfsClient(c.HdfsAddr)
		if err != nil {
			log.Fatalf("%v", err)
		}

		if err := SerializeResult(r.AlarmTempData, outdir, filename, easyfiles.GZ_TRUE, client); err != nil {
			log.Errorf(err.Error())
		}
	*/
}

func (c *AlarmTempCollector) Finish() {
	// Nothing to do here
}
