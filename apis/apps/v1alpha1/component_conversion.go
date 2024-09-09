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
	"encoding/json"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jinzhu/copier"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ConvertTo converts this Component to the Hub version (v1).
func (r *Component) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv1.Component)

	// objectMeta
	dst.ObjectMeta = r.ObjectMeta

	// spec
	copier.Copy(&dst.Spec, &r.Spec)
	convertor := &incrementConvertor{
		deleted: &componentDeleted{},
	}
	if err := convertor.convertTo(r, dst); err != nil {
		return err
	}

	// status
	copier.Copy(&dst.Status, &r.Status)

	return nil
}

// ConvertFrom converts from the Hub version (v1) to this version.
func (r *Component) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv1.Component)

	// objectMeta
	r.ObjectMeta = src.ObjectMeta

	// spec
	copier.Copy(&r.Spec, &src.Spec)
	convertor := &incrementConvertor{
		deleted: &componentDeleted{},
	}
	if err := convertor.convertFrom(src, r); err != nil {
		return err
	}

	// status
	copier.Copy(&r.Status, &src.Status)

	return nil
}

type componentDeleted struct {
	affinity    *Affinity           `json:"affinity,omitempty"`
	tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

func (r *componentDeleted) To(obj runtime.Object) ([]byte, error) {
	comp := obj.(*Component)
	diff := &componentDeleted{
		affinity:    comp.Spec.Affinity,
		tolerations: comp.Spec.Tolerations,
	}
	out, err := json.Marshal(diff)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *componentDeleted) From(data []byte, obj runtime.Object) error {
	diff := &componentDeleted{}
	err := json.Unmarshal(data, diff)
	if err != nil {
		return err
	}
	comp := obj.(*Component)
	comp.Spec.Affinity = diff.affinity
	comp.Spec.Tolerations = diff.tolerations
	return nil
}
