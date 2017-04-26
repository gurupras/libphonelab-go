package libphonelab

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-external-sort"
	"github.com/gurupras/gocommons/gsync"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

type StitchGenerator struct{}

func (t *StitchGenerator) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {
	return &StitchProcessor{
		Source: source.Processor,
		Info:   source.Info,
	}
}

type StitchProcessor struct {
	Source phonelab.Processor
	Info   phonelab.PipelineSourceInfo
}

type SortableLogline phonelab.Logline
type SortableLoglines []SortableLogline

func (l *SortableLogline) String() string {
	return l.Line
}

func (l *SortableLogline) Less(s extsort.SortInterface) (ret bool, err error) {
	var o *SortableLogline
	var ok bool
	if s != nil {
		if o, ok = s.(*SortableLogline); !ok {
			err = errors.New(fmt.Sprintf("Failed to convert from SortInterface to *Logline: %v", reflect.TypeOf(s)))
			ret = false
			goto out
		}
	}
	if l != nil && o != nil {
		bootComparison := strings.Compare(l.BootId, o.BootId)
		if bootComparison == -1 {
			ret = true
		} else if bootComparison == 1 {
			ret = false
		} else {
			// Same boot ID..compare the other fields
			if l.LogcatToken == o.LogcatToken {
				ret = l.TraceTime < o.TraceTime
			} else {
				ret = l.LogcatToken < o.LogcatToken
			}
		}
	} else if l != nil {
		ret = true
	} else {
		ret = false
	}
out:
	return
}

var parser = phonelab.NewLogcatParser()

func ParseConvert(line string) extsort.SortInterface {
	logline, err := parser.Parse(line)
	if err != nil {
		log.Errorf("Failed to parse logline: \n%v\nError: %v\n", line, err)
		return nil
	}
	ret := SortableLogline(*logline)
	return &ret
}

var SortParams = extsort.SortParams{
	Instance: func() extsort.SortInterface {
		ret := SortableLogline{}
		return &ret
	},
	LineConvert: ParseConvert,
	Lines:       make(extsort.SortCollection, 0),
	FSInterface: nil,
}

type ChunkData struct {
	*phonelab.StitchInfo
	File   string
	Chunks []string
}

func (p *StitchProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	var stitchInfo *phonelab.StitchInfo
	if SortParams.FSInterface == nil {
		sourceInfo := p.Info.(*phonelab.PhonelabRawInfo)
		stitchInfo = sourceInfo.StitchInfo
		SortParams.FSInterface = sourceInfo.FSInterface
	}

	inChan := p.Source.Process()

	sem := gsync.NewSem(12)
	go func() {
		defer close(outChan)
		wg := sync.WaitGroup{}

		sentOne := false
		for obj := range inChan {
			//file, ok := obj.(string)
			//if !ok {
			//	log.Fatalf("Failed to get a file from channel. Got: \n%v\n", obj)
			//}
			file := obj.(string)
			sem.P()
			wg.Add(1)
			go func(file string) {
				defer sem.V()
				defer wg.Done()
				log.Infof("Processing file=%v", file)
				// Call external sort on this
				bufsize := 64 * 1048576
				chunks, err := extsort.ExternalSort(file, bufsize, SortParams)
				if err != nil {
					log.Fatalf("Failed to run external sort on file: %v: %v", file, err)
				}
				outChan <- &ChunkData{stitchInfo, file, chunks}
				sentOne = true
			}(file)
		}
		if !sentOne {
			outChan <- &ChunkData{stitchInfo, "", []string{}}
		}
		wg.Wait()
	}()
	return outChan
}

type StitchCollector struct {
	deviceId    string
	delete      bool
	outPath     string
	chunkMap    map[string][]string
	initialized bool
	*phonelab.StitchInfo
}

func (s *StitchCollector) OnData(data interface{}, info phonelab.PipelineSourceInfo) {
	s.deviceId = info.Context()

	log.Infof("s.OnData()")
	log.Infof("s.outPath=%v", s.outPath)

	chunkData := data.(*ChunkData)

	stitchInfo := chunkData.StitchInfo
	if !s.initialized {
		if stitchInfo == nil {
			s.delete = true
			s.StitchInfo = phonelab.NewStitchInfo()
			log.Infof("StitchInfo was nil..created a new one")
		} else {
			s.delete = false
			s.StitchInfo = stitchInfo
			log.Infof("Using existing StitchInfo")
		}
		s.initialized = true
	}

	if strings.Compare(chunkData.File, "") != 0 {
		s.chunkMap[chunkData.File] = chunkData.Chunks
	}
}

