package libphonelab

import (
	"path/filepath"
	"sync"

	"github.com/gurupras/gocommons/gsync"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type SleepDurationAnalysisProcGenerator struct{}

func (t *SleepDurationAnalysisProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &SleepDurationAnalysisProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type SleepDurationAnalysisProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type SleepDurationAnalysisData struct {
	Date string
	Temp int
}

func (p *SleepDurationAnalysisProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	tracker := trackers.New()
	chargingStateTracker := trackers.NewChargingStateTracker(tracker)
	chargingStateTracker.Callback = func(state trackers.ChargingState, logline *phonelab.Logline) {
	}

	scpSem.P()
	go func() {
		defer close(outChan)
		defer scpSem.V()

		var (
			lastSuspendEntry *phonelab.Logline
			lastSuspendExit  *phonelab.Logline
		)

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
					lastSuspendEntry = ll
				case phonelab.PM_SUSPEND_EXIT:
					lastSuspendExit = ll
					if lastSuspendEntry != nil {
						outChan <- lastSuspendExit.Datetime.Sub(lastSuspendEntry.Datetime).Nanoseconds()
					}
				}
			}
		}

	}()
	return outChan
}

type SleepDurationAnalysisCollector struct {
	sync.Mutex
	wg sync.WaitGroup
	*gsync.Semaphore
	*phonelab.DefaultCollector
	deviceDataMap map[string]map[string][]int64
}

func (c *SleepDurationAnalysisCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	log.Debugf("Got Data")
	sourceInfo := info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId
	bootId := sourceInfo.BootId

	r := data.(int64)

	if _, ok := c.deviceDataMap[deviceId]; !ok {
		c.Lock()
		c.deviceDataMap[deviceId] = make(map[string][]int64)
		c.Unlock()
	}
	dMap := c.deviceDataMap[deviceId]
	c.Lock()
	if _, ok := dMap[bootId]; !ok {
		dMap[bootId] = make([]int64, 0)
	}
	dMap[bootId] = append(dMap[bootId], r)
	c.Unlock()
}

func (c *SleepDurationAnalysisCollector) Finish() {
	for deviceId, data := range c.deviceDataMap {
		path := filepath.Join(deviceId, "analysis", "sleep_durations")
		info := &CustomInfo{path, "custom"}
		c.DefaultCollector.OnData(data, info)
	}
}
