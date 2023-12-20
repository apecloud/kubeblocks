package rsm

import (
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func buildPod(rsm workloads.ReplicatedStateMachine, podName string, nodeName types.NodeName) *corev1.Pod {
	annotations := ParseAnnotationsOfScope(RootScope, rsm.Annotations)
	delete(annotations, constant.ComponentReplicasAnnotationKey)
	delete(annotations, constant.KubeBlocksGenerationKey)
	labels := getLabels(&rsm)
	delete(labels, rsmGenerationLabelKey)
	return builder.NewPodBuilder(rsm.Namespace, podName).
		SetNodeName(nodeName).
		SetContainers(rsm.Spec.Template.Spec.Containers).
		SetInitContainers(rsm.Spec.Template.Spec.InitContainers).
		SetFinalizers().
		AddVolumes(rsm.Spec.Template.Spec.Volumes...).
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
