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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
	objectList client.ObjectList, matchLabels client.ListOption, opts ...client.ListOption) error {
	inNamespace := client.InNamespace(cluster.Namespace)
	if opts == nil {
		opts = []client.ListOption{matchLabels, inNamespace}
	} else {
		opts = append(opts, matchLabels, inNamespace)
	}
	return cli.List(ctx, objectList, opts...)
}

func DelayUpdateRsmSystemFields(obj v1alpha1.ReplicatedStateMachineSpec, pobj *v1alpha1.ReplicatedStateMachineSpec) {
	DelayUpdatePodSpecSystemFields(obj.Template.Spec, &pobj.Template.Spec)

	if pobj.RoleProbe != nil && obj.RoleProbe != nil {
		pobj.RoleProbe.FailureThreshold = obj.RoleProbe.FailureThreshold
		pobj.RoleProbe.SuccessThreshold = obj.RoleProbe.SuccessThreshold
	}
}

// DelayUpdatePodSpecSystemFields to delay the updating to system fields in pod spec.
func DelayUpdatePodSpecSystemFields(obj corev1.PodSpec, pobj *corev1.PodSpec) {
	for i := range pobj.Containers {
		delayUpdateKubeBlocksToolsImage(obj.Containers, &pobj.Containers[i])
	}
	for i := range pobj.InitContainers {
		delayUpdateKubeBlocksToolsImage(obj.InitContainers, &pobj.InitContainers[i])
	}
	updateLorryContainer(obj.Containers, pobj.Containers)
}

func UpdateRsmSystemFields(obj v1alpha1.ReplicatedStateMachineSpec, pobj *v1alpha1.ReplicatedStateMachineSpec) {
	UpdatePodSpecSystemFields(obj.Template.Spec, &pobj.Template.Spec)
	if pobj.RoleProbe != nil && obj.RoleProbe != nil {
		pobj.RoleProbe.FailureThreshold = obj.RoleProbe.FailureThreshold
		pobj.RoleProbe.SuccessThreshold = obj.RoleProbe.SuccessThreshold
	}
}

// UpdatePodSpecSystemFields to update system fields in pod spec.
func UpdatePodSpecSystemFields(obj corev1.PodSpec, pobj *corev1.PodSpec) {
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
