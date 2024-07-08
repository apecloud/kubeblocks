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

package cronjobs

import (
	"encoding/json"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewManager(t *testing.T) {
	handlers.ResetHandlerSpecs()
	actionHandlerSpecs := map[string]util.HandlerSpec{
		constant.RoleProbeAction: {
			CronJob: &util.CronJob{
				PeriodSeconds:    1,
				SuccessThreshold: 2,
				FailureThreshold: 2,
				ReportFrequency:  2,
			},
		},
		"test": {},
	}
	actionJson, _ := json.Marshal(actionHandlerSpecs)
	viper.Set(constant.KBEnvActionHandlers, string(actionJson))
	assert.Nil(t, handlers.InitHandlers())
	t.Run("NewManager", func(t *testing.T) {
		manager, err := NewManager()
		assert.NotNil(t, manager)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(manager.Jobs))
	})
}
