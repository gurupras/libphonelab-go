package libphonelab

import (
	"github.com/gurupras/libphonelab-go/alarms"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
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

type AlarmResult struct {
	*alarms.DeliverAlarmsLocked
	Info phonelab.PipelineSourceInfo
}

func (p *AlarmProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	go func() {
		defer close(outChan)
		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			switch ll.Payload.(type) {
			default:
				deliverAlarm, err := alarms.ParseDeliverAlarmsLocked(ll)
				if err != nil {
					log.Errorf("Failed to parse deliverAlarm: %v", err)
				}
				if deliverAlarm != nil {
					outChan <- &AlarmResult{deliverAlarm, p.Info}
				}
			}
		}
	}()
	return outChan
}
