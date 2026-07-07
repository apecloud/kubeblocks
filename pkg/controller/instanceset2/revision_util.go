/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instanceset2

import (
	"fmt"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/dump"
	"k8s.io/apimachinery/pkg/util/rand"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

const instanceSetRevisionAnnotationKey = "workloads.kubeblocks.io/instance-revision-hash"

type instanceRevisionIntent struct {
	Template                   podTemplateRevisionIntent
	MinReadySeconds            int32
	VolumeClaimTemplates       []pvcTemplateRevisionIntent
	InstanceSetName            string
	InstanceTemplateName       string
	InstanceUpdateStrategyType *kbappsv1.InstanceUpdateStrategyType
	PodUpdatePolicy            workloads.PodUpdatePolicyType
	PodUpgradePolicy           workloads.PodUpdatePolicyType
	Roles                      []workloads.ReplicaRole
	Configs                    []workloads.ConfigTemplate
}

type podTemplateRevisionIntent struct {
	Labels      map[string]string
	Annotations map[string]string
	Spec        corev1.PodSpec
}

type pvcTemplateRevisionIntent struct {
	Name        string
	Labels      map[string]string
	Annotations map[string]string
	Spec        corev1.PersistentVolumeClaimSpec
}

func buildInstanceRevision(inst *workloads.Instance) string {
	return buildRevisionIntentHash(buildInstanceRevisionIntent(inst))
}

func stampInstanceRevision(inst *workloads.Instance) string {
	revision := buildInstanceRevision(inst)
	if inst.Annotations == nil {
		inst.Annotations = make(map[string]string)
	}
	inst.Annotations[instanceSetRevisionAnnotationKey] = revision
	return revision
}

func getInstanceRevision(inst *workloads.Instance) string {
	if inst.Annotations == nil {
		return ""
	}
	return inst.Annotations[instanceSetRevisionAnnotationKey]
}

func buildInstanceRevisionIntent(inst *workloads.Instance) instanceRevisionIntent {
	spec := inst.Spec.DeepCopy()
	return instanceRevisionIntent{
		Template:                   buildPodTemplateRevisionIntent(spec.Template),
		MinReadySeconds:            spec.MinReadySeconds,
		VolumeClaimTemplates:       buildPVCTemplateRevisionIntents(spec.VolumeClaimTemplates),
		InstanceSetName:            spec.InstanceSetName,
		InstanceTemplateName:       spec.InstanceTemplateName,
		InstanceUpdateStrategyType: copyInstanceUpdateStrategyType(spec.InstanceUpdateStrategyType),
		PodUpdatePolicy:            spec.PodUpdatePolicy,
		PodUpgradePolicy:           spec.PodUpgradePolicy,
		Roles:                      copyReplicaRoles(spec.Roles),
		Configs:                    copyConfigTemplates(spec.Configs),
	}
}

func buildRevisionIntentHash(intent instanceRevisionIntent) string {
	hasher := fnv.New32()
	fmt.Fprintf(hasher, "%v", dump.ForHash(intent))
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

func buildPodTemplateRevisionIntent(template corev1.PodTemplateSpec) podTemplateRevisionIntent {
	return podTemplateRevisionIntent{
		Labels:      copyStringMap(template.Labels),
		Annotations: copyStringMap(template.Annotations),
		Spec:        *template.Spec.DeepCopy(),
	}
}

func buildPVCTemplateRevisionIntents(templates []corev1.PersistentVolumeClaimTemplate) []pvcTemplateRevisionIntent {
	if len(templates) == 0 {
		return nil
	}
	intents := make([]pvcTemplateRevisionIntent, len(templates))
	for i := range templates {
		intents[i] = pvcTemplateRevisionIntent{
			Name:        templates[i].Name,
			Labels:      copyStringMap(templates[i].Labels),
			Annotations: copyStringMap(templates[i].Annotations),
			Spec:        *templates[i].Spec.DeepCopy(),
		}
	}
	return intents
}

func copyInstanceUpdateStrategyType(strategy *kbappsv1.InstanceUpdateStrategyType) *kbappsv1.InstanceUpdateStrategyType {
	if strategy == nil {
		return nil
	}
	copied := *strategy
	return &copied
}

func copyStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	copied := make(map[string]string, len(m))
	for k, v := range m {
		copied[k] = v
	}
	return copied
}

func copyReplicaRoles(roles []workloads.ReplicaRole) []workloads.ReplicaRole {
	if len(roles) == 0 {
		return nil
	}
	copied := make([]workloads.ReplicaRole, len(roles))
	copy(copied, roles)
	return copied
}

func copyConfigTemplates(configs []workloads.ConfigTemplate) []workloads.ConfigTemplate {
	if len(configs) == 0 {
		return nil
	}
	copied := make([]workloads.ConfigTemplate, len(configs))
	for i := range configs {
		configs[i].DeepCopyInto(&copied[i])
	}
	return copied
}
