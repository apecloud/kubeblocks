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

package rsm

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

func buildPod(rsm workloads.ReplicatedStateMachine, podName string, nodeName types.NodeName) *corev1.Pod {
	annotations := ParseAnnotationsOfScope(RootScope, rsm.Annotations)
	delete(annotations, constant.ComponentReplicasAnnotationKey)
	delete(annotations, constant.KubeBlocksGenerationKey)
	labels := getLabels(&rsm)
	delete(labels, rsmGenerationLabelKey)
	return builder.NewPodBuilder(rsm.Namespace, podName).
		SetPodSpec(rsm.Spec.Template.Spec).
		SetFinalizers().
		SetNodeName(nodeName).
		AddAnnotationsInMap(annotations).
		AddLabelsInMap(labels).
		GetObject()
}

func buildPods(rsm workloads.ReplicatedStateMachine) []*corev1.Pod {
	pods := make([]*corev1.Pod, 0)
	for idx := range rsm.Spec.NodeAssignment {
		nodeAssignment := rsm.Spec.NodeAssignment[idx]
		pod := buildPod(rsm, nodeAssignment.Name, nodeAssignment.NodeSpec.NodeName)
		pods = append(pods, pod)
	}
	return pods
}
