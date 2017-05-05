package libphonelab

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/set"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
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
	DeviceId             string `json:"device_id"`
	BootId               string `json:"boot_id"`
	BootIdFail           bool   `json:"boot_id_fail"`
	IncreasingTokenFail  bool   `json:"increasing_token_fail"`
	DuplicateLoglineFail bool   `json:"duplicate_logline_fail"`
	LineCount            int64  `json:"line_count"`
}

func (s *StitchCheckerData) String() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (p *StitchCheckerProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)

	data := &StitchCheckerData{}
	data.DeviceId = sourceInfo.DeviceId
	data.BootId = sourceInfo.BootId

	checksums := set.New()
	ch := make(map[string]struct{})

	var lastLogcatToken int64
	go func() {
		defer close(outChan)
		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			data.LineCount++

			h := md5.New()
			io.WriteString(h, ll.Line)
			checksum := fmt.Sprintf("%x", h.Sum(nil))
			if _, ok := ch[checksum]; ok {
				log.Errorf("%v->%v Duplicate logline: \n%v", data.DeviceId, data.BootId, ll.Line)
				log.Infof("Checksum: %v", checksum)
				for idx, obj := range checksums.List() {
					log.Infof("list[%d]=%v", idx, obj)
				}
				data.DuplicateLoglineFail = true
				break
			}
			checksums.Add(checksum)
			ch[checksum] = struct{}{}

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
