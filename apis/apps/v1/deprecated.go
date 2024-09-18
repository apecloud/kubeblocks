/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	defaultInstanceTemplateReplicas = 1
)

func (r *Cluster) IsDeleting() bool {
	if r.GetDeletionTimestamp().IsZero() {
		return false
	}
	return r.Spec.TerminationPolicy != DoNotTerminate
}

func (r *Cluster) IsUpdating() bool {
	return r.Status.ObservedGeneration != r.Generation
}

func (r *Cluster) IsStatusUpdating() bool {
	return !r.IsDeleting() && !r.IsUpdating()
}

func (r *Cluster) GetComponentByName(componentName string) *ClusterComponentSpec {
	for _, v := range r.Spec.ComponentSpecs {
		if v.Name == componentName {
			return &v
		}
	}
	return nil
}

// GetVolumeClaimNames gets all PVC names of component compName.
//
// r.Spec.GetComponentByName(compName).VolumeClaimTemplates[*].Name will be used if no claimNames provided
//
// nil return if:
// 1. component compName not found or
// 2. len(VolumeClaimTemplates)==0 or
// 3. any claimNames not found
func (r *Cluster) GetVolumeClaimNames(compName string, claimNames ...string) []string {
	if r == nil {
		return nil
	}
	comp := r.Spec.GetComponentByName(compName)
	if comp == nil {
		return nil
	}
	if len(comp.VolumeClaimTemplates) == 0 {
		return nil
	}
	if len(claimNames) == 0 {
		for _, template := range comp.VolumeClaimTemplates {
			claimNames = append(claimNames, template.Name)
		}
	}
	allExist := true
	for _, name := range claimNames {
		found := false
		for _, template := range comp.VolumeClaimTemplates {
			if template.Name == name {
				found = true
				break
			}
		}
		if !found {
			allExist = false
			break
		}
	}
	if !allExist {
		return nil
	}

	pvcNames := make([]string, 0)
	for _, claimName := range claimNames {
		for i := 0; i < int(comp.Replicas); i++ {
			pvcName := fmt.Sprintf("%s-%s-%s-%d", claimName, r.Name, compName, i)
			pvcNames = append(pvcNames, pvcName)
		}
	}
	return pvcNames
}

func (r *ClusterSpec) GetComponentByName(componentName string) *ClusterComponentSpec {
	for _, v := range r.ComponentSpecs {
		if v.Name == componentName {
			return &v
		}
	}
	return nil
}

func (r *ClusterSpec) GetShardingByName(shardingName string) *ShardingSpec {
	for _, v := range r.ShardingSpecs {
		if v.Name == shardingName {
			return &v
		}
	}
	return nil
}

// SetComponentStatus does safe operation on ClusterStatus.Components map object update.
func (r *ClusterStatus) SetComponentStatus(name string, status ClusterComponentStatus) {
	if r.Components == nil {
		r.Components = map[string]ClusterComponentStatus{}
	}
	r.Components[name] = status
}

func (r *ClusterComponentSpec) GetDisableExporter() *bool {
	if r.DisableExporter != nil {
		return r.DisableExporter
	}

	toPointer := func(b bool) *bool {
		p := b
		return &p
	}

	// Compatible with previous versions of kb
	if r.Monitor != nil {
		return toPointer(!*r.Monitor)
	}
	return nil
}

func (r *ClusterComponentSpec) ToVolumeClaimTemplates() []corev1.PersistentVolumeClaimTemplate {
	if r == nil {
		return nil
	}
	var ts []corev1.PersistentVolumeClaimTemplate
	for _, t := range r.VolumeClaimTemplates {
		ts = append(ts, t.toVolumeClaimTemplate())
	}
	return ts
}

func (r *ClusterComponentStatus) GetObjectMessage(objectKind, objectName string) string {
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	return r.Message[messageKey]
}

func (r *ClusterComponentVolumeClaimTemplate) toVolumeClaimTemplate() corev1.PersistentVolumeClaimTemplate {
	return corev1.PersistentVolumeClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Name,
		},
		Spec: r.Spec.ToV1PersistentVolumeClaimSpec(),
	}
}

func (r *PersistentVolumeClaimSpec) ToV1PersistentVolumeClaimSpec() corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes:      r.AccessModes,
		Resources:        r.Resources,
		StorageClassName: r.getStorageClassName(viper.GetString(constant.CfgKeyDefaultStorageClass)),
		VolumeMode:       r.VolumeMode,
	}
}

// getStorageClassName returns PersistentVolumeClaimSpec.StorageClassName if a value is assigned; otherwise,
// it returns preferSC argument.
func (r *PersistentVolumeClaimSpec) getStorageClassName(preferSC string) *string {
	if r.StorageClassName != nil && *r.StorageClassName != "" {
		return r.StorageClassName
	}
	if preferSC != "" {
		return &preferSC
	}
	return nil
}

func GetClusterUpRunningPhases() []ClusterPhase {
	return []ClusterPhase{
		RunningClusterPhase,
		AbnormalClusterPhase,
		FailedClusterPhase,
	}
}

func GetComponentTerminalPhases() []ClusterComponentPhase {
	return []ClusterComponentPhase{
		RunningClusterCompPhase,
		StoppedClusterCompPhase,
		FailedClusterCompPhase,
		AbnormalClusterCompPhase,
	}
}

func GetReconfiguringRunningPhases() []ClusterPhase {
	return []ClusterPhase{
		RunningClusterPhase,
		UpdatingClusterPhase, // enable partial running for reconfiguring
		AbnormalClusterPhase,
		FailedClusterPhase,
	}
}

func GetInstanceTemplateName(clusterName, componentName, instanceName string) string {
	workloadPrefix := fmt.Sprintf("%s-%s", clusterName, componentName)
	compInsKey := instanceName[:strings.LastIndex(instanceName, "-")]
	if compInsKey == workloadPrefix {
		return ""
	}
	return strings.Replace(compInsKey, workloadPrefix+"-", "", 1)
}

func (t *InstanceTemplate) GetName() string {
	return t.Name
}

func (t *InstanceTemplate) GetReplicas() int32 {
	if t.Replicas != nil {
		return *t.Replicas
	}
	return defaultInstanceTemplateReplicas
}

// GetOrdinals TODO(free6om): Remove after resolving the circular dependencies between apps and workloads.
func (t *InstanceTemplate) GetOrdinals() workloads.Ordinals {
	return workloads.Ordinals{}
}
