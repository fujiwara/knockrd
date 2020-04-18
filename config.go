package knockrd

import (
	"os"

	"github.com/kayac/go-config"
	"github.com/natureglobal/realip"
)

const (
	DefaultPort    = 9876
	DefaultTable   = "knockrd"
	DefaultExpires = 86400
)

type Config struct {
	Port         int
	TableName    string
	RealIPFrom   []string
	RealIPHeader string
	Expires      int64
	AWS          AWSConfig
}

type AWSConfig struct {
	Region   string
	Endpoint string
}

func LoadConfig(path string) (*Config, error) {
	c := Config{
		Port:         DefaultPort,
		TableName:    DefaultTable,
		RealIPHeader: realip.HeaderXForwardedFor,
		Expires:      DefaultExpires,
		AWS: AWSConfig{
			Region:   os.Getenv("AWS_REGION"),
			Endpoint: os.Getenv("AWS_ENDPOINT"),
		},
	}
	if path == "" {
		return &c, nil
	}
	err := config.LoadWithEnv(&c, path)
	return &c, err
}
