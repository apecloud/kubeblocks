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

package mysql

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	// configurations to connect to MySQL, either a data source name represent by URL.
	connectionURLKey = "url"

	// To connect to MySQL running over SSL you have to download a
	// SSL certificate. If this is provided the driver will connect using
	// SSL. If you have disabled SSL you can leave this empty.
	// When the user provides a pem path their connection string must end with
	// &tls=custom
	// The connection string should be in the following format
	// "%s:%s@tcp(%s:3306)/%s?allowNativePasswords=true&tls=custom",'myadmin@mydemoserver', 'yourpassword', 'mydemoserver.mysql.database.azure.com', 'targetdb'.
	pemPathKey = "pemPath"

	// other general settings for DB connections.
	maxIdleConnsKey    = "maxIdleConns"
	maxOpenConnsKey    = "maxOpenConns"
	connMaxLifetimeKey = "connMaxLifetime"
	connMaxIdleTimeKey = "connMaxIdleTime"
)

const (
	databaseName  = "databaseName"
	adminDatabase = "mysql"
	defaultDBPort = 3306
)

type Config struct {
	url             string
	username        string
	password        string
	pemPath         string
	maxIdleConns    int
	maxOpenConns    int
	connMaxLifetime time.Duration
	connMaxIdletime time.Duration
}

var config *Config

func NewConfig(properties map[string]string) (*Config, error) {
	config = &Config{}

	if val, ok := properties[connectionURLKey]; ok && val != "" {
		config.url = val
	} else {
		return nil, fmt.Errorf("missing MySQL connection string")
	}

	if viper.IsSet("KB_SERVICE_USER") {
		config.username = viper.GetString("KB_SERVICE_USER")
	}

	if viper.IsSet("KB_SERVICE_PASSWORD") {
		config.password = viper.GetString("KB_SERVICE_PASSWORD")
	}

	if val, ok := properties[pemPathKey]; ok {
		config.pemPath = val
	}

	if val, ok := properties[maxIdleConnsKey]; ok {
		if i, err := strconv.Atoi(val); err == nil {
			config.maxIdleConns = i
		}
	}

	if val, ok := properties[maxOpenConnsKey]; ok {
		if i, err := strconv.Atoi(val); err == nil {
			config.maxOpenConns = i
		}
	}

	if val, ok := properties[connMaxLifetimeKey]; ok {
		if d, err := time.ParseDuration(val); err == nil {
			config.connMaxLifetime = d
		}
	}

	if val, ok := properties[connMaxIdleTimeKey]; ok {
		if d, err := time.ParseDuration(val); err == nil {
			config.connMaxIdletime = d
		}
	}

	if config.pemPath != "" {
		rootCertPool := x509.NewCertPool()
		pem, err := os.ReadFile(config.pemPath)
		if err != nil {
			return nil, errors.Wrapf(err, "Error reading PEM file from %s", config.pemPath)
		}

		ok := rootCertPool.AppendCertsFromPEM(pem)
		if !ok {
			return nil, fmt.Errorf("failed to append PEM")
		}

		err = mysql.RegisterTLSConfig("custom", &tls.Config{RootCAs: rootCertPool, MinVersion: tls.VersionTLS12})
		if err != nil {
			return nil, errors.Wrap(err, "Error register TLS config")
		}
	}
	return config, nil
}

func (config *Config) GetLocalDBConn() (*sql.DB, error) {
	mysqlConfig, err := mysql.ParseDSN(config.url)
	if err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", connectionURLKey)
	}
	mysqlConfig.User = config.username
	mysqlConfig.Passwd = config.password
	db, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		return nil, errors.Wrap(err, "error opening DB connection")
	}

	return db, nil
}

func (config *Config) GetDBConnWithAddr(addr string) (*sql.DB, error) {
	mysqlConfig, err := mysql.ParseDSN(config.url)
	if err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", connectionURLKey)
	}
	mysqlConfig.User = config.username
	mysqlConfig.Passwd = config.password
	mysqlConfig.Addr = addr
	db, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		return nil, errors.Wrap(err, "error opening DB connection")
	}

	return db, nil
}

func (config *Config) GetDBPort() int {
	mysqlConfig, err := mysql.ParseDSN(config.url)
	if err != nil {
		return defaultDBPort
	}

	_, portStr, err := net.SplitHostPort(mysqlConfig.Addr)
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
