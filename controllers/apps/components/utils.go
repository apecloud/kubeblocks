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

package components

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
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
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

var (
	errReqClusterObj = errors.New("required arg *appsv1alpha1.Cluster is nil")
)

func listObjWithLabelsInNamespace[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](
	ctx context.Context, cli client.Client, _ func(T, L), namespace string, labels client.MatchingLabels) ([]PT, error) {
	var objList L
	if err := cli.List(ctx, PL(&objList), labels, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	objs := make([]PT, 0)
	items := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
	for i := range items {
		objs = append(objs, &items[i])
	}
	return objs, nil
}

func listStsOwnedByComponent(ctx context.Context, cli client.Client, namespace string, labels client.MatchingLabels) ([]*appsv1.StatefulSet, error) {
	return listObjWithLabelsInNamespace(ctx, cli, generics.StatefulSetSignature, namespace, labels)
}

func listDeployOwnedByComponent(ctx context.Context, cli client.Client, namespace string, labels client.MatchingLabels) ([]*appsv1.Deployment, error) {
	return listObjWithLabelsInNamespace(ctx, cli, generics.DeploymentSignature, namespace, labels)
}

func listPodOwnedByComponent(ctx context.Context, cli client.Client, namespace string, labels client.MatchingLabels) ([]*corev1.Pod, error) {
	return listObjWithLabelsInNamespace(ctx, cli, generics.PodSignature, namespace, labels)
}

// restartPod restarts a Pod through updating the pod's annotation
func restartPod(podTemplate *corev1.PodTemplateSpec) error {
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = map[string]string{}
	}

	startTimestamp := time.Now() // TODO(impl): opsRes.OpsRequest.Status.StartTimestamp
	restartTimestamp := podTemplate.Annotations[constant.RestartAnnotationKey]
	// if res, _ := time.Parse(time.RFC3339, restartTimestamp); startTimestamp.After(res) {
	if res, _ := time.Parse(time.RFC3339, restartTimestamp); startTimestamp.Before(res) {
		podTemplate.Annotations[constant.RestartAnnotationKey] = startTimestamp.Format(time.RFC3339)
	}
	return nil
}

// mergeAnnotations keeps the original annotations.
// if annotations exist and are replaced, the Deployment/StatefulSet will be updated.
func mergeAnnotations(originalAnnotations map[string]string, targetAnnotations *map[string]string) {
	if targetAnnotations == nil || originalAnnotations == nil {
		return
	}
	if *targetAnnotations == nil {
		*targetAnnotations = map[string]string{}
	}
	for k, v := range originalAnnotations {
		// if the annotation not exist in targetAnnotations, copy it from original.
		if _, ok := (*targetAnnotations)[k]; !ok {
			(*targetAnnotations)[k] = v
		}
	}
}

// buildWorkLoadAnnotations builds the annotations for Deployment/StatefulSet
func buildWorkLoadAnnotations(obj client.Object, cluster *appsv1alpha1.Cluster) {
	workloadAnnotations := obj.GetAnnotations()
	if workloadAnnotations == nil {
		workloadAnnotations = map[string]string{}
	}
	// record the cluster generation to check if the sts is latest
	workloadAnnotations[constant.KubeBlocksGenerationKey] = strconv.FormatInt(cluster.Generation, 10)
	obj.SetAnnotations(workloadAnnotations)
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

// getComponentMatchLabels gets the labels for matching the cluster component
func getComponentMatchLabels(clusterName, componentName string) map[string]string {
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
		client.MatchingLabels(getComponentMatchLabels(cluster.Name, componentName)))
	return podList, err
}

// GetComponentPodListWithRole gets the pod list with target role by cluster and componentName
func GetComponentPodListWithRole(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, compSpecName, role string) (*corev1.PodList, error) {
	matchLabels := client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.KBAppComponentLabelKey: compSpecName,
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.RoleLabelKey:           role,
	}
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace), matchLabels); err != nil {
		return nil, err
	}
	return podList, nil
}

