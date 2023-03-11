/*
Copyright ApeCloud, Inc.

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

package util

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	ComponentStatusDefaultPodName = "Unknown"
)

// GetClusterByObject gets cluster by related k8s workloads.
func GetClusterByObject(ctx context.Context,
	cli client.Client,
	obj client.Object) (*appsv1alpha1.Cluster, error) {
	labels := obj.GetLabels()
	if labels == nil {
		return nil, nil
	}
	cluster := &appsv1alpha1.Cluster{}
	if err := cli.Get(ctx, client.ObjectKey{
		Name:      labels[constant.AppInstanceLabelKey],
		Namespace: obj.GetNamespace(),
	}, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}

// IsCompleted checks whether the component has completed the operation
func IsCompleted(phase appsv1alpha1.Phase) bool {
	return slices.Index([]appsv1alpha1.Phase{appsv1alpha1.RunningPhase, appsv1alpha1.FailedPhase,
		appsv1alpha1.AbnormalPhase, appsv1alpha1.StoppedPhase}, phase) != -1
}

func IsFailedOrAbnormal(phase appsv1alpha1.Phase) bool {
	return slices.Index([]appsv1alpha1.Phase{appsv1alpha1.FailedPhase, appsv1alpha1.AbnormalPhase}, phase) != -1
}

// GetComponentMatchLabels gets the labels for matching the cluster component
func GetComponentMatchLabels(clusterName, componentName string) client.ListOption {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.AppManagedByLabelKey:   constant.AppName,
	}
}

// GetComponentPodList gets the pod list by cluster and componentName
func GetComponentPodList(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, componentName string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace),
		GetComponentMatchLabels(cluster.Name, componentName))
	return podList, err
}

func GetComponentStatusMessageKey(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

// IsProbeTimeout checks if the application of the pod is probe timed out.
func IsProbeTimeout(componentDef *appsv1alpha1.ClusterComponentDefinition, podsReadyTime *metav1.Time) bool {
	if podsReadyTime == nil {
		return false
	}
	probes := componentDef.Probes
	if probes == nil || probes.RoleChangedProbe == nil {
		return false
	}
	roleProbeTimeout := time.Duration(appsv1alpha1.DefaultRoleProbeTimeoutAfterPodsReady) * time.Second
	if probes.RoleProbeTimeoutAfterPodsReady != 0 {
		roleProbeTimeout = time.Duration(probes.RoleProbeTimeoutAfterPodsReady) * time.Second
	}
	return time.Now().After(podsReadyTime.Add(roleProbeTimeout))
}

func GetComponentPhase(isFailed, isAbnormal bool) appsv1alpha1.Phase {
	var componentPhase appsv1alpha1.Phase
	if isFailed {
		componentPhase = appsv1alpha1.FailedPhase
	} else if isAbnormal {
		componentPhase = appsv1alpha1.AbnormalPhase
	}
	return componentPhase
}

// GetObjectListByComponentName gets k8s workload list with component
func GetObjectListByComponentName(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, objectList client.ObjectList, componentName string) error {
	matchLabels := GetComponentMatchLabels(cluster.Name, componentName)
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, matchLabels, inNamespace)
}

// GetComponentDefByCluster gets component from ClusterDefinition with compDefName
func GetComponentDefByCluster(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, compDefName string) (*appsv1alpha1.ClusterComponentDefinition, error) {
	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return nil, err
	}

	for _, component := range clusterDef.Spec.ComponentDefs {
		if component.Name == compDefName {
			return &component, nil
		}
	}
	return nil, nil
}

// GetClusterComponentSpecByName gets componentSpec from cluster with compSpecName.
func GetClusterComponentSpecByName(cluster *appsv1alpha1.Cluster, compSpecName string) *appsv1alpha1.ClusterComponentSpec {
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if compSpec.Name == compSpecName {
			return &compSpec
		}
	}
	return nil
}

// InitClusterComponentStatusIfNeed Initialize the state of the corresponding component in cluster.status.components
func InitClusterComponentStatusIfNeed(cluster *appsv1alpha1.Cluster,
	componentName string,
	componentDef *appsv1alpha1.ClusterComponentDefinition) {
	if componentDef == nil {
		return
	}
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]appsv1alpha1.ClusterComponentStatus)
	}
	if _, ok := cluster.Status.Components[componentName]; !ok {
		cluster.Status.Components[componentName] = appsv1alpha1.ClusterComponentStatus{
			Phase: cluster.Status.Phase,
		}
	}
	componentStatus := cluster.Status.Components[componentName]
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus:
		if componentStatus.ConsensusSetStatus != nil {
			break
		}
		componentStatus.ConsensusSetStatus = &appsv1alpha1.ConsensusSetStatus{
			Leader: appsv1alpha1.ConsensusMemberStatus{
				Pod:        ComponentStatusDefaultPodName,
				AccessMode: appsv1alpha1.None,
				Name:       "",
			},
		}
	case appsv1alpha1.Replication:
		if componentStatus.ReplicationSetStatus != nil {
			break
		}
		componentStatus.ReplicationSetStatus = &appsv1alpha1.ReplicationSetStatus{
			Primary: appsv1alpha1.ReplicationMemberStatus{
				Pod: ComponentStatusDefaultPodName,
			},
		}
	}
	cluster.Status.Components[componentName] = componentStatus
}

// GetComponentDeployMinReadySeconds gets the deployment minReadySeconds of the component.
func GetComponentDeployMinReadySeconds(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentName string) (minReadySeconds int32, err error) {
	deployList := &appsv1.DeploymentList{}
	if err = GetObjectListByComponentName(ctx, cli, cluster, deployList, componentName); err != nil {
		return
	}
	if len(deployList.Items) > 0 {
		minReadySeconds = deployList.Items[0].Spec.MinReadySeconds
		return
	}
	return minReadySeconds, err
}

// GetComponentStsMinReadySeconds gets the statefulSet minReadySeconds of the component.
func GetComponentStsMinReadySeconds(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentName string) (minReadySeconds int32, err error) {
	stsList := &appsv1.StatefulSetList{}
	if err = GetObjectListByComponentName(ctx, cli, cluster, stsList, componentName); err != nil {
		return
	}
	if len(stsList.Items) > 0 {
		minReadySeconds = stsList.Items[0].Spec.MinReadySeconds
		return
	}
	return minReadySeconds, err
}

// GetComponentWorkloadMinReadySeconds gets the workload minReadySeconds of the component.
func GetComponentWorkloadMinReadySeconds(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	workloadType appsv1alpha1.WorkloadType,
	componentName string) (minReadySeconds int32, err error) {
	switch workloadType {
	case appsv1alpha1.Stateless:
		return GetComponentDeployMinReadySeconds(ctx, cli, cluster, componentName)
	default:
		return GetComponentStsMinReadySeconds(ctx, cli, cluster, componentName)
	}
}

// GetComponentInfoByPod gets componentName and componentDefinition info by Pod.
func GetComponentInfoByPod(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	pod *corev1.Pod) (componentName string, componentDef *appsv1alpha1.ClusterComponentDefinition, err error) {
	if pod == nil || pod.Labels == nil {
		return "", nil, fmt.Errorf("pod %s or pod's label is nil", pod.Name)
	}
	componentName, ok := pod.Labels[constant.KBAppComponentLabelKey]
	if !ok {
		return "", nil, fmt.Errorf("pod %s component name label %s is nil", pod.Name, constant.KBAppComponentLabelKey)
	}
	compDefName := cluster.GetComponentDefRefName(componentName)
	componentDef, err = GetComponentDefByCluster(ctx, cli, cluster, compDefName)
	if err != nil {
		return componentName, componentDef, err
	}
	return componentName, componentDef, nil
}

// GetCompRelatedObjectList gets the related pods and workloads of the component
func GetCompRelatedObjectList(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	compName string,
	relatedWorkloads client.ObjectList) (*corev1.PodList, error) {
	podList, err := GetComponentPodList(ctx, cli, cluster, compName)
	if err != nil {
		return nil, err
	}
	if err = GetObjectListByComponentName(ctx,
		cli, cluster, relatedWorkloads, compName); err != nil {
		return nil, err
	}
	return podList, nil
}

// AvailableReplicasAreConsistent checks if expected replicas number of component is consistent with
// the number of available workload replicas.
func AvailableReplicasAreConsistent(componentReplicas, podCount, workloadAvailableReplicas int32) bool {
	return workloadAvailableReplicas == componentReplicas && componentReplicas == podCount
}

// GetPhaseWithNoAvailableReplicas gets the component phase when the workload of component has no available replicas.
func GetPhaseWithNoAvailableReplicas(componentReplicas int32) appsv1alpha1.Phase {
	if componentReplicas == 0 {
		return ""
	}
	return appsv1alpha1.FailedPhase
}

// GetComponentPhaseWhenPodsNotReady gets the component phase when pods of component are not ready.
func GetComponentPhaseWhenPodsNotReady(podList *corev1.PodList,
	workload metav1.Object,
	componentReplicas,
	availableReplicas int32,
	checkFailedPodRevision func(pod *corev1.Pod, workload metav1.Object) bool) appsv1alpha1.Phase {
	podCount := len(podList.Items)
	if podCount == 0 || availableReplicas == 0 {
		return GetPhaseWithNoAvailableReplicas(componentReplicas)
	}
	var existLatestRevisionFailedPod bool
	for _, v := range podList.Items {
		// if the pod is terminating, ignore it
		if v.DeletionTimestamp != nil {
			return ""
		}
		if checkFailedPodRevision != nil && checkFailedPodRevision(&v, workload) {
			existLatestRevisionFailedPod = true
		}
	}
	return GetCompPhaseByConditions(existLatestRevisionFailedPod, true,
		componentReplicas, int32(podCount), availableReplicas)
}

// GetCompPhaseByConditions gets the component phase according to the following conditions:
// 1. if the failed pod is not controlled by the latest revision, ignore it.
// 2. if the primary replicas are not available, the component is failed.
// 3. finally if expected replicas number of component is inconsistent with
// the number of available workload replicas, the component is abnormal.
func GetCompPhaseByConditions(existLatestRevisionFailedPod bool,
	primaryReplicasAvailable bool,
	compReplicas,
	podCount,
	availableReplicas int32) appsv1alpha1.Phase {
	// if the failed pod is not controlled by the latest revision, ignore it.
	if !existLatestRevisionFailedPod {
		return ""
	}
	if !primaryReplicasAvailable {
		return appsv1alpha1.FailedPhase
	}
	// checks if expected replicas number of component is consistent with the number of available workload replicas.
	if !AvailableReplicasAreConsistent(compReplicas, podCount, availableReplicas) {
		return appsv1alpha1.AbnormalPhase
	}
	return ""
}
