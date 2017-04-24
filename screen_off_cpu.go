package libphonelab

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
)

type ScreenOffCpuProcGenerator struct{}

func (t *ScreenOffCpuProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &ScreenOffCpuProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type ScreenOffCpuProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type ScreenOffCpuData struct {
	Frequency  map[string][]int   `json:"frequency"`
	Duration   map[string][]int64 `json:"duration"`
	Temps      []int              `json:"temps"`
	Timestamps []int64            `json:"timestamps"`
}

func NewScreenOffCpuData() *ScreenOffCpuData {
	data := &ScreenOffCpuData{}
	data.Frequency = make(map[string][]int)
	data.Duration = make(map[string][]int64)
	data.Temps = make([]int, 0)
	data.Timestamps = make([]int64, 0)
	return data
}

func (p *ScreenOffCpuProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)

	go func() {
		defer close(outChan)

		data := NewScreenOffCpuData()

		_ = sourceInfo

		tracker := trackers.New()
		dayTracker := trackers.NewDayTracker(tracker)
		dayTracker.Callback = func(logline *phonelab.Logline) {
			//log.Infof("%v: dayTrack=%v", sourceInfo.BootId, logline.Datetime)
			outChan <- data
			data = NewScreenOffCpuData()
		}

		screenStateTracker := trackers.NewScreenStateTracker(tracker)

		cpuTracker := trackers.NewCpuTracker(tracker)
		cpuTracker.Callback = func(cpu int, lineType trackers.CpuLineType, logline *phonelab.Logline) {
			if screenStateTracker.CurrentState != trackers.SCREEN_STATE_OFF {
				// We're only tracking screen state off
				return
			}
			if lineType == trackers.HOTPLUG_LINE_TYPE {
				sch := logline.Payload.(*phonelab.SchedCpuHotplug)
				if sch.Error != 0 {
					// This failed. Just skip
					return
				}
				if strings.Compare(sch.State, "online") == 0 {
					// This line says the cpu came online.
					// We would've already seen the cpu_frequency just before this
					// So skip it
					return
				}
			}
			curFreq := cpuTracker.CurrentState[cpu].Frequency
			if curFreq == trackers.FREQUENCY_STATE_UNKNOWN {
				return
			}

			lastLogline := cpuTracker.CurrentState[cpu].FrequencyLogline
			if lastLogline == nil {
				return
			}
			duration := int64((logline.TraceTime - lastLogline.TraceTime) * float64(time.Second))
			if duration < 0 {
				fmt.Printf("%v\n%v\n\n", logline.Line, lastLogline.Line)
			}
			cpuStr := fmt.Sprintf("%v", cpu)
			data.Frequency[cpuStr] = append(data.Frequency[cpuStr], curFreq)
			data.Duration[cpuStr] = append(data.Duration[cpuStr], duration)
		}

		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}

			// Update all trackers
			tracker.ApplyLogline(ll)

			switch t := ll.Payload.(type) {
			case *phonelab.ThermalTemp:
				data.Temps = append(data.Temps, t.Temp)
				data.Timestamps = append(data.Timestamps, ll.Datetime.UnixNano())
			}
		}
	}()
	return outChan
}

type ScreenOffCpuCollector struct {
	sync.Mutex
	outPath string
	Idx     int
}

func (c *ScreenOffCpuCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	r := data.(*ScreenOffCpuData)

	sourceInfo := info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId
	//log.Infof("deviceid=%v", deviceId)

	outdir := filepath.Join(c.outPath, deviceId, "analysis", "alarm_cpu")
	c.Lock()
	filename := filepath.Join(outdir, fmt.Sprintf("%08d.gz", c.Idx))
	c.Idx++
	c.Unlock()

	b, _ := json.MarshalIndent(r, "", "    ")

	sourceInfo.FSInterface.Makedirs(outdir)
	sourceInfo.FSInterface.WriteFile(filename, b, 0664)
}

func (c *ScreenOffCpuCollector) Finish() {
	// Nothing to do here
}
