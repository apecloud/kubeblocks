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

package wesql

import (
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/mysql"
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
