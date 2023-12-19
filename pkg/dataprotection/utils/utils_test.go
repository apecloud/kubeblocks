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

package utils

import (
	"errors"
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
		wantMajor   int
		wantMinor   int
		wantErr     error
	}{
		{
			name:        "valid version info",
			versionInfo: version.Info{Major: "1", Minor: "20+"},
			wantMajor:   1,
			wantMinor:   20,
			wantErr:     nil,
		},
		{
			name:        "invalid version info",
			versionInfo: "invalid",
			wantMajor:   0,
			wantMinor:   0,
			wantErr:     errors.New("failed to get kubernetes version, major , minor "),
		},
		{
			name:        "invalid major version",
			versionInfo: version.Info{Major: "not-a-number", Minor: "20+"},
			wantMajor:   0,
			wantMinor:   0,
			wantErr:     errors.New("strconv.Atoi: parsing \"not-a-number\": invalid syntax"),
		},
		{
			name:        "invalid minor version",
			versionInfo: version.Info{Major: "1", Minor: "not-a-number+"},
			wantMajor:   0,
			wantMinor:   0,
			wantErr:     errors.New("failed to get kubernetes version, major 1, minor not-a-number+"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(constant.CfgKeyServerInfo, tt.versionInfo)

			gotMajor, gotMinor, gotErr := GetKubeVersion()

			assert.Equal(t, tt.wantMajor, gotMajor)
			assert.Equal(t, tt.wantMinor, gotMinor)
			if tt.wantErr != nil {
				assert.EqualError(t, gotErr, tt.wantErr.Error())
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}
