package libphonelab

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gurupras/go-easyfiles"
	"github.com/shaseley/phonelab-go"
	"github.com/shaseley/phonelab-go/hdfs"
)

type TagCountProcGenerator struct{}

func (t *TagCountProcGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &TagCountProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type TagCountProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type TagCountResult struct {
	*phonelab.PhonelabSourceInfo
	TagMap map[string]int64
}

func (p *TagCountProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	result := &TagCountResult{}
	result.PhonelabSourceInfo = p.Info["source_info"].(*phonelab.PhonelabSourceInfo)
	result.TagMap = make(map[string]int64)

	go func() {
		defer close(outChan)
		inChan := p.Source.Process()
		for obj := range inChan {
			ll, ok := obj.(*phonelab.Logline)
			if !ok {
				continue
			}
			var tag string
			switch t := ll.Payload.(type) {
			case phonelab.TraceInterface:
				tag = t.TraceTag()
			default:
				tag = ll.Tag
			}
			result.TagMap[tag] += 1
		}
		outChan <- result
	}()
	return outChan
}

type TagCountCollector struct {
	sync.Mutex
	tagMap map[string]int64
	*phonelab.PhonelabSourceInfo
}

func (c *TagCountCollector) OnData(data interface{}) {
	r := data.(*TagCountResult)
	c.Lock()
	defer c.Unlock()
	if c.PhonelabSourceInfo == nil {
		c.PhonelabSourceInfo = r.PhonelabSourceInfo
	}
	for k, v := range r.TagMap {
		c.tagMap[k] += v
	}
}

func (c *TagCountCollector) Finish() {
	deviceId := c.DeviceId
	basePath := c.Path

	outdir := filepath.Join(basePath, deviceId, "analysis")

	client, err := hdfs.NewHdfsClient(c.HdfsAddr)
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

	b, err := json.Marshal(&c.tagMap)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal final results: %v", err))
	}
	fpath := filepath.Join(outdir, "tag_count.gz")

	file, err := hdfs.OpenFile(fpath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, easyfiles.GZ_TRUE, client)
	if err != nil {
		panic(fmt.Sprintf("Failed to open file: %v", fpath))
	}
	defer file.Close()
	writer, _ := file.Writer(0)
	defer writer.Close()
	defer writer.Flush()

	writer.Write(b)
}

func TagCountMain() {
	env := phonelab.NewEnvironment()
	InitEnv(env)
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
