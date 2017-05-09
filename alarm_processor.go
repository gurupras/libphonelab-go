package libphonelab

import (
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/shaseley/phonelab-go"
)

type AlarmProcGenerator struct{}

func (t *AlarmProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &AlarmProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type AlarmProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

func (p *AlarmProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{}, 100)

	go func() {
		defer close(outChan)
		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			switch ll.Payload.(type) {
			case *alarms.DeliverAlarmsLocked:
				deliverAlarm := ll.Payload.(*alarms.DeliverAlarmsLocked)
				outChan <- deliverAlarm
			}
		}
	}()
	return outChan
}
