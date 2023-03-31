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
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// componentContext wrapper for handling component status procedure context parameters.
type componentContext struct {
	reqCtx        intctrlutil.RequestCtx
	cli           client.Client
	recorder      record.EventRecorder
	component     types.Component
	obj           client.Object
	componentSpec *appsv1alpha1.ClusterComponentSpec
}

// NewComponentByType creates a component object.
func NewComponentByType(
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef appsv1alpha1.ClusterComponentDefinition,
) (types.Component, error) {
	if err := util.ComponentRuntimeReqArgsCheck(cli, cluster, component); err != nil {
		return nil, err
	}
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return consensusset.NewConsensusSet(cli, cluster, component, componentDef)
	case appsv1alpha1.Replication:
		return replicationset.NewReplicationSet(cli, cluster, component, componentDef)
	case appsv1alpha1.Stateful:
		return stateful.NewStateful(cli, cluster, component, componentDef)
	case appsv1alpha1.Stateless:
		return stateless.NewStateless(cli, cluster, component, componentDef)
	default:
		panic("unknown workload type")
	}
}

// newComponentContext creates a componentContext object.
func newComponentContext(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	recorder record.EventRecorder,
	component types.Component,
	obj client.Object,
	componentSpec *appsv1alpha1.ClusterComponentSpec) componentContext {

	return componentContext{
		reqCtx:        reqCtx,
		cli:           cli,
		recorder:      recorder,
		component:     component,
		obj:           obj,
		componentSpec: componentSpec,
	}
}
