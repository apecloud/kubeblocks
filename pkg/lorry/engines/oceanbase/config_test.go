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

package oceanbase

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

var (
	fakeProperties = engines.Properties{
		"url":          "root:@tcp(127.0.0.1:3306)/mysql?multiStatements=true",
		"maxOpenConns": "5",
	}
	fakePropertiesWithWrongPem = engines.Properties{
		"pemPath": "fake-path",
	}
)

func TestNewConfig(t *testing.T) {
	t.Run("new config failed", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakePropertiesWithWrongPem)

		assert.Nil(t, fakeConfig)
		assert.NotNil(t, err)
	})

	t.Run("new config successfully", func(t *testing.T) {
		fakeConfig, err := NewConfig(fakeProperties)

		assert.NotNil(t, fakeConfig)
		assert.Nil(t, err)
	})
}
