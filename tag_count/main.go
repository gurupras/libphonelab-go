package main

import (
	"os"
	"runtime/pprof"

	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/libphonelab-go"
	log "github.com/sirupsen/logrus"
)

func main() {
	var f *os.File
	var err error
	if easyfiles.Exists("cpuprofile.prof") {
		os.Remove("cpuprofile.prof")
	}
	f, err = os.Create("cpuprofile.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	libphonelab.TagCountMain()
}
