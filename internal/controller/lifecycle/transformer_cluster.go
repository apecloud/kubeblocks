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
	"reflect"

	corev1 "k8s.io/api/core/v1"
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

		if err := fillComponentClass(cluster, compSpec, synthesizedComp, clsMgr); err != nil {
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

func fillComponentClass(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec, synthesizedComp *component.SynthesizedComponent, clsMgr *class.Manager) error {
	cls, err := clsMgr.ChooseClass(compSpec)
	if err != nil {
		return err
	}
	if cls == nil {
		// TODO reconsider handling policy for this case
		return nil
	}
	compSpec.ClassDefRef = &cls.ClassDefRef

	// update pod resource requests and limits
	requests := corev1.ResourceList{
		corev1.ResourceCPU:    cls.CPU,
		corev1.ResourceMemory: cls.Memory,
	}
	synthesizedComp.PodSpec.Containers[0].Resources = corev1.ResourceRequirements{
		Requests: requests,
		Limits:   requests,
	}
	var volumes []corev1.PersistentVolumeClaimTemplate
	if len(synthesizedComp.VolumeClaimTemplates) > 0 {
		volumes = synthesizedComp.VolumeClaimTemplates
	} else {
		for _, volume := range cls.Volumes {
			volumes = append(volumes, corev1.PersistentVolumeClaimTemplate{
				Spec: corev1.PersistentVolumeClaimSpec{
					// TODO define access mode in class
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: volume.Size,
						},
					},
				},
			})
		}
	}
	synthesizedComp.VolumeClaimTemplates = volumes

	// update tenancy affinity
	affinity := cluster.Spec.Affinity
	if compSpec.Affinity != nil {
		affinity = compSpec.Affinity
	}

	podAntiAffinity := synthesizedComp.PodSpec.Affinity.PodAntiAffinity
	// add pod anti-affinity to ensure isolated with other pods
	if affinity.Tenancy == appsv1alpha1.DedicatedNode {
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
	}

	// if class is specified, add node affinity to ensure node resource match with the class
	nodeAffinity := synthesizedComp.PodSpec.Affinity.NodeAffinity
	if affinity.Tenancy == appsv1alpha1.DedicatedNode && cls != nil {
		var nodeSelectorTerms []corev1.NodeSelectorTerm
		nodeSelectorTerms = append(nodeSelectorTerms, []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      constant.ResourceCPULabelKey,
						Operator: "In",
						Values:   []string{cls.CPU.AsDec().String()},
					},
					{
						Key:      constant.ResourceMemoryLabelKey,
						Operator: "In",
						Values:   []string{cls.Memory.AsDec().String()},
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      constant.ResourceCPULabelKey,
						Operator: "Gt",
						Values:   []string{cls.CPU.AsDec().String()},
					},
					{
						Key:      constant.ResourceMemoryLabelKey,
						Operator: "Gt",
						Values:   []string{cls.Memory.AsDec().String()},
					},
				},
			},
		}...)
		nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(
			nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, nodeSelectorTerms...)
	}
	return nil
}

func fillDedicatedComponentResources(cluster *appsv1alpha1.Cluster, synthesizedComp *component.SynthesizedComponent, cls *class.ComponentClassWithRef) {
	requests := corev1.ResourceList{
		corev1.ResourceCPU:    cls.CPU,
		corev1.ResourceMemory: cls.Memory,
	}

	policy, ok := synthesizedComp.ResourceAllocationPolicies[corev1.ResourceCPU]
	if ok && policy.OverAllocationRatio != nil {
		requests.Cpu()
	}
}

func fillSharedComponentResources(cluster *appsv1alpha1.Cluster, synthesizedComp *component.SynthesizedComponent, cls *class.ComponentClassWithRef) {

}

var _ graph.Transformer = &ClusterTransformer{}
