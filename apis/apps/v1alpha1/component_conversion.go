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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ConvertTo converts this Component to the Hub version (v1).
func (r *Component) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv1.Component)

	// objectMeta
	dst.ObjectMeta = r.ObjectMeta

	// spec
	if err := copier.Copy(&dst.Spec, &r.Spec); err != nil {
		return err
	}
	if err := incrementConvertTo(r, dst); err != nil {
		return err
	}

	// status
	if err := copier.Copy(&dst.Status, &r.Status); err != nil {
		return err
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1) to this version.
func (r *Component) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv1.Component)

	// objectMeta
	r.ObjectMeta = src.ObjectMeta

	// spec
	if err := copier.Copy(&r.Spec, &src.Spec); err != nil {
		return err
	}
	if err := incrementConvertFrom(r, src, &componentConverter{}); err != nil {
		return err
	}

	// status
	if err := copier.Copy(&r.Status, &src.Status); err != nil {
		return err
	}

	return nil
}

func (r *Component) incrementConvertTo(dstRaw metav1.Object) (incrementChange, error) {
	// changed
	comp := dstRaw.(*appsv1.Component)
	comp.Status.Message = r.Status.Message

	// deleted
	return &componentConverter{
		EnabledLogs:            r.Spec.EnabledLogs,
		Affinity:               r.Spec.Affinity,
		Tolerations:            r.Spec.Tolerations,
		InstanceUpdateStrategy: r.Spec.InstanceUpdateStrategy,
	}, nil
}

func (r *Component) incrementConvertFrom(srcRaw metav1.Object, ic incrementChange) error {
	// deleted
	c := ic.(*componentConverter)
	r.Spec.EnabledLogs = c.EnabledLogs
	r.Spec.Affinity = c.Affinity
	r.Spec.Tolerations = c.Tolerations
	r.Spec.InstanceUpdateStrategy = c.InstanceUpdateStrategy

	// changed
	comp := srcRaw.(*appsv1.Component)
	r.Status.Message = comp.Status.Message

	return nil
}

type componentConverter struct {
	EnabledLogs            []string                `json:"enabledLogs,omitempty"`
	Affinity               *Affinity               `json:"affinity,omitempty"`
	Tolerations            []corev1.Toleration     `json:"tolerations,omitempty"`
	InstanceUpdateStrategy *InstanceUpdateStrategy `json:"instanceUpdateStrategy,omitempty"`
}
