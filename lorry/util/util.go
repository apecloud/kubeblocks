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

package util

import (
	"strings"

	"github.com/apecloud/kubeblocks/internal/constant"
)

func IsHAAvailable(characterType, workloadType string) bool {
	switch strings.ToLower(characterType) {
	case "mongodb":
		return true
	case "mysql":
		if strings.EqualFold(workloadType, constant.Replication) || strings.EqualFold(workloadType, constant.Consensus) {
			return true
		}
	case "postgresql":
		if strings.EqualFold(workloadType, constant.Consensus) {
			return true
		}
	}
	return false
}
