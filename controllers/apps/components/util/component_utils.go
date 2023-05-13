/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package util

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	client2 "github.com/apecloud/kubeblocks/internal/controller/client"
	componentutil "github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/generics"
)

const (
	ComponentStatusDefaultPodName = "Unknown"
)

var (
	ErrReqCtrlClient              = errors.New("required arg client.Client is nil")
	ErrReqClusterObj              = errors.New("required arg *appsv1alpha1.Cluster is nil")
	ErrReqClusterComponentDefObj  = errors.New("required arg *appsv1alpha1.ClusterComponentDefinition is nil")
	ErrReqClusterComponentSpecObj = errors.New("required arg *appsv1alpha1.ClusterComponentSpec is nil")
)

func ComponentRuntimeReqArgsCheck(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec) error {
	if cli == nil {
		return ErrReqCtrlClient
	}
	if cluster == nil {
		return ErrReqCtrlClient
	}
	if component == nil {
		return ErrReqClusterComponentSpecObj
	}
	return nil
}

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

func IsFailedOrAbnormal(phase appsv1alpha1.ClusterComponentPhase) bool {
	return slices.Index([]appsv1alpha1.ClusterComponentPhase{
		appsv1alpha1.FailedClusterCompPhase,
		appsv1alpha1.AbnormalClusterCompPhase}, phase) != -1
}

// GetComponentMatchLabels gets the labels for matching the cluster component
func GetComponentMatchLabels(clusterName, componentName string) map[string]string {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.AppManagedByLabelKey:   constant.AppName,
	}
}

// GetComponentPodList gets the pod list by cluster and componentName
func GetComponentPodList(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, componentName string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace),
		client.MatchingLabels(GetComponentMatchLabels(cluster.Name, componentName)))
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
	if probes == nil || probes.RoleProbe == nil {
		return false
	}
	roleProbeTimeout := time.Duration(appsv1alpha1.DefaultRoleProbeTimeoutAfterPodsReady) * time.Second
	if probes.RoleProbeTimeoutAfterPodsReady != 0 {
		roleProbeTimeout = time.Duration(probes.RoleProbeTimeoutAfterPodsReady) * time.Second
	}
	return time.Now().After(podsReadyTime.Add(roleProbeTimeout))
}

func GetComponentPhase(isFailed, isAbnormal bool) appsv1alpha1.ClusterComponentPhase {
	var componentPhase appsv1alpha1.ClusterComponentPhase
	if isFailed {
		componentPhase = appsv1alpha1.FailedClusterCompPhase
	} else if isAbnormal {
		componentPhase = appsv1alpha1.AbnormalClusterCompPhase
	}
	return componentPhase
}

// GetObjectListByComponentName gets k8s workload list with component
func GetObjectListByComponentName(ctx context.Context, cli client2.ReadonlyClient, cluster appsv1alpha1.Cluster,
	objectList client.ObjectList, componentName string) error {
	matchLabels := GetComponentMatchLabels(cluster.Name, componentName)
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, client.MatchingLabels(matchLabels), inNamespace)
}

// GetObjectListByCustomLabels gets k8s workload list with custom labels
func GetObjectListByCustomLabels(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster,
	objectList client.ObjectList, matchLabels client.ListOption) error {
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, matchLabels, inNamespace)
}

// GetComponentDefByCluster gets component from ClusterDefinition with compDefName
func GetComponentDefByCluster(ctx context.Context, cli client2.ReadonlyClient, cluster appsv1alpha1.Cluster,
	compDefName string) (*appsv1alpha1.ClusterComponentDefinition, error) {
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
func GetClusterComponentSpecByName(cluster appsv1alpha1.Cluster, compSpecName string) *appsv1alpha1.ClusterComponentSpec {
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if compSpec.Name == compSpecName {
			return &compSpec
		}
	}
	return nil
}

