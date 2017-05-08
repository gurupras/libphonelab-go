package main

import (
	"os"

	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/libphonelab-go"
	"github.com/shaseley/phonelab-go"
	log "github.com/sirupsen/logrus"
)

func InitEnv(env *phonelab.Environment) {
	libphonelab.InitEnv(env)
}

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
	defer f.Close()

	//log.SetLevel(log.DebugLevel)
	log.Debugf("Starting...")

	//pprof.StartCPUProfile(f)
	//defer pprof.StopCPUProfile()

	if easyfiles.Exists("memprofile.prof") {
		os.Remove("memprofile.prof")
	}
	f, err = os.Create("memprofile.prof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

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
	//pprof.WriteHeapProfile(f)
}
