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

package components

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/constant"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// PodIsAvailable checks whether a pod is available with respect to the workload type.
// Deprecated: provide for ops request using, remove this interface later.
func PodIsAvailable(workloadType appsv1alpha1.WorkloadType, pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	switch workloadType {
	case appsv1alpha1.Consensus, appsv1alpha1.Replication:
		return intctrlutil.PodIsReadyWithLabel(*pod)
	case appsv1alpha1.Stateful, appsv1alpha1.Stateless:
		return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
	default:
		panic("unknown workload type")
	}
}

func NewComponent(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	definition *appsv1alpha1.ClusterDefinition,
	version *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	compName string,
	dag *graph.DAG) (Component, error) {
	var compDef *appsv1alpha1.ClusterComponentDefinition
	var compVer *appsv1alpha1.ClusterComponentVersion
	compSpec := cluster.Spec.GetComponentByName(compName)
	if compSpec != nil {
		compDef = definition.GetComponentDefByName(compSpec.ComponentDefRef)
		if compDef == nil {
			return nil, fmt.Errorf("referenced component definition does not exist, cluster: %s, component: %s, component definition ref:%s",
				cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
		}
		if version != nil {
			compVer = version.Spec.GetDefNameMappingComponents()[compSpec.ComponentDefRef]
		}
	} else {
		compDef = definition.GetComponentDefByName(compName)
		if version != nil {
			compVer = version.Spec.GetDefNameMappingComponents()[compName]
		}
	}

	if compDef == nil {
		return nil, nil
	}

	synthesizedComp, err := composeSynthesizedComponent(reqCtx, cli, cluster, definition, compDef, compSpec, compVer)
	if err != nil {
		return nil, err
	}
	if synthesizedComp == nil {
		return nil, nil
	}

	if viper.GetBool(constant.FeatureGateReplicatedStateMachine) {
		return newRSMComponent(cli, reqCtx.Recorder, cluster, version, synthesizedComp, dag), nil
	}

	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		return newReplicationComponent(cli, reqCtx.Recorder, cluster, version, synthesizedComp, dag), nil
	case appsv1alpha1.Consensus:
		return newConsensusComponent(cli, reqCtx.Recorder, cluster, version, synthesizedComp, dag), nil
	case appsv1alpha1.Stateful:
		return newStatefulComponent(cli, reqCtx.Recorder, cluster, version, synthesizedComp, dag), nil
	case appsv1alpha1.Stateless:
		return newStatelessComponent(cli, reqCtx.Recorder, cluster, version, synthesizedComp, dag), nil
	}
	panic(fmt.Sprintf("unknown workload type: %s, cluster: %s, component: %s, component definition ref: %s",
		compDef.WorkloadType, cluster.Name, compSpec.Name, compSpec.ComponentDefRef))
}

func composeSynthesizedComponent(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	compVer *appsv1alpha1.ClusterComponentVersion) (*component.SynthesizedComponent, error) {
	clsMgr, err := getClassManager(reqCtx.Ctx, cli, cluster)
	if err != nil {
		return nil, err
	}
	synthesizedComp, err := component.BuildComponent(reqCtx, clsMgr, cluster, clusterDef, compDef, compSpec, compVer)
	if err != nil {
		return nil, err
	}
	return synthesizedComp, nil
}

func getClassManager(ctx context.Context, cli types2.ReadonlyClient, cluster *appsv1alpha1.Cluster) (*class.Manager, error) {
	var classDefinitionList appsv1alpha1.ComponentClassDefinitionList
	ml := []client.ListOption{
		client.MatchingLabels{constant.ClusterDefLabelKey: cluster.Spec.ClusterDefRef},
	}
	if err := cli.List(ctx, &classDefinitionList, ml...); err != nil {
		return nil, err
	}

	var constraintList appsv1alpha1.ComponentResourceConstraintList
	if err := cli.List(ctx, &constraintList); err != nil {
		return nil, err
	}
	return class.NewManager(classDefinitionList, constraintList)
}
