package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fujiwara/knockrd"
	"github.com/hashicorp/logutils"
)

const version = "0.0.3"

var filter = &logutils.LevelFilter{
	Levels:   []logutils.LogLevel{"debug", "info", "warn", "error"},
	MinLevel: logutils.LogLevel("info"),
	Writer:   os.Stderr,
}

func main() {
	var configFile, run string
	var debug, showVersion bool

	flag.StringVar(&configFile, "config", "", "config file name")
	flag.BoolVar(&debug, "debug", false, "enable debug log")
	flag.StringVar(&run, "run", "http", "run mode. http or stream")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.VisitAll(func(f *flag.Flag) {
		if s := os.Getenv(strings.ToUpper("KNOCKRD_" + f.Name)); s != "" {
			f.Value.Set(s)
		}
	})
	flag.Parse()

	if showVersion {
		fmt.Println("knockrd version", version)
		return
	}

	if debug {
		filter.MinLevel = logutils.LogLevel("debug")
	}
	log.SetOutput(filter)

	cfg, err := knockrd.LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	switch run {
	case "http", "stream":
	default:
		log.Fatalf("invalid run mode %s", run)
	}
	log.Fatal(knockrd.Run(cfg, run == "stream"))
}
