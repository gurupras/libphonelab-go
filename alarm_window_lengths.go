package libphonelab

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fatih/set"
	"github.com/gurupras/gocommons/gsync"
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type AlarmWindowLengthsProcGenerator struct{}

func (t *AlarmWindowLengthsProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	var bootIds []string
	if v, ok := kwargs["boot_ids"]; ok {
		if bootIds, ok = v.([]string); !ok {
			panic(fmt.Sprintf("Bad value for key 'boot_ids'. Expected []string got %t", v))
		}
	}
	s := set.NewNonTS()
	for _, b := range bootIds {
		s.Add(b)
	}

	return &AlarmWindowLengthsProcessor{
		Source:  source.Processor,
		Info:    source.Info,
		bootIds: s,
	}
}

type AlarmWindowLengthsProcessor struct {
	Source  phonelab.Processor
	Info    phonelab.PipelineSourceInfo
	bootIds *set.SetNonTS
}

func (p *AlarmWindowLengthsProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{}, 100)

	//log.Infof("Processing: %v", uid)
	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId
	bootId := sourceInfo.BootId

	go func() {
		defer close(outChan)

		bypass := false
		if p.bootIds.Size() > 0 && !p.bootIds.Has(bootId) {
			bypass = true
		}

		uuids := set.NewNonTS()
		windowLengths := make([]int64, 0)

		inChan := p.Source.Process()
		for obj := range inChan {
			if bypass {
				continue
			}
			ll := obj.(*phonelab.Logline)

			switch ll.Payload.(type) {
			case *alarms.DeliverAlarmsLocked:
				deliverAlarm := ll.Payload.(*alarms.DeliverAlarmsLocked)

				if deliverAlarm.WindowLength == 0 {
					// Nothing to do
					continue
				}
				if uuids.Has(deliverAlarm.Uuid) {
					// Nothing to do
					continue
				}
				uuids.Add(deliverAlarm.Uuid)
				windowLengths = append(windowLengths, deliverAlarm.WindowLength)
				if len(windowLengths)%100 == 0 {
					log.Debugf("size=%d", len(windowLengths))
				}
			}
		}
		outChan <- windowLengths
		_ = deviceId
	}()
	return outChan
}

type AlarmWindowLengthsCollector struct {
	sync.Mutex
	wg sync.WaitGroup
	*phonelab.DefaultCollector
	*gsync.Semaphore
	deviceDataMap map[string]map[string][]int64
}

func (c *AlarmWindowLengthsCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	c.wg.Add(1)
	c.P()
	go func() {
		defer c.wg.Done()
		defer c.V()
		r := data.([]int64)
		sourceInfo := info.(*phonelab.PhonelabSourceInfo)
		deviceId := sourceInfo.DeviceId
		bootId := sourceInfo.BootId

		if _, ok := c.deviceDataMap[deviceId]; !ok {
			c.deviceDataMap[deviceId] = make(map[string][]int64)
		}
		c.deviceDataMap[deviceId][bootId] = r
	}()
}

func (c *AlarmWindowLengthsCollector) Finish() {
	// Nothing to do here
	c.wg.Wait()

	for deviceId, data := range c.deviceDataMap {
		path := filepath.Join(deviceId, "analysis", "window_lengths")
		info := &CustomInfo{path, "custom"}
		c.DefaultCollector.OnData(data, info)
	}
}
