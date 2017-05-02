package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var (
	app  = kingpin.New("http-receiver", "HTTP receiver for phonelab-go")
	port = app.Flag("port", "Port on which to listen").Short('p').Default("31442").Int()
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

	r := mux.NewRouter()
	r.HandleFunc("/upload/{relpath:[\\S+/]+}", handle)
	http.Handle("/", r)

	addr := fmt.Sprintf(":%v", *port)
	log.Errorf("%v", http.ListenAndServe(addr, nil))
}
