package libphonelab

import (
	"io/ioutil"
	"log"
	"os"
	"sync"

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
	Info    phonelab.PipelineSourceInfo
	Start   int64           `json:"start"`
	End     int64           `json:"end"`
	Missing []*MissingChunk `json:"missing"`
}

func (p *MissingLoglinesProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	result := &MissingLoglinesResult{}
	result.Missing = make([]*MissingChunk, 0)
	result.Info = p.Info

	go func() {
		defer close(outChan)
		inChan := p.Source.Process()
		var (
			last    int64
			current int64
		)
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			if last == 0 {
				result.Start = ll.LogcatToken
				last = ll.LogcatToken
				continue
			}
			current = ll.LogcatToken

			if current > last+1 {
				missing := &MissingChunk{
					last,
					current,
				}
				result.Missing = append(result.Missing, missing)
			}
			last = current
		}
		result.End = last
		outChan <- result
	}()
	return outChan
}

type MissingLoglinesCollector struct {
	sync.Mutex
	Data []*MissingLoglinesResult
}

func (c *MissingLoglinesCollector) OnData(data interface{}) {
	// FIXME: Fix this once collector is finalized
	/*
		r := data.(*MissingLoglinesResult)
		c.Lock()
		defer c.Unlock()
		if strings.Compare(c.DeviceId, "") == 0 {
			c.DeviceId = r.DeviceId
			c.BootId = r.BootId
			c.BasePath = r.BasePath
			c.hdfsAddr = r.hdfsAddr
		}
		c.Data = append(c.Data, r)
	*/
}

func (c *MissingLoglinesCollector) Finish() {
	// FIXME: Fix this once collector is finalized
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

		b, err := json.Marshal(&c.Data)
		if err != nil {
			panic(fmt.Sprintf("Failed to marshal final results: %v", err))
		}
		fpath := filepath.Join(outdir, "missing_loglines.gz")
		file, err := hdfs.OpenFile(fpath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, easyfiles.GZ_TRUE, client)
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

func MissingLoglinesMain() {
	log.SetOutput(ioutil.Discard)
	env := phonelab.NewEnvironment()

	/*
		env.DataCollectors["missing_loglines_collector"] = func() phonelab.DataCollector {
			c := &MissingLoglinesCollector{}
			c.Data = make([]*MissingLoglinesResult, 0)
			return c
		}
	*/
	env.Processors["missing_loglines_processor"] = &MissingLoglinesProcGenerator{}

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
