package postgres

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
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
	database string
	maxConns int32
	minConns int32
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
	config.database = poolConfig.ConnConfig.Database
	config.maxConns = poolConfig.MaxConns
	config.minConns = poolConfig.MinConns

	if viper.IsSet("KB_SERVICE_USER") {
		config.username = viper.GetString("KB_SERVICE_USER")
	}
	if viper.IsSet("KB_SERVICE_PASSWORD") {
		config.password = viper.GetString("KB_SERVICE_PASSWORD")
	}

	config.url = config.GetConnectURLWithHost(config.host)
	pool, err := pgxpool.ParseConfig(config.url)
	if err != nil {
		return nil, errors.Errorf("error opening DB connection: %v", err)
	}
	config.pool = pool

	return config, nil
}

func (config *Config) GetDBPort() int {
	if config.port == 0 {
		return DefaultPort
	}

	return config.port
}

func (config *Config) GetConnectURLWithHost(host string) string {
	return fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s pool_min_conns=%d pool_max_conns=%d",
		config.username, config.password, host, config.port, config.database, config.minConns, config.maxConns)
}

func (config *Config) GetConnectToDB(db string) string {
	return fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s pool_min_conns=%d pool_max_conns=%d",
		config.username, config.password, config.host, config.port, db, config.minConns, config.maxConns)
}
