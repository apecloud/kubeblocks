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
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensus"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replication"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// NewComponentByType creates a component object.
func NewComponentByType(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	compDef appsv1alpha1.ClusterComponentDefinition,
	dag *graph.DAG) (types.ComponentSet, error) {
	if err := util.ComponentRuntimeReqArgsCheck(cli, cluster, compSpec); err != nil {
		return nil, err
	}
	switch compDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return consensusset.NewConsensusSet(cli, cluster, compSpec, compDef)
	case appsv1alpha1.Replication:
		return replicationset.NewReplicationSet(cli, cluster, compSpec, compDef)
	case appsv1alpha1.Stateful:
		return stateful.NewStateful(cli, cluster, compSpec, compDef)
	case appsv1alpha1.Stateless:
		return stateless.NewStateless(cli, cluster, compSpec, compDef)
	default:
		panic("unknown workload type")
	}
}

func NewComponent(cli client.Client, definition *appsv1alpha1.ClusterDefinition, version *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster, compName string, dag *graph.DAG) (types.Component, error) {
	var compDef *appsv1alpha1.ClusterComponentDefinition
	var compVer *appsv1alpha1.ClusterComponentVersion
	compSpec := cluster.GetComponentByName(compName)
	if compSpec != nil {
		compDef = definition.GetComponentDefByName(compSpec.ComponentDefRef)
		if compDef == nil {
			return nil, fmt.Errorf("referenced component definition is not exist, cluster: %s, component: %s, component definition ref:%s",
				cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
		}
		if version != nil {
			compVer = version.GetDefNameMappingComponents()[compSpec.ComponentDefRef]
		}
	}

	if compSpec == nil || compDef == nil {
		// TODO(refactor): fix me
		return nil, fmt.Errorf("NotSupported")
	}

	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		return replication.NewReplicationComponent(cli, definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Consensus:
		return consensus.NewConsensusComponent(cli, definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Stateful:
		return stateful.NewStatefulComponent(cli, definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Stateless:
		return stateless.NewStatelessComponent(cli, definition, cluster, compDef, compVer, compSpec, dag), nil
	}
	return nil, fmt.Errorf("unknown workload type: %s, cluster: %s, component: %s, component definition ref: %s",
		compDef.WorkloadType, cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
}