// InitClusterComponentStatusIfNeed Initialize the state of the corresponding component in cluster.status.components
func InitClusterComponentStatusIfNeed(
	cluster *appsv1alpha1.Cluster,
	componentName string,
	componentDef appsv1alpha1.ClusterComponentDefinition) error {
	if cluster == nil {
		return ErrReqClusterObj
	}

	// REVIEW: should have following removed
	// if _, ok := cluster.Status.Components[componentName]; !ok {
	// 	cluster.Status.SetComponentStatus(componentName, appsv1alpha1.ClusterComponentStatus{
	// 		Phase: cluster.Status.Phase,
	// 	})
	// }
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
	cluster.Status.SetComponentStatus(componentName, componentStatus)
	return nil
}

// GetComponentDeployMinReadySeconds gets the deployment minReadySeconds of the component.
func GetComponentDeployMinReadySeconds(ctx context.Context,
	cli client.Client,
	cluster appsv1alpha1.Cluster,
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
	cluster appsv1alpha1.Cluster,
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
	cluster appsv1alpha1.Cluster,
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
	cluster appsv1alpha1.Cluster,
	pod *corev1.Pod) (componentName string, componentDef *appsv1alpha1.ClusterComponentDefinition, err error) {
	if pod == nil || pod.Labels == nil {
		return "", nil, errors.New("pod or pod's label is nil")
	}
	componentName, ok := pod.Labels[constant.KBAppComponentLabelKey]
	if !ok {
		return "", nil, errors.New("pod component name label is nil")
	}
	compDefName := cluster.Spec.GetComponentDefRefName(componentName)
	componentDef, err = GetComponentDefByCluster(ctx, cli, cluster, compDefName)
	if err != nil {
		return componentName, componentDef, err
	}
	return componentName, componentDef, nil
}

