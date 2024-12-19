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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

const (
	kbIncrementConverterAK = "kb-increment-converter"
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

	if err := r.incrementConvertTo(dst); err != nil {
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
	r.changesFromInstanceSet(src)

	// status
	if err := copier.Copy(&r.Status, &src.Status); err != nil {
		return err
	}

	if err := r.incrementConvertFrom(src); err != nil {
		return err
	}
	return nil
}

func (r *InstanceSet) incrementConvertTo(dstRaw metav1.Object) error {
	if r.Spec.RoleProbe == nil {
		return nil
	}
	// changed
	instanceConvert := instanceSetConverter{
		RoleProbe: r.Spec.RoleProbe,
	}
	bytes, err := json.Marshal(instanceConvert)
	if err != nil {
		return err
	}
	annotations := dstRaw.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[kbIncrementConverterAK] = string(bytes)
	dstRaw.SetAnnotations(annotations)
	return nil
}

func (r *InstanceSet) incrementConvertFrom(srcRaw metav1.Object) error {
	data, ok := srcRaw.GetAnnotations()[kbIncrementConverterAK]
	if !ok {
		return nil
	}
	instanceConvert := instanceSetConverter{}
	if err := json.Unmarshal([]byte(data), &instanceConvert); err != nil {
		return err
	}
	delete(srcRaw.GetAnnotations(), kbIncrementConverterAK)
	r.Spec.RoleProbe = instanceConvert.RoleProbe
	return nil
}

type instanceSetConverter struct {
	RoleProbe *RoleProbe `json:"roleProbe,omitempty"`
}

func (r *InstanceSet) changesToInstanceSet(its *workloadsv1.InstanceSet) {
	// changed:
	// spec
	//   podUpdatePolicy -> updateStrategy.instanceUpdatePolicy
	//   memberUpdateStrategy -> updateStrategy.rollingUpdate.updateConcurrency
	//   updateStrategy.partition -> updateStrategy.rollingUpdate.replicas
	//   updateStrategy.maxUnavailable -> updateStrategy.rollingUpdate.maxUnavailable
	//   updateStrategy.memberUpdateStrategy -> updateStrategy.rollingUpdate.updateConcurrency
	if its.Spec.UpdateStrategy == nil {
		its.Spec.UpdateStrategy = &workloadsv1.UpdateStrategy{}
	}
	its.Spec.UpdateStrategy.InstanceUpdatePolicy = (*workloadsv1.InstanceUpdatePolicyType)(&r.Spec.PodUpdatePolicy)
	initRollingUpdate := func() {
		if its.Spec.UpdateStrategy.RollingUpdate == nil {
			its.Spec.UpdateStrategy.RollingUpdate = &workloadsv1.RollingUpdate{}
		}
	}
	setUpdateConcurrency := func(strategy *MemberUpdateStrategy) {
		if strategy == nil {
			return
		}
		initRollingUpdate()
		its.Spec.UpdateStrategy.RollingUpdate.UpdateConcurrency = (*workloadsv1.UpdateConcurrency)(strategy)
	}
	setUpdateConcurrency(r.Spec.MemberUpdateStrategy)
	if r.Spec.UpdateStrategy != nil {
		setUpdateConcurrency(r.Spec.UpdateStrategy.MemberUpdateStrategy)
		if r.Spec.UpdateStrategy.Partition != nil {
			initRollingUpdate()
			replicas := intstr.FromInt32(*r.Spec.UpdateStrategy.Partition)
			its.Spec.UpdateStrategy.RollingUpdate.Replicas = &replicas
		}
		if r.Spec.UpdateStrategy.MaxUnavailable != nil {
			initRollingUpdate()
			its.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = r.Spec.UpdateStrategy.MaxUnavailable
		}
	}
}

func (r *InstanceSet) changesFromInstanceSet(its *workloadsv1.InstanceSet) {
	// changed:
	// spec
	//   podUpdatePolicy -> updateStrategy.instanceUpdatePolicy
	//   memberUpdateStrategy -> updateStrategy.rollingUpdate.updateConcurrency
	//   updateStrategy.partition -> updateStrategy.rollingUpdate.replicas
	//   updateStrategy.maxUnavailable -> updateStrategy.rollingUpdate.maxUnavailable
	//   updateStrategy.memberUpdateStrategy -> updateStrategy.rollingUpdate.updateConcurrency
	if its.Spec.UpdateStrategy == nil {
		return
	}
	if its.Spec.UpdateStrategy.InstanceUpdatePolicy != nil {
		r.Spec.PodUpdatePolicy = PodUpdatePolicyType(*its.Spec.UpdateStrategy.InstanceUpdatePolicy)
	}
	if its.Spec.UpdateStrategy.RollingUpdate == nil {
		return
	}
	if r.Spec.UpdateStrategy == nil {
		r.Spec.UpdateStrategy = &InstanceUpdateStrategy{}
	}
	if its.Spec.UpdateStrategy.RollingUpdate.UpdateConcurrency != nil {
		r.Spec.MemberUpdateStrategy = (*MemberUpdateStrategy)(its.Spec.UpdateStrategy.RollingUpdate.UpdateConcurrency)
		r.Spec.UpdateStrategy.MemberUpdateStrategy = r.Spec.MemberUpdateStrategy
	}
	if its.Spec.UpdateStrategy.RollingUpdate.Replicas != nil {
		partition, _ := intstr.GetScaledValueFromIntOrPercent(its.Spec.UpdateStrategy.RollingUpdate.Replicas, int(*its.Spec.Replicas), false)
		r.Spec.UpdateStrategy.Partition = pointer.Int32(int32(partition))
	}
	if its.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable != nil {
		r.Spec.UpdateStrategy.MaxUnavailable = its.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable
	}
}
