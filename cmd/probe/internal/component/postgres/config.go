package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
)

const (
	connectionURLKey = "url"
	DefaultPort      = 5432
)

type Config struct {
	url      string
	username string
	password string
	host     string
	port     int
	pool     *pgxpool.Config
}

var config *Config

func NewConfig(properties map[string]string) (*Config, error) {
	config = &Config{}

	url, ok := properties[connectionURLKey]
	if !ok || url == "" {
		return nil, errors.Errorf("required metadata not set: %s", connectionURLKey)
	}

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.Errorf("error opening DB connection: %v", err)
	}

	config.username = poolConfig.ConnConfig.User
	config.password = poolConfig.ConnConfig.Password
	config.host = poolConfig.ConnConfig.Host
	config.port = int(poolConfig.ConnConfig.Port)
	config.pool = poolConfig
	config.url = url

	return config, nil
}

func (config *Config) GetDBPort() int {
	if config.port == 0 {
		return DefaultPort
	}

	return config.port
}
