package libphonelab

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/gurupras/gocommons/gsync"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type AmbientTemperatureFromSuspendProcGenerator struct{}

func (t *AmbientTemperatureFromSuspendProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &AmbientTemperatureFromSuspendProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type AmbientTemperatureFromSuspendProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type AmbientTemperatureData struct {
	Date string
	Temp int
}

func (p *AmbientTemperatureFromSuspendProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	var (
		lastSuspendEntry       *phonelab.Logline
		lastSuspendExit        *phonelab.Logline
		lastTemperature        int
		tempBeforeSuspendEntry int
		waitingForTemp         bool
	)

	tracker := trackers.New()
	chargingStateTracker := trackers.NewChargingStateTracker(tracker)
	chargingStateTracker.Callback = func(state trackers.ChargingState, logline *phonelab.Logline) {
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

			tracker.ApplyLogline(ll)

			switch t := ll.Payload.(type) {
			case *phonelab.PowerManagementPrintk:
				switch t.State {
				case phonelab.PM_SUSPEND_ENTRY:
					if chargingStateTracker.CurrentState != trackers.CHARGE_STATE_UNPLUGGED {
						// Nothing to do
						//continue
					}
					if lastSuspendExit != nil {
						//diff := ll.Datetime.Sub(lastSuspendExit.Datetime)
						diff := 1 * time.Nanosecond
						if diff < 1*time.Minute && tempBeforeSuspendEntry != 0 && lastTemperature <= tempBeforeSuspendEntry {
							// Do nothing. Treat it as if this wakeup never happened
						} else {
							lastSuspendEntry = ll
							tempBeforeSuspendEntry = lastTemperature
							waitingForTemp = false
						}
					} else {
						// Was nil
						lastSuspendEntry = ll
						tempBeforeSuspendEntry = lastTemperature
					}
				case phonelab.PM_SUSPEND_EXIT:
					if chargingStateTracker.CurrentState != trackers.CHARGE_STATE_UNPLUGGED {
						// Nothing to do
						//continue
					}
					if lastSuspendEntry != nil {
						diff := ll.Datetime.Sub(lastSuspendEntry.Datetime)
						if !waitingForTemp && diff > 20*time.Minute {
							waitingForTemp = true
						}
					}
					lastSuspendExit = ll
				}
			case *phonelab.ThermalTemp:
				lastTemperature = t.Temp
				if waitingForTemp && lastTemperature <= 35 {
					dt := ll.Datetime
					date := fmt.Sprintf("%04d%02d%02d", dt.Year(), int(dt.Month()), dt.Day())
					data := &AmbientTemperatureData{date, t.Temp}
					outChan <- data
					waitingForTemp = false

					log.Infof("Entry: %v", lastSuspendEntry.Line)
					log.Infof("Exit: %v", ll.Line)

					lastSuspendEntry = nil
					lastSuspendExit = nil
				}
			}
		}

	}()
	return outChan
}

type AmbientTemperatureFromDistributionProcGenerator struct{}

func (t *AmbientTemperatureFromDistributionProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &AmbientTemperatureFromDistributionProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type AmbientTemperatureFromDistributionProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

func (p *AmbientTemperatureFromDistributionProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	scpSem.P()
	go func() {
		defer close(outChan)
		defer scpSem.V()

		distribution := trackers.NewDistribution(nil, 24*time.Hour)

		tracker := trackers.New()
		dayTracker := trackers.NewDayTracker(tracker)
		dayTracker.Callback = func(logline *phonelab.Logline) {
			dt := dayTracker.DayStartLogline.Datetime
			date := fmt.Sprintf("%04d%02d%02d", dt.Year(), int(dt.Month()), dt.Day())
			for idx := 0; idx < 6; idx++ {
				p := int(distribution.NthPercentileTemp(idx))
				outChan <- &AmbientTemperatureData{date, p}
			}
		}

		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			tracker.ApplyLogline(ll)
			switch t := ll.Payload.(type) {
			case *phonelab.ThermalTemp:
				distribution.Update(int32(t.Temp), ll.Datetime.UnixNano())
			}
		}
	}()
	return outChan
}

type AmbientTemperatureCollector struct {
	sync.Mutex
	wg sync.WaitGroup
	*gsync.Semaphore
	*phonelab.DefaultCollector
	deviceDataMap map[string]map[string][]int
}

func (c *AmbientTemperatureCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	log.Infof("Got Data")
	sourceInfo := info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	r := data.(*AmbientTemperatureData)

	if _, ok := c.deviceDataMap[deviceId]; !ok {
		c.Lock()
		c.deviceDataMap[deviceId] = make(map[string][]int)
		c.Unlock()
	}
	dMap := c.deviceDataMap[deviceId]
	if _, ok := dMap[r.Date]; !ok {
		dMap[r.Date] = make([]int, 0)
	}
	dMap[r.Date] = append(dMap[r.Date], r.Temp)
}

func (c *AmbientTemperatureCollector) Finish() {
	for deviceId, data := range c.deviceDataMap {
		path := filepath.Join(deviceId, "analysis", "suspend_ambient_temperatures")
		info := &CustomInfo{path, "custom"}
		c.DefaultCollector.OnData(data, info)
	}
}
