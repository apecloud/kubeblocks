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

package accounts

import (
	"errors"
	"strings"
	"testing"

	. "github.com/apecloud/kubeblocks/pkg/sqlchannel/util"
	"github.com/stretchr/testify/assert"
)

func TestParseSqlChannelResult(t *testing.T) {
	t.Run("Binding Not Supported", func(t *testing.T) {
		result := `
	{"errorCode":"ERR_INVOKE_OUTPUT_BINDING","message":"error when invoke output binding mongodb: binding mongodb does not support operation listUsers. supported operations:checkRunning checkRole getRole"}
	`
		sqlResponse, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.NotNil(t, err)
		assert.True(t, IsUnSupportedError(err))
		assert.Equal(t, sqlResponse.Event, RespEveFail)
		assert.Contains(t, sqlResponse.Message, "not supported")
	})

	t.Run("Binding Exec Failed", func(t *testing.T) {
		result := `
	{"event":"Failed","message":"db not ready"}
	`
		sqlResponse, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.Nil(t, err)
		assert.Equal(t, sqlResponse.Event, RespEveFail)
		assert.Contains(t, sqlResponse.Message, "db not ready")
	})

	t.Run("Binding Exec Success", func(t *testing.T) {
		result := `
	{"event":"Success","message":"[]"}
	`
		sqlResponse, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.Nil(t, err)
		assert.Equal(t, sqlResponse.Event, RespEveSucc)
	})

	t.Run("Invalid Response Format", func(t *testing.T) {
		// msg cannot be parsed to json
		result := `
	{"event":"Success","message":"[]
	`
		_, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.NotNil(t, err)
	})
}

func TestErrMsg(t *testing.T) {
	err := SQLChannelError{
		Reason: UnsupportedOps,
	}
	assert.True(t, strings.Contains(err.Error(), "unsupported"))
	assert.False(t, IsUnSupportedError(nil))
	assert.True(t, IsUnSupportedError(err))
	assert.False(t, IsUnSupportedError(errors.New("test")))
}
