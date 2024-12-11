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
	"k8s.io/apimachinery/pkg/util/json"
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

	if err := r.incrementConvertTo(dst); err != nil {
		return err
	}

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

	if err := r.incrementConvertFrom(src); err != nil {
		return err
	}
	return nil
}

func (r *InstanceSet) incrementConvertTo(dstRaw metav1.Object) error {
	if r.Spec.RoleProbe == nil && r.Spec.UpdateStrategy == nil {
		return nil
	}
	// changed
	instanceConvert := instanceSetConverter{
		RoleProbe:      r.Spec.RoleProbe,
		UpdateStrategy: r.Spec.UpdateStrategy,
	}

	if r.Spec.UpdateStrategy == nil || r.Spec.UpdateStrategy.MemberUpdateStrategy == nil {
		// 1. set default update strategy
		updateStrategy := SerialUpdateStrategy
		instanceConvert.UpdateStrategy = &InstanceUpdateStrategy{
			MemberUpdateStrategy: &updateStrategy,
		}
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
	r.Spec.UpdateStrategy = instanceConvert.UpdateStrategy
	return nil
}

type instanceSetConverter struct {
	RoleProbe      *RoleProbe              `json:"roleProbe,omitempty"`
	UpdateStrategy *InstanceUpdateStrategy `json:"updateStrategy,omitempty"`
}
