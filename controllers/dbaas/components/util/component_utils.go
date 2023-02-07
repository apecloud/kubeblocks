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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	ComponentStatusDefaultPodName = "Unknown"
)

// GetClusterByObject gets cluster by related k8s workloads.
func GetClusterByObject(ctx context.Context,
	cli client.Client,
	obj client.Object) (*dbaasv1alpha1.Cluster, error) {
	labels := obj.GetLabels()
	if labels == nil {
		return nil, nil
	}
	cluster := &dbaasv1alpha1.Cluster{}
	if err := cli.Get(ctx, client.ObjectKey{
		Name:      labels[intctrlutil.AppInstanceLabelKey],
		Namespace: obj.GetNamespace(),
	}, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}

// IsCompleted checks whether the component has completed the operation
func IsCompleted(phase dbaasv1alpha1.Phase) bool {
	return slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase, dbaasv1alpha1.AbnormalPhase}, phase) != -1
}

func IsFailedOrAbnormal(phase dbaasv1alpha1.Phase) bool {
	return slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.FailedPhase, dbaasv1alpha1.AbnormalPhase}, phase) != -1
}

// GetComponentMatchLabels gets the labels for matching the cluster component
func GetComponentMatchLabels(clusterName, componentName string) client.ListOption {
	return client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey:  clusterName,
		intctrlutil.AppComponentLabelKey: componentName,
		intctrlutil.AppManagedByLabelKey: intctrlutil.AppName,
	}
}

// GetComponentPodList gets the pod list by cluster and componentName
func GetComponentPodList(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, componentName string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace),
		GetComponentMatchLabels(cluster.Name, componentName))
	return podList, err
}

func GetStatusComponentMessageKey(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

// IsProbeTimeout checks if the application of the pod is probe timed out.
func IsProbeTimeout(componentDef *dbaasv1alpha1.ClusterDefinitionComponent, podsReadyTime *metav1.Time) bool {
	if podsReadyTime == nil {
		return false
	}
	probes := componentDef.Probes
	if probes == nil || probes.RoleChangedProbe == nil {
		return false
	}
	roleProbeTimeout := time.Duration(dbaasv1alpha1.DefaultRoleProbeTimeoutAfterPodsReady) * time.Second
	if probes.RoleProbeTimeoutAfterPodsReady != 0 {
		roleProbeTimeout = time.Duration(probes.RoleProbeTimeoutAfterPodsReady) * time.Second
	}
	return time.Now().After(podsReadyTime.Add(roleProbeTimeout))
}

func GetComponentPhase(isFailed, isAbnormal bool) dbaasv1alpha1.Phase {
	var componentPhase dbaasv1alpha1.Phase
	if isFailed {
		componentPhase = dbaasv1alpha1.FailedPhase
	} else if isAbnormal {
		componentPhase = dbaasv1alpha1.AbnormalPhase
	}
	return componentPhase
}

// GetObjectListByComponentName gets k8s workload list with component
func GetObjectListByComponentName(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, objectList client.ObjectList, componentName string) error {
	matchLabels := GetComponentMatchLabels(cluster.Name, componentName)
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, matchLabels, inNamespace)
}

// CheckRelatedPodIsTerminating checks related pods is terminating for Stateless/Stateful
func CheckRelatedPodIsTerminating(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, componentName string) (bool, error) {
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace),
		GetComponentMatchLabels(cluster.Name, componentName)); err != nil {
		return false, err
	}
	for _, v := range podList.Items {
		// if the pod is terminating, ignore the warning event
		if v.DeletionTimestamp != nil {
			return true, nil
		}
	}
	return false, nil
}

// GetComponentDefByCluster gets component from ClusterDefinition with typeName
func GetComponentDefByCluster(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, typeName string) (*dbaasv1alpha1.ClusterDefinitionComponent, error) {
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return nil, err
	}

	for _, component := range clusterDef.Spec.Components {
		if component.TypeName == typeName {
			return &component, nil
		}
	}
	return nil, nil
}

