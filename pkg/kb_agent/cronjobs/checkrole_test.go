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
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

func TestCheckRoleJob(t *testing.T) {
	actionHandlerSpecs := map[string]util.HandlerSpec{
		constant.RoleProbeAction: {},
	}
	actionJSON, _ := json.Marshal(actionHandlerSpecs)
	viper.Set(constant.KBEnvActionHandlers, string(actionJSON))
	assert.Nil(t, handlers.InitHandlers())

	commonJob := CommonJob{
		Name:           constant.RoleProbeAction,
		TimeoutSeconds: 10,
	}
	job := NewCheckRoleJob(commonJob)

	t.Run("do - role unchanged with send role event periodically", func(t *testing.T) {
		job.originRole = "role1"
		job.roleUnchangedEventCount = 0

		handler := &MockHandler{}
		handler.DoFunc = func(ctx context.Context, setting util.HandlerSpec, args map[string]interface{}) (*handlers.Response, error) {
			return &handlers.Response{
				Message: "role1",
			}, nil
		}
		handlers.SetDefaultHandler(handler)

		err := job.do()

		assert.NoError(t, err)
		assert.Equal(t, 0, job.roleUnchangedEventCount)
	})

	t.Run("do - role unchanged with send role event periodically", func(t *testing.T) {
		job.originRole = "role1"
		job.roleUnchangedEventCount = 0
		sendRoleEventPeriodically = true

		handler := &MockHandler{}
		handler.DoFunc = func(ctx context.Context, setting util.HandlerSpec, args map[string]interface{}) (*handlers.Response, error) {
			return &handlers.Response{
				Message: "role1",
			}, nil
		}
		handlers.SetDefaultHandler(handler)

		err := job.do()

		assert.NoError(t, err)
		assert.Equal(t, 1, job.roleUnchangedEventCount)
	})

	t.Run("do - role changed", func(t *testing.T) {
		job.originRole = "role1"
		job.roleUnchangedEventCount = 0

		handler := &MockHandler{
			DoFunc: func(ctx context.Context, setting util.HandlerSpec, args map[string]interface{}) (*handlers.Response, error) {
				return &handlers.Response{
					Message: "role2",
				}, nil
			},
		}
		handlers.SetDefaultHandler(handler)

		err := job.do()

		assert.NoError(t, err)
		assert.Equal(t, 0, job.roleUnchangedEventCount)
	})

	t.Run("do - error", func(t *testing.T) {
		job.originRole = "role1"
		job.roleUnchangedEventCount = 0

		handler := &MockHandler{
			DoFunc: func(ctx context.Context, setting util.HandlerSpec, args map[string]interface{}) (*handlers.Response, error) {
				return nil, errors.New("some error")
			},
		}
		handlers.SetDefaultHandler(handler)

		err := job.do()

		assert.Error(t, err)
		assert.Equal(t, 0, job.roleUnchangedEventCount)
	})
}

type MockHandler struct {
	DoFunc func(ctx context.Context, setting util.HandlerSpec, args map[string]interface{}) (*handlers.Response, error)
}

func (h *MockHandler) Do(ctx context.Context, setting util.HandlerSpec, args map[string]interface{}) (*handlers.Response, error) {
	if h.DoFunc != nil {
		return h.DoFunc(ctx, setting, args)
	}
	return nil, handlers.ErrNotImplemented
}
