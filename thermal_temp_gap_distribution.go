package libphonelab

import (
	"os"
	"sync"

	"github.com/shaseley/phonelab-go"
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

type TTGDResult struct {
	phonelab.PipelineSourceInfo
	Diffs []float64
	Lines []string
}

func (p *TTGDProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})
	lastTimestamp := float64(0)

	result := &TTGDResult{}
	result.Diffs = make([]float64, 0)
	result.Lines = make([]string, 0)
	result.PipelineSourceInfo = p.Info

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

type TTGDCollector struct {
	sync.Mutex
	data []*TTGDResult
}

func (c *TTGDCollector) OnData(data interface{}) {
	/*
		r := data.(*TTGDResult)
		c.Lock()
		defer c.Unlock()
		if strings.Compare(c.DeviceId, "") == 0 {
			c.DeviceId = r.DeviceId
			c.BootId = r.BootId
			c.BasePath = r.BasePath
			c.hdfsAddr = r.hdfsAddr
		}
		c.data = append(c.data, r)
	*/
}

func (c *TTGDCollector) Finish() {
	/*
		deviceId := c.DeviceId
		basePath := c.BasePath
		outdir := filepath.Join(basePath, deviceId, "analysis")

		client, err := hdfs.NewHdfsClient(c.hdfsAddr)
		if err != nil {
			panic(fmt.Sprintf("Failed to get hdfs client: %v", err))
		}
		if client != nil {
			err = client.MkdirAll(outdir, 0775)
		} else {
			if !easyfiles.Exists(outdir) {
				err = easyfiles.Makedirs(outdir)
			}
		}
		if err != nil {
			panic(fmt.Sprintf("Failed to makedir: %v", outdir))
		}

		b, err := json.Marshal(&c.data)
		if err != nil {
			panic(fmt.Sprintf("Failed to marshal final results: %v", err))
		}
		fpath := filepath.Join(outdir, "thermal_temp_gap_distribution.gz")
		file, err := hdfs.OpenFile(fpath, os.O_CREATE|os.O_WRONLY, easyfiles.GZ_TRUE, client)
		if err != nil {
			panic(fmt.Sprintf("Failed to open file: %v", fpath))
		}
		defer file.Close()
		writer, _ := file.Writer(0)
		defer writer.Close()
		defer writer.Flush()

		writer.Write(b)
	*/
}

func TTGDMain() {
	env := phonelab.NewEnvironment()
	env.Parsers["Kernel-Trace"] = func() phonelab.Parser {
		return phonelab.NewKernelTraceParser()
	}
	/*
		env.DataCollectors["ttgd_collector"] = func() phonelab.DataCollector {
			c := &TTGDCollector{}
			c.data = make([]*TTGDResult, 0)
			return c
		}
	*/
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