func (s *StitchCollector) Finish() {
	log.Infof("StitchCollector finish()")
	//log.Infof("outPath=%v", s.outPath)
	devicePath := filepath.Join(s.outPath, s.deviceId)
	//log.Infof("devicePath=%v", devicePath)

	log.Debugf("Calling doNWayMerge")
	doNWayMerge(devicePath, s.chunkMap, s.StitchInfo, s.delete, 100000)

	// Update files
	if len(s.chunkMap) == 0 {
		// Special case. If no files were present, then there is a chance
		// that s.StitchInfo was never initialized
		if s.StitchInfo == nil {
			s.StitchInfo = phonelab.NewStitchInfo()
		}
	} else {
		for file, _ := range s.chunkMap {
			s.StitchInfo.Files = append(s.StitchInfo.Files, file)
		}
	}
	sort.Sort(sort.StringSlice(s.StitchInfo.Files))

	// Write info.json
	infoJsonPath := filepath.Join(devicePath, "info.json")
	log.Infof("Writing info.json: %v", infoJsonPath)
	b, err := json.MarshalIndent(s.StitchInfo, "", "    ")
	if err != nil {
		log.Fatalf("Failed to marshal StitchInfo to write to info.json: %v", err)
	}
	f, err := SortParams.FSInterface.Open(infoJsonPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, easyfiles.GZ_FALSE)
	if err != nil {
		log.Fatalf("Failed to create/open: %v: %v", infoJsonPath, err)
	}
	defer f.Close()

	writer, err := f.Writer(0)
	if err != nil {
		log.Fatalf("Failed to get writer to info.json: %v", err)
	}
	defer writer.Close()
	defer writer.Flush()

	if _, err = writer.Write(b); err != nil {
		log.Fatalf("Failed to write to info.json: %v", err)
	}

	for _, chunks := range s.chunkMap {
		for _, chunk := range chunks {
			if err := SortParams.FSInterface.Remove(chunk); err != nil {
				log.Fatalf("Failed to remove: %v", chunk)
			}
		}
	}
}

