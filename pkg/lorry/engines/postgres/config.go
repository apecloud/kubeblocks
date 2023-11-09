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

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	ConnectionURLKey = "url"
	DefaultPort      = 5432
)

type Config struct {
	URL            string
	Username       string
	Password       string
	Host           string
	Port           int
	Database       string
	MaxConnections int32
	MinConnections int32
	pgxConfig      *pgxpool.Config
}

var config *Config

func NewConfig(properties map[string]string) (*Config, error) {
	config = &Config{}

	url, ok := properties[ConnectionURLKey]
	if !ok || url == "" {
		return nil, errors.Errorf("required metadata not set: %s", ConnectionURLKey)
	}

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.Errorf("error opening DB connection: %v", err)
	}

	config.Username = poolConfig.ConnConfig.User
	config.Password = poolConfig.ConnConfig.Password
	config.Host = poolConfig.ConnConfig.Host
	config.Port = int(poolConfig.ConnConfig.Port)
	config.Database = poolConfig.ConnConfig.Database
	config.MaxConnections = poolConfig.MaxConns
	config.MinConnections = poolConfig.MinConns

	if viper.IsSet("KB_SERVICE_USER") {
		config.Username = viper.GetString("KB_SERVICE_USER")
	}
	if viper.IsSet("KB_SERVICE_PASSWORD") {
		config.Password = viper.GetString("KB_SERVICE_PASSWORD")
	}

	config.URL = config.GetConnectURLWithHost(config.Host)
	pgxConfig, _ := pgxpool.ParseConfig(config.URL)
	config.pgxConfig = pgxConfig

	return config, nil
}

func (config *Config) GetDBPort() int {
	if config.Port == 0 {
		return DefaultPort
	}

	return config.Port
}

func (config *Config) GetConnectURLWithHost(host string) string {
	return fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s",
		config.Username, config.Password, host, config.Port, config.Database)
}

func (config *Config) GetConsensusIPPort(cluster *dcs.Cluster, name string) string {
	return fmt.Sprintf("%s.%s-headless.%s.svc:1%d", name, cluster.ClusterCompName, cluster.Namespace, config.GetDBPort())
}
