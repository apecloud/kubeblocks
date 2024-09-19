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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

// ConvertTo converts this Cluster to the Hub version (v1).
func (r *Cluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv1.Cluster)

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
func (r *Cluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv1.Cluster)

	// objectMeta
	r.ObjectMeta = src.ObjectMeta

	// spec
	if err := copier.Copy(&r.Spec, &src.Spec); err != nil {
		return err
	}
	if err := incrementConvertFrom(r, src, &clusterConverter{}); err != nil {
		return err
	}

	// status
	if err := copier.Copy(&r.Status, &src.Status); err != nil {
		return err
	}

	return nil
}

func (r *Cluster) incrementConvertTo(dstRaw metav1.Object) (incrementChange, error) {
	// changed
	r.changesToCluster(dstRaw.(*appsv1.Cluster))

	// deleted
	c := &clusterConverter{}
	c.fromCluster(r)
	return c, nil
}

func (r *Cluster) incrementConvertFrom(srcRaw metav1.Object, ic incrementChange) error {
	// deleted
	c := ic.(*clusterConverter)
	c.toCluster(r)

	// changed
	r.changesFromCluster(srcRaw.(*appsv1.Cluster))

	return nil
}

func (r *Cluster) changesToCluster(cluster *appsv1.Cluster) {
	// changed:
	//   spec
	//     components
	//       - volumeClaimTemplates
	//           spec:
	//             resources: corev1.ResourceRequirements -> corev1.VolumeResourceRequirements
	//         podUpdatePolicy: *workloads.PodUpdatePolicyType -> *PodUpdatePolicyType
	//     sharings
	//       - template
	//           volumeClaimTemplates
	//             spec:
	//               resources: corev1.ResourceRequirements -> corev1.VolumeResourceRequirements
	//           podUpdatePolicy: *workloads.PodUpdatePolicyType -> *PodUpdatePolicyType
	//   status
	//     components
	//       - message: ComponentMessageMap -> map[string]string
}

func (r *Cluster) changesFromCluster(cluster *appsv1.Cluster) {
	// changed:
	//   spec
	//     components
	//       - volumeClaimTemplates
	//           spec:
	//             resources: corev1.ResourceRequirements -> corev1.VolumeResourceRequirements
	//         podUpdatePolicy: *workloads.PodUpdatePolicyType -> *PodUpdatePolicyType
	//     sharings
	//       - template
	//           volumeClaimTemplates
	//             spec:
	//               resources: corev1.ResourceRequirements -> corev1.VolumeResourceRequirements
	//           podUpdatePolicy: *workloads.PodUpdatePolicyType -> *PodUpdatePolicyType
	//   status
	//     components
	//       - message: ComponentMessageMap -> map[string]string
}

type clusterConverter struct {
	Spec   clusterSpecConverter   `json:"spec,omitempty"`
	Status clusterStatusConverter `json:"status,omitempty"`
}

type clusterSpecConverter struct {
	Affinity           *Affinity                       `json:"affinity,omitempty"`
	Tolerations        []corev1.Toleration             `json:"tolerations,omitempty"`
	Tenancy            TenancyType                     `json:"tenancy,omitempty"`
	AvailabilityPolicy AvailabilityPolicyType          `json:"availabilityPolicy,omitempty"`
	Replicas           *int32                          `json:"replicas,omitempty"`
	Resources          ClusterResources                `json:"resources,omitempty"`
	Storage            ClusterStorage                  `json:"storage,omitempty"`
	Network            *ClusterNetwork                 `json:"network,omitempty"`
	Components         map[string]clusterCompConverter `json:"components,omitempty"`
	Shardings          map[string]clusterCompConverter `json:"shardings,omitempty"`
}

type clusterCompConverter struct {
	ClassDefRef            *ClassDefRef            `json:"classDefRef,omitempty"`
	Affinity               *Affinity               `json:"affinity,omitempty"`
	Tolerations            []corev1.Toleration     `json:"tolerations,omitempty"`
	SwitchPolicy           *ClusterSwitchPolicy    `json:"switchPolicy,omitempty"`
	InstanceUpdateStrategy *InstanceUpdateStrategy `json:"instanceUpdateStrategy,omitempty"`
}

