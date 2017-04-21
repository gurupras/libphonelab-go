package main

import (
	"os"

	"github.com/gurupras/libphonelab-go"
	"github.com/shaseley/phonelab-go"
)

func main() {
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
