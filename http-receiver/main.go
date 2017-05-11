package main

import (
	"bytes"
	"io"
	"net/http"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/gorilla/mux"
	"github.com/shaseley/phonelab-go/serialize"
	log "github.com/sirupsen/logrus"
)

var (
	app             = kingpin.New("http-receiver", "HTTP receiver for phonelab-go")
	outdir          = app.Flag("outdir", "Output directory").Short('o').Required().String()
	port            = app.Flag("port", "Port on which to listen").Short('p').Default("31442").Int()
	shouldSerialize = app.Flag("serialize", "Serialize data").Short('s').Default("false").Bool()
)

func handle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	_ = vars
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, r.Body)

	log.Infof("Received:\n%v\n", string(buf.Bytes()))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	receiver := serialize.NewHTTPReceiver(*outdir)
	if *shouldSerialize {
		receiver.AddHTTPSerializeCallback()
	}

	c := make(chan interface{})
	receiver.RunHTTPReceiver(*port)
	_ = <-c
}
