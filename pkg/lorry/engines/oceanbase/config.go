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

package oceanbase

import (
	"database/sql"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	mysqlengine "github.com/apecloud/kubeblocks/pkg/lorry/engines/mysql"
)

type Config struct {
	*mysqlengine.Config
}

var config *Config

func NewConfig(properties map[string]string) (*Config, error) {
	mysqlConfig, err := mysqlengine.NewConfig(properties)
	if err != nil {
		return nil, err
	}
	if mysqlConfig.Username == "" {
		mysqlConfig.Username = "root"
	}

	config = &Config{
		Config: mysqlConfig,
	}
	return config, nil
}

func getRootPassword(compName string) string {
	rootPasswordEnv := "OB_ROOT_PASSWD"

	if compName == "" {
		compName = viper.GetString("KB_COMP_NAME")
	}

	if compName != "" {
		compName = strings.ToUpper(compName)
		compName = strings.ReplaceAll(compName, "-", "_")
		rootPasswordEnv = rootPasswordEnv + "_" + compName
	}
	return viper.GetString(rootPasswordEnv)
}

func (config *Config) GetMemberRootDBConn(cluster *dcs.Cluster, member *dcs.Member) (*sql.DB, error) {
	addr := cluster.GetMemberAddrWithPort(*member)
	mysqlConfig, err := mysql.ParseDSN(config.URL)
	if err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", config.URL)
	}
	mysqlConfig.User = config.Username
	mysqlConfig.Passwd = getRootPassword(member.ComponentName)
	mysqlConfig.Addr = addr
	mysqlConfig.Timeout = time.Second * 5
	mysqlConfig.ReadTimeout = time.Second * 5
	mysqlConfig.WriteTimeout = time.Second * 5
	db, err := mysqlengine.GetDBConnection(mysqlConfig.FormatDSN())
	if err != nil {
		return nil, errors.Wrap(err, "get DB connection failed")
	}

	return db, nil
}
