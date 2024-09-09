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
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ConvertTo converts this ClusterDefinition to the Hub version (v1).
func (r *ClusterDefinition) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv1.ClusterDefinition)

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
func (r *ClusterDefinition) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv1.ClusterDefinition)

	// objectMeta
	r.ObjectMeta = src.ObjectMeta

	// spec
	if err := copier.Copy(&r.Spec, &src.Spec); err != nil {
		return err
	}
	if err := incrementConvertFrom(r, src, &clusterDefinitionConverter{}); err != nil {
		return err
	}

	// status
	if err := copier.Copy(&r.Status, &src.Status); err != nil {
		return err
	}

	return nil
}

func (r *ClusterDefinition) incrementConvertTo(metav1.Object) (incrementChange, error) {
	return &clusterDefinitionConverter{
		Spec: clusterDefinitionSpecConverter{
			Type:                 r.Spec.Type,
			ComponentDefs:        r.Spec.ComponentDefs,
			ConnectionCredential: r.Spec.ConnectionCredential,
		},
		Status: clusterDefinitionStatusConverter{
			ServiceRefs: r.Status.ServiceRefs,
		},
	}, nil
}

func (r *ClusterDefinition) incrementConvertFrom(_ metav1.Object, ic incrementChange) error {
	c := ic.(*clusterDefinitionConverter)
	r.Spec.Type = c.Spec.Type
	r.Spec.ComponentDefs = c.Spec.ComponentDefs
	r.Spec.ConnectionCredential = c.Spec.ConnectionCredential
	r.Status.ServiceRefs = c.Status.ServiceRefs
	return nil
}

type clusterDefinitionConverter struct {
	Spec   clusterDefinitionSpecConverter   `json:"spec,omitempty"`
	Status clusterDefinitionStatusConverter `json:"status,omitempty"`
}

type clusterDefinitionSpecConverter struct {
	Type                 string                       `json:"type,omitempty"`
	ComponentDefs        []ClusterComponentDefinition `json:"componentDefs"`
	ConnectionCredential map[string]string            `json:"connectionCredential,omitempty"`
}

type clusterDefinitionStatusConverter struct {
	ServiceRefs string `json:"serviceRefs,omitempty"`
}
