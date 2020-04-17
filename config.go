package knockrd

type Config struct {
	Port       int
	TableName  string
	RealIPFrom []string
	AWS        AWSConfig
}

type AWSConfig struct {
	Region   string
	Endpoint string
}
