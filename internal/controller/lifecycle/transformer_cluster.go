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

package lifecycle

import (
	"encoding/json"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
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

	var (
		classDefinitionList appsv1alpha1.ComponentClassDefinitionList
	)

	ml := []client.ListOption{
		client.MatchingLabels{constant.ClusterDefLabelKey: transCtx.ClusterDef.Name},
	}
	if err := transCtx.Client.List(transCtx.Context, &classDefinitionList, ml...); err != nil {
		return err
	}
	clsMgr, err := class.NewManager(classDefinitionList)
	if err != nil {
		return err
	}

	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	// TODO: should move credential secrets creation from system_account_controller & here into credential_transformer,
	// TODO: as those secrets are owned by the cluster
	prepareComp := func(compSpec *appsv1alpha1.ClusterComponentSpec, synthesizedComp *component.SynthesizedComponent) error {
		iParams := task
		iParams.Component = synthesizedComp

		if err := prepareDedicatedComponent(cluster, compSpec, synthesizedComp, clsMgr); err != nil {
			return err
		}

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
		if err = plan.DoPITRPrepare(transCtx.Context, c.Client, cluster, synthesizedComp); err != nil {
			return err
		}

		return plan.PrepareComponentResources(reqCtx, c.Client, &iParams)
	}

	for _, compDef := range transCtx.ClusterDef.Spec.ComponentDefs {
		compDefName := compDef.Name
		compVer := clusterCompVerMap[compDefName]
		compSpecs := clusterCompSpecMap[compDefName]
		for _, compSpec := range compSpecs {
			if err := prepareComp(&compSpec, component.BuildComponent(reqCtx, *cluster, *transCtx.ClusterDef, compDef, compSpec, compVer)); err != nil {
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
	for k := range compBackupMap {
		if cluster.Spec.GetComponentByName(k) == nil {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeNotFound, "restore: not found componentSpecs[*].name %s", k)
		}
	}
	return compBackupMap, err
}

func prepareDedicatedComponent(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec, synthesizedComp *component.SynthesizedComponent, clsMgr *class.Manager) error {
	affinity := cluster.Spec.Affinity
	if compSpec.Affinity != nil {
		affinity = compSpec.Affinity
	}
	if affinity.Tenancy != appsv1alpha1.DedicatedNode {
		return nil
	}

	cls, err := clsMgr.ChooseClass(compSpec)
	if err != nil {
		return err
	}
	if cls == nil {
		// TODO reconsider handling policy for this case
		return nil
	}
	return fillDedicatedComponentAffinity(cluster, compSpec, synthesizedComp, cls)
}

func fillDedicatedComponentAffinity(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec, synthesizedComp *component.SynthesizedComponent, cls *class.ComponentClassWithRef) error {
	policies := cluster.Spec.ResourceAllocationPolicies
	if compSpec.ResourceAllocationPolicies != nil {
		policies = compSpec.ResourceAllocationPolicies
	}
	resources := getDedicatedResourceTotalAndLimit(policies, cls)

	podAntiAffinity := synthesizedComp.PodSpec.Affinity.PodAntiAffinity
	// add pod anti-affinity to ensure isolated with other pods
	var labelSelectorReqs []metav1.LabelSelectorRequirement
	labelSelectorReqs = append(labelSelectorReqs, metav1.LabelSelectorRequirement{
		Key:      constant.WorkloadTypeLabelKey,
		Operator: metav1.LabelSelectorOpIn,
		Values:   appsv1alpha1.WorkloadTypes,
	})
	podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
		podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, corev1.PodAffinityTerm{
			TopologyKey: corev1.LabelHostname,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: labelSelectorReqs,
			},
		},
	)

	normalizeQuantity := func(q resource.Quantity) string {
		return fmt.Sprintf("%d", q.MilliValue())
	}

	// if class is specified, add node affinity to ensure node resource match with the class
	var nodeSelectorTerms []corev1.NodeSelectorTerm
	nodeSelectorTerms = append(nodeSelectorTerms, corev1.NodeSelectorTerm{
		MatchExpressions: []corev1.NodeSelectorRequirement{
			{
				Key:      constant.ResourceCPULabelKey,
				Operator: "In",
				Values:   []string{normalizeQuantity(cls.CPU)},
			},
			{
				Key:      constant.ResourceMemoryLabelKey,
				Operator: "In",
				Values:   []string{normalizeQuantity(cls.Memory)},
			},
		},
	})
	if resources.CPUTotal.Cmp(cls.CPU) > 0 || resources.MemTotal.Cmp(cls.Memory) > 0 {
		nodeSelectorTerms = append(nodeSelectorTerms, corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      constant.ResourceCPULabelKey,
					Operator: "In",
					Values:   []string{normalizeQuantity(resources.CPUTotal)},
				},
				{
					Key:      constant.ResourceMemoryLabelKey,
					Operator: "In",
					Values:   []string{normalizeQuantity(resources.MemTotal)},
				},
			},
		})
		nodeSelectorTerms = append(nodeSelectorTerms, corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      constant.ResourceCPULabelKey,
					Operator: "Gt",
					Values:   []string{normalizeQuantity(cls.CPU)},
				},
				{
					Key:      constant.ResourceMemoryLabelKey,
					Operator: "Gt",
					Values:   []string{normalizeQuantity(cls.Memory)},
				},
				{
					Key:      constant.ResourceCPULabelKey,
					Operator: "Lt",
					Values:   []string{normalizeQuantity(resources.CPUTotal)},
				},
				{
					Key:      constant.ResourceMemoryLabelKey,
					Operator: "Lt",
					Values:   []string{normalizeQuantity(resources.MemTotal)},
				},
			},
		})
	}
	nodeAffinity := &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: nodeSelectorTerms,
		},
	}
	nodeAffinity.DeepCopyInto(synthesizedComp.PodSpec.Affinity.NodeAffinity)
	return nil
}

var _ graph.Transformer = &ClusterTransformer{}
