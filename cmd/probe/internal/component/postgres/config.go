/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

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
	config.pool = poolConfig
	config.url = url
	config.database = poolConfig.ConnConfig.Database
	config.maxConns = poolConfig.MaxConns
	config.minConns = poolConfig.MinConns

	if viper.IsSet("KB_SERVICE_USER") {
		config.username = viper.GetString("KB_SERVICE_USER")
	}
	if viper.IsSet("KB_SERVICE_PASSWORD") {
		config.password = viper.GetString("KB_SERVICE_PASSWORD")
	}

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
