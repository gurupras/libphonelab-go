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
		c.TemperatureMap = make(map[string]map[string]int64)
		return c
	}

	env.Processors["tag_count_processor"] = &TagCountProcGenerator{}
	env.Processors["alarm_temp_processor"] = &AlarmTempProcGenerator{}
	env.Processors["alarm_trigger_temp_processor"] = &AlarmTriggerTempProcGenerator{}
	env.Processors["alarm_cpu_processor"] = &AlarmCpuProcGenerator{}
	env.Processors["alarm_wakelock_cpu_processor"] = &AlarmWakelockCpuProcGenerator{}
	env.Processors["alarms_per_device_day_processor"] = &AlarmsPerDDProcGenerator{}
	env.Processors["screen_off_cpu_processor"] = &ScreenOffCpuProcGenerator{}
	env.Processors["stitch_processor"] = &StitchGenerator{}
	env.Processors["stitch_checker_processor"] = &StitchCheckerProcGenerator{}
	env.Processors["temperature_distribution_processor"] = &TDProcGenerator{}

	// Parsers
	env.RegisterParserGenerator("ThermaPlan->AlarmManagerService", alarms.NewDeliverAlarmsLockedParser)
	env.RegisterParserGenerator("SurfaceFlinger", parsers.NewScreenStateParser)
	env.RegisterParserGenerator("ThermaPlan->WakeLock", parsers.NewThermaPlanWakelockParser)
}
