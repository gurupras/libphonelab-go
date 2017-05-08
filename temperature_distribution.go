package libphonelab

import (
	"fmt"
	"path/filepath"
	"sync"

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

type TDData struct {
	Temps map[string]int64
}

func NewTDData() *TDData {
	atd := &TDData{}
	atd.Temps = make(map[string]int64)
	return atd
}

func (p *TDProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	tempData := NewTDData()

	go func() {
		defer close(outChan)

		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			switch ll.Payload.(type) {
			case *phonelab.ThermalTemp:
				tt := ll.Payload.(*phonelab.ThermalTemp)
				temp := tt.Temp
				tempData.Temps[fmt.Sprintf("%v", temp)] += 1
			default:
				log.Fatalf("Unknown line: %v", ll.Line)
			}
		}
		outChan <- tempData
	}()
	return outChan
}

type TDCollector struct {
	sync.Mutex
	wg sync.WaitGroup
	*phonelab.DefaultCollector
	TemperatureMap map[string]map[string]int64
}

func (c *TDCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	r := data.(*TDData)
	sourceInfo := info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	if _, ok := c.TemperatureMap[deviceId]; !ok {
		c.TemperatureMap[deviceId] = make(map[string]int64)
	}
	dMap := c.TemperatureMap[deviceId]
	// Update the map
	for k, v := range r.Temps {
		dMap[k] += v
	}
}

func (c *TDCollector) Finish() {
	c.wg.Add(len(c.TemperatureMap))
	for deviceId := range c.TemperatureMap {
		go func(deviceId string) {
			defer c.wg.Done()
			path := filepath.Join(deviceId, "analysis", "temp_distribution.gz")
			info := &CustomInfo{path, "custom"}
			c.DefaultCollector.OnData(c.TemperatureMap[deviceId], info)
		}(deviceId)
	}
	c.wg.Wait()
}
