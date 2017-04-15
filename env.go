package libphonelab

import "github.com/shaseley/phonelab-go"

func InitEnv(env *phonelab.Environment) {
	env.Parsers["Kernel-Trace"] = func() phonelab.Parser {
		p := phonelab.NewKernelTraceParser()
		p.ErrOnUnknownTag = false
		return p
	}
	env.DataCollectors["tag_count_collector"] = func() phonelab.DataCollector {
		c := &TagCountCollector{}
		c.tagMap = make(map[string]int64)
		return c
	}
	env.Processors["tag_count_processor"] = &TagCountProcGenerator{}
}
