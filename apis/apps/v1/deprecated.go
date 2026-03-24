/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

func (r *ClusterSpec) GetComponentByName(compName string) *ClusterComponentSpec {
	for i, v := range r.ComponentSpecs {
		if v.Name == compName {
			return &r.ComponentSpecs[i]
		}
	}
	return nil
}

func (r *ClusterSpec) GetShardingByName(shardingName string) *ClusterSharding {
	for i, v := range r.Shardings {
		if v.Name == shardingName {
			return &r.Shardings[i]
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

func (r *ClusterComponentStatus) GetObjectMessage(objectKind, objectName string) string {
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	return r.Message[messageKey]
}

func GetClusterUpRunningPhases() []ClusterPhase {
	return []ClusterPhase{
		RunningClusterPhase,
		AbnormalClusterPhase,
		FailedClusterPhase,
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

func (t *InstanceTemplate) GetOrdinals() Ordinals {
	return t.Ordinals
}
