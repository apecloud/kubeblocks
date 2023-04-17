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

package stateful

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type Stateful struct {
	types.ComponentSetBase
}

var _ types.ComponentSet = &Stateful{}

func (r *Stateful) getReplicas() int32 {
	if r.Component != nil {
		return r.Component.GetReplicas()
	}
	return r.ComponentSpec.Replicas
}

func (r *Stateful) SetComponent(comp types.Component) {
	r.Component = comp
}

func (r *Stateful) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
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

func (r *Stateful) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	return util.StatefulSetPodsAreReady(sts, r.getReplicas()), nil
}

func (r *Stateful) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

// HandleProbeTimeoutWhenPodsReady the Stateful component has no role detection, empty implementation here.
func (r *Stateful) HandleProbeTimeoutWhenPodsReady(status *appsv1alpha1.ClusterComponentStatus, pods []*corev1.Pod) {
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (r *Stateful) GetPhaseWhenPodsNotReady(ctx context.Context, componentName string) (appsv1alpha1.ClusterComponentPhase, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := util.GetCompRelatedObjectList(ctx, r.Cli, *r.Cluster, componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", err
	}
	// if the failed pod is not controlled by the latest revision
	checkExistFailedPodOfLatestRevision := func(pod *corev1.Pod, workload metav1.Object) bool {
		sts := workload.(*appsv1.StatefulSet)
		return !intctrlutil.PodIsReady(pod) && intctrlutil.PodIsControlledByLatestRevision(pod, sts)
	}
	stsObj := stsList.Items[0]
	return util.GetComponentPhaseWhenPodsNotReady(podList, &stsObj, r.getReplicas(),
		stsObj.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision), nil
}

func (r *Stateful) HandleRestart(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func (r *Stateful) HandleRoleChange(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func newStateful(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *Stateful {
	return &Stateful{
		ComponentSetBase: types.ComponentSetBase{
			Cli:           cli,
			Cluster:       cluster,
			ComponentSpec: spec,
			ComponentDef:  &def,
			Component:     nil,
		},
	}
}
