package libphonelab

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	"github.com/shaseley/phonelab-go/serialize"
	log "github.com/sirupsen/logrus"
)

type AlarmsPerDDProcGenerator struct{}

func (t *AlarmsPerDDProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &AlarmsPerDDProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type AlarmsPerDDProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type AlarmsPerDDData struct {
	DeviceId  string `json:"device_id"`
	Date      string `json:"date"`
	NumAlarms int64  `json:"num_alarms"`
}

func (p *AlarmsPerDDProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	// XXX: This is expected to be phonelab type
	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	data := &AlarmsPerDDData{}

	tracker := trackers.New()
	dayTracker := trackers.NewDayTracker(tracker)
	dayTracker.Callback = func(logline *phonelab.Logline) {
		dt := dayTracker.DayStartLogline.Datetime
		date := fmt.Sprintf("%04d%02d%02d", dt.Year(), int(dt.Month()), dt.Day())
		data.Date = date
		data.DeviceId = deviceId
		log.Infof("%v", data)
		outChan <- data
		data = &AlarmsPerDDData{}
	}

	go func() {
		defer close(outChan)

		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}

			tracker.ApplyLogline(ll)

			switch ll.Payload.(type) {
			case *alarms.DeliverAlarmsLocked:
				data.NumAlarms++
			}
		}
	}()
	return outChan
}

type AlarmsPerDDCollector struct {
	sync.Mutex
	outPath       string
	deviceDataMap map[string]map[string]int64
	Serializer    serialize.Serializer
	wg            sync.WaitGroup
}

func (c *AlarmsPerDDCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		r := data.(*AlarmsPerDDData)

		sourceInfo := info.(*phonelab.PhonelabSourceInfo)
		// Make sure deviceIds match
		deviceId := sourceInfo.DeviceId
		if deviceId != r.DeviceId {
			log.Fatalf("Device IDs don't match. Context DeviceId (%v) != data DeviceId (%v)", deviceId, r.DeviceId)
		}

		c.Lock()
		defer c.Unlock()
		if _, ok := c.deviceDataMap[deviceId]; !ok {
			c.deviceDataMap[deviceId] = make(map[string]int64)
		}
		c.deviceDataMap[deviceId][r.Date] = r.NumAlarms
	}()
}

func (c *AlarmsPerDDCollector) Finish() {
	c.wg.Wait()

	for deviceId := range c.deviceDataMap {
		c.wg.Add(1)
		go func(deviceId string) {
			defer c.wg.Done()
			u, err := url.Parse(c.outPath)
			if err != nil {
				log.Fatalf("Failed to parse URL from string: %v: %v", c.outPath, err)
			}
			u.Path = filepath.Join(u.Path, deviceId, "analysis", "alarms_per_device_day.gz")
			filename := u.String()

			c.Serializer.Serialize(c.deviceDataMap[deviceId], filename)
		}(deviceId)
	}
	c.wg.Wait()
}
