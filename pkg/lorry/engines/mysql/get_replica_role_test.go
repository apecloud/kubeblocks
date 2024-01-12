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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

func TestManager_GetRole(t *testing.T) {
	ctx := context.TODO()
	manager, _, _ := mockDatabase(t)

	t.Run("cluster not found", func(t *testing.T) {
		role, err := manager.GetReplicaRole(ctx, nil)
		assert.Empty(t, role)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "cluster not found")
	})

	t.Run("cluster has no leader lease", func(t *testing.T) {
		cluster := &dcs.Cluster{}

		role, err := manager.GetReplicaRole(ctx, cluster)
		assert.Empty(t, role)
		assert.NotNil(t, err)
	})

	t.Run("get role successfully", func(t *testing.T) {
		leaderNames := []string{fakePodName, "fake-mysql-1"}
		expectedRoles := []string{models.PRIMARY, models.SECONDARY}

		for i, leaderName := range leaderNames {
			cluster := &dcs.Cluster{Leader: &dcs.Leader{Name: leaderName}}
			role, err := manager.GetReplicaRole(ctx, cluster)
			assert.Nil(t, err)
			assert.Equal(t, expectedRoles[i], role)
		}
	})
}
