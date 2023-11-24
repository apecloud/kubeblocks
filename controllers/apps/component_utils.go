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

package apps

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	errReqClusterObj = errors.New("required arg *appsv1alpha1.Cluster is nil")
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

// IsProbeTimeout checks if the application of the pod is probe timed out.
func IsProbeTimeout(probes *appsv1alpha1.ClusterDefinitionProbes, podsReadyTime *metav1.Time) bool {
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

// getObjectListByCustomLabels gets k8s workload list with custom labels
func getObjectListByCustomLabels(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster,
	objectList client.ObjectList, matchLabels client.ListOption) error {
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, matchLabels, inNamespace)
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

// getCompRelatedObjectList gets the related pods and workloads of the component
func getCompRelatedObjectList(ctx context.Context,
	cli client.Client,
	cluster appsv1alpha1.Cluster,
	compName string,
	relatedWorkloads client.ObjectList) (*corev1.PodList, error) {
	podList, err := intctrlcomp.GetComponentPodList(ctx, cli, cluster, compName)
	if err != nil {
		return nil, err
	}
	if err = intctrlcomp.GetObjectListByComponentName(ctx,
		cli, cluster, relatedWorkloads, compName); err != nil {
		return nil, err
	}
	return podList, nil
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

// replaceKBEnvPlaceholderTokens replaces the placeholder tokens in the string strToReplace with builtInEnvMap and return new string.
func replaceKBEnvPlaceholderTokens(clusterName, uid, componentName, strToReplace string) string {
	builtInEnvMap := intctrlcomp.GetReplacementMapForBuiltInEnv(clusterName, uid, componentName)
	return intctrlcomp.ReplaceNamedVars(builtInEnvMap, strToReplace, -1, true)
}

// ResolvePodSpecDefaultFields set default value for some known fields of proto PodSpec @pobj.
func ResolvePodSpecDefaultFields(obj corev1.PodSpec, pobj *corev1.PodSpec) {
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
				if p.HTTPGet != nil {
					pp.HTTPGet.Scheme = p.HTTPGet.Scheme
				}
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

// DelayUpdatePodSpecSystemFields to delay the updating to system fields in pod spec.
func DelayUpdatePodSpecSystemFields(obj corev1.PodSpec, pobj *corev1.PodSpec) {
	for i := range pobj.Containers {
		delayUpdateKubeBlocksToolsImage(obj.Containers, &pobj.Containers[i])
	}
}

// UpdatePodSpecSystemFields to update system fields in pod spec.
func UpdatePodSpecSystemFields(pobj *corev1.PodSpec) {
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
	switch len(subs) {
	case 2:
		return subs[0]
	case 3:
		lastIndex := strings.LastIndex(image, ":")
		return image[:lastIndex]
	default:
		return ""
	}
}

// UpdateComponentInfoToPods patches current component's replicas to all belonging pods, as an annotation.
func UpdateComponentInfoToPods(
	ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *intctrlcomp.SynthesizedComponent,
	dag *graph.DAG) error {
	if cluster == nil || component == nil {
		return nil
	}
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.GetName(),
		constant.KBAppComponentLabelKey: component.Name,
	}
	// list all pods in cache
	podList := corev1.PodList{}
	if err := cli.List(ctx, &podList, client.InNamespace(cluster.Namespace), ml); err != nil {
		return err
	}
	// list all pods in dag
	graphCli := model.NewGraphClient(cli)
	pods := graphCli.FindAll(dag, &corev1.Pod{})

	replicasStr := strconv.Itoa(int(component.Replicas))
	updateAnnotation := func(obj client.Object) {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 0)
		}
		annotations[constant.ComponentReplicasAnnotationKey] = replicasStr
		obj.SetAnnotations(annotations)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Annotations != nil &&
			pod.Annotations[constant.ComponentReplicasAnnotationKey] == replicasStr {
			continue
		}
		idx := slices.IndexFunc(pods, func(obj client.Object) bool {
			return obj.GetName() == pod.Name
		})
		// pod already in dag, merge annotations
		if idx >= 0 {
			updateAnnotation(pods[idx])
			continue
		}
		// pod not in dag, add a new vertex
		updateAnnotation(pod)
		graphCli.Do(dag, nil, pod, model.ActionUpdatePtr(), nil)
	}
	return nil
}

// UpdateCustomLabelToPods updates custom label to pods
func UpdateCustomLabelToPods(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *intctrlcomp.SynthesizedComponent,
	dag *graph.DAG) error {
	if cluster == nil || component == nil {
		return nil
	}
	// list all pods in dag
	graphCli := model.NewGraphClient(cli)
	pods := graphCli.FindAll(dag, &corev1.Pod{})

	for labelKey, labelValue := range component.Labels {
		podList := &corev1.PodList{}
		matchLabels := constant.GetComponentWellKnownLabels(cluster.Name, component.Name)
		if err := getObjectListByCustomLabels(ctx, cli, *cluster, podList, client.MatchingLabels(matchLabels)); err != nil {
			return err
		}

		for i := range podList.Items {
			idx := slices.IndexFunc(pods, func(obj client.Object) bool {
				return obj.GetName() == podList.Items[i].Name
			})
			// pod already in dag, merge labels
			if idx >= 0 {
				updateObjLabel(cluster.Name, string(cluster.UID), component.Name, labelKey, string(labelValue), pods[idx])
				continue
			}
			pod := &podList.Items[i]
			updateObjLabel(cluster.Name, string(cluster.UID), component.Name, labelKey, string(labelValue), pod)
			graphCli.Do(dag, nil, pod, model.ActionUpdatePtr(), nil)
		}
	}
	return nil
}

func updateObjLabel(clusterName, uid, componentName, labelKey, labelValue string, obj client.Object) {
	key := replaceKBEnvPlaceholderTokens(clusterName, uid, componentName, labelKey)
	value := replaceKBEnvPlaceholderTokens(clusterName, uid, componentName, labelValue)

	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 0)
	}
	labels[key] = value
	obj.SetLabels(labels)
}
