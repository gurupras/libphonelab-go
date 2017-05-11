package libphonelab

import (
	"path/filepath"
	"sync"

	"github.com/fatih/set"
	"github.com/gurupras/gocommons/gsync"
	"github.com/shaseley/phonelab-go"
)

type MissingLoglinesProcGenerator struct{}

func (t *MissingLoglinesProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &MissingLoglinesProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type MissingLoglinesProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type MissingChunk struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}
type MissingLoglinesResult struct {
	BootId  string          `json:"boot_id"`
	Start   int64           `json:"start"`
	End     int64           `json:"end"`
	Missing []*MissingChunk `json:"missing"`
}

// First thing to remember is that tokens are not always in order.
// Stitch sorts them by timestamps since these being sequential is more
// important than tokens being sequential. Thus, we may find tokens that
// are out of order and have to account for this while attempting to compute
// missing tokens.
func (p *MissingLoglinesProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	sourceInfo := p.Info.(*phonelab.PhonelabSourceInfo)

	result := &MissingLoglinesResult{}
	result.Missing = make([]*MissingChunk, 0)
	result.BootId = sourceInfo.BootId

	seen := set.NewNonTS()
	last := int64(0)

	go func() {
		defer close(outChan)
		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			if last == 0 {
				result.Start = ll.LogcatToken
			}
			seen.Add(ll.LogcatToken)
			last = ll.LogcatToken
		}

		result.End = last
		for idx := result.Start; idx < result.End; idx++ {
			if !seen.Has(idx) {
				start := idx
				var end int64
				for end = start + 1; end < result.End; end++ {
					if seen.Has(end) {
						break
					}
				}
				missing := &MissingChunk{
					start,
					end - 1,
				}
				result.Missing = append(result.Missing, missing)
			}
		}
		outChan <- result
	}()
	return outChan
}

type MissingLoglinesCollector struct {
	*phonelab.DefaultCollector
	sync.Mutex
	wg sync.WaitGroup
	*gsync.Semaphore
	deviceDataMap map[string][]*MissingLoglinesResult
}

func (c *MissingLoglinesCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	sourceInfo := info.(*phonelab.PhonelabSourceInfo)
	deviceId := sourceInfo.DeviceId

	r := data.(*MissingLoglinesResult)
	if _, ok := c.deviceDataMap[deviceId]; !ok {
		c.Lock()
		c.deviceDataMap[deviceId] = make([]*MissingLoglinesResult, 0)
		c.Unlock()
	}
	c.deviceDataMap[deviceId] = append(c.deviceDataMap[deviceId], r)
}

func (c *MissingLoglinesCollector) Finish() {
	for deviceId, data := range c.deviceDataMap {
		path := filepath.Join(deviceId, "analysis", "missing_loglines")
		info := &CustomInfo{path, "custom"}
		c.DefaultCollector.OnData(data, info)
	}
}

func MissingLoglinesMain() {
}
