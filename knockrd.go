package knockrd

import (
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/fujiwara/ridge"
)

// Run runs knockrd
func Run(conf *Config, stream bool) error {
	hh, sh, err := conf.Setup()
	if err != nil {
		return err
	}
	if stream {
		log.Printf("[info] starting knockrd stream function")
		lambda.Start(sh)
		return nil
	}
	addr := fmt.Sprintf(":%d", conf.Port)
	log.Printf("[info] knockrd starting up on %s", addr)
	ridge.ProxyProtocol = conf.ProxyProtocol
	ridge.Run(addr, "/", hh)
	return nil
}
