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

package rsm

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type RSM struct {
	internal.ComponentSetBase
}

var _ internal.ComponentSet = &RSM{}

func (r *RSM) getReplicas() int32 {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Replicas
	}
	return r.ComponentSpec.Replicas
}

func (r *RSM) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	targetReplicas := r.getReplicas()
	return util.StatefulSetOfComponentIsReady(sts, isRevisionConsistent, &targetReplicas), nil
}

func (r *RSM) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	return util.StatefulSetPodsAreReady(sts, r.getReplicas()), nil
}

func (r *RSM) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

func (r *RSM) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	return "", nil
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (r *RSM) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string,
	originPhaseIsUpRunning bool) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := util.GetCompRelatedObjectList(ctx, r.Cli, *r.Cluster, componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", nil, err
	}
	statusMessages := appsv1alpha1.ComponentMessageMap{}
	// if the failed pod is not controlled by the latest revision
	checkExistFailedPodOfLatestRevision := func(pod *corev1.Pod, workload metav1.Object) bool {
		sts := workload.(*appsv1.StatefulSet)
		// if component is up running but pod is not ready, this pod should be failed.
		// for example: full disk cause readiness probe failed and serve is not available.
		// but kubelet only sets the container is not ready and pod is also Running.
		if originPhaseIsUpRunning {
			return !intctrlutil.PodIsReady(pod) && intctrlutil.PodIsControlledByLatestRevision(pod, sts)
		}
		isFailed, _, message := internal.IsPodFailedAndTimedOut(pod)
		existLatestRevisionFailedPod := isFailed && intctrlutil.PodIsControlledByLatestRevision(pod, sts)
		if existLatestRevisionFailedPod {
			statusMessages.SetObjectMessage(pod.Kind, pod.Name, message)
		}
		return existLatestRevisionFailedPod
	}
	stsObj := stsList.Items[0]
	return util.GetComponentPhaseWhenPodsNotReady(podList, &stsObj, r.getReplicas(),
		stsObj.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision), statusMessages, nil
}

func (r *RSM) HandleRestart(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func (r *RSM) HandleRoleChange(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func newRSM(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *RSM {
	return &RSM{
		ComponentSetBase: internal.ComponentSetBase{
			Cli:                  cli,
			Cluster:              cluster,
			SynthesizedComponent: nil,
			ComponentSpec:        spec,
			ComponentDef:         &def,
		},
	}
}