// isProbeTimeout checks if the application of the pod is probe timed out.
func isProbeTimeout(probes *appsv1alpha1.ClusterDefinitionProbes, podsReadyTime *metav1.Time) bool {
	if podsReadyTime == nil {
		return false
	}
	if probes == nil || probes.RoleProbe == nil {
		return false
	}
	roleProbeTimeout := time.Duration(appsv1alpha1.DefaultRoleProbeTimeoutAfterPodsReady) * time.Second
	if probes.RoleProbeTimeoutAfterPodsReady != 0 {
		roleProbeTimeout = time.Duration(probes.RoleProbeTimeoutAfterPodsReady) * time.Second
	}
	return time.Now().After(podsReadyTime.Add(roleProbeTimeout))
}

// getObjectListByComponentName gets k8s workload list with component
func getObjectListByComponentName(ctx context.Context, cli client2.ReadonlyClient, cluster appsv1alpha1.Cluster,
	objectList client.ObjectList, componentName string) error {
	matchLabels := getComponentMatchLabels(cluster.Name, componentName)
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, client.MatchingLabels(matchLabels), inNamespace)
}

// getObjectListByCustomLabels gets k8s workload list with custom labels
func getObjectListByCustomLabels(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster,
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

// getClusterComponentSpecByName gets componentSpec from cluster with compSpecName.
func getClusterComponentSpecByName(cluster appsv1alpha1.Cluster, compSpecName string) *appsv1alpha1.ClusterComponentSpec {
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if compSpec.Name == compSpecName {
			return &compSpec
		}
	}
	return nil
}

