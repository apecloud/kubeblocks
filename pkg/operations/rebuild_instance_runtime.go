/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// rebuildInstanceRuntime contains the native resource access that is specific
// to RebuildInstance. It must not be used by other operation handlers.
type rebuildInstanceRuntime struct {
	ctx          context.Context
	cli          client.Client
	dataGetOpts  []client.GetOption
	dataListOpts []client.ListOption
}

func newRebuildInstanceRuntime(ctx context.Context, cli client.Client, cluster *appsv1.Cluster) *rebuildInstanceRuntime {
	if ctx == nil {
		ctx = context.Background()
	}
	r := &rebuildInstanceRuntime{ctx: ctx, cli: cli}
	if !enabledMultiCluster(cluster) {
		return r
	}
	placement := ""
	if cluster.Annotations != nil {
		placement = cluster.Annotations[constant.KBAppMultiClusterPlacementKey]
	}
	if len(strings.TrimSpace(placement)) > 0 {
		r.ctx = multicluster.IntoContext(ctx, placement)
		r.dataGetOpts = []client.GetOption{multicluster.InDataContext()}
		r.dataListOpts = []client.ListOption{multicluster.InDataContext()}
	}
	return r
}

func (r *rebuildInstanceRuntime) getInstance(namespace, clusterName, compName, instanceName string) (*rebuildRuntimeInstance, error) {
	pod := &corev1.Pod{}
	if err := r.cli.Get(r.ctx, client.ObjectKey{Name: instanceName, Namespace: namespace}, pod, r.dataGetOpts...); err != nil {
		if apierrors.IsNotFound(err) {
			pvcMap, loadErr := r.loadVolumes(namespace, clusterName, compName)
			if loadErr != nil {
				return nil, loadErr
			}
			for pvcName := range pvcMap {
				if strings.HasSuffix(pvcName, "-"+instanceName) {
					return &rebuildRuntimeInstance{name: instanceName, componentName: compName}, nil
				}
			}
		}
		return nil, err
	}
	if pod.Labels[constant.AppInstanceLabelKey] != clusterName || pod.Labels[constant.KBAppComponentLabelKey] != compName {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" does not belong to component "%s"`, instanceName, compName))
	}
	return &rebuildRuntimeInstance{name: pod.Name, componentName: compName, pod: pod}, nil
}

func (r *rebuildInstanceRuntime) listInstances(namespace, clusterName, compName string) ([]*rebuildRuntimeInstance, error) {
	pods, err := component.ListOwnedPods(r.ctx, r.cli, namespace, clusterName, compName, r.dataListOpts...)
	if err != nil {
		return nil, err
	}
	result := make([]*rebuildRuntimeInstance, 0, len(pods))
	for _, pod := range pods {
		result = append(result, &rebuildRuntimeInstance{name: pod.Name, componentName: compName, pod: pod})
	}
	return result, nil
}

func (r *rebuildInstanceRuntime) generateInstanceNameSet(clusterName, compName string, compReplicas int32,
	instances []appsv1.InstanceTemplate, offlineInstances []string) (map[string]string, error) {
	workloadName := constant.GenerateClusterComponentName(clusterName, compName)
	templates := make([]instanceset.InstanceTemplate, 0, len(instances))
	for i := range instances {
		templates = append(templates, &workloads.InstanceTemplate{
			Name:     instances[i].Name,
			Replicas: instances[i].Replicas,
			Ordinals: instances[i].Ordinals,
		})
	}
	instanceNames, err := instanceset.GenerateAllInstanceNames(workloadName, compReplicas, templates, offlineInstances, appsv1.Ordinals{})
	if err != nil {
		return nil, err
	}
	instanceSet := make(map[string]string, len(instanceNames))
	for _, instanceName := range instanceNames {
		instanceSet[instanceName] = appsv1.GetInstanceTemplateName(clusterName, compName, instanceName)
	}
	return instanceSet, nil
}

func (r *rebuildInstanceRuntime) generateTemplateInstanceNames(clusterName, compName, templateName string, replicas int32,
	offlineInstances []string, ordinals appsv1.Ordinals) ([]string, error) {
	workloadName := constant.GenerateWorkloadNamePattern(clusterName, compName)
	ordinalList, err := instanceset.ConvertOrdinalsToSortedList(ordinals)
	if err != nil {
		return nil, err
	}
	return instanceset.GenerateInstanceNamesFromTemplate(workloadName, templateName, replicas, offlineInstances, ordinalList)
}

func (r *rebuildInstanceRuntime) loadVolumes(namespace, clusterName, compName string) (map[string]*corev1.PersistentVolumeClaim, error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterName,
			constant.KBAppComponentLabelKey: compName,
		},
	}
	opts = append(opts, r.dataListOpts...)
	if err := r.cli.List(r.ctx, pvcList, opts...); err != nil {
		return nil, err
	}
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim, len(pvcList.Items))
	for i := range pvcList.Items {
		pvc := pvcList.Items[i]
		pvcMap[pvc.Name] = pvc.DeepCopy()
	}
	return pvcMap, nil
}

type rebuildRuntimeInstance struct {
	name          string
	componentName string
	pod           *corev1.Pod
}

func (i *rebuildRuntimeInstance) isDeleting() bool {
	return i.pod != nil && !i.pod.DeletionTimestamp.IsZero()
}

func (i *rebuildRuntimeInstance) isAvailable(minReadySeconds int32, roleAware bool) bool {
	if i.pod == nil || i.isDeleting() {
		return false
	}
	if roleAware {
		return intctrlutil.PodIsReadyWithLabel(*i.pod)
	}
	return podutils.IsPodAvailable(i.pod, minReadySeconds, metav1.Now())
}

func (i *rebuildRuntimeInstance) isFailedAndTimedOut() bool {
	if i.pod == nil {
		return false
	}
	isFailed, isTimeout, _ := intctrlutil.IsPodFailedAndTimedOut(i.pod)
	return isFailed && isTimeout
}
