package handlers

import (
	"context"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewGRPCHandler(t *testing.T) {
	handler, err := NewGRPCHandler(nil)
	assert.NotNil(t, handler)
	assert.Nil(t, err)
}

func TestGRPCHandlerDo(t *testing.T) {
	ctx := context.Background()
	handler := &GRPCHandler{}
	t.Run("grpc handler is nil", func(t *testing.T) {
		setting := util.HandlerSpec{
			GPRC: nil,
		}
		do, err := handler.Do(ctx, setting, nil)
		assert.Nil(t, do)
		assert.NotNil(t, err)
		assert.Error(t, err, errors.New("grpc setting is nil"))
	})

	t.Run("grpc handler is not nil but not implemented", func(t *testing.T) {
		setting := util.HandlerSpec{
			GPRC: map[string]string{"test": "test"},
		}
		do, err := handler.Do(ctx, setting, nil)
		assert.Nil(t, do)
		assert.NotNil(t, err)
		assert.Error(t, err, ErrNotImplemented)
	})
}
