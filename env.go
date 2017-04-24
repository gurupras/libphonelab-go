package libphonelab

import (
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/gurupras/libphonelab-go/parsers"
	"github.com/shaseley/phonelab-go"
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
		env.DataCollectors["alarm_temp_collector"] = func() phonelab.DataCollector {
			c := &AlarmTempCollector{}
			return c
		}
	*/

	env.DataCollectors["stitch_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &StitchCollector{}
		c.chunks = make([]string, 0)
		c.files = make([]string, 0)
		c.outPath = kwargs["path"].(string)
		return c
	}
	env.DataCollectors["alarm_cpu_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &AlarmCpuCollector{}
		c.outPath = kwargs["path"].(string)
		return c
	}
	env.DataCollectors["screen_off_cpu_collector"] = func(kwargs map[string]interface{}) phonelab.DataCollector {
		c := &ScreenOffCpuCollector{}
		c.outPath = kwargs["path"].(string)
		return c
	}

	env.Processors["tag_count_processor"] = &TagCountProcGenerator{}
	env.Processors["alarm_temp_processor"] = &AlarmTempProcGenerator{}
	env.Processors["alarm_cpu_processor"] = &AlarmCpuProcGenerator{}
	env.Processors["screen_off_cpu_processor"] = &ScreenOffCpuProcGenerator{}
	env.Processors["stitch_processor"] = &StitchGenerator{}

	// Parsers
	env.RegisterParserGenerator("ThermaPlan->AlarmManagerService", alarms.NewDeliverAlarmsLockedParser)
	env.RegisterParserGenerator("SurfaceFlinger", parsers.NewScreenStateParser)
}
