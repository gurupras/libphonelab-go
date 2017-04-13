package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/phonelab-go"
)

type TTGDProcGenerator struct{}

func (t *TTGDProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &TTGDProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type TTGDProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type Result struct {
	BasePath string
	DeviceId string
	BootId   string
	Diffs    []float64
	Lines    []string
}

func (p *TTGDProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})
	lastTimestamp := float64(0)

	result := &Result{}
	result.DeviceId = p.Info["deviceid"].(string)
	result.BootId = p.Info["bootid"].(string)
	result.BasePath = p.Info["basePath"].(string)
	result.Diffs = make([]float64, 0)
	result.Lines = make([]string, 0)
	go func() {
		defer close(outChan)
		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			switch t := ll.Payload.(type) {
			case *phonelab.ThermalTemp:
				if lastTimestamp != 0 {
					diff := t.Timestamp - lastTimestamp
					result.Diffs = append(result.Diffs, diff)
					if diff > 10 {
						result.Lines = append(result.Lines, ll.Line)
					}
				}
				lastTimestamp = t.Timestamp
			}
		}
		outChan <- result
	}()
	return outChan
}

type collector struct {
	sync.Mutex
	data []*Result
}

func (c *collector) OnData(data interface{}) {
	r := data.(*Result)
	c.Lock()
	defer c.Unlock()
	c.data = append(c.data, r)
}

func (c *collector) Finish() {
	deviceId := c.data[0].DeviceId
	basePath := c.data[0].BasePath
	outdir := filepath.Join(basePath, deviceId, "analysis")
	if !easyfiles.Exists(outdir) {
		err := easyfiles.Makedirs(outdir)
		if err != nil {
			panic(fmt.Sprintf("Failed to makedir: %v", outdir))
		}
	}

	b, err := json.Marshal(&c.data)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal final results: %v", err))
	}
	fpath := filepath.Join(outdir, "thermal_temp_gap_distribution.gz")
	f, err := easyfiles.Open(fpath, os.O_CREATE|os.O_WRONLY, easyfiles.GZ_TRUE)
	if err != nil {
		panic(fmt.Sprintf("Failed to open file: %v", fpath))
	}
	defer f.Close()
	writer, _ := f.Writer(0)
	defer writer.Close()
	defer writer.Flush()

	writer.Write(b)
}

func main() {
	env := phonelab.NewEnvironment()
	env.Parsers["Kernel-Trace"] = func() phonelab.Parser {
		return phonelab.NewKernelTraceParser()
	}
	env.DataCollectors["ttgd_collector"] = func() phonelab.DataCollector {
		c := &collector{}
		c.data = make([]*Result, 0)
		return c
	}
	env.Processors["ttgd_processor"] = &TTGDProcGenerator{}

	conf, err := phonelab.RunnerConfFromFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	runner, err := conf.ToRunner(env)
	if err != nil {
		panic(err)
	}
	runner.Run()
}
