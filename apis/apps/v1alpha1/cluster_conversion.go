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
	"sort"
	"strings"

	"github.com/jinzhu/copier"
	"golang.org/x/exp/maps"
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
	// status
	if err := copier.Copy(&r.Status, &src.Status); err != nil {
		return err
	}
	if err := incrementConvertFrom(r, src, &clusterConverter{}); err != nil {
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
	//     clusterDefRef -> clusterDef
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
	if len(r.Spec.ClusterDefRef) > 0 {
		cluster.Spec.ClusterDef = r.Spec.ClusterDefRef
	}

	if r.Spec.TerminationPolicy == Halt {
		cluster.Spec.TerminationPolicy = appsv1.DoNotTerminate
	} else {
		cluster.Spec.TerminationPolicy = appsv1.TerminationPolicyType(r.Spec.TerminationPolicy)
	}

	r.toClusterServices(cluster)

	if cluster.Spec.SchedulingPolicy == nil {
		var componentName string
		if len(r.Spec.ComponentSpecs) > 0 {
			componentName = r.Spec.ComponentSpecs[0].Name
		}
		// TODO: support convert the schedulingPolicy of the shardings
		affinity := r.Spec.Affinity
		if r.Spec.Affinity == nil {
			affinity = buildAffinity(r)
		}
		cluster.Spec.SchedulingPolicy = r.toSchedulingPolicy(affinity, r.Spec.Tolerations, componentName)
	}

	for i := range r.Spec.ComponentSpecs {
		compSpec := r.Spec.ComponentSpecs[i]
		r.toComponentSpec(compSpec, &cluster.Spec.ComponentSpecs[i], compSpec.Name)
	}
	// TODO: support convert the shardings
}

func (r *Cluster) toClusterServices(cluster *appsv1.Cluster) {
	for i := range r.Spec.Services {
		clusterSVC := r.Spec.Services[i]
		if len(clusterSVC.ShardingSelector) > 0 && len(clusterSVC.ComponentSelector) == 0 {
			cluster.Spec.Services[i].ComponentSelector = clusterSVC.ShardingSelector
		}
	}
}

func (r *Cluster) toSchedulingPolicy(affinity *Affinity, tolerations []corev1.Toleration, compName string) *appsv1.SchedulingPolicy {
	if len(compName) == 0 {
		return nil
	}
	if affinity == nil && len(tolerations) == 0 {
		return nil
	}
	schedulingPolicy := &appsv1.SchedulingPolicy{}
	schedulingPolicy.Tolerations = tolerations

	schedulingPolicy.Affinity = convertToAffinity(r.Name, compName, affinity)
	schedulingPolicy.TopologySpreadConstraints = convertTopologySpreadConstraints4Legacy(r.Name, compName, affinity)
	return schedulingPolicy
}

func (r *Cluster) toComponentSpec(fromCompSpec ClusterComponentSpec, toCompSpec *appsv1.ClusterComponentSpec, componentName string) {
	if toCompSpec.SchedulingPolicy == nil {
		toCompSpec.SchedulingPolicy = r.toSchedulingPolicy(fromCompSpec.Affinity, fromCompSpec.Tolerations, componentName)
	}
}

func (r *Cluster) changesFromCluster(cluster *appsv1.Cluster) {
	// changed:
	//   spec
	//     clusterDefRef -> clusterDef
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
	// appsv1.TerminationPolicyType is a subset of TerminationPolicyType, it can be converted directly.
	if len(cluster.Spec.ClusterDef) > 0 {
		r.Spec.ClusterDefRef = cluster.Spec.ClusterDef
	}
}

type clusterConverter struct {
	Spec   clusterSpecConverter   `json:"spec,omitempty"`
	Status clusterStatusConverter `json:"status,omitempty"`
}

