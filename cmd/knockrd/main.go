package main

import (
	"flag"
	"log"
	"os"

	"github.com/fujiwara/knockrd"
)

func main() {
	var port int
	var name string
	flag.IntVar(&port, "port", 9876, "Listen port")
	flag.StringVar(&name, "table", "knockrd", "backend table name")
	flag.Parse()
	log.Fatal(knockrd.Run(&knockrd.Config{
		Port:      port,
		TableName: name,
		AWS: knockrd.AWSConfig{
			Region:   os.Getenv("AWS_REGION"),
			Endpoint: os.Getenv("AWS_ENDPOINT"),
		},
	}))
}