// InitClusterComponentStatusIfNeed Initialize the state of the corresponding component in cluster.status.components
func InitClusterComponentStatusIfNeed(cluster *dbaasv1alpha1.Cluster,
	componentName string,
	componentDef *dbaasv1alpha1.ClusterDefinitionComponent) {
	if componentDef == nil {
		return
	}
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]dbaasv1alpha1.ClusterStatusComponent)
	}
	if _, ok := cluster.Status.Components[componentName]; !ok {
		typeName := cluster.GetComponentTypeName(componentName)

		cluster.Status.Components[componentName] = dbaasv1alpha1.ClusterStatusComponent{
			Type:  typeName,
			Phase: cluster.Status.Phase,
		}
	}
	componentStatus := cluster.Status.Components[componentName]
	if componentDef.ComponentType == dbaasv1alpha1.Consensus && componentStatus.ConsensusSetStatus == nil {
		componentStatus.ConsensusSetStatus = &dbaasv1alpha1.ConsensusSetStatus{
			Leader: dbaasv1alpha1.ConsensusMemberStatus{
				Pod:        ComponentStatusDefaultPodName,
				AccessMode: dbaasv1alpha1.None,
				Name:       "",
			},
		}
	}
	if componentDef.ComponentType == dbaasv1alpha1.Replication && componentStatus.ReplicationSetStatus == nil {
		componentStatus.ReplicationSetStatus = &dbaasv1alpha1.ReplicationSetStatus{
			Primary: dbaasv1alpha1.ReplicationMemberStatus{
				Pod: ComponentStatusDefaultPodName,
			},
		}
	}
	cluster.Status.Components[componentName] = componentStatus
}

// GetComponentReplicas gets the actual replicas of component
func GetComponentReplicas(component *dbaasv1alpha1.ClusterComponent,
	componentDef *dbaasv1alpha1.ClusterDefinitionComponent) int32 {
	replicas := componentDef.DefaultReplicas
	if component.Replicas != nil {
		replicas = *component.Replicas
	}
	return replicas
}

// GetComponentDeployMinReadySeconds gets the deployment minReadySeconds of the component.
func GetComponentDeployMinReadySeconds(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
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
	cluster *dbaasv1alpha1.Cluster,
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
	cluster *dbaasv1alpha1.Cluster,
	componentType dbaasv1alpha1.ComponentType,
	componentName string) (minReadySeconds int32, err error) {
	switch componentType {
	case dbaasv1alpha1.Stateless:
		return GetComponentDeployMinReadySeconds(ctx, cli, cluster, componentName)
	default:
		return GetComponentStsMinReadySeconds(ctx, cli, cluster, componentName)
	}
}

// GetComponentDefaultReplicas gets component default replicas.
func GetComponentDefaultReplicas(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	componentName string) (int32, error) {
	typeName := cluster.GetComponentTypeName(componentName)
	component, err := GetComponentDefByCluster(ctx, cli, cluster, typeName)
	if err != nil || component == nil {
		return -1, err
	}
	return component.DefaultReplicas, nil
}

// GetComponentInfoByPod gets componentName and componentDefinition info by Pod.
func GetComponentInfoByPod(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	pod *corev1.Pod) (componentName string, componentDef *dbaasv1alpha1.ClusterDefinitionComponent, err error) {
	if pod == nil || pod.Labels == nil {
		return "", nil, fmt.Errorf("pod %s or pod's label is nil", pod.Name)
	}
	componentName, ok := pod.Labels[intctrlutil.AppComponentLabelKey]
	if !ok {
		return "", nil, fmt.Errorf("pod %s component name label %s is nil", pod.Name, intctrlutil.AppComponentLabelKey)
	}
	typeName := cluster.GetComponentTypeName(componentName)
	componentDef, err = GetComponentDefByCluster(ctx, cli, cluster, typeName)
	if err != nil {
		return componentName, componentDef, err
	}
	return componentName, componentDef, nil
}