type clusterSpecConverter struct {
	ClusterDefRef      string                          `json:"clusterDefinitionRef,omitempty"`
	ClusterVersionRef  string                          `json:"clusterVersionRef,omitempty"`
	TerminationPolicy  TerminationPolicyType           `json:"terminationPolicy"`
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
	ComponentDefRef        string                  `json:"componentDefRef,omitempty"`
	ClassDefRef            *ClassDefRef            `json:"classDefRef,omitempty"`
	EnabledLogs            []string                `json:"enabledLogs,omitempty"`
	Affinity               *Affinity               `json:"affinity,omitempty"`
	Tolerations            []corev1.Toleration     `json:"tolerations,omitempty"`
	SwitchPolicy           *ClusterSwitchPolicy    `json:"switchPolicy,omitempty"`
	UserResourceRefs       *UserResourceRefs       `json:"userResourceRefs,omitempty"`
	UpdateStrategy         *UpdateStrategy         `json:"updateStrategy,omitempty"`
	InstanceUpdateStrategy *InstanceUpdateStrategy `json:"instanceUpdateStrategy,omitempty"`
	Monitor                *bool                   `json:"monitor,omitempty"`
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
	c.Spec.ClusterVersionRef = cluster.Spec.ClusterVersionRef
	c.Spec.ClusterDefRef = cluster.Spec.ClusterDefRef
	c.Spec.TerminationPolicy = cluster.Spec.TerminationPolicy
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
			ComponentDefRef:        spec.ComponentDefRef,
			ClassDefRef:            spec.ClassDefRef,
			EnabledLogs:            spec.EnabledLogs,
			Affinity:               spec.Affinity,
			Tolerations:            spec.Tolerations,
			SwitchPolicy:           spec.SwitchPolicy,
			UserResourceRefs:       spec.UserResourceRefs,
			UpdateStrategy:         spec.UpdateStrategy,
			InstanceUpdateStrategy: spec.InstanceUpdateStrategy,
			Monitor:                spec.Monitor,
		}
	}
	if len(cluster.Spec.ComponentSpecs) > 0 {
		c.Spec.Components = make(map[string]clusterCompConverter)
		for _, comp := range cluster.Spec.ComponentSpecs {
			c.Spec.Components[comp.Name] = deletedComp(comp)
		}
	}
	if len(cluster.Spec.ShardingSpecs) > 0 {
		c.Spec.Shardings = make(map[string]clusterCompConverter)
		for _, sharding := range cluster.Spec.ShardingSpecs {
			c.Spec.Shardings[sharding.Name] = deletedComp(sharding.Template)
		}
	}

	if len(cluster.Status.Components) > 0 {
		c.Status.Components = make(map[string]clusterCompStatusConverter)
		for name, status := range cluster.Status.Components {
			c.Status.Components[name] = clusterCompStatusConverter{
				PodsReady:     status.PodsReady,
				PodsReadyTime: status.PodsReadyTime,
				MembersStatus: status.MembersStatus,
			}
		}
	}
}

