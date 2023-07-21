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
	hosts            []string
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
		username:         "root",
		operationTimeout: defaultTimeout,
	}

	if val, ok := properties[host]; ok && val != "" {
		config.hosts = []string{val}
	}

	if viper.IsSet("KB_SERVICE_PORT") {
		config.hosts = []string{"localhost:" + viper.GetString("KB_SERVICE_PORT")}
	}

	if len(config.hosts) == 0 {
		return nil, errors.New("must set 'host' in metadata or KB_SERVICE_PORT environment variable")
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

	if viper.IsSet("KB_CLUSTER_COMP_NAME") {
		config.replSetName = viper.GetString("KB_CLUSTER_COMP_NAME")
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
	_, portStr, err := net.SplitHostPort(config.hosts[0])
	if err != nil {
		return defaultDBPort
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return defaultDBPort
	}

	return port
}

func (config *Config) DeepCopy() *Config {
	newConf := *config
	newConf.hosts = make([]string, len(config.hosts))
	copy(newConf.hosts, config.hosts)
	return &newConf
}

func GetConfig() *Config {
	return config
}
