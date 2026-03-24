/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package component

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type componentWorkloadOps struct {
	transCtx       *componentTransformContext
	cli            client.Client
	component      *appsv1.Component
	synthesizeComp *component.SynthesizedComponent
	dag            *graph.DAG

	runningITS            *workloads.InstanceSet
	protoITS              *workloads.InstanceSet
	desiredCompPodNameSet sets.Set[string]
	runningItsPodNameSet  sets.Set[string]
}

func newComponentWorkloadOps(transCtx *componentTransformContext,
	cli client.Client,
	synthesizedComp *component.SynthesizedComponent,
	comp *appsv1.Component,
	runningITS *workloads.InstanceSet,
	protoITS *workloads.InstanceSet,
	dag *graph.DAG) (*componentWorkloadOps, error) {
	runningITSPodNames, err := component.GetCurrentPodNamesByITS(runningITS)
	if err != nil {
		return nil, err
	}
	protoITSPodNames, err := component.GetDesiredPodNamesByITS(runningITS, protoITS)
	if err != nil {
		return nil, err
	}
	return &componentWorkloadOps{
		transCtx:              transCtx,
		cli:                   cli,
		component:             comp,
		synthesizeComp:        synthesizedComp,
		runningITS:            runningITS,
		protoITS:              protoITS,
		dag:                   dag,
		desiredCompPodNameSet: sets.New(protoITSPodNames...),
		runningItsPodNameSet:  sets.New(runningITSPodNames...),
	}, nil
}

func (r *componentWorkloadOps) horizontalScale() error {
	var (
		in  = r.runningItsPodNameSet.Difference(r.desiredCompPodNameSet)
		out = r.desiredCompPodNameSet.Difference(r.runningItsPodNameSet)
	)
	if err := r.dataReplicationTask(); err != nil {
		return err
	}
	if in.Len() != 0 || out.Len() != 0 {
		r.transCtx.EventRecorder.Eventf(r.component,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"start horizontal scale component %s of cluster %s from %d to %d",
			r.synthesizeComp.Name, r.synthesizeComp.ClusterName, int(*r.runningITS.Spec.Replicas), r.synthesizeComp.Replicas)
	}
	return nil
}

func (r *componentWorkloadOps) dataReplicationTask() error {
	_, hasDataActionDefined := hasMemberJoinNDataActionDefined(r.synthesizeComp.LifecycleActions.ComponentLifecycleActions)
	if !hasDataActionDefined {
		return nil
	}

	var (
		// new replicas to be submitted to InstanceSet
		newReplicas = r.desiredCompPodNameSet.Difference(r.runningItsPodNameSet).UnsortedList()
		// replicas can be used as the source replica to dump data
		sourceReplicas = sets.New[string]()
		// replicas are in provisioning and the data has not been loaded
		provisioningReplicas []string
		// replicas are not provisioned
		unprovisionedReplicas = r.runningItsPodNameSet.Clone()
	)
	for _, replica := range r.runningITS.Status.InstanceStatus {
		if !r.runningItsPodNameSet.Has(replica.PodName) {
			continue // to be deleted
		}
		if replica.Provisioned {
			unprovisionedReplicas.Delete(replica.PodName)
		}
		if replica.DataLoaded != nil && !*replica.DataLoaded {
			provisioningReplicas = append(provisioningReplicas, replica.PodName)
			continue
		}
		if replica.MemberJoined == nil || *replica.MemberJoined {
			sourceReplicas.Insert(replica.PodName)
		}
	}

	if r.runningITS.IsInInitializing() || len(newReplicas) == 0 && unprovisionedReplicas.Len() == 0 && len(provisioningReplicas) == 0 {
		return nil
	}

	// choose the source replica
	source, err := r.sourceReplica(r.synthesizeComp.LifecycleActions.DataDump, sourceReplicas)
	if err != nil {
		return err
	}

	replicas := slices.Clone(newReplicas)
	replicas = append(replicas, unprovisionedReplicas.UnsortedList()...)
	replicas = append(replicas, provisioningReplicas...)
	slices.Sort(replicas)
	parameters, err := component.NewReplicaTask(r.synthesizeComp.FullCompName, r.synthesizeComp.Generation, source, replicas)
	if err != nil {
		return err
	}
	// apply the updated env to the env CM
	transCtx := &componentTransformContext{
		Context:             r.transCtx.Context,
		Client:              model.NewGraphClient(r.cli),
		SynthesizeComponent: r.synthesizeComp,
		Component:           r.component,
	}
	return createOrUpdateEnvConfigMap(transCtx, r.dag, nil, parameters)
}

func (r *componentWorkloadOps) sourceReplica(dataDump *appsv1.Action, sourceReplicas sets.Set[string]) (lifecycle.Replica, error) {
	var replicas []lifecycle.Replica
	for i, inst := range r.runningITS.Status.InstanceStatus {
		if sourceReplicas.Has(inst.PodName) {
			replicas = append(replicas, &lifecycleReplica{
				synthesizedComp: r.synthesizeComp,
				instance:        r.runningITS.Status.InstanceStatus[i],
			})
		}
	}
	if len(replicas) > 0 {
		if len(dataDump.TargetPodSelector) == 0 && (dataDump.Exec == nil || len(dataDump.Exec.TargetPodSelector) == 0) {
			dataDump.TargetPodSelector = appsv1.AnyReplica
		}
		// TODO: idempotence for provisioning replicas
		var err error
		replicas, err = lifecycle.SelectTargetPods(replicas, nil, dataDump)
		if err != nil {
			return nil, err
		}
		if len(replicas) > 0 {
			return replicas[0], nil
		}
	}
	return nil, fmt.Errorf("no available pod to dump data")
}
