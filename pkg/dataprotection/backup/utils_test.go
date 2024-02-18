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

package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestBuildCronJobSchedule(t *testing.T) {
	const (
		cronExpression       = "0 0 * * *"
		cronExpressionWithTZ = "CRON_TZ=UTC 0 0 * * *"
		timeZone             = "UTC"
	)

	tests := []struct {
		name           string
		versionInfo    interface{}
		cronExpression string
		timeZone       *string
	}{
		{
			name:           "version v1.20",
			versionInfo:    version.Info{GitVersion: "v1.20.1-eks-12345"},
			cronExpression: cronExpression,
			timeZone:       nil,
		},
		{
			name:           "version v1.21",
			versionInfo:    version.Info{GitVersion: "v1.21.1-eks-12345"},
			cronExpression: cronExpression,
			timeZone:       nil,
		},
		{
			name:           "version v1.22",
			versionInfo:    version.Info{GitVersion: "v1.22.1-eks-12345"},
			cronExpression: cronExpressionWithTZ,
			timeZone:       nil,
		},
		{
			name:           "version v1.25",
			versionInfo:    version.Info{GitVersion: "v1.25.0-eks-12345"},
			cronExpression: cronExpression,
			timeZone:       pointer.String(timeZone),
		},
		{
			name:           "invalid version",
			versionInfo:    version.Info{GitVersion: "invalid-version"},
			cronExpression: cronExpression,
			timeZone:       nil,
		},
	}

	oldVer := viper.Get(constant.CfgKeyServerInfo)
	defer viper.Set(constant.CfgKeyServerInfo, oldVer)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(constant.CfgKeyServerInfo, tt.versionInfo)
			tz, cronExp := BuildCronJobSchedule(cronExpression)
			assert.Equal(t, tt.cronExpression, cronExp)
			assert.Equal(t, tt.timeZone, tz)
		})
	}
}
