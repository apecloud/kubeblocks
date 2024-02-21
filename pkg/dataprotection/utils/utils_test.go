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

package utils

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/version"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestGetKubeVersion(t *testing.T) {
	tests := []struct {
		name        string
		versionInfo interface{}
		expected    string
		withError   bool
	}{
		{
			name:        "valid version info",
			versionInfo: version.Info{GitVersion: "v1.20"},
			expected:    "v1.20",
			withError:   false,
		},
		{
			name:        "invalid version info",
			versionInfo: "invalid",
			expected:    "",
			withError:   true,
		},
		{
			name:        "invalid major version",
			versionInfo: version.Info{GitVersion: "vmajor.20"},
			expected:    "",
			withError:   true,
		},
		{
			name:        "invalid minor version",
			versionInfo: version.Info{GitVersion: "v1.minor"},
			expected:    "",
			withError:   true,
		},
		{
			name:        "version with suffix",
			versionInfo: version.Info{GitVersion: "v1.20.0-rc1"},
			expected:    "v1.20",
			withError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(constant.CfgKeyServerInfo, tt.versionInfo)
			ver, err := GetKubeVersion()
			assert.Equal(t, tt.expected, ver)
			if tt.withError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
