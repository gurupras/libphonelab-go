package libphonelab

import (
	"strings"

	"github.com/labstack/gommon/log"
	"github.com/shaseley/phonelab-go"
)

type StitchCheckerProcGenerator struct{}

func (t *StitchCheckerProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &StitchCheckerProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type StitchCheckerProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type StitchCheckerData struct {
	DeviceId            string `json:"deviceId"`
	BootId              string `json:"bootId"`
	BootIdFail          bool   `json:"bootIdFail"`
	IncreasingTokenFail bool   `json:"increasingTokenFail"`
}

func (p *StitchCheckerProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)

	data := &StitchCheckerData{}
	data.DeviceId = sourceInfo.DeviceId
	data.BootId = sourceInfo.BootId

	var lastLogcatToken int64
	go func() {
		defer close(outChan)
		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}

			if strings.Compare(ll.BootId, data.BootId) != 0 {
				if !data.BootIdFail {
					log.Errorf("%v->%v inconsistent bootID: \n%v", data.DeviceId, data.BootId, ll.Line)
					data.BootIdFail = true
					break
				}
			}

			if lastLogcatToken == 0 {
				lastLogcatToken = ll.LogcatToken
			} else if lastLogcatToken > ll.LogcatToken {
				if !data.IncreasingTokenFail {
					log.Errorf("%v->%v tokens not in sorted order: \n%v", data.DeviceId, data.BootId, ll.Line)
					data.IncreasingTokenFail = true
					break
				}
			}
		}
		outChan <- data
	}()
	return outChan
}
