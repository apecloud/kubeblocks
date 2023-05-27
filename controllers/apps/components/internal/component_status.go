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

package internal

import (
	"sort"

	"golang.org/x/exp/maps"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var componentPhasePriority = map[appsv1alpha1.ClusterComponentPhase]int{
	appsv1alpha1.FailedClusterCompPhase:          1,
	appsv1alpha1.AbnormalClusterCompPhase:        2,
	appsv1alpha1.SpecReconcilingClusterCompPhase: 3,
	appsv1alpha1.StoppedClusterCompPhase:         4,
	appsv1alpha1.RunningClusterCompPhase:         5,
	appsv1alpha1.CreatingClusterCompPhase:        6,
}

type statusReconciliationTxn struct {
	proposals map[appsv1alpha1.ClusterComponentPhase]func()
}

func (t *statusReconciliationTxn) propose(phase appsv1alpha1.ClusterComponentPhase, mutator func()) {
	if t.proposals == nil {
		t.proposals = make(map[appsv1alpha1.ClusterComponentPhase]func())
	}
	if _, ok := t.proposals[phase]; ok {
		return // keep first
	}
	t.proposals[phase] = mutator
}

func (t *statusReconciliationTxn) commit() error {
	if len(t.proposals) == 0 {
		return nil
	}
	phases := maps.Keys(t.proposals)
	sort.Slice(phases, func(i, j int) bool {
		return componentPhasePriority[phases[i]] < componentPhasePriority[phases[j]]
	})
	t.proposals[phases[0]]()
	return nil
}
