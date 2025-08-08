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

package builder

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

type InstanceBuilder struct {
	BaseBuilder[workloads.Instance, *workloads.Instance, InstanceBuilder]
}

func NewInstanceBuilder(namespace, name string) *InstanceBuilder {
	builder := &InstanceBuilder{}
	builder.init(namespace, name, &workloads.Instance{}, builder)
	return builder
}

func (builder *InstanceBuilder) SetFinalizers() *InstanceBuilder {
	builder.get().Finalizers = nil
	return builder
}

func (builder *InstanceBuilder) SetPodTemplate(template corev1.PodTemplateSpec) *InstanceBuilder {
	builder.get().Spec.Template = template
	return builder
}

func (builder *InstanceBuilder) podSpec() *corev1.PodSpec {
	return &builder.get().Spec.Template.Spec
}

func (builder *InstanceBuilder) SetContainers(containers []corev1.Container) *InstanceBuilder {
	builder.podSpec().Containers = containers
	return builder
}

func (builder *InstanceBuilder) SetInitContainers(initContainers []corev1.Container) *InstanceBuilder {
	builder.podSpec().InitContainers = initContainers
	return builder
}

func (builder *InstanceBuilder) SetNodeName(nodeName types.NodeName) *InstanceBuilder {
	builder.podSpec().NodeName = string(nodeName)
	return builder
}

func (builder *InstanceBuilder) SetHostname(hostname string) *InstanceBuilder {
	builder.podSpec().Hostname = hostname
	return builder
}

func (builder *InstanceBuilder) SetSubdomain(subdomain string) *InstanceBuilder {
	builder.podSpec().Subdomain = subdomain
	return builder
}

func (builder *InstanceBuilder) AddInitContainer(container corev1.Container) *InstanceBuilder {
	builder.podSpec().InitContainers = append(builder.podSpec().InitContainers, container)
	return builder
}

func (builder *InstanceBuilder) AddContainer(container corev1.Container) *InstanceBuilder {
	builder.podSpec().Containers = append(builder.podSpec().Containers, container)
	return builder
}

func (builder *InstanceBuilder) AddVolumes(volumes ...corev1.Volume) *InstanceBuilder {
	builder.podSpec().Volumes = append(builder.podSpec().Volumes, volumes...)
	return builder
}

func (builder *InstanceBuilder) SetRestartPolicy(policy corev1.RestartPolicy) *InstanceBuilder {
	builder.podSpec().RestartPolicy = policy
	return builder
}

func (builder *InstanceBuilder) SetSecurityContext(ctx corev1.PodSecurityContext) *InstanceBuilder {
	builder.podSpec().SecurityContext = &ctx
	return builder
}

func (builder *InstanceBuilder) AddTolerations(tolerations ...corev1.Toleration) *InstanceBuilder {
	builder.podSpec().Tolerations = append(builder.podSpec().Tolerations, tolerations...)
	return builder
}

func (builder *InstanceBuilder) AddServiceAccount(serviceAccount string) *InstanceBuilder {
	builder.podSpec().ServiceAccountName = serviceAccount
	return builder
}

func (builder *InstanceBuilder) SetNodeSelector(nodeSelector map[string]string) *InstanceBuilder {
	builder.podSpec().NodeSelector = nodeSelector
	return builder
}

func (builder *InstanceBuilder) SetAffinity(affinity *corev1.Affinity) *InstanceBuilder {
	builder.podSpec().Affinity = affinity
	return builder
}

func (builder *InstanceBuilder) SetTopologySpreadConstraints(topologySpreadConstraints []corev1.TopologySpreadConstraint) *InstanceBuilder {
	builder.podSpec().TopologySpreadConstraints = topologySpreadConstraints
	return builder
}

func (builder *InstanceBuilder) SetActiveDeadlineSeconds(activeDeadline *int64) *InstanceBuilder {
	builder.podSpec().ActiveDeadlineSeconds = activeDeadline
	return builder
}

func (builder *InstanceBuilder) SetImagePullSecrets(secrets []corev1.LocalObjectReference) *InstanceBuilder {
	builder.podSpec().ImagePullSecrets = secrets
	return builder
}

func (builder *InstanceBuilder) SetSelector(selector *metav1.LabelSelector) *InstanceBuilder {
	builder.get().Spec.Selector = selector
	return builder
}

func (builder *InstanceBuilder) SetSelectorMatchLabels(labels map[string]string) *InstanceBuilder {
	selector := builder.get().Spec.Selector
	if selector == nil {
		selector = &metav1.LabelSelector{}
		builder.get().Spec.Selector = selector
	}
	matchLabels := make(map[string]string, len(labels))
	for k, v := range labels {
		matchLabels[k] = v
	}
	builder.get().Spec.Selector.MatchLabels = matchLabels
	return builder
}

func (builder *InstanceBuilder) SetMinReadySeconds(seconds int32) *InstanceBuilder {
	builder.get().Spec.MinReadySeconds = seconds
	return builder
}

func (builder *InstanceBuilder) AddVolumeClaimTemplate(pvc corev1.PersistentVolumeClaim) *InstanceBuilder {
	builder.get().Spec.VolumeClaimTemplates = append(builder.get().Spec.VolumeClaimTemplates,
		corev1.PersistentVolumeClaimTemplate{
			ObjectMeta: pvc.ObjectMeta,
			Spec:       pvc.Spec,
		})
	return builder
}

func (builder *InstanceBuilder) SetPVCRetentionPolicy(policy *workloads.PersistentVolumeClaimRetentionPolicy) *InstanceBuilder {
	builder.get().Spec.PersistentVolumeClaimRetentionPolicy = policy
	return builder
}

func (builder *InstanceBuilder) SetInstanceSetName(name string) *InstanceBuilder {
	builder.get().Spec.InstanceSetName = name
	return builder
}

func (builder *InstanceBuilder) SetInstanceTemplateName(name string) *InstanceBuilder {
	builder.get().Spec.InstanceTemplateName = name
	return builder
}

func (builder *InstanceBuilder) SetInstanceUpdateStrategyType(strategy *workloads.InstanceUpdateStrategy) *InstanceBuilder {
	if strategy != nil {
		builder.get().Spec.InstanceUpdateStrategyType = &strategy.Type
	}
	return builder
}

func (builder *InstanceBuilder) SetPodUpdatePolicy(policy workloads.PodUpdatePolicyType) *InstanceBuilder {
	builder.get().Spec.PodUpdatePolicy = policy
	return builder
}

func (builder *InstanceBuilder) SetRoles(roles []workloads.ReplicaRole) *InstanceBuilder {
	builder.get().Spec.Roles = roles
	return builder
}

func (builder *InstanceBuilder) SetMembershipReconfiguration(arg *workloads.MembershipReconfiguration) *InstanceBuilder {
	builder.get().Spec.MembershipReconfiguration = arg
	return builder
}

func (builder *InstanceBuilder) SetTemplateVars(vars map[string]string) *InstanceBuilder {
	builder.get().Spec.TemplateVars = vars
	return builder
}

func (builder *InstanceBuilder) SetInstanceAssistantObjects(objs []workloads.InstanceAssistantObject) *InstanceBuilder {
	builder.get().Spec.InstanceAssistantObjects = objs
	return builder
}
