package libphonelab

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gurupras/gocommons/gsync"
	"github.com/gurupras/libphonelab-go/parsers"
	"github.com/gurupras/libphonelab-go/trackers"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
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
	Date       int64              `json:"date"`
}

func (s *ScreenOffCpuData) Update(o *ScreenOffCpuData) {
	for oK, oV := range o.Frequency {
		if _, ok := s.Frequency[oK]; !ok {
			s.Frequency[oK] = make([]int, 0)
		}
		s.Frequency[oK] = append(s.Frequency[oK], oV...)
	}
	for oK, oV := range o.Duration {
		if _, ok := s.Duration[oK]; !ok {
			s.Duration[oK] = make([]int64, 0)
		}
		s.Duration[oK] = append(s.Duration[oK], oV...)
	}
	s.Temps = append(s.Temps, o.Temps...)
	s.Timestamps = append(s.Timestamps, o.Timestamps...)

	if s.Date == 0 {
		s.Date = o.Date
	}
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
		unknown := NewScreenOffCpuData()
		current := NewScreenOffCpuData()

		_ = sourceInfo

		tracker := trackers.New()
		missingLoglinesTracker := trackers.NewMissingLoglinesTracker(tracker)
		missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
			// We cannot be sure if we missed a screen off/on logline
			// So reset
			data.Update(current)
			current = NewScreenOffCpuData()
		})

		chargeStateTracker := trackers.NewChargingStateTracker(tracker)
		missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
			chargeStateTracker.CurrentState = trackers.CHARGE_STATE_UNKNOWN
		})

		dayTracker := trackers.NewDayTracker(tracker)
		dayTracker.Callback = func(logline *phonelab.Logline) {
			//log.Infof("%v: dayTrack=%v", sourceInfo.BootId, logline.Datetime)
			data.Update(current)
			data.Date = dayTracker.DayStartLogline.Datetime.UnixNano()
			if len(data.Temps) != 0 {
				log.Infof("Shipped out data")
			}
			outChan <- data
			data = NewScreenOffCpuData()
			current = NewScreenOffCpuData()
		}

		screenStateTracker := trackers.NewScreenStateTracker(tracker)
		screenStateTracker.Callback = func(state trackers.ScreenState, logline *phonelab.Logline) {
			if state != trackers.SCREEN_STATE_ON {
				// It's going off..which means it was on till now
				// Clear out unknown
				unknown = NewScreenOffCpuData()
			}
			if screenStateTracker.CurrentState == trackers.SCREEN_STATE_OFF {
				// Update data with current and reset current
				data.Update(current)
				current = NewScreenOffCpuData()
			} else {
				// We didn't know what the state was, but then
				// it turned on now..so it had to be off
				data.Update(unknown)
				unknown = NewScreenOffCpuData()
			}
			log.Debugf("Screen State=%v", state)
		}
		missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
			screenStateTracker.CurrentState = trackers.SCREEN_STATE_UNKNOWN
		})

		cpuTracker := trackers.NewCpuTracker(tracker)
		missingLoglinesTracker.AddCallback(func(logline *phonelab.Logline) {
			for _, data := range cpuTracker.CurrentState {
				data.Frequency = trackers.FREQUENCY_STATE_UNKNOWN
				data.CpuState = trackers.CPU_STATE_UNKNOWN
				data.FrequencyLogline = nil
				data.CpuStateLogline = nil
			}
		})
		cpuTracker.Callback = func(cpu int, lineType trackers.CpuLineType, logline *phonelab.Logline) {
			var updateObj *ScreenOffCpuData
			if screenStateTracker.CurrentState == trackers.SCREEN_STATE_OFF {
				updateObj = current
			} else {
				updateObj = unknown
			}

			if chargeStateTracker.CurrentState != trackers.CHARGE_STATE_UNPLUGGED {
				// We're only tracking when unplugged
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
			if _, ok := data.Frequency[cpuStr]; !ok {
				updateObj.Frequency[cpuStr] = make([]int, 0)
				updateObj.Duration[cpuStr] = make([]int64, 0)
			}
			updateObj.Frequency[cpuStr] = append(updateObj.Frequency[cpuStr], curFreq)
			updateObj.Duration[cpuStr] = append(updateObj.Duration[cpuStr], duration)
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
			case *phonelab.PLPowerBatteryLog:
				//log.Infof("battery log")
			case *phonelab.PLPowerBatteryProps:
				//log.Infof("battery props")
			case *phonelab.CpuFrequency:
				//log.Infof("cpufrequency")
			case *phonelab.SchedCpuHotplug:
				//log.Infof("hotplug")
			case *parsers.ScreenState:
				//log.Infof("screen state")
			case *phonelab.ThermalTemp:
				if screenStateTracker.CurrentState == trackers.SCREEN_STATE_OFF {
					current.Temps = append(current.Temps, t.Temp)
					current.Timestamps = append(current.Timestamps, ll.Datetime.UnixNano())
				}
			}
		}
	}()
	return outChan
}

type ScreenOffCpuCollector struct {
	sync.Mutex
	wg sync.WaitGroup
	*gsync.Semaphore
	*phonelab.DefaultCollector
	deviceDateMap map[string]map[string]*ScreenOffCpuData
}

func (c *ScreenOffCpuCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	c.wg.Add(1)
	c.P()
	go func() {
		defer c.wg.Done()
		defer c.V()
		r := data.(*ScreenOffCpuData)

		sourceInfo := info.(*phonelab.PhonelabSourceInfo)
		deviceId := sourceInfo.DeviceId
		//log.Infof("deviceid=%v", deviceId)

		date := time.Unix(0, r.Date)
		dateStr := fmt.Sprintf("%04d%02d%02d", date.Year(), int(date.Month()), date.Day())

		c.Lock()
		defer c.Unlock()
		if _, ok := c.deviceDateMap[deviceId]; !ok {
			c.deviceDateMap[deviceId] = make(map[string]*ScreenOffCpuData)
		}
		dateMap := c.deviceDateMap[deviceId]
		if _, ok := dateMap[dateStr]; !ok {
			dateMap[dateStr] = NewScreenOffCpuData()
		}
		dateMap[dateStr].Update(r)
	}()
}

func (c *ScreenOffCpuCollector) Finish() {
	c.wg.Wait()

	for deviceId, dateMap := range c.deviceDateMap {
		for dateStr, r := range dateMap {
			path := filepath.Join(deviceId, "analysis", "screen_off_cpu", fmt.Sprintf("%v", dateStr))
			info := &CustomInfo{path, "custom"}
			c.DefaultCollector.OnData(r, info)
		}
	}
}
