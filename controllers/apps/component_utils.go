/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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

// DelayUpdatePodSpecSystemFields to delay the updating to system fields in pod spec.
func DelayUpdatePodSpecSystemFields(obj corev1.PodSpec, pobj *corev1.PodSpec) {
	for i := range pobj.Containers {
		delayUpdateKubeBlocksToolsImage(obj.Containers, &pobj.Containers[i])
	}
	updateLorryContainer(obj.Containers, pobj.Containers)
}

// UpdatePodSpecSystemFields to update system fields in pod spec.
func UpdatePodSpecSystemFields(obj *corev1.PodSpec, pobj *corev1.PodSpec) {
	for i := range pobj.Containers {
		updateKubeBlocksToolsImage(&pobj.Containers[i])
	}

	updateLorryContainer(obj.Containers, pobj.Containers)
}

func updateLorryContainer(containers []corev1.Container, pcontainers []corev1.Container) {
	srcLorryContainer := controllerutil.GetLorryContainer(containers)
	dstLorryContainer := controllerutil.GetLorryContainer(pcontainers)
	if srcLorryContainer == nil || dstLorryContainer == nil {
		return
	}
	for i, c := range pcontainers {
		if c.Name == dstLorryContainer.Name {
			pcontainers[i] = *srcLorryContainer.DeepCopy()
			return
		}
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
	if cluster == nil || component == nil || component.RsmTransformPolicy == v1alpha1.ToPod {
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
				updateObjLabel(labelKey, labelValue, pods[idx])
				continue
			}
			pod := &podList.Items[i]
			updateObjLabel(labelKey, labelValue, pod)
			graphCli.Do(dag, nil, pod, model.ActionUpdatePtr(), nil)
		}
	}
	return nil
}

func updateObjLabel(labelKey, labelValue string, obj client.Object) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 0)
	}
	labels[labelKey] = labelValue
	obj.SetLabels(labels)
}
