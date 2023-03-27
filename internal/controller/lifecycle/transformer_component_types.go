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
	"fmt"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

// TODO: status management
type Component interface {
	GetName() string
	GetNamespace() string
	GetClusterName() string
	GetWorkloadType() appsv1alpha1.WorkloadType

	GetDefinition() *appsv1alpha1.ClusterDefinition
	GetVersion() *appsv1alpha1.ClusterVersion
	GetCluster() *appsv1alpha1.Cluster
	GetSynthesizedComponent() *component.SynthesizedComponent

	GetPhase() appsv1alpha1.ClusterComponentPhase
	GetStatus() appsv1alpha1.ClusterComponentStatus

	// Exist checks whether the component exists in cluster
	Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error)

	Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	addResource(obj client.Object, action *Action, parent *lifecycleVertex) *lifecycleVertex
	addWorkload(obj client.Object, action *Action, parent *lifecycleVertex)
}

func NewComponent(definition *appsv1alpha1.ClusterDefinition,
	version *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	compName string,
	dag *graph.DAG) (Component, error) {
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

	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		return newComponent[replicationComponent](definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Consensus:
		return newComponent[consensusComponent](definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Stateful:
		return newComponent[statefulComponent](definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Stateless:
		return newComponent[statelessComponent](definition, cluster, compDef, compVer, compSpec, dag), nil
	}
	return nil, fmt.Errorf("unknown workload type: %s, cluster: %s, component: %s, component definition ref: %s",
		compDef.WorkloadType, cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
}

func newComponent[Tp statelessComponent | statefulComponent | replicationComponent | consensusComponent](
	definition *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compVer *appsv1alpha1.ClusterComponentVersion,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	dag *graph.DAG) *Tp {
	return &Tp{
		componentBase: componentBase{
			Definition:      definition,
			Cluster:         cluster,
			CompDef:         compDef,
			CompVer:         compVer,
			CompSpec:        compSpec,
			Component:       nil,
			workloadVertexs: make([]*lifecycleVertex, 0),
			dag:             dag,
		},
	}
}

type componentBase struct {
	Definition *appsv1alpha1.ClusterDefinition
	Version    *appsv1alpha1.ClusterVersion
	Cluster    *appsv1alpha1.Cluster

	// TODO: should remove those members in future.
	CompDef  *appsv1alpha1.ClusterComponentDefinition
	CompVer  *appsv1alpha1.ClusterComponentVersion
	CompSpec *appsv1alpha1.ClusterComponentSpec

	// built synthesized component
	Component *component.SynthesizedComponent

	// DAG vertex of main workload object(s)
	workloadVertexs []*lifecycleVertex

	dag *graph.DAG
}

func (c *componentBase) addResource(obj client.Object, action *Action, parent *lifecycleVertex) *lifecycleVertex {
	if obj == nil {
		panic("try to add nil object")
	}
	vertex := &lifecycleVertex{
		obj:    obj,
		action: action,
	}
	c.dag.AddVertex(vertex)

	if parent != nil {
		c.dag.Connect(parent, vertex)
	}
	return vertex
}

func (c *componentBase) addWorkload(obj client.Object, action *Action, parent *lifecycleVertex) {
	c.workloadVertexs = append(c.workloadVertexs, c.addResource(obj, action, parent))
}

func (c *componentBase) createResource(obj client.Object, parent *lifecycleVertex) *lifecycleVertex {
	return c.addResource(obj, actionPtr(CREATE), parent)
}

func (c *componentBase) deleteResource(obj client.Object, parent *lifecycleVertex) *lifecycleVertex {
	return c.addResource(obj, actionPtr(DELETE), parent)
}

func (c *componentBase) updateResource(obj client.Object, parent *lifecycleVertex) *lifecycleVertex {
	return c.addResource(obj, actionPtr(UPDATE), parent)
}

func (c *componentBase) GetName() string {
	return c.CompSpec.Name
}

func (c *componentBase) GetNamespace() string {
	return c.Cluster.Namespace
}

func (c *componentBase) GetClusterName() string {
	return c.Cluster.Name
}

func (c *componentBase) GetDefinition() *appsv1alpha1.ClusterDefinition {
	return c.Definition
}

func (c *componentBase) GetVersion() *appsv1alpha1.ClusterVersion {
	return c.Version
}

func (c *componentBase) GetCluster() *appsv1alpha1.Cluster {
	return c.Cluster
}

func (c *componentBase) GetSynthesizedComponent() *component.SynthesizedComponent {
	return c.Component
}

func (c *componentBase) GetPhase() appsv1alpha1.ClusterComponentPhase {
	return c.GetStatus().Phase // TODO: impl
}

func (c *componentBase) GetStatus() appsv1alpha1.ClusterComponentStatus {
	return appsv1alpha1.ClusterComponentStatus{} // TODO: impl
}

func (c *componentBase) updateService(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	labels := map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    c.GetClusterName(),
		constant.KBAppComponentLabelKey: c.GetName(),
	}
	svcObjList, err := listObjWithLabelsInNamespace(reqCtx, cli, generics.ServiceSignature, c.GetNamespace(), labels)
	if err != nil {
		return err
	}

	svcProtoList := findAll[*corev1.Service](c.dag)

	// create new services or update existed services
	for _, vertex := range svcProtoList {
		node, _ := vertex.(*lifecycleVertex)
		svcProto, _ := node.obj.(*corev1.Service)

		if pos := slices.IndexFunc(svcObjList, func(svc *corev1.Service) bool {
			return svc.GetName() == svcProto.GetName()
		}); pos < 0 {
			node.action = actionPtr(CREATE)
		} else {
			svcProto.Annotations = mergeServiceAnnotations(svcObjList[pos].Annotations, svcProto.Annotations)
			node.action = actionPtr(UPDATE)
		}
	}

	// delete useless services
	for _, svc := range svcObjList {
		if pos := slices.IndexFunc(svcProtoList, func(vertex graph.Vertex) bool {
			node, _ := vertex.(*lifecycleVertex)
			svcProto, _ := node.obj.(*corev1.Service)
			return svcProto.GetName() == svc.GetName()
		}); pos < 0 {
			c.deleteResource(svc, nil)
		}
	}
	return nil
}

func (c *componentBase) updateStatefulSetWorkload(stsObj *appsv1.StatefulSet, idx int32) error {
	stsObjCopy := stsObj.DeepCopy()
	stsProto := c.workloadVertexs[idx].obj.(*appsv1.StatefulSet)

	// keep the original template annotations.
	// if annotations exist and are replaced, the statefulSet will be updated.
	stsProto.Spec.Template.Annotations = mergeAnnotations(stsObj.Spec.Template.Annotations, stsProto.Spec.Template.Annotations)
	stsObjCopy.Spec.Template = stsProto.Spec.Template
	stsObjCopy.Spec.Replicas = stsProto.Spec.Replicas
	stsObjCopy.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
	if !reflect.DeepEqual(&stsObj.Spec, &stsObjCopy.Spec) {
		c.workloadVertexs[idx].action = actionPtr(UPDATE)
		// sync component phase
		updateComponentPhaseWithOperation(c.GetCluster(), c.GetName())
	}
	return nil
}

func (c *componentBase) updateDeploymentWorkload(deployObj *appsv1.Deployment) error {
	deployProto := c.workloadVertexs[0].obj.(*appsv1.Deployment)
	deployProto.Spec.Template.Annotations = mergeAnnotations(deployObj.Spec.Template.Annotations, deployProto.Spec.Template.Annotations)
	if !reflect.DeepEqual(&deployObj.Spec, &deployProto.Spec) {
		c.workloadVertexs[0].action = actionPtr(UPDATE)
		// sync component phase
		updateComponentPhaseWithOperation(c.Cluster, c.GetName())
	}
	return nil
}

func (c *componentBase) restartWorkload(podTemplate *corev1.PodTemplateSpec) error {
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = map[string]string{}
	}

	// startTimestamp := opsRes.OpsRequest.Status.StartTimestamp
	startTimestamp := time.Now() // TODO: impl
	restartTimestamp := podTemplate.Annotations[constant.RestartAnnotationKey]
	// if res, _ := time.Parse(time.RFC3339, restartTimestamp); startTimestamp.After(res) {
	if res, _ := time.Parse(time.RFC3339, restartTimestamp); startTimestamp.Before(res) {
		podTemplate.Annotations[constant.RestartAnnotationKey] = startTimestamp.Format(time.RFC3339)
	}

	return nil
}
