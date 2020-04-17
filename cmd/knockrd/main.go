package main

import (
	"flag"
	"log"

	"github.com/fujiwara/knockrd"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "", "config file name")
	flag.Parse()
	cfg, err := knockrd.LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(knockrd.Run(cfg))
}
