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
)

type Stateful struct {
	Cli          client.Client
	Ctx          context.Context
	Cluster      *appsv1alpha1.Cluster
	ComponentDef *appsv1alpha1.ClusterDefinitionComponent
	Component    *appsv1alpha1.ClusterComponent
}

var _ types.Component = &Stateful{}

func (stateful *Stateful) IsRunning(obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.CovertToStatefulSet(obj)
	statefulStatusRevisionIsEquals := sts.Status.UpdateRevision == sts.Status.CurrentRevision
	targetReplicas := util.GetComponentReplicas(stateful.Component, stateful.ComponentDef)
	return util.StatefulSetIsReady(sts, statefulStatusRevisionIsEquals, &targetReplicas), nil
}

func (stateful *Stateful) PodsReady(obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.CovertToStatefulSet(obj)
	return util.StatefulSetPodsIsReady(sts), nil
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

func (stateful *Stateful) GetPhaseWhenPodsNotReady(componentName string) (appsv1alpha1.Phase, error) {
	var (
		isFailed          = true
		isAbnormal        bool
		stsList           = &appsv1.StatefulSetList{}
		podsIsTerminating bool
		err               error
	)
	if podsIsTerminating, err = util.CheckRelatedPodIsTerminating(stateful.Ctx,
		stateful.Cli, stateful.Cluster, componentName); err != nil || podsIsTerminating {
		return "", err
	}
	if err = util.GetObjectListByComponentName(stateful.Ctx,
		stateful.Cli, stateful.Cluster, stsList, componentName); err != nil {
		return "", err
	}
	for _, v := range stsList.Items {
		if v.Status.AvailableReplicas < 1 {
			continue
		}
		isFailed = false
		if v.Status.AvailableReplicas < *v.Spec.Replicas {
			isAbnormal = true
		}
	}
	return util.GetComponentPhase(isFailed, isAbnormal), nil
}

func NewStateful(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponent,
	componentDef *appsv1alpha1.ClusterDefinitionComponent) types.Component {
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
