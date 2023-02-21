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
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type Stateful struct {
	Cli          client.Client
	Ctx          context.Context
	Cluster      *appsv1alpha1.Cluster
	ComponentDef *appsv1alpha1.ClusterComponentDefinition
	Component    *appsv1alpha1.ClusterComponentSpec
}

var _ types.Component = &Stateful{}

func (stateful *Stateful) IsRunning(obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.CovertToStatefulSet(obj)
	isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(stateful.Ctx, stateful.Cli, sts)
	if err != nil {
		return false, err
	}
	return util.StatefulSetOfComponentIsReady(sts, isRevisionConsistent, &stateful.Component.Replicas), nil
}

func (stateful *Stateful) PodsReady(obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.CovertToStatefulSet(obj)
	return util.StatefulSetPodsAreReady(sts, stateful.Component.Replicas), nil
}

func (stateful *Stateful) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

// HandleProbeTimeoutWhenPodsReady the Stateful component has no role detection, empty implementation here.
func (stateful *Stateful) HandleProbeTimeoutWhenPodsReady(recorder record.EventRecorder) (bool, error) {
	return false, nil
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (stateful *Stateful) GetPhaseWhenPodsNotReady(componentName string) (appsv1alpha1.Phase, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := util.GetCompRelatedObjectList(stateful.Ctx, stateful.Cli, stateful.Cluster, componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", err
	}
	podCount := len(podList.Items)
	stsObj := stsList.Items[0]
	componentReplicas := stateful.Component.Replicas
	if podCount == 0 || stsObj.Status.AvailableReplicas == 0 {
		return util.GetPhaseWithNoAvailableReplicas(componentReplicas), nil
	}
	var existLatestRevisionFailedPod bool
	for _, v := range podList.Items {
		// if the pod is terminating, ignore it
		if v.DeletionTimestamp != nil {
			return "", nil
		}
		if !intctrlutil.PodIsReady(&v) && util.PodIsControlledByLatestRevision(&v, &stsObj) {
			existLatestRevisionFailedPod = true
		}
	}
	//  if pod is not controlled by the latest controller revision, ignore it.
	if !existLatestRevisionFailedPod {
		return "", nil
	}
	// checks if the available replicas of component and workload are consistent.
	if !util.AvailableReplicasAreConsistent(componentReplicas, int32(podCount), stsObj.Status.AvailableReplicas) {
		return appsv1alpha1.AbnormalPhase, nil
	}
	return "", nil
}

func NewStateful(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition) types.Component {
	if component == nil || componentDef == nil {
		return nil
	}
	return &Stateful{
		Ctx:          ctx,
		Cli:          cli,
		Cluster:      cluster,
		Component:    component,
		ComponentDef: componentDef,
	}
}