// initClusterComponentStatusIfNeed Initialize the state of the corresponding component in cluster.status.components
func initClusterComponentStatusIfNeed(
	cluster *appsv1alpha1.Cluster,
	componentName string,
	workloadType appsv1alpha1.WorkloadType) error {
	if cluster == nil {
		return errReqClusterObj
	}

	// REVIEW: should have following removed
	// if _, ok := cluster.Status.Components[componentName]; !ok {
	// 	cluster.Status.SetComponentStatus(componentName, appsv1alpha1.ClusterComponentStatus{
	// 		Phase: cluster.Status.Phase,
	// 	})
	// }
	componentStatus := cluster.Status.Components[componentName]
	switch workloadType {
	case appsv1alpha1.Consensus:
		if componentStatus.ConsensusSetStatus != nil {
			break
		}
		componentStatus.ConsensusSetStatus = &appsv1alpha1.ConsensusSetStatus{
			Leader: appsv1alpha1.ConsensusMemberStatus{
				Pod:        constant.ComponentStatusDefaultPodName,
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
				Pod: constant.ComponentStatusDefaultPodName,
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
	if err = getObjectListByComponentName(ctx, cli, cluster, deployList, componentName); err != nil {
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
	if err = getObjectListByComponentName(ctx, cli, cluster, stsList, componentName); err != nil {
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
	// if no componentSpec found, then componentName is componentDefName
	if len(compDefName) == 0 && len(cluster.Spec.ComponentSpecs) == 0 {
		compDefName = componentName
	}
	componentDef, err = GetComponentDefByCluster(ctx, cli, cluster, compDefName)
	if err != nil {
		return componentName, componentDef, err
	}
	return componentName, componentDef, nil
}

// getCompRelatedObjectList gets the related pods and workloads of the component
func getCompRelatedObjectList(ctx context.Context,
	cli client.Client,
	cluster appsv1alpha1.Cluster,
	compName string,
	relatedWorkloads client.ObjectList) (*corev1.PodList, error) {
	podList, err := GetComponentPodList(ctx, cli, cluster, compName)
	if err != nil {
		return nil, err
	}
	if err = getObjectListByComponentName(ctx,
		cli, cluster, relatedWorkloads, compName); err != nil {
		return nil, err
	}
	return podList, nil
}

// availableReplicasAreConsistent checks if expected replicas number of component is consistent with
// the number of available workload replicas.
func availableReplicasAreConsistent(componentReplicas, podCount, workloadAvailableReplicas int32) bool {
	return workloadAvailableReplicas == componentReplicas && componentReplicas == podCount
}

// getPhaseWithNoAvailableReplicas gets the component phase when the workload of component has no available replicas.
func getPhaseWithNoAvailableReplicas(componentReplicas int32) appsv1alpha1.ClusterComponentPhase {
	if componentReplicas == 0 {
		return ""
	}
	return appsv1alpha1.FailedClusterCompPhase
}

// getComponentPhaseWhenPodsNotReady gets the component phase when pods of component are not ready.
func getComponentPhaseWhenPodsNotReady(podList *corev1.PodList,
	workload metav1.Object,
	componentReplicas,
	availableReplicas int32,
	checkFailedPodRevision func(pod *corev1.Pod, workload metav1.Object) bool) appsv1alpha1.ClusterComponentPhase {
	podCount := len(podList.Items)
	if podCount == 0 || availableReplicas == 0 {
		return getPhaseWithNoAvailableReplicas(componentReplicas)
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
	return getCompPhaseByConditions(existLatestRevisionFailedPod, true,
		componentReplicas, int32(podCount), availableReplicas)
}

// getCompPhaseByConditions gets the component phase according to the following conditions:
// 1. if the failed pod is not controlled by the latest revision, ignore it.
// 2. if the primary replicas are not available, the component is failed.
// 3. finally if expected replicas number of component is inconsistent with
// the number of available workload replicas, the component is abnormal.
func getCompPhaseByConditions(existLatestRevisionFailedPod bool,
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
	if !availableReplicasAreConsistent(compReplicas, podCount, availableReplicas) {
		return appsv1alpha1.AbnormalClusterCompPhase
	}
	return ""
}

// updateObjLabel updates the value of the role label of the object.
func updateObjLabel[T generics.Object, PT generics.PObject[T]](
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

// patchGVRCustomLabels patches the custom labels to the object list of the specified GVK.
func patchGVRCustomLabels(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster,
	resource appsv1alpha1.GVKResource, componentName, labelKey, labelValue string) error {
	gvk, err := parseCustomLabelPattern(resource.GVK)
	if err != nil {
		return err
	}
	if !slices.Contains(getCustomLabelSupportKind(), gvk.Kind) {
		return errors.New("kind is not supported for custom labels")
	}

	objectList := getObjectListMapOfResourceKind()[gvk.Kind]
	matchLabels := getComponentMatchLabels(cluster.Name, componentName)
	for k, v := range resource.Selector {
		matchLabels[k] = v
	}
	if err := getObjectListByCustomLabels(ctx, cli, *cluster, objectList, client.MatchingLabels(matchLabels)); err != nil {
		return err
	}
	labelKey = replaceKBEnvPlaceholderTokens(cluster, componentName, labelKey)
	labelValue = replaceKBEnvPlaceholderTokens(cluster, componentName, labelValue)
	switch gvk.Kind {
	case constant.StatefulSetKind:
		stsList := objectList.(*appsv1.StatefulSetList)
		for _, sts := range stsList.Items {
			if err := updateObjLabel(ctx, cli, sts, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.DeploymentKind:
		deployList := objectList.(*appsv1.DeploymentList)
		for _, deploy := range deployList.Items {
			if err := updateObjLabel(ctx, cli, deploy, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.PodKind:
		podList := objectList.(*corev1.PodList)
		for _, pod := range podList.Items {
			if err := updateObjLabel(ctx, cli, pod, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.ServiceKind:
		svcList := objectList.(*corev1.ServiceList)
		for _, svc := range svcList.Items {
			if err := updateObjLabel(ctx, cli, svc, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.ConfigMapKind:
		cmList := objectList.(*corev1.ConfigMapList)
		for _, cm := range cmList.Items {
			if err := updateObjLabel(ctx, cli, cm, labelKey, labelValue); err != nil {
				return err
			}
		}
	case constant.CronJobKind:
		cjList := objectList.(*batchv1.CronJobList)
		for _, cj := range cjList.Items {
			if err := updateObjLabel(ctx, cli, cj, labelKey, labelValue); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseCustomLabelPattern parses the custom label pattern to GroupVersionKind.
func parseCustomLabelPattern(pattern string) (schema.GroupVersionKind, error) {
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

// getCustomLabelWorkloadKind returns the kinds that support custom label.
func getCustomLabelWorkloadKind() []string {
	return []string{
		constant.CronJobKind,
		constant.StatefulSetKind,
		constant.DeploymentKind,
		constant.ReplicaSetKind,
		constant.PodKind,
	}
}

// SortPods sorts pods by their role priority
func SortPods(pods []corev1.Pod, priorityMap map[string]int, idLabelKey string) {
	// make a Serial pod list,
	// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name < follower2.Name
	sort.SliceStable(pods, func(i, j int) bool {
		roleI := pods[i].Labels[idLabelKey]
		roleJ := pods[j].Labels[idLabelKey]
		if priorityMap[roleI] == priorityMap[roleJ] {
			_, ordinal1 := intctrlutil.GetParentNameAndOrdinal(&pods[i])
			_, ordinal2 := intctrlutil.GetParentNameAndOrdinal(&pods[j])
			return ordinal1 < ordinal2
		}
		return priorityMap[roleI] < priorityMap[roleJ]
	})
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
	builtInEnvMap := componentutil.GetReplacementMapForBuiltInEnv(cluster.Name, string(cluster.UID), componentName)
	return componentutil.ReplaceNamedVars(builtInEnvMap, strToReplace, -1, true)
}

// getRunningPods gets the running pods of the specified statefulSet.
func getRunningPods(ctx context.Context, cli client.Client, obj client.Object) ([]corev1.Pod, error) {
	sts := convertToStatefulSet(obj)
	if sts == nil || sts.Generation != sts.Status.ObservedGeneration {
		return nil, nil
	}
	return GetPodListByStatefulSet(ctx, cli, sts)
}

// resolvePodSpecDefaultFields set default value for some known fields of proto PodSpec @pobj.
func resolvePodSpecDefaultFields(obj corev1.PodSpec, pobj *corev1.PodSpec) {
	resolveVolume := func(v corev1.Volume, vv *corev1.Volume) {
		if vv.DownwardAPI != nil && v.DownwardAPI != nil {
			for i := range vv.DownwardAPI.Items {
				vf := v.DownwardAPI.Items[i]
				if vf.FieldRef == nil {
					continue
				}
				vvf := &vv.DownwardAPI.Items[i]
				if vvf.FieldRef != nil && len(vvf.FieldRef.APIVersion) == 0 {
					vvf.FieldRef.APIVersion = vf.FieldRef.APIVersion
				}
			}
			if vv.DownwardAPI.DefaultMode == nil {
				vv.DownwardAPI.DefaultMode = v.DownwardAPI.DefaultMode
			}
		}
		if vv.ConfigMap != nil && v.ConfigMap != nil {
			if vv.ConfigMap.DefaultMode == nil {
				vv.ConfigMap.DefaultMode = v.ConfigMap.DefaultMode
			}
		}
	}
	resolveContainer := func(c corev1.Container, cc *corev1.Container) {
		if len(cc.TerminationMessagePath) == 0 {
			cc.TerminationMessagePath = c.TerminationMessagePath
		}
		if len(cc.TerminationMessagePolicy) == 0 {
			cc.TerminationMessagePolicy = c.TerminationMessagePolicy
		}
		if len(cc.ImagePullPolicy) == 0 {
			cc.ImagePullPolicy = c.ImagePullPolicy
		}

		resolveContainerProbe := func(p corev1.Probe, pp *corev1.Probe) {
			if pp.TimeoutSeconds == 0 {
				pp.TimeoutSeconds = p.TimeoutSeconds
			}
			if pp.PeriodSeconds == 0 {
				pp.PeriodSeconds = p.PeriodSeconds
			}
			if pp.SuccessThreshold == 0 {
				pp.SuccessThreshold = p.SuccessThreshold
			}
			if pp.FailureThreshold == 0 {
				pp.FailureThreshold = p.FailureThreshold
			}
			if pp.HTTPGet != nil && len(pp.HTTPGet.Scheme) == 0 {
				pp.HTTPGet.Scheme = p.HTTPGet.Scheme
			}
		}
		if cc.LivenessProbe != nil && c.LivenessProbe != nil {
			resolveContainerProbe(*c.LivenessProbe, cc.LivenessProbe)
		}
		if cc.ReadinessProbe != nil && c.ReadinessProbe != nil {
			resolveContainerProbe(*c.ReadinessProbe, cc.ReadinessProbe)
		}
		if cc.StartupProbe != nil && c.StartupProbe != nil {
			resolveContainerProbe(*c.StartupProbe, cc.StartupProbe)
		}
	}
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	for i := 0; i < min(len(obj.Volumes), len(pobj.Volumes)); i++ {
		resolveVolume(obj.Volumes[i], &pobj.Volumes[i])
	}
	for i := 0; i < min(len(obj.InitContainers), len(pobj.InitContainers)); i++ {
		resolveContainer(obj.InitContainers[i], &pobj.InitContainers[i])
	}
	for i := 0; i < min(len(obj.Containers), len(pobj.Containers)); i++ {
		resolveContainer(obj.Containers[i], &pobj.Containers[i])
	}
	if len(pobj.RestartPolicy) == 0 {
		pobj.RestartPolicy = obj.RestartPolicy
	}
	if pobj.TerminationGracePeriodSeconds == nil {
		pobj.TerminationGracePeriodSeconds = obj.TerminationGracePeriodSeconds
	}
	if len(pobj.DNSPolicy) == 0 {
		pobj.DNSPolicy = obj.DNSPolicy
	}
	if len(pobj.DeprecatedServiceAccount) == 0 {
		pobj.DeprecatedServiceAccount = obj.DeprecatedServiceAccount
	}
	if pobj.SecurityContext == nil {
		pobj.SecurityContext = obj.SecurityContext
	}
	if len(pobj.SchedulerName) == 0 {
		pobj.SchedulerName = obj.SchedulerName
	}
	if len(pobj.Tolerations) == 0 {
		pobj.Tolerations = obj.Tolerations
	}
	if pobj.Priority == nil {
		pobj.Priority = obj.Priority
	}
	if pobj.EnableServiceLinks == nil {
		pobj.EnableServiceLinks = obj.EnableServiceLinks
	}
	if pobj.PreemptionPolicy == nil {
		pobj.PreemptionPolicy = obj.PreemptionPolicy
	}
}

// delayUpdatePodSpecSystemFields to delay the updating to system fields in pod spec.
func delayUpdatePodSpecSystemFields(obj corev1.PodSpec, pobj *corev1.PodSpec) {
	for i := range pobj.Containers {
		delayUpdateKubeBlocksToolsImage(obj.Containers, &pobj.Containers[i])
	}
}

// updatePodSpecSystemFields to update system fields in pod spec.
func updatePodSpecSystemFields(pobj *corev1.PodSpec) {
	for i := range pobj.Containers {
		updateKubeBlocksToolsImage(&pobj.Containers[i])
	}
}

func delayUpdateKubeBlocksToolsImage(containers []corev1.Container, pc *corev1.Container) {
	if pc.Image != viper.GetString(constant.KBToolsImage) {
		return
	}
	for _, c := range containers {
		if c.Name == pc.Name {
			if getImageName(c.Image) == getImageName(pc.Image) {
				pc.Image = c.Image
			}
			break
		}
	}
}

func updateKubeBlocksToolsImage(pc *corev1.Container) {
	if getImageName(pc.Image) == getImageName(viper.GetString(constant.KBToolsImage)) {
		pc.Image = viper.GetString(constant.KBToolsImage)
	}
}

func getImageName(image string) string {
	subs := strings.Split(image, ":")
	if len(subs) != 2 {
		return ""
	}
	return subs[0]
}
