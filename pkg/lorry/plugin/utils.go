/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package plugin

import (
	"fmt"
	"strings"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/spf13/viper"
)

func ParseEndpoint(ep string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(ep), "unix://") || strings.HasPrefix(strings.ToLower(ep), "tcp://") {
		s := strings.SplitN(ep, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
	}
	return "", "", fmt.Errorf("invalid endpoint: %v", ep)
}

func GetDBInfo() *DBInfo {
	return &DBInfo{
		Fqdn:          viper.GetString(constant.KBEnvPodFQDN),
		Port:          viper.GetString(constant.KBEnvServicePort),
		AdminUser:     viper.GetString(constant.KBEnvServiceUser),
		AdminPassword: viper.GetString(constant.KBEnvServicePassword),
	}
}
