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

package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

func TestDo(t *testing.T) {
	ctx := context.Background()

	t.Run("action empty", func(t *testing.T) {
		resp, err := Do(ctx, "", nil)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "action is empty", err.Error())
	})

	t.Run("action handler spec not found", func(t *testing.T) {
		resp, err := Do(ctx, "unknown-action", nil)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "action handler spec not found", err.Error())
	})

	t.Run("no handler found", func(t *testing.T) {
		actionHandlerSpecs := map[string]util.HandlerSpec{
			"action1": {},
		}
		actionJSON, _ := json.Marshal(actionHandlerSpecs)
		viper.Set(constant.KBEnvActionHandlers, string(actionJSON))
		assert.Nil(t, InitHandlers())

		resp, err := Do(ctx, "action1", nil)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "no handler found", err.Error())
	})

	t.Run("action exec failed", func(t *testing.T) {
		actionHandlerSpecs := map[string]util.HandlerSpec{
			"action1": {},
		}
		actionJSON, _ := json.Marshal(actionHandlerSpecs)
		viper.Set(constant.KBEnvActionHandlers, string(actionJSON))
		assert.Nil(t, InitHandlers())

		handler := &MockHandler{}
		handler.DoFunc = func(ctx context.Context, handlerSpec util.HandlerSpec, args map[string]interface{}) (*Response, error) {
			return nil, errors.New("execution failed")
		}
		SetDefaultHandler(handler)

		resp, err := Do(ctx, "action1", nil)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "execution failed", err.Error())
	})

	t.Run("action exec success", func(t *testing.T) {
		actionHandlerSpecs := map[string]util.HandlerSpec{
			"action1": {},
		}
		actionJSON, _ := json.Marshal(actionHandlerSpecs)
		viper.Set(constant.KBEnvActionHandlers, string(actionJSON))
		assert.Nil(t, InitHandlers())

		handler := &MockHandler{}
		handler.DoFunc = func(ctx context.Context, handlerSpec util.HandlerSpec, args map[string]interface{}) (*Response, error) {
			return &Response{
				Message: "success",
			}, nil
		}
		SetDefaultHandler(handler)

		resp, err := Do(ctx, "action1", nil)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "success", resp.Message)
	})
}

type MockHandler struct {
	DoFunc func(ctx context.Context, setting util.HandlerSpec, args map[string]interface{}) (*Response, error)
}

func (h *MockHandler) Do(ctx context.Context, setting util.HandlerSpec, args map[string]interface{}) (*Response, error) {
	if h.DoFunc != nil {
		return h.DoFunc(ctx, setting, args)
	}
	return nil, ErrNotImplemented
}
