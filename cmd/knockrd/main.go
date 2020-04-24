package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/fujiwara/knockrd"
	"github.com/hashicorp/logutils"
)

var filter = &logutils.LevelFilter{
	Levels:   []logutils.LogLevel{"debug", "info", "warn", "error"},
	MinLevel: logutils.LogLevel("info"),
	Writer:   os.Stderr,
}

func main() {
	var configFile, run string
	var debug bool

	flag.StringVar(&configFile, "config", "", "config file name")
	flag.BoolVar(&debug, "debug", false, "enable debug log")
	flag.StringVar(&run, "run", "http", "run mode. http or stream")
	flag.VisitAll(func(f *flag.Flag) {
		if s := os.Getenv(strings.ToUpper("KNOCKRD_" + f.Name)); s != "" {
			f.Value.Set(s)
		}
	})
	flag.Parse()

	if debug {
		filter.MinLevel = logutils.LogLevel("debug")
	}
	log.SetOutput(filter)

	cfg, err := knockrd.LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(knockrd.Run(cfg, run == "stream"))
}
