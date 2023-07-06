package mongodb

import (
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

const (
	host             = "host"
	username         = "username"
	password         = "password"
	server           = "server"
	databaseName     = "databaseName"
	operationTimeout = "operationTimeout"
	params           = "params"
	adminDatabase    = "admin"

	defaultTimeout = 5 * time.Second
	defaultDBPort  = 27017
)

type Config struct {
	host             string
	username         string
	password         string
	replSetName      string
	databaseName     string
	params           string
	direct           bool
	operationTimeout time.Duration
}

var config *Config

func NewConfig(properties map[string]string) (*Config, error) {
	config = &Config{
		direct:           true,
		operationTimeout: defaultTimeout,
	}

	if val, ok := properties[host]; ok && val != "" {
		config.host = val
	}

	if viper.IsSet("KB_SERVICE_PORT") {
		config.host = "localhost:" + viper.GetString("KB_SERVICE_PORT")
	}

	if len(config.host) == 0 {
		return nil, errors.New("must set 'host' in metadata or KB_SERVICE_PORT enviroment variable")
	}

	if val, ok := properties[username]; ok && val != "" {
		config.username = val
	}

	if val, ok := properties[password]; ok && val != "" {
		config.password = val
	}

	if viper.IsSet("KB_SERVICE_USER") {
		config.username = viper.GetString("KB_SERVICE_USER")
	}

	if viper.IsSet("KB_SERVICE_PASSWORD") {
		config.password = viper.GetString("KB_SERVICE_PASSWORD")
	}

	config.databaseName = adminDatabase
	if val, ok := properties[databaseName]; ok && val != "" {
		config.databaseName = val
	}

	if val, ok := properties[params]; ok && val != "" {
		config.params = val
	}

	var err error
	if val, ok := properties[operationTimeout]; ok && val != "" {
		config.operationTimeout, err = time.ParseDuration(val)
		if err != nil {
			return nil, errors.New("incorrect operationTimeout field from metadata")
		}
	}

	return config, nil
}

func (config *Config) GetDBPort() int {
	_, portStr, err := net.SplitHostPort(config.host)
	if err != nil {
		return defaultDBPort
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return defaultDBPort
	}

	return port
}

func GetConfig() *Config {
	return config
}
