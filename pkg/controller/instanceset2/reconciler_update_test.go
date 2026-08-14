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

package instanceset2

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestParseReplicasNMaxUnavailable(t *testing.T) {
	tests := []struct {
		name                   string
		totalReplicas          int
		replicas               intstr.IntOrString
		maxUnavailable         intstr.IntOrString
		expectedReplicas       int
		expectedMaxUnavailable int
	}{
		{"round replicas up and maxUnavailable down", 3, intstr.FromString("34%"), intstr.FromString("34%"), 2, 1},
		{"keep maxUnavailable at least one", 3, intstr.FromString("10%"), intstr.FromString("10%"), 1, 1},
		{"keep exact percentage results", 4, intstr.FromString("50%"), intstr.FromString("50%"), 2, 2},
		{"keep absolute values", 3, intstr.FromInt32(2), intstr.FromInt32(1), 2, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       &tt.replicas,
					MaxUnavailable: &tt.maxUnavailable,
				},
			}

			replicas, maxUnavailable, err := parseReplicasNMaxUnavailable(strategy, tt.totalReplicas)
			if err != nil {
				t.Fatalf("parse rolling update quotas: %v", err)
			}
			if replicas != tt.expectedReplicas || maxUnavailable != tt.expectedMaxUnavailable {
				t.Fatalf("quotas = (%d, %d), want (%d, %d)",
					replicas, maxUnavailable, tt.expectedReplicas, tt.expectedMaxUnavailable)
			}
		})
	}
}
