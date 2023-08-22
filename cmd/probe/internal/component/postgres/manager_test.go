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
	"testing"

	"github.com/dapr/kit/logger"
	"github.com/pashagolub/pgxmock/v2"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/internal/constant"
)

func mockDatabase(t *testing.T, workloadType string) (*Manager, pgxmock.PgxPoolIface, error) {
	config = newMockConfig(t)

	viper.Set(constant.KBEnvPodName, "test-pod-0")
	viper.Set(constant.KBEnvClusterCompName, "test")
	viper.Set(constant.KBEnvNamespace, "default")
	viper.Set(PGDATA, "test")
	viper.Set(constant.KBEnvWorkloadType, workloadType)
	mock, err := pgxmock.NewPool(pgxmock.MonitorPingsOption(true))

	manager, err := NewManager(logger.NewLogger("test"))
	if err != nil {
		t.Fatal(err)
	}
	manager.Pool = mock

	return manager, mock, err
}
