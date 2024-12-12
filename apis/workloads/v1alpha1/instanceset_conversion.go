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

package v1alpha1

import (
	"github.com/jinzhu/copier"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// ConvertTo converts this InstanceSet to the Hub version (v1).
func (r *InstanceSet) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*workloadsv1.InstanceSet)

	// objectMeta
	dst.ObjectMeta = r.ObjectMeta

	// spec
	if err := copier.Copy(&dst.Spec, &r.Spec); err != nil {
		return err
	}
	r.changesToInstanceSet(dst)

	// status
	if err := copier.Copy(&dst.Status, &r.Status); err != nil {
		return err
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1) to this version.
func (r *InstanceSet) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*workloadsv1.InstanceSet)

	// objectMeta
	r.ObjectMeta = src.ObjectMeta

	// spec
	if err := copier.Copy(&r.Spec, &src.Spec); err != nil {
		return err
	}

	// status
	if err := copier.Copy(&r.Status, &src.Status); err != nil {
		return err
	}

	return nil
}

func (r *InstanceSet) changesToInstanceSet(its *workloadsv1.InstanceSet) {
	// changed:
	// spec
	//   podUpdatePolicy -> updateStrategy.instanceUpdatePolicy
	//   memberUpdateStrategy -> updateStrategy.rollingUpdate.updateConcurrency
	//   updateStrategy.rollingUpdate.partition -> updateStrategy.rollingUpdate.replicas
	if its.Spec.UpdateStrategy == nil {
		its.Spec.UpdateStrategy = &workloadsv1.UpdateStrategy{}
	}
	its.Spec.UpdateStrategy.InstanceUpdatePolicy = (*workloadsv1.InstanceUpdatePolicyType)(&r.Spec.PodUpdatePolicy)
	if r.Spec.MemberUpdateStrategy != nil {
		if its.Spec.UpdateStrategy.RollingUpdate == nil {
			its.Spec.UpdateStrategy.RollingUpdate = &workloadsv1.RollingUpdate{}
		}
		its.Spec.UpdateStrategy.RollingUpdate.UpdateConcurrency = (*workloadsv1.UpdateConcurrency)(r.Spec.MemberUpdateStrategy)
	}
	if r.Spec.UpdateStrategy.RollingUpdate != nil {
		if r.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			if its.Spec.UpdateStrategy.RollingUpdate == nil {
				its.Spec.UpdateStrategy.RollingUpdate = &workloadsv1.RollingUpdate{}
			}
			replicas := intstr.FromInt32(*r.Spec.UpdateStrategy.RollingUpdate.Partition)
			its.Spec.UpdateStrategy.RollingUpdate.Replicas = &replicas
		}
	}
}

func (r *InstanceSet) changesFromInstanceSet(its *workloadsv1.InstanceSet) {
	// changed:
	// spec
	//   podUpdatePolicy -> updateStrategy.instanceUpdatePolicy
	//   memberUpdateStrategy -> updateStrategy.rollingUpdate.updateConcurrency
	//   updateStrategy.rollingUpdate.partition -> updateStrategy.rollingUpdate.replicas
	if its.Spec.UpdateStrategy == nil {
		return
	}
	if its.Spec.UpdateStrategy.InstanceUpdatePolicy != nil {
		r.Spec.PodUpdatePolicy = PodUpdatePolicyType(*its.Spec.UpdateStrategy.InstanceUpdatePolicy)
	}
	if its.Spec.UpdateStrategy.RollingUpdate == nil {
		return
	}
	if its.Spec.UpdateStrategy.RollingUpdate.UpdateConcurrency != nil {
		r.Spec.MemberUpdateStrategy = (*MemberUpdateStrategy)(its.Spec.UpdateStrategy.RollingUpdate.UpdateConcurrency)
	}
	if its.Spec.UpdateStrategy.RollingUpdate.Replicas != nil {
		if r.Spec.UpdateStrategy.RollingUpdate == nil {
			r.Spec.UpdateStrategy.RollingUpdate = &appsv1.RollingUpdateStatefulSetStrategy{}
		}
		partition, _ := intstr.GetScaledValueFromIntOrPercent(its.Spec.UpdateStrategy.RollingUpdate.Replicas, int(*its.Spec.Replicas), false)
		r.Spec.UpdateStrategy.RollingUpdate.Partition = pointer.Int32(int32(partition))
	}
}
