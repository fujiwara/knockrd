package knockrd

import (
	"encoding/json"
	"os"
	"time"

	"github.com/kayac/go-config"
	"github.com/natureglobal/realip"
)

const (
	DefaultPort        = 9876
	DefaultTable       = "knockrd"
	DefaultTTL         = time.Hour
	DefaultNegativeTTL = 10 * time.Second
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
	Port         int           `yaml:"port"`
	TableName    string        `yaml:"table_name"`
	RealIPFrom   []string      `yaml:"real_ip_from"`
	RealIPHeader string        `yaml:"real_ip_header"`
	TTL          time.Duration `yaml:"ttl"`
	NegativeTTL  time.Duration `yaml:"negative_ttl"`
	AWS          AWSConfig     `yaml:"aws"`
}

type AWSConfig struct {
	Region   string `yaml:"region"`
	Endpoint string `yaml:"endpoint"`
}

func LoadConfig(path string) (*Config, error) {
	c := Config{
		Port:         DefaultPort,
		TableName:    DefaultTable,
		RealIPFrom:   DefaultRealIPFrom,
		RealIPHeader: realip.HeaderXForwardedFor,
		TTL:          DefaultTTL,
		NegativeTTL:  DefaultNegativeTTL,
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
