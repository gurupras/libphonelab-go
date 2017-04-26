package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"
	"testing"

	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/libphonelab-go"
	"github.com/shaseley/phonelab-go"
)

var (
	file = flag.String("file", "", "yaml file")
)

func TestMain(t *testing.T) {
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

	env := phonelab.NewEnvironment()
	libphonelab.InitEnv(env)
	conf, err := phonelab.RunnerConfFromFile(*file)
	if err != nil {
		panic(err)
	}
	runner, err := conf.ToRunner(env)
	if err != nil {
		panic(err)
	}
	runner.Run()
}
