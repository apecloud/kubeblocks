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

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
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
