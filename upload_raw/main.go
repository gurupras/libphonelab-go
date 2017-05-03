package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/bmatcuk/doublestar"
	"github.com/colinmarc/hdfs"
	"github.com/gurupras/go-easyfiles/easyhdfs"
	"github.com/gurupras/gocommons/gsync"
	log "github.com/sirupsen/logrus"
)

var (
	app        = kingpin.New("upload-raw", "upload raw files to HDFS")
	hdfsAddr   = app.Arg("hdfs-addr", "HDFS address").String()
	sourcePath = app.Arg("source-path", "Path to source").String()
	outPath    = app.Arg("output-path", "Output path").String()
	regex      = app.Flag("regex", "Regex to find files to upload").Default("**/*.out.gz").String()
)

func isNewer(src string, dest string, client *hdfs.Client) (bool, string) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		log.Fatalf("Failed src info: %v", err)
	}
	destInfo, err := client.Stat(dest)
	if err != nil || destInfo == nil {
		return true, fmt.Sprintf("missing")
	}
	if srcInfo.ModTime().After(destInfo.ModTime()) && srcInfo.ModTime().Sub(destInfo.ModTime()) > 1*time.Second {
		return true, fmt.Sprintf("modtime: %v > %v", srcInfo.ModTime(), destInfo.ModTime())
	}
	if srcInfo.Size() != destInfo.Size() {
		return true, fmt.Sprintf("size: %v != %v", srcInfo.Size(), destInfo.Size())
	}
	return false, "equal"
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	client, err := hdfs.New(*hdfsAddr)
	if err != nil {
		log.Fatalf("Failed to get HDFS client: %v", err)
	}
	_ = client
	log.Infof("Got client")

	fullRegex := filepath.Join(*sourcePath, *regex)
	log.Infof("Regex=%v", fullRegex)
	sourceFiles, err := doublestar.Glob(fullRegex)
	if err != nil {
		log.Fatalf("Failed to glob: %v", err)
	}
	log.Infof("Found %d files", len(sourceFiles))

	sem := gsync.NewSem(16)
	wg := sync.WaitGroup{}

	fs := easyhdfs.NewHDFSFileSystem(*hdfsAddr)

	for _, file := range sourceFiles {
		sem.P()
		wg.Add(1)
		go func(file string) {
			defer sem.V()
			defer wg.Done()
			relativePath, err := filepath.Rel(*sourcePath, file)
			if err != nil {
				log.Fatalf("Failed to get relative path: %v", err)
			}
			outputPath := filepath.Join(*outPath, relativePath)
			if newer, reason := isNewer(file, outputPath, client); newer {
				log.Infof("Copying '%v' -> '%v' (%v)", file, outputPath, reason)
				dir := path.Dir(outputPath)
				fs.Makedirs(dir)

				err := client.CopyToRemote(file, outputPath)
				if err != nil {
					log.Fatalf("Failed to copy to remote: %v", err)
				}
				info, err := os.Stat(file)
				if err != nil {
					log.Fatalf("Failed to stat local file: %v", err)
				}
				if err = client.Chtimes(outputPath, info.ModTime(), info.ModTime()); err != nil {
					log.Fatalf("Failed to chtimes file: %v", err)
				}
			}
		}(file)
	}
	wg.Wait()
}
