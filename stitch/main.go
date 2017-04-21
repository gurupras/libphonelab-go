package main

import (
	"log"
	"os"
	"runtime/pprof"

	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/libphonelab-go"
	"github.com/shaseley/phonelab-go"
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

	env := phonelab.NewEnvironment()
	libphonelab.InitEnv(env)
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
