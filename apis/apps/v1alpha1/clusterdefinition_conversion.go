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
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ConvertTo converts this ClusterDefinition to the Hub version (v1).
func (r *ClusterDefinition) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv1.ClusterDefinition)

	// objectMeta
	dst.ObjectMeta = r.ObjectMeta

	// spec
	dst.Spec.Topologies = r.topologiesTo(r.Spec.Topologies)

	// status
	dst.Status.ObservedGeneration = r.Status.ObservedGeneration
	dst.Status.Phase = appsv1.Phase(r.Status.Phase)
	dst.Status.Message = r.Status.Message
	dst.Status.Topologies = r.Status.Topologies

	return nil
}

// ConvertFrom converts from the Hub version (v1) to this version.
func (r *ClusterDefinition) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv1.ClusterDefinition)

	// objectMeta
	r.ObjectMeta = src.ObjectMeta

	// spec
	r.Spec.Topologies = r.topologiesFrom(src.Spec.Topologies)

	// status
	r.Status.ObservedGeneration = src.Status.ObservedGeneration
	r.Status.Phase = Phase(src.Status.Phase)
	r.Status.Message = src.Status.Message
	r.Status.Topologies = src.Status.Topologies

	return nil
}

func (r *ClusterDefinition) topologiesTo(src []ClusterTopology) []appsv1.ClusterTopology {
	if src != nil {
		topologies := make([]appsv1.ClusterTopology, 0)
		for _, topology := range src {
			topologies = append(topologies, appsv1.ClusterTopology{
				Name:       topology.Name,
				Components: r.topologyComponentTo(topology.Components),
				Orders:     r.topologyOrdersTo(topology.Orders),
				Default:    topology.Default,
			})
		}
		return topologies
	}
	return nil
}

func (r *ClusterDefinition) topologyComponentTo(src []ClusterTopologyComponent) []appsv1.ClusterTopologyComponent {
	if src != nil {
		comps := make([]appsv1.ClusterTopologyComponent, 0)
		for _, comp := range src {
			comps = append(comps, appsv1.ClusterTopologyComponent{
				Name:    comp.Name,
				CompDef: comp.CompDef,
			})
		}
		return comps
	}
	return nil
}

func (r *ClusterDefinition) topologyOrdersTo(src *ClusterTopologyOrders) *appsv1.ClusterTopologyOrders {
	if src != nil {
		return &appsv1.ClusterTopologyOrders{
			Provision: src.Provision,
			Terminate: src.Terminate,
			Update:    src.Update,
		}
	}
	return nil
}

func (r *ClusterDefinition) topologiesFrom(src []appsv1.ClusterTopology) []ClusterTopology {
	if src != nil {
		topologies := make([]ClusterTopology, 0)
		for _, topology := range src {
			topologies = append(topologies, ClusterTopology{
				Name:       topology.Name,
				Components: r.topologyComponentFrom(topology.Components),
				Orders:     r.topologyOrdersFrom(topology.Orders),
				Default:    topology.Default,
			})
		}
		return topologies
	}
	return nil
}

func (r *ClusterDefinition) topologyComponentFrom(src []appsv1.ClusterTopologyComponent) []ClusterTopologyComponent {
	if src != nil {
		comps := make([]ClusterTopologyComponent, 0)
		for _, comp := range src {
			comps = append(comps, ClusterTopologyComponent{
				Name:    comp.Name,
				CompDef: comp.CompDef,
			})
		}
		return comps
	}
	return nil
}

func (r *ClusterDefinition) topologyOrdersFrom(src *appsv1.ClusterTopologyOrders) *ClusterTopologyOrders {
	if src != nil {
		if src != nil {
			return &ClusterTopologyOrders{
				Provision: src.Provision,
				Terminate: src.Terminate,
				Update:    src.Update,
			}
		}
	}
	return nil
}
