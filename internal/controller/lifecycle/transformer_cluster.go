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
	"encoding/json"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ClusterTransformer builds a Cluster into K8s objects and put them into a DAG
// TODO: remove cli and ctx, we should read all objects needed, and then do pure objects computation
// TODO: only replication set left
type ClusterTransformer struct {
	client.Client
}

func (c *ClusterTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	origCluster := transCtx.OrigCluster
	cluster := transCtx.Cluster
	if isClusterDeleting(*origCluster) {
		return nil
	}

	// we copy the K8s objects prepare stage directly first
	// TODO: refactor plan.PrepareComponentResources
	resourcesQueue := make([]client.Object, 0, 3)
	task := intctrltypes.ReconcileTask{
		Cluster:           cluster,
		ClusterDefinition: transCtx.ClusterDef,
		ClusterVersion:    transCtx.ClusterVer,
		Resources:         &resourcesQueue,
	}

	clusterBackupResourceMap, err := getClusterBackupSourceMap(cluster)
	if err != nil {
		return err
	}

	clusterCompSpecMap := cluster.Spec.GetDefNameMappingComponents()
	clusterCompVerMap := transCtx.ClusterVer.Spec.GetDefNameMappingComponents()
	process1stComp := true

	reqCtx := intctrlutil.RequestCtx{
		Ctx: transCtx.Context,
		Log: transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	// TODO: should move credential secrets creation from system_account_controller & here into credential_transformer,
	// TODO: as those secrets are owned by the cluster
	prepareComp := func(synthesizedComp *component.SynthesizedComponent) error {
		iParams := task
		iParams.Component = synthesizedComp
		if process1stComp && len(synthesizedComp.Services) > 0 {
			if err := prepareConnCredential(&iParams); err != nil {
				return err
			}
			process1stComp = false
		}

		// build info that needs to be restored from backup
		backupSourceName := clusterBackupResourceMap[synthesizedComp.Name]
		if len(backupSourceName) > 0 {
			backup, backupTool, err := getBackupObjects(transCtx.Context, c.Client, cluster.Namespace, backupSourceName)
			if err != nil {
				return err
			}
			if err := component.BuildRestoredInfo2(synthesizedComp, backup, backupTool); err != nil {
				return err
			}
		}
		return plan.PrepareComponentResources(reqCtx, c.Client, &iParams)
	}

	for _, compDef := range transCtx.ClusterDef.Spec.ComponentDefs {
		compDefName := compDef.Name
		compVer := clusterCompVerMap[compDefName]
		compSpecs := clusterCompSpecMap[compDefName]
		for _, compSpec := range compSpecs {
			if err := prepareComp(component.BuildComponent(reqCtx, *cluster, *transCtx.ClusterDef, compDef, compSpec, compVer)); err != nil {
				return err
			}
		}
	}

	// replication set will create duplicate env configmap and headless service
	// dedup them
	root, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	objects := deDupResources(*task.Resources)
	// now task.Resources to DAG vertices
	for _, object := range objects {
		vertex := &lifecycleVertex{obj: object}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
	}
	return nil
}

func deDupResources(resources []client.Object) []client.Object {
	objects := make([]client.Object, 0)
	for _, resource := range resources {
		contains := false
		for _, object := range objects {
			if reflect.DeepEqual(resource, object) {
				contains = true
				break
			}
		}
		if !contains {
			objects = append(objects, resource)
		}
	}
	return objects
}

func prepareConnCredential(task *intctrltypes.ReconcileTask) error {
	secret, err := builder.BuildConnCredential(task.GetBuilderParams())
	if err != nil {
		return err
	}
	// must make sure secret resources are created before workloads resources
	task.AppendResource(secret)
	return nil
}

// getClusterBackupSourceMap gets the backup source map from cluster.annotations
func getClusterBackupSourceMap(cluster *appsv1alpha1.Cluster) (map[string]string, error) {
	compBackupMapString := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	if len(compBackupMapString) == 0 {
		return nil, nil
	}
	compBackupMap := map[string]string{}
	err := json.Unmarshal([]byte(compBackupMapString), &compBackupMap)
	return compBackupMap, err
}

var _ graph.Transformer = &ClusterTransformer{}