func (c *clusterConverter) toCluster(cluster *Cluster) {
	cluster.Spec.ClusterVersionRef = c.Spec.ClusterVersionRef
	cluster.Spec.ClusterDefRef = c.Spec.ClusterDefRef
	cluster.Spec.TerminationPolicy = c.Spec.TerminationPolicy
	cluster.Spec.Affinity = c.Spec.Affinity
	cluster.Spec.Tolerations = c.Spec.Tolerations
	cluster.Spec.Tenancy = c.Spec.Tenancy
	cluster.Spec.AvailabilityPolicy = c.Spec.AvailabilityPolicy
	cluster.Spec.Replicas = c.Spec.Replicas
	cluster.Spec.Resources = c.Spec.Resources
	cluster.Spec.Storage = c.Spec.Storage
	cluster.Spec.Network = c.Spec.Network

	deletedComp := func(comp clusterCompConverter, spec *ClusterComponentSpec) {
		spec.ComponentDefRef = comp.ComponentDefRef
		spec.ClassDefRef = comp.ClassDefRef
		spec.EnabledLogs = comp.EnabledLogs
		spec.Affinity = comp.Affinity
		spec.Tolerations = comp.Tolerations
		spec.SwitchPolicy = comp.SwitchPolicy
		spec.UserResourceRefs = comp.UserResourceRefs
		spec.UpdateStrategy = comp.UpdateStrategy
		spec.InstanceUpdateStrategy = comp.InstanceUpdateStrategy
		spec.Monitor = comp.Monitor
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

func convertToAffinity(clusterName, compName string, compAffinity *Affinity) *corev1.Affinity {
	if compAffinity == nil {
		return nil
	}
	affinity := new(corev1.Affinity)
	// Build NodeAffinity
	var matchExpressions []corev1.NodeSelectorRequirement
	nodeLabelKeys := maps.Keys(compAffinity.NodeLabels)
	// NodeLabels must be ordered
	sort.Strings(nodeLabelKeys)
	for _, key := range nodeLabelKeys {
		values := strings.Split(compAffinity.NodeLabels[key], ",")
		matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
			Key:      key,
			Operator: corev1.NodeSelectorOpIn,
			Values:   values,
		})
	}
	if len(matchExpressions) > 0 {
		nodeSelectorTerm := corev1.NodeSelectorTerm{
			MatchExpressions: matchExpressions,
		}
		affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{nodeSelectorTerm},
			},
		}
	}
	// Build PodAntiAffinity
	var podAntiAffinity *corev1.PodAntiAffinity
	var podAffinityTerms []corev1.PodAffinityTerm
	for _, topologyKey := range compAffinity.TopologyKeys {
		podAffinityTerms = append(podAffinityTerms, corev1.PodAffinityTerm{
			TopologyKey: topologyKey,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance":        clusterName,
					"apps.kubeblocks.io/component-name": compName,
				},
			},
		})
	}
	if compAffinity.PodAntiAffinity == Required {
		podAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: podAffinityTerms,
		}
	} else {
		var weightedPodAffinityTerms []corev1.WeightedPodAffinityTerm
		for _, podAffinityTerm := range podAffinityTerms {
			weightedPodAffinityTerms = append(weightedPodAffinityTerms, corev1.WeightedPodAffinityTerm{
				Weight:          100,
				PodAffinityTerm: podAffinityTerm,
			})
		}
		podAntiAffinity = &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: weightedPodAffinityTerms,
		}
	}
	affinity.PodAntiAffinity = podAntiAffinity
	return affinity
}

func convertTopologySpreadConstraints4Legacy(clusterName, compName string, compAffinity *Affinity) []corev1.TopologySpreadConstraint {
	if compAffinity == nil {
		return nil
	}

	var topologySpreadConstraints []corev1.TopologySpreadConstraint

	var whenUnsatisfiable corev1.UnsatisfiableConstraintAction
	if compAffinity.PodAntiAffinity == Required {
		whenUnsatisfiable = corev1.DoNotSchedule
	} else {
		whenUnsatisfiable = corev1.ScheduleAnyway
	}
	for _, topologyKey := range compAffinity.TopologyKeys {
		topologySpreadConstraints = append(topologySpreadConstraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			WhenUnsatisfiable: whenUnsatisfiable,
			TopologyKey:       topologyKey,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance":        clusterName,
					"apps.kubeblocks.io/component-name": compName,
				},
			},
		})
	}
	return topologySpreadConstraints
}

func buildAffinity(cluster *Cluster) *Affinity {
	affinityTopoKey := func(policyType AvailabilityPolicyType) string {
		switch policyType {
		case AvailabilityPolicyZone:
			return "topology.kubernetes.io/zone"
		case AvailabilityPolicyNode:
			return "kubernetes.io/hostname"
		}
		return ""
	}
	var affinity *Affinity
	if len(cluster.Spec.Tenancy) > 0 || len(cluster.Spec.AvailabilityPolicy) > 0 {
		affinity = &Affinity{
			PodAntiAffinity: Preferred,
			TopologyKeys:    []string{affinityTopoKey(cluster.Spec.AvailabilityPolicy)},
			Tenancy:         cluster.Spec.Tenancy,
		}
	}
	return affinity
}
