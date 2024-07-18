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

package util

import (
	"testing"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/plugin"
	"github.com/stretchr/testify/assert"
)

func TestWrapperArgs(t *testing.T) {
	t.Run("supported type", func(t *testing.T) {
		args := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}
		parameters, err := WrapperArgs(args)
		assert.Nil(t, err)
		assert.NotNil(t, parameters)
		assert.Equal(t, 2, len(parameters))
		assert.Equal(t, "key1", parameters[0].GetKey())
		assert.Equal(t, "value1", parameters[0].GetValue())
		assert.Equal(t, "key2", parameters[1].GetKey())
		assert.Equal(t, "value2", parameters[1].GetValue())
	})
	t.Run("unsupported type", func(t *testing.T) {
		args := map[string]interface{}{
			"key1": 1,
		}
		wrapperArgs, err := WrapperArgs(args)
		assert.NotNil(t, err)
		assert.Nil(t, wrapperArgs)
	})
}

func TestCreateParameter(t *testing.T) {
	t.Run("supported type", func(t *testing.T) {
		param, err := CreateParameter("key", "value")
		assert.Nil(t, err)
		assert.NotNil(t, param)
		assert.Equal(t, "key", param.GetKey())
		assert.Equal(t, "value", param.GetValue())
	})
	t.Run("unsupported type", func(t *testing.T) {
		param, err := CreateParameter("key", 1)
		assert.NotNil(t, err)
		assert.Nil(t, param)
	})
}

func TestParseArgs(t *testing.T) {
	t.Run("parse map", func(t *testing.T) {
		parameters := []*plugin.Parameter{
			{
				Key:   "key1",
				Value: "value1",
			},
			{
				Key:   "key2",
				Value: "value2",
			},
		}
		args, err := ParseArgs(parameters)
		assert.Nil(t, err)
		assert.NotNil(t, args)
		assert.Equal(t, 2, len(args))
		assert.Equal(t, "value1", args["key1"])
		assert.Equal(t, "value2", args["key2"])
	})
}
