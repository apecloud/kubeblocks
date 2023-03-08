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

package lifecycle

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// clusterTransformer transforms a Cluster to a K8s objects DAG
// TODO: remove cli and ctx, we should read all objects needed, and then do pure objects computation
type clusterTransformer struct {
	cc  compoundCluster
	cli client.Client
	ctx intctrlutil.RequestCtx
}

func (c *clusterTransformer) Transform(dag *graph.DAG) error {
	// put the cluster object first, it will be root vertex of DAG
	patch := client.MergeFrom(c.cc.cluster.DeepCopy())
	rootVertex := &lifecycleVertex{obj: c.cc.cluster, patch: patch}
	dag.AddVertex(rootVertex)

	// we copy the K8s objects prepare stage directly first
	// TODO: refactor plan.PrepareComponentResources
	resourcesQueue := make([]client.Object, 0, 3)
	task := intctrltypes.ReconcileTask{
		Cluster:           c.cc.cluster,
		ClusterDefinition: &c.cc.cd,
		ClusterVersion:    &c.cc.cv,
		Resources:         &resourcesQueue,
	}

	secret, err := builder.BuildConnCredential(task.GetBuilderParams())
	if err != nil {
		return err
	}
	secretVertex := &lifecycleVertex{obj: secret}
	dag.AddVertex(secretVertex)
	dag.Connect(rootVertex, secretVertex)

	clusterCompSpecMap := c.cc.cluster.GetDefNameMappingComponents()
	clusterCompVerMap := c.cc.cv.GetDefNameMappingComponents()

	prepareComp := func(component *component.SynthesizedComponent) error {
		iParams := task
		iParams.Component = component
		return plan.PrepareComponentResources(c.ctx, c.cli, &iParams)
	}

	for _, compDef := range c.cc.cd.Spec.ComponentDefs {
		compDefName := compDef.Name
		compVer := clusterCompVerMap[compDefName]
		compSpecs := clusterCompSpecMap[compDefName]
		for _, compSpec := range compSpecs {
			if err := prepareComp(component.BuildComponent(c.ctx, *c.cc.cluster, c.cc.cd, compDef, compSpec, compVer)); err != nil {
				return err
			}
		}
	}

	// now task.Resources to DAG vertices
	for _, object := range *task.Resources {
		vertex := &lifecycleVertex{obj: object}
		dag.AddVertex(vertex)
		dag.Connect(rootVertex, vertex)
	}
	return nil
}
