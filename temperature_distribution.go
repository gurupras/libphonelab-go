package libphonelab

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type TDProcGenerator struct{}

func (t *TDProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &TDProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type TDProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

func (p *TDProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	tempData := TemperatureDistribution{}

	tracker := trackers.New()
	dayTracker := trackers.NewDayTracker(tracker)
	dayTracker.Callback = func(logline *phonelab.Logline) {
		date := time.Unix(0, dayTracker.DayStartLogline.Datetime.UnixNano())
		dateStr := fmt.Sprintf("%04d%02d%02d", date.Year(), int(date.Month()), date.Day())
		dayTempDist := DayTemperatureDistribution{}
		dayTempDist[dateStr] = tempData
		outChan <- dayTempDist
		tempData = TemperatureDistribution{}
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
			case *phonelab.ThermalTemp:
				tt := ll.Payload.(*phonelab.ThermalTemp)
				temp := tt.Temp
				tempData[fmt.Sprintf("%v", temp)] += 1
			}
		}
		if len(tempData) > 0 {
			dayTracker.Callback(nil)
		}
	}()
	return outChan
}

type TemperatureDistribution map[string]int64
type DayTemperatureDistribution map[string]TemperatureDistribution

type TDCollector struct {
	sync.Mutex
	wg sync.WaitGroup
	*phonelab.DefaultCollector
	TemperatureMap map[string]DayTemperatureDistribution
}

func (c *TDCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	log.Debugf("onData()")
	r := data.(DayTemperatureDistribution)
	sourceInfo := info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	if _, ok := c.TemperatureMap[deviceId]; !ok {
		c.Lock()
		c.TemperatureMap[deviceId] = DayTemperatureDistribution{}
		c.Unlock()
	}
	dMap := c.TemperatureMap[deviceId]
	// Update the map
	for k, v := range r {
		dMap[k] = v
	}
}

func (c *TDCollector) Finish() {
	log.Debugf("Finish()")
	c.wg.Add(len(c.TemperatureMap))
	for deviceId := range c.TemperatureMap {
		go func(deviceId string) {
			defer c.wg.Done()
			log.Infof("Shipping out data")
			path := filepath.Join(deviceId, "analysis", "temp_distribution")
			info := &CustomInfo{path, "custom"}
			c.DefaultCollector.OnData(c.TemperatureMap[deviceId], info)
		}(deviceId)
	}
	c.wg.Wait()
}
