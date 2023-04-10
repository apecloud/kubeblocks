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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type Stateful struct {
	Cli           client.Client
	Cluster       *appsv1alpha1.Cluster
	Component     types.Component
	ComponentSpec *appsv1alpha1.ClusterComponentSpec
}

var _ types.ComponentSet = &Stateful{}

func (stateful *Stateful) getReplicas() int32 {
	if stateful.Component != nil {
		return stateful.Component.GetReplicas()
	}
	return stateful.ComponentSpec.Replicas
}

func (stateful *Stateful) SetComponent(comp types.Component) {
	stateful.Component = comp
}

func (stateful *Stateful) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(ctx, stateful.Cli, sts)
	if err != nil {
		return false, err
	}
	targetReplicas := stateful.getReplicas()
	return util.StatefulSetOfComponentIsReady(sts, isRevisionConsistent, &targetReplicas), nil
}

func (stateful *Stateful) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	return util.StatefulSetPodsAreReady(sts, stateful.getReplicas()), nil
}

func (stateful *Stateful) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	return util.PodIsAvailable(appsv1alpha1.Stateful, pod, minReadySeconds)
}

func (stateful *Stateful) HandleProbeTimeoutWhenPodsReady(status *appsv1alpha1.ClusterComponentStatus, pods []*corev1.Pod) {
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (stateful *Stateful) GetPhaseWhenPodsNotReady(ctx context.Context, componentName string) (appsv1alpha1.ClusterComponentPhase, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := util.GetCompRelatedObjectList(ctx, stateful.Cli, *stateful.Cluster, componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", err
	}
	// if the failed pod is not controlled by the latest revision
	checkExistFailedPodOfLatestRevision := func(pod *corev1.Pod, workload metav1.Object) bool {
		sts := workload.(*appsv1.StatefulSet)
		return !intctrlutil.PodIsReady(pod) && intctrlutil.PodIsControlledByLatestRevision(pod, sts)
	}
	stsObj := stsList.Items[0]
	return util.GetComponentPhaseWhenPodsNotReady(podList, &stsObj, stateful.getReplicas(),
		stsObj.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision), nil
}

func (stateful *Stateful) HandleRestart(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func (stateful *Stateful) HandleRoleChange(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func newStateful(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	_ appsv1alpha1.ClusterComponentDefinition) (*Stateful, error) {
	stateful := &Stateful{
		Cli:           cli,
		Cluster:       cluster,
		Component:     nil,
		ComponentSpec: component,
	}
	return stateful, nil
}
