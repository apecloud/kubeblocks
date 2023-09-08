package polarx

import (
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/mysql"
)

type Config struct {
	*mysql.Config
}

var config *Config

func NewConfig(properties map[string]string) (*Config, error) {
	mysqlConfig, err := mysql.NewConfig(properties)
	if err != nil {
		return nil, err
	}
	config = &Config{
		Config: mysqlConfig,
	}
	return config, nil
}
