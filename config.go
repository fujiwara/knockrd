package knockrd

import (
	"encoding/json"
	"os"

	"github.com/kayac/go-config"
	"github.com/natureglobal/realip"
)

const (
	DefaultPort    = 9876
	DefaultTable   = "knockrd"
	DefaultExpires = 86400
)

var DefaultRealIPFrom = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.0/8",
	"fe80::/10",
	"::1/128",
}

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
		RealIPFrom:   DefaultRealIPFrom,
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

func (c *Config) String() string {
	b, _ := json.Marshal(c)
	return string(b)
}