// GetCompRelatedObjectList gets the related pods and workloads of the component
func GetCompRelatedObjectList(ctx context.Context,
	cli client.Client,
	cluster appsv1alpha1.Cluster,
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
func GetPhaseWithNoAvailableReplicas(componentReplicas int32) appsv1alpha1.ClusterComponentPhase {
	if componentReplicas == 0 {
		return ""
	}
	return appsv1alpha1.FailedClusterCompPhase
}

// GetComponentPhaseWhenPodsNotReady gets the component phase when pods of component are not ready.
func GetComponentPhaseWhenPodsNotReady(podList *corev1.PodList,
	workload metav1.Object,
	componentReplicas,
	availableReplicas int32,
	checkFailedPodRevision func(pod *corev1.Pod, workload metav1.Object) bool) appsv1alpha1.ClusterComponentPhase {
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
	availableReplicas int32) appsv1alpha1.ClusterComponentPhase {
	// if the failed pod is not controlled by the latest revision, ignore it.
	if !existLatestRevisionFailedPod {
		return ""
	}
	if !primaryReplicasAvailable {
		return appsv1alpha1.FailedClusterCompPhase
	}
	// checks if expected replicas number of component is consistent with the number of available workload replicas.
	if !AvailableReplicasAreConsistent(compReplicas, podCount, availableReplicas) {
		return appsv1alpha1.AbnormalClusterCompPhase
	}
	return ""
}

// UpdateObjLabel updates the value of the role label of the object.
func UpdateObjLabel[T generics.Object, PT generics.PObject[T]](
	ctx context.Context, cli client.Client, obj T, labelKey, labelValue string) error {
	pObj := PT(&obj)
	patch := client.MergeFrom(PT(pObj.DeepCopy()))
	if v, ok := pObj.GetLabels()[labelKey]; ok && v == labelValue {
		return nil
	}
	pObj.GetLabels()[labelKey] = labelValue
	if err := cli.Patch(ctx, pObj, patch); err != nil {
		return err
	}
	return nil
}

// PatchGVRCustomLabels patches the custom labels to the object list of the specified GVK.
func PatchGVRCustomLabels(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster,
	resource appsv1alpha1.GVKResource, componentName, labelKey, labelValue string) error {
	gvk, err := ParseCustomLabelPattern(resource.GVK)
	if err != nil {
		return err
	}
	if !slices.Contains(getCustomLabelSupportKind(), gvk.Kind) {
		return errors.New("kind is not supported for custom labels")
	}

	objectList := getObjectListMapOfResourceKind()[gvk.Kind]
	matchLabels := GetComponentMatchLabels(cluster.Name, componentName)
	for k, v := range resource.Selector {
		matchLabels[k] = v
	}
	if err := GetObjectListByCustomLabels(ctx, cli, *cluster, objectList, client.MatchingLabels(matchLabels)); err != nil {
		return err
	}
	labelKey = replaceKBEnvPlaceholderTokens(cluster, componentName, labelKey)
	labelValue = replaceKBEnvPlaceholderTokens(cluster, componentName, labelValue)
	switch gvk.Kind {
	case constant.StatefulSetKind:
		stsList := objectList.(*appsv1.StatefulSetList)
		for _, sts := range stsList.Items {
			if err := UpdateObjLabel(ctx, cli, sts, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.DeploymentKind:
		deployList := objectList.(*appsv1.DeploymentList)
		for _, deploy := range deployList.Items {
			if err := UpdateObjLabel(ctx, cli, deploy, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.PodKind:
		podList := objectList.(*corev1.PodList)
		for _, pod := range podList.Items {
			if err := UpdateObjLabel(ctx, cli, pod, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.ServiceKind:
		svcList := objectList.(*corev1.ServiceList)
		for _, svc := range svcList.Items {
			if err := UpdateObjLabel(ctx, cli, svc, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.ConfigMapKind:
		cmList := objectList.(*corev1.ConfigMapList)
		for _, cm := range cmList.Items {
			if err := UpdateObjLabel(ctx, cli, cm, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.CronJobKind:
		cjList := objectList.(*batchv1.CronJobList)
		for _, cj := range cjList.Items {
			if err := UpdateObjLabel(ctx, cli, cj, labelKey, labelValue); err != nil {
				return err
			}
		}
	}
	return nil
}

// ParseCustomLabelPattern parses the custom label pattern to GroupVersionKind.
func ParseCustomLabelPattern(pattern string) (schema.GroupVersionKind, error) {
	patterns := strings.Split(pattern, "/")
	switch len(patterns) {
	case 2:
		return schema.GroupVersionKind{
			Group:   "",
			Version: patterns[0],
			Kind:    patterns[1],
		}, nil
	case 3:
		return schema.GroupVersionKind{
			Group:   patterns[0],
			Version: patterns[1],
			Kind:    patterns[2],
		}, nil
	}
	return schema.GroupVersionKind{}, fmt.Errorf("invalid pattern %s", pattern)
}

// getCustomLabelSupportKind returns the kinds that support custom label.
func getCustomLabelSupportKind() []string {
	return []string{
		constant.CronJobKind,
		constant.StatefulSetKind,
		constant.DeploymentKind,
		constant.ReplicaSetKind,
		constant.ServiceKind,
		constant.ConfigMapKind,
		constant.PodKind,
	}
}

// GetCustomLabelWorkloadKind returns the kinds that support custom label.
func GetCustomLabelWorkloadKind() []string {
	return []string{
		constant.CronJobKind,
		constant.StatefulSetKind,
		constant.DeploymentKind,
		constant.ReplicaSetKind,
		constant.PodKind,
	}
}

// getObjectListMapOfResourceKind returns the mapping of resource kind and its object list.
func getObjectListMapOfResourceKind() map[string]client.ObjectList {
	return map[string]client.ObjectList{
		constant.CronJobKind:     &batchv1.CronJobList{},
		constant.StatefulSetKind: &appsv1.StatefulSetList{},
		constant.DeploymentKind:  &appsv1.DeploymentList{},
		constant.ReplicaSetKind:  &appsv1.ReplicaSetList{},
		constant.ServiceKind:     &corev1.ServiceList{},
		constant.ConfigMapKind:   &corev1.ConfigMapList{},
		constant.PodKind:         &corev1.PodList{},
	}
}

// replaceKBEnvPlaceholderTokens replaces the placeholder tokens in the string strToReplace with builtInEnvMap and return new string.
func replaceKBEnvPlaceholderTokens(cluster *appsv1alpha1.Cluster, componentName, strToReplace string) string {
	builtInEnvMap := componentutil.GetReplacementMapForBuiltInEnv(cluster, componentName)
	return componentutil.ReplaceNamedVars(builtInEnvMap, strToReplace, -1, true)
}
