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

package consensusset

import (
	"reflect"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

type ObjectGenerationTransformer struct {}

func (t *ObjectGenerationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	// get root vertex(i.e. consensus set)
	root, err := model.FindRootVertex(dag)
	if err != nil {
		return err
	}
	csSet, _ := root.Obj.(*workloads.ConsensusSet)
	oriSet, _ := root.OriObj.(*workloads.ConsensusSet)

	// generate objects by current spec
	svc := builder.NewServiceBuilder(csSet.Namespace, csSet.Namespace).
		AddLabels(constant.AppInstanceLabelKey, csSet.Name).
		AddLabels(constant.KBManagedByKey, ConsensusSetKind).
		// AddAnnotationsInMap(csSet.Annotations).
		AddSelectors(constant.AppInstanceLabelKey, csSet.Name).
		AddSelectors(constant.KBManagedByKey, ConsensusSetKind).
		AddPorts(csSet.Spec.Service.Ports...).
		SetType(csSet.Spec.Service.Type).
		GetObject()
	hdlBuilder := builder.NewHeadlessServiceBuilder(csSet.Namespace, csSet.Name+"-headless").
		AddLabels(constant.AppInstanceLabelKey, csSet.Name).
		AddLabels(constant.KBManagedByKey, ConsensusSetKind).
		AddSelectors(constant.AppInstanceLabelKey, csSet.Name).
		AddSelectors(constant.KBManagedByKey, ConsensusSetKind)
	//	.AddAnnotations("prometheus.io/scrape", strconv.FormatBool(component.Monitor.Enable))
	// if component.Monitor.Enable {
	//	hdBuilder.AddAnnotations("prometheus.io/path", component.Monitor.ScrapePath).
	//		AddAnnotations("prometheus.io/port", strconv.Itoa(int(component.Monitor.ScrapePort))).
	//		AddAnnotations("prometheus.io/scheme", "http")
	// }
	for _, container := range csSet.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			servicePort := corev1.ServicePort{
				Name:       port.Name,
				Protocol:   port.Protocol,
				Port:       port.ContainerPort,
				TargetPort: intstr.FromString(port.Name),
			}
			hdlBuilder.AddPorts(servicePort)
		}
	}
	headLessSvc := hdlBuilder.GetObject()

	stsBuilder := builder.NewStatefulSetBuilder(csSet.Namespace, csSet.Name)
	stsBuilder.AddLabels(constant.AppInstanceLabelKey, csSet.Name).
		AddLabels(constant.KBManagedByKey, ConsensusSetKind).
		AddMatchLabel(constant.AppInstanceLabelKey, csSet.Name).
		AddMatchLabel(constant.KBManagedByKey, ConsensusSetKind).
		SetServiceName(headLessSvc.Name).
		SetReplicas(csSet.Spec.Replicas).
		SetMinReadySeconds(10).
		SetPodManagementPolicy(apps.ParallelPodManagement).
		SetVolumeClaimTemplates(csSet.Spec.VolumeClaimTemplates...).
		SetTemplate(csSet.Spec.Template).
		SetUpdateStrategyType(apps.OnDeleteStatefulSetStrategyType)
	sts := stsBuilder.GetObject()
	// TODO: builds env config map

	// put all objects into the dag
	svcVertex := &model.ObjectVertex{Obj: svc}
	headlessSvcVertex := &model.ObjectVertex{Obj: headLessSvc}
	stsVertex := &model.ObjectVertex{Obj: sts}
	dag.AddVertex(svcVertex)
	dag.AddVertex(headlessSvcVertex)
	dag.AddVertex(stsVertex)
	dag.Connect(root, svcVertex)
	dag.Connect(root, headlessSvcVertex)
	dag.Connect(root, stsVertex)
	dag.Connect(stsVertex, svcVertex)
	dag.Connect(stsVertex, headlessSvcVertex)

	// read cache snapshot
	oldSnapshot, err := t.readCacheSnapshot(csSet)
	if err != nil {
		return err
	}

	// compute create/update/delete set
	// we have the target objects snapshot in dag
	allNoneRootVertices := model.FindAllNot[*workloads.ConsensusSet](dag)
	newNameVertices := make(map[model.GVKName]graph.Vertex)
	for _, vertex := range allNoneRootVertices {
		v, _ := vertex.(*model.ObjectVertex)
		name, err := model.GetGVKName(v.Obj)
		if err != nil {
			return err
		}
		newNameVertices[*name] = vertex
	}

	// now compute the diff between old and target snapshot and generate the plan
	oldNameSet := sets.KeySet(oldSnapshot)
	newNameSet := sets.KeySet(newNameVertices)

	createSet := newNameSet.Difference(oldNameSet)
	updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)

	createNewVertices := func() {
		for name := range createSet {
			v, _ := newNameVertices[name].(*model.ObjectVertex)
			if v.Action == nil {
				v.Action = model.ActionPtr(model.CREATE)
			}
		}
	}
	updateVertices := func() {
		for name := range updateSet {
			v, _ := newNameVertices[name].(*model.ObjectVertex)
			v.OriObj = oldSnapshot[name]
			if v.Action == nil || *v.Action != model.DELETE {
				v.Action = model.ActionPtr(model.UPDATE)
			}
		}
	}
	deleteOrphanVertices := func() {
		for name := range deleteSet {
			v := &model.ObjectVertex{
				Obj:      oldSnapshot[name],
				OriObj:   oldSnapshot[name],
				IsOrphan: true,
				Action:   model.ActionPtr(model.DELETE),
			}
			dag.AddVertex(v)
			dag.Connect(root, v)
		}
	}

	// update dag by root vertex's status
	switch {
	case model.IsObjectDeleting(oriSet):
		for _, vertex := range dag.Vertices() {
			v, _ := vertex.(*model.ObjectVertex)
			v.Action = model.ActionPtr(model.DELETE)
		}
		deleteOrphanVertices()
	case model.IsObjectStatusUpdating(oriSet):
		defer func() {
			vertices := model.FindAllNot[*workloads.ConsensusSet](dag)
			for _, vertex := range vertices {
				v, _ := vertex.(*model.ObjectVertex)
				// TODO: fix me, workaround for h-scaling to update stateful set
				if _, ok := v.Obj.(*apps.StatefulSet); !ok {
					v.Immutable = true
				}
			}
		}()
		fallthrough
	case model.IsObjectUpdating(oriSet):
		// vertices to be created
		createNewVertices()
		// vertices to be updated
		updateVertices()
		// vertices to be deleted
		deleteOrphanVertices()
	}

	return nil
}

func ownKinds() []client.ObjectList {
	return []client.ObjectList{
		&apps.StatefulSetList{},
		&corev1.ServiceList{},
	}
}

// read all objects owned by our cluster
func (t *ObjectGenerationTransformer) readCacheSnapshot(owner client.Object) (model.ObjectSnapshot, error) {
	// list what kinds of object cluster owns
	kinds := ownKinds()
	snapshot := make(model.ObjectSnapshot)
	ml := client.MatchingLabels{constant.AppInstanceLabelKey: owner.GetName()}
	inNS := client.InNamespace(owner.GetNamespace())
	for _, list := range kinds {
		if err := t.cli.List(t.ctx, list, inNS, ml); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			// put to snapshot if owned by our cluster
			if model.IsOwnerOf(owner, object) {
				name, err := model.GetGVKName(object)
				if err != nil {
					return nil, err
				}
				snapshot[*name] = object
			}
		}
	}

	return snapshot, nil
}

var _ graph.Transformer = &ObjectGenerationTransformer{}
