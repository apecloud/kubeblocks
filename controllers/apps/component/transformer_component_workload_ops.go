/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package component

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

type componentWorkloadOps struct {
	synthesizeComp *component.SynthesizedComponent

	desiredCompPodNameSet sets.Set[string]
	runningItsPodNameSet  sets.Set[string]
}

func newComponentWorkloadOps(synthesizedComp *component.SynthesizedComponent,
	runningITS *workloads.InstanceSet,
	protoITS *workloads.InstanceSet) (*componentWorkloadOps, error) {
	runningITSPodNames, err := component.GetCurrentPodNamesByITS(runningITS)
	if err != nil {
		return nil, err
	}
	protoITSPodNames, err := component.GetDesiredPodNamesByITS(runningITS, protoITS)
	if err != nil {
		return nil, err
	}
	return &componentWorkloadOps{
		synthesizeComp:        synthesizedComp,
		desiredCompPodNameSet: sets.New(protoITSPodNames...),
		runningItsPodNameSet:  sets.New(runningITSPodNames...),
	}, nil
}

func (r *componentWorkloadOps) validateHorizontalScale() error {
	in := r.runningItsPodNameSet.Difference(r.desiredCompPodNameSet)
	if in.Len() == 0 {
		return nil
	}
	if r.synthesizeComp.Replicas == 0 && len(r.synthesizeComp.VolumeClaimTemplates) > 0 &&
		r.synthesizeComp.PVCRetentionPolicy.WhenScaled != appsv1.RetainPersistentVolumeClaimRetentionPolicyType {
		return fmt.Errorf("when intending to scale-in to 0, only the \"Retain\" option is supported for the PVC retention policy")
	}
	return nil
}