type clusterStatusConverter struct {
	Components map[string]clusterCompStatusConverter `json:"components,omitempty"`
}

type clusterCompStatusConverter struct {
	PodsReady     *bool                    `json:"podsReady,omitempty"`
	PodsReadyTime *metav1.Time             `json:"podsReadyTime,omitempty"`
	MembersStatus []workloads.MemberStatus `json:"membersStatus,omitempty"`
}

func (c *clusterConverter) fromCluster(cluster *Cluster) {
	c.Spec.Affinity = cluster.Spec.Affinity
	c.Spec.Tolerations = cluster.Spec.Tolerations
	c.Spec.Tenancy = cluster.Spec.Tenancy
	c.Spec.AvailabilityPolicy = cluster.Spec.AvailabilityPolicy
	c.Spec.Replicas = cluster.Spec.Replicas
	c.Spec.Resources = cluster.Spec.Resources
	c.Spec.Storage = cluster.Spec.Storage
	c.Spec.Network = cluster.Spec.Network

	deletedComp := func(spec ClusterComponentSpec) clusterCompConverter {
		return clusterCompConverter{
			ClassDefRef:            spec.ClassDefRef,
			Affinity:               spec.Affinity,
			Tolerations:            spec.Tolerations,
			SwitchPolicy:           spec.SwitchPolicy,
			InstanceUpdateStrategy: spec.InstanceUpdateStrategy,
		}
	}
	for _, comp := range cluster.Spec.ComponentSpecs {
		c.Spec.Components[comp.Name] = deletedComp(comp)
	}
	for _, sharding := range cluster.Spec.ShardingSpecs {
		c.Spec.Shardings[sharding.Name] = deletedComp(sharding.Template)
	}

	for name, status := range cluster.Status.Components {
		c.Status.Components[name] = clusterCompStatusConverter{
			PodsReady:     status.PodsReady,
			PodsReadyTime: status.PodsReadyTime,
			MembersStatus: status.MembersStatus,
		}
	}
}

func (c *clusterConverter) toCluster(cluster *Cluster) {
	cluster.Spec.Affinity = c.Spec.Affinity
	cluster.Spec.Tolerations = c.Spec.Tolerations
	cluster.Spec.Tenancy = c.Spec.Tenancy
	cluster.Spec.AvailabilityPolicy = c.Spec.AvailabilityPolicy
	cluster.Spec.Replicas = c.Spec.Replicas
	cluster.Spec.Resources = c.Spec.Resources
	cluster.Spec.Storage = c.Spec.Storage
	cluster.Spec.Network = c.Spec.Network

	deletedComp := func(comp clusterCompConverter, spec *ClusterComponentSpec) {
		spec.ClassDefRef = comp.ClassDefRef
		spec.Affinity = comp.Affinity
		spec.Tolerations = comp.Tolerations
		spec.SwitchPolicy = comp.SwitchPolicy
		spec.InstanceUpdateStrategy = comp.InstanceUpdateStrategy
	}
	for i, spec := range cluster.Spec.ComponentSpecs {
		comp, ok := c.Spec.Components[spec.Name]
		if ok {
			deletedComp(comp, &cluster.Spec.ComponentSpecs[i])
		}
	}
	for i, spec := range cluster.Spec.ShardingSpecs {
		template, ok := c.Spec.Shardings[spec.Name]
		if ok {
			deletedComp(template, &cluster.Spec.ShardingSpecs[i].Template)
		}
	}

	for name, comp := range cluster.Status.Components {
		status, ok := c.Status.Components[name]
		if ok {
			comp.PodsReady = status.PodsReady
			comp.PodsReadyTime = status.PodsReadyTime
			comp.MembersStatus = status.MembersStatus
			cluster.Status.Components[name] = comp
		}
	}
}
