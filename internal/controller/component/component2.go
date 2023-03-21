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

package component

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Component interface {
	GetName() string
	GetWorkloadType() appsv1alpha1.WorkloadType

	Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error)

	Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	// horizontal and vertical scaling
	HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error
}

func NewComponent(definition appsv1alpha1.ClusterDefinition,
	version appsv1alpha1.ClusterVersion,
	cluster appsv1alpha1.Cluster,
	compSpec appsv1alpha1.ClusterComponentSpec,
	dag *graph.DAG) Component {
	compDef := (&definition).GetComponentDefByName(compSpec.ComponentDefRef)
	if compDef == nil {
		return nil
	}

	switch compDef.WorkloadType {
	case appsv1alpha1.Stateless:
		return NewStatelessComponent(definition, version, cluster, compSpec, dag)
	case appsv1alpha1.Stateful:
		return NewStatefulComponent(definition, version, cluster, compSpec, dag)
	case appsv1alpha1.Consensus:
		return NewConsensusComponent(definition, version, cluster, compSpec, dag)
	case appsv1alpha1.Replication:
		return NewReplicationComponent(definition, version, cluster, compSpec, dag)
	}
	return nil
}

type componentBase struct {
	Definition appsv1alpha1.ClusterDefinition
	Cluster    appsv1alpha1.Cluster

	CompDef  appsv1alpha1.ClusterComponentDefinition
	CompVer  *appsv1alpha1.ClusterComponentVersion // optional
	CompSpec appsv1alpha1.ClusterComponentSpec

	// built synthesized component
	Component *SynthesizedComponent

	// DAG vertex of main workload object
	WorkloadVertex *lifecycleVertex

	Dag *graph.DAG
}

// TODO: replace it with lifecycle's definition
type lifecycleVertex struct {
	obj       client.Object
	oriObj    client.Object
	immutable bool
	action    lifecycle.Action
}

func (c *componentBase) addResource(obj client.Object, action lifecycle.Action, parent *lifecycleVertex) *lifecycleVertex {
	vertex := &lifecycleVertex{
		obj:    obj,
		action: action,
	}
	c.Dag.AddVertex(vertex)

	if parent != nil {
		c.Dag.Connect(parent, vertex)
	}

	return vertex
}

func (c *componentBase) createResource(obj client.Object, parent *lifecycleVertex) *lifecycleVertex {
	return c.addResource(obj, lifecycle.CREATE, parent)
}

func (c *componentBase) deleteResource(obj client.Object, parent *lifecycleVertex) *lifecycleVertex {
	return c.addResource(obj, lifecycle.DELETE, parent)
}

func (c *componentBase) updateResource(obj client.Object, parent *lifecycleVertex) *lifecycleVertex {
	return c.addResource(obj, lifecycle.UPDATE, parent)
}
