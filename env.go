package libphonelab

import (
	"fmt"

	"github.com/gurupras/gocommons/gsync"
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/parsers"
	"github.com/shaseley/phonelab-go"
	"github.com/shaseley/phonelab-go/serialize"
	log "github.com/sirupsen/logrus"
)

func InitEnv(env *phonelab.Environment) {
	env.Parsers["Kernel-Trace"] = func() phonelab.Parser {
		p := phonelab.NewKernelTraceParser()
		p.ErrOnUnknownTag = false
		return p
	}

	// FIXME: Fix this once collector stuff is finalzied
	/*
		env.DataCollectors["tag_count_collector"] = func() phonelab.DataCollector {
			c := &TagCountCollector{}
			c.tagMap = make(map[string]int64)
			return c
		}
	*/
	env.DataCollectors["alarm_temp_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &AlarmTempCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.Semaphore = gsync.NewSem(100)
		return c
	}
	env.DataCollectors["alarm_trigger_temp_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &AlarmTriggerTempCollector{}
		path := kwargs["path"].(string)
		serializer, err := serialize.DetectSerializer(path)
		if err != nil {
			panic(err)
		}
		log.Debugf("Got serializer: %t", serializer)
		c.Serializer = serializer
		c.outPath = path
		c.Semaphore = gsync.NewSem(100)
		c.deviceMap = make(map[string]map[string]int32)
		return c
	}

	env.DataCollectors["stitch_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &StitchCollector{}
		c.chunkMap = make(map[string]map[string][]string)
		c.StitchInfo = make(map[string]*phonelab.StitchInfo)
		c.initialized = make(map[string]bool)
		c.delete = make(map[string]bool)
		c.outPath = kwargs["path"].(string)
		return c
	}
	env.DataCollectors["alarm_cpu_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &AlarmCpuCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.Semaphore = gsync.NewSem(100)
		return c
	}
	env.DataCollectors["alarm_wakelock_cpu_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &AlarmWakelockCpuCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.Semaphore = gsync.NewSem(100)
		return c
	}
	env.DataCollectors["screen_off_cpu_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &ScreenOffCpuCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.deviceDateMap = make(map[string]map[string]*ScreenOffCpuData)
		c.Semaphore = gsync.NewSem(10)
		return c
	}
	env.DataCollectors["alarms_per_device_day_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &AlarmsPerDDCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.deviceDataMap = make(map[string]map[string]int64)
		return c
	}
	env.DataCollectors["temperature_distribution_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &TDCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.TemperatureMap = make(map[string]DayTemperatureDistribution)
		return c
	}
	env.DataCollectors["suspend_cpu_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &SuspendCpuCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.Semaphore = gsync.NewSem(100)
		c.deviceDateMap = make(map[string]map[string][]*BusynessData)
		return c
	}
	env.DataCollectors["alarm_window_lengths_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &AlarmWindowLengthsCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.Semaphore = gsync.NewSem(100)
		c.deviceDataMap = make(map[string]map[string][]int64)
		return c
	}
	env.DataCollectors["missing_loglines_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &MissingLoglinesCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.Semaphore = gsync.NewSem(100)
		c.deviceDataMap = make(map[string][]*MissingLoglinesResult)
		return c
	}
	env.DataCollectors["algorithm_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &AlgorithmCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.Semaphore = gsync.NewSem(100)
		c.deviceDataMap = make(map[string]map[string][]int)
		return c
	}
	env.DataCollectors["ambient_temperature_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &AmbientTemperatureCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.Semaphore = gsync.NewSem(100)
		c.deviceDataMap = make(map[string]map[string][]int)
		return c
	}
	env.DataCollectors["sleep_duration_analysis_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &SleepDurationAnalysisCollector{}
		d, err := phonelab.NewDefaultCollector(kwargs)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		c.DefaultCollector = d.(*phonelab.DefaultCollector)
		c.Semaphore = gsync.NewSem(100)
		c.deviceDataMap = make(map[string]map[string][]int64)
		return c
	}

	env.Processors["tag_count_processor"] = &TagCountProcGenerator{}
	env.Processors["alarm_processor"] = &AlarmProcGenerator{}
	env.Processors["alarm_temp_processor"] = &AlarmTempProcGenerator{}
	env.Processors["alarm_trigger_temp_processor"] = &AlarmTriggerTempProcGenerator{}
	env.Processors["alarm_cpu_processor"] = &AlarmCpuProcGenerator{}
	env.Processors["alarm_wakelock_cpu_processor"] = &AlarmWakelockCpuProcGenerator{}
	env.Processors["alarms_per_device_day_processor"] = &AlarmsPerDDProcGenerator{}
	env.Processors["screen_off_cpu_processor"] = &ScreenOffCpuProcGenerator{}
	env.Processors["stitch_processor"] = &StitchGenerator{}
	env.Processors["stitch_checker_processor"] = &StitchCheckerProcGenerator{}
	env.Processors["temperature_distribution_processor"] = &TDProcGenerator{}
	env.Processors["suspend_cpu_processor"] = &SuspendCpuProcGenerator{}
	env.Processors["alarm_window_lengths_processor"] = &AlarmWindowLengthsProcGenerator{}
	env.Processors["missing_loglines_processor"] = &MissingLoglinesProcGenerator{}
	env.Processors["algorithm_processor"] = &AlgorithmProcGenerator{}
	env.Processors["ambient_temperature_from_suspend_processor"] = &AmbientTemperatureFromSuspendProcGenerator{}
	env.Processors["ambient_temperature_from_distribution_processor"] = &AmbientTemperatureFromDistributionProcGenerator{}
	env.Processors["sleep_duration_analysis_processor"] = &SleepDurationAnalysisProcGenerator{}

	// Parsers
	env.RegisterParserGenerator("ThermaPlan->AlarmManagerService", alarms.NewDeliverAlarmsLockedParser)
	env.RegisterParserGenerator("SurfaceFlinger", parsers.NewScreenStateParser)
	env.RegisterParserGenerator("ThermaPlan->WakeLock", parsers.NewThermaPlanWakelockParser)
}
