package handlers

import (
	"context"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/stretchr/testify/assert"
	"testing"
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
