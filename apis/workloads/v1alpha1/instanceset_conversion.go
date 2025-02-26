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
	if r.Spec.RoleProbe == nil && r.Spec.Credential == nil {
		return nil
	}
	instanceConvert := instanceSetConverter{
		RoleProbe:  r.Spec.RoleProbe,
		Credential: r.Spec.Credential,
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
	r.Spec.Credential = instanceConvert.Credential
	return nil
}

type instanceSetConverter struct {
	RoleProbe  *RoleProbe  `json:"roleProbe,omitempty"`
	Credential *Credential `json:"credential,omitempty"`
}

func (r *InstanceSet) changesToInstanceSet(its *workloadsv1.InstanceSet) {
	// changed:
	// spec
	//   updateStrategy.partition -> instanceUpdateStrategy.rollingUpdate.replicas
	//   updateStrategy.maxUnavailable -> instanceUpdateStrategy.rollingUpdate.maxUnavailable
	//   updateStrategy.memberUpdateStrategy -> memberUpdateStrategy
	if its.Spec.InstanceUpdateStrategy == nil {
		its.Spec.InstanceUpdateStrategy = &workloadsv1.InstanceUpdateStrategy{}
	}
	initRollingUpdate := func() {
		if its.Spec.InstanceUpdateStrategy.RollingUpdate == nil {
			its.Spec.InstanceUpdateStrategy.RollingUpdate = &workloadsv1.RollingUpdate{}
		}
	}
	setMemberUpdateStrategy := func(strategy *MemberUpdateStrategy) {
		if strategy == nil {
			return
		}
		its.Spec.MemberUpdateStrategy = (*workloadsv1.MemberUpdateStrategy)(strategy)
	}
	setMemberUpdateStrategy(r.Spec.MemberUpdateStrategy)
	if r.Spec.UpdateStrategy != nil {
		setMemberUpdateStrategy(r.Spec.UpdateStrategy.MemberUpdateStrategy)
		if r.Spec.UpdateStrategy.Partition != nil {
			initRollingUpdate()
			replicas := intstr.FromInt32(*r.Spec.UpdateStrategy.Partition)
			its.Spec.InstanceUpdateStrategy.RollingUpdate.Replicas = &replicas
		}
		if r.Spec.UpdateStrategy.MaxUnavailable != nil {
			initRollingUpdate()
			its.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable = r.Spec.UpdateStrategy.MaxUnavailable
		}
	}
}

func (r *InstanceSet) changesFromInstanceSet(its *workloadsv1.InstanceSet) {
	// changed:
	// spec
	//   updateStrategy.partition -> instanceUpdateStrategy.rollingUpdate.replicas
	//   updateStrategy.maxUnavailable -> instanceUpdateStrategy.rollingUpdate.maxUnavailable
	//   updateStrategy.memberUpdateStrategy -> memberUpdateStrategy
	r.Spec.MemberUpdateStrategy = (*MemberUpdateStrategy)(its.Spec.MemberUpdateStrategy)
	if its.Spec.InstanceUpdateStrategy == nil {
		return
	}
	if its.Spec.InstanceUpdateStrategy.RollingUpdate == nil {
		return
	}
	if r.Spec.UpdateStrategy == nil {
		r.Spec.UpdateStrategy = &InstanceUpdateStrategy{
			MemberUpdateStrategy: r.Spec.MemberUpdateStrategy,
		}
	}
	if its.Spec.InstanceUpdateStrategy.RollingUpdate.Replicas != nil {
		partition, _ := intstr.GetScaledValueFromIntOrPercent(its.Spec.InstanceUpdateStrategy.RollingUpdate.Replicas, int(*its.Spec.Replicas), false)
		r.Spec.UpdateStrategy.Partition = pointer.Int32(int32(partition))
	}
	if its.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable != nil {
		r.Spec.UpdateStrategy.MaxUnavailable = its.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable
	}
}
