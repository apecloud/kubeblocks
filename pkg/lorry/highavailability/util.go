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

package highavailability

import (
	"strings"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

const (
	WorkloadTypeKey = "workloadType"
	Replication     = "Replication"
	Consensus       = "Consensus"
)

func IsHAAvailable(characterType, workloadType string) bool {
	switch models.EngineType(strings.ToLower(characterType)) {
	case models.MongoDB:
		return true
	case models.MySQL:
		if strings.EqualFold(workloadType, Replication) || strings.EqualFold(workloadType, Consensus) {
			return true
		}
	case models.WeSQL:
		return true
	case models.PostgreSQL:
		if strings.EqualFold(workloadType, Consensus) {
			return true
		}
	case models.ApecloudPostgreSQL:
		return true
	case models.OfficialPostgreSQL:
		return true
	}
	return false
}
