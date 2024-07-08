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
	"testing"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/stretchr/testify/assert"
)

func TestNewExecHandler(t *testing.T) {
	execHandler, err := NewExecHandler(nil)
	assert.NotNil(t, execHandler)
	assert.Nil(t, err)
}

func TestExecHandlerDo(t *testing.T) {
	ctx := context.Background()

	t.Run("action command is empty", func(t *testing.T) {
		execHandler := &ExecHandler{}
		setting := util.HandlerSpec{}
		args := map[string]interface{}{}
		resp, err := execHandler.Do(ctx, setting, args)
		assert.Nil(t, resp)
		assert.Nil(t, err)
	})

	t.Run("execute with timeout failed", func(t *testing.T) {
		execHandler := &ExecHandler{
			Executor: &util.ExecutorImpl{},
		}
		setting := util.HandlerSpec{
			Command:        []string{"sleep", "2"},
			TimeoutSeconds: 1,
		}
		args := map[string]interface{}{}
		resp, err := execHandler.Do(ctx, setting, args)
		assert.Nil(t, resp)
		assert.NotNil(t, err)
	})

	t.Run("execute with timeout success", func(t *testing.T) {
		execHandler := &ExecHandler{
			Executor: &util.ExecutorImpl{},
		}
		setting := util.HandlerSpec{
			Command:        []string{"sleep", "2"},
			TimeoutSeconds: 3,
		}
		args := map[string]interface{}{}
		resp, err := execHandler.Do(ctx, setting, args)
		assert.NotNil(t, resp)
		assert.Nil(t, err)
	})

	t.Run("execute action failed", func(t *testing.T) {
		execHandler := &ExecHandler{
			Executor: &util.ExecutorImpl{},
		}
		setting := util.HandlerSpec{
			Command: []string{"wcg"},
		}
		args := map[string]interface{}{}
		resp, err := execHandler.Do(ctx, setting, args)
		t.Logf("Response: %+v, Error: %v", resp, err)
		assert.Nil(t, resp)
		assert.NotNil(t, err)
	})

	t.Run("execute action success", func(t *testing.T) {
		execHandler := &ExecHandler{
			Executor: &util.ExecutorImpl{},
		}
		setting := util.HandlerSpec{
			Command: []string{"echo", "hello"},
		}
		args := map[string]interface{}{}
		resp, err := execHandler.Do(ctx, setting, args)
		assert.NotNil(t, resp)
		assert.Nil(t, err)
		assert.Equal(t, "hello\n", resp.Message)
	})
}
