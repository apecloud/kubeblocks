/*
Copyright ApeCloud Inc.

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
)

type Stateful struct {
	Cli     client.Client
	Ctx     context.Context
	Cluster *dbaasv1alpha1.Cluster
}

func (stateful *Stateful) IsRunning(obj client.Object) (bool, error) {
	sts := util.CovertToStatefulSet(obj)
	if sts == nil {
		return false, nil
	}
	statefulStatusRevisionIsEquals := sts.Status.UpdateRevision == sts.Status.CurrentRevision
	return util.StatefulSetIsReady(sts, statefulStatusRevisionIsEquals), nil
}

func (stateful *Stateful) PodsReady(obj client.Object) (bool, error) {
	sts := util.CovertToStatefulSet(obj)
	return util.StatefulSetPodsIsReady(sts), nil
}

func (stateful *Stateful) HandleProbeTimeoutWhenPodsReady() (bool, error) {
	return false, nil
}

func (stateful *Stateful) CalculatePhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error) {
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
	return util.CalculateComponentPhase(isFailed, isAbnormal), nil
}

func NewStateful(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster) *Stateful {
	return &Stateful{
		Ctx:     ctx,
		Cli:     cli,
		Cluster: cluster,
	}
}
