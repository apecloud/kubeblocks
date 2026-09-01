/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package restore

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestRestoreJobBuilderOverridePostReadyTargetEnv(t *testing.T) {
	builder := &restoreJobBuilder{env: []corev1.EnvVar{
		{Name: "KEEP", Value: "kept"},
		{Name: dptypes.DPTargetClusterTopology, Value: "restore-value"},
		{Name: dptypes.DPTargetClusterTopology, Value: "pod-value"},
		{Name: dptypes.DPTargetComponentServiceVersion, Value: "spoofed-version"},
	}}
	builder.overridePostReadyTargetEnv([]corev1.EnvVar{
		{Name: dptypes.DPTargetClusterTopology, Value: "shared-nothing"},
		{Name: dptypes.DPTargetComponentServiceVersion, Value: "3.4.0"},
	})

	values := func(name string) []string {
		var result []string
		for i := range builder.env {
			if builder.env[i].Name == name {
				result = append(result, builder.env[i].Value)
			}
		}
		return result
	}
	require.Equal(t, []string{"kept"}, values("KEEP"))
	require.Equal(t, []string{"shared-nothing"}, values(dptypes.DPTargetClusterTopology))
	require.Equal(t, []string{"3.4.0"}, values(dptypes.DPTargetComponentServiceVersion))
}