func doNWayMerge(devicePath string, chunkMap map[string][]string, info *phonelab.StitchInfo, delete bool, lines_per_file int) {
	bootid_channel_map := make(map[string]chan *SortableLogline)

	linesWritten := uint32(0)
	boot_id_consumer := func(boot_id string, channel chan *SortableLogline, wg *sync.WaitGroup) {
		defer wg.Done()
		var err error

		//fmt.Println("Starting consumer for bootid:", boot_id)
		cur_idx := 0
		cur_line_count := 0
		outdir := filepath.Join(devicePath, boot_id)
		var cur_filename string
		var cur_file *easyfiles.File
		var cur_file_writer *easyfiles.Writer

		fs := SortParams.FSInterface
		// Make directory if it doesn't exist
		exists, err := fs.Exists(outdir)
		if err != nil {
			log.Fatalf("Failed to check if dir exists: %v: %v", outdir, err)
		}
		if delete && exists {
			// Does exit
			log.Infof("Attempting to delete existing directory: %v", outdir)
			fs.RemoveAll(outdir)
		}
		log.Infof("Attempting to create directory:%s...", outdir)
		if err = fs.Makedirs(outdir); err != nil {
			log.Fatalf("Failed to create directory: %v", outdir)
		} else {
		}
		if !delete {
			// We need to get list of files so we know the
			// index from where we can start adding new
			// files
			var files []string

			if files, err = fs.Glob(filepath.Join(outdir, "*.gz")); err != nil {
				log.Fatalf("Failed to list files: %v", outdir)
			}
			sort.Sort(sort.StringSlice(files))
			if len(files) > 0 {
				// Move it ahead by 1
				cur_idx = len(files)
				log.Infof("New idx: %v", cur_idx)
			}
		}

		var fileInfo *phonelab.StitchFileInfo

		cleanup := func() {
			if err = cur_file_writer.Flush(); err != nil {
				log.Fatalf("Failed writer flush: %v: %v", cur_filename, err)
			}
			if err = cur_file_writer.Close(); err != nil {
				log.Fatalf("Failed writer close: %v: %v", cur_filename, err)
			}
			if err = cur_file.Close(); err != nil {
				log.Fatalf("Failed file close: %v: %v", cur_filename, err)
			}
			basename := path.Base(cur_filename)
			info.BootInfo[boot_id][basename] = fileInfo
		}

		first_file := true
		for {
			if cur_line_count == 0 {
				// Line count is 0. Either this is the first file or we just reset stuff
				// and so we need to open a new file
				if !first_file {
					// Close the old file
					//fmt.Println("\tClosing old file")
					cleanup()
				} else {
					first_file = false
				}
				// Open up a new file
				cur_filename = filepath.Join(outdir, fmt.Sprintf("%08d.gz", cur_idx))
				fileInfo = &phonelab.StitchFileInfo{}
				//fmt.Println("New File:", cur_filename)
				if cur_file, err = fs.Open(cur_filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, easyfiles.GZ_TRUE); err != nil {
					log.Fatalf("Could not open: %v, %v", cur_filename, err)
					return
				}
				if cur_file_writer, err = cur_file.Writer(0); err != nil {
					log.Fatalf("Could not get writer: %v: %v", cur_filename, err)
				}
			}
			if cur_line_count == lines_per_file {
				// We've reached the allotted lines per file. Rotate.
				//fmt.Println("Rotating to new file ...")
				cur_idx++
				cur_line_count = 0
				// We haven't read a line yet. So we can re-enter the loop here
				continue
			}
			// All the file stuff has been set up.
			// Go ahead and read a line from the channel
			if logline, ok := <-channel; !ok {
				// Channel was closed. We're finished reading.
				// Cleanup
				log.Infof("Cleaning up: %v", boot_id)
				cleanup()
				break
			} else {
				switch cur_line_count {
				case 0:
					// First line..update fileInfo's start
					fileInfo.Start = logline.Datetime.UnixNano()
				default:
					fileInfo.End = logline.Datetime.UnixNano()
				}
				if cur_line_count != lines_per_file {
					_, err = cur_file_writer.Write([]byte(logline.Line + "\n"))
				} else {
					_, err = cur_file_writer.Write([]byte(logline.Line))
				}
				if err != nil {
					log.Fatalf("Failed to write to file: %v: %v", cur_filename, err)
				}
				cur_line_count++
			}
		}
	}

	var wg sync.WaitGroup

	// Now start the n-way merge generator for each file
	keys := make([]string, len(chunkMap))
	idx := 0
	for k, _ := range chunkMap {
		keys[idx] = k
		idx++
	}
	sort.Sort(sort.StringSlice(keys))
	for _, key := range keys {
		chunks := chunkMap[key]
		localOutChan, err := extsort.NWayMergeGenerator(chunks, SortParams)
		if err != nil {
			log.Fatalf("%v", err)
		}
		for {
			si, ok := <-localOutChan
			if !ok {
				break
			}
			logline, ok := si.(*SortableLogline)
			if !ok {
				log.Fatalf("Could not convert to logline: \n%v\n%v", si.String(), err)
			}

			boot_id := logline.BootId
			// Check if the map has this bootid.
			if _, ok := bootid_channel_map[boot_id]; !ok {
				// Does not exist
				// Check if this bootid exists in info. If not,
				// create it.
				if _, ok := info.BootInfo[boot_id]; !ok {
					// Does not exist. Create it
					log.Infof("Creating new BootInfo entry for bootid: %v", boot_id)
					info.BootInfo[boot_id] = make(map[string]*phonelab.StitchFileInfo)
				}

				// Add it in and create a consumer
				bootid_channel_map[boot_id] = make(chan *SortableLogline, 10)
				wg.Add(1)
				go boot_id_consumer(boot_id, bootid_channel_map[boot_id], &wg)
			}
			// Write line to channel
			bootid_channel_map[boot_id] <- logline
			linesWritten++
		}
	}

	log.Infof("Cleaning up.. Wrote a total of %d lines", linesWritten)
	// Done reading the file. Now close the channels
	for boot_id := range bootid_channel_map {
		close(bootid_channel_map[boot_id])
	}
	log.Infof("Waiting for bootid_consumers to complete...")
	wg.Wait()
	log.Infof("doNWayMerge: Done")
}
