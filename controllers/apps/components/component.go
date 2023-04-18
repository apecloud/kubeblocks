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

package components

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensus1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replication"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// PodIsAvailable checks whether a pod is available respect to the workload type.
// Deprecated: provide for ops request using, remove this interface later.
func PodIsAvailable(workloadType appsv1alpha1.WorkloadType, pod *corev1.Pod, minReadySeconds int32) bool {
	return util.PodIsAvailable(workloadType, pod, minReadySeconds)
}

func NewComponent(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	definition *appsv1alpha1.ClusterDefinition,
	version *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	compName string,
	dag *graph.DAG) (types.Component, error) {
	var compDef *appsv1alpha1.ClusterComponentDefinition
	var compVer *appsv1alpha1.ClusterComponentVersion
	compSpec := cluster.Spec.GetComponentByName(compName)
	if compSpec != nil {
		compDef = definition.GetComponentDefByName(compSpec.ComponentDefRef)
		if compDef == nil {
			return nil, fmt.Errorf("referenced component definition is not exist, cluster: %s, component: %s, component definition ref:%s",
				cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
		}
		if version != nil {
			compVer = version.Spec.GetDefNameMappingComponents()[compSpec.ComponentDefRef]
		}
	}

	if compSpec == nil || compDef == nil {
		// TODO(refactor): fix me
		return nil, fmt.Errorf("NotSupported")
	}

	synthesizedComp, err := composeSynthesizedComponent(reqCtx, cli, cluster, definition, compDef, compSpec, compVer)
	if err != nil {
		return nil, err
	}

	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		return replication.NewReplicationComponent(cli, cluster, version, synthesizedComp, dag), nil
	case appsv1alpha1.Consensus:
		return consensus1.NewConsensusComponent(cli, cluster, version, synthesizedComp, dag), nil
	case appsv1alpha1.Stateful:
		return stateful1.NewStatefulComponent(cli, cluster, version, synthesizedComp, dag), nil
	case appsv1alpha1.Stateless:
		return stateless.NewStatelessComponent(cli, cluster, version, synthesizedComp, dag), nil
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
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, cli, *cluster, *clusterDef, *compDef, *compSpec, compVer)
	if err != nil {
		return nil, err
	}
	return synthesizedComp, nil
}
