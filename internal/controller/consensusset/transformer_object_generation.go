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

package consensusset

import (
	"fmt"
	"strconv"
	"strings"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	"github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ObjectGenerationTransformer struct{}

func (t *ObjectGenerationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	csSet := transCtx.CSSet
	oriSet := transCtx.OrigCSSet

	if model.IsObjectDeleting(oriSet) {
		return nil
	}

	// get root vertex(i.e. consensus set)
	root, err := model.FindRootVertex(dag)
	if err != nil {
		return err
	}

	// generate objects by current spec
	svc := builder.NewServiceBuilder(csSet.Namespace, csSet.Name).
		AddLabels(constant.AppInstanceLabelKey, csSet.Name).
		AddLabels(constant.KBManagedByKey, ConsensusSetKind).
		// AddAnnotationsInMap(csSet.Annotations).
		AddSelectors(constant.AppInstanceLabelKey, csSet.Name).
		AddSelectors(constant.KBManagedByKey, ConsensusSetKind).
		AddPorts(csSet.Spec.Service.Ports...).
		SetType(csSet.Spec.Service.Type).
		GetObject()
	hdlBuilder := builder.NewHeadlessServiceBuilder(csSet.Namespace, getHeadlessSvcName(*csSet)).
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
	template := csSet.Spec.Template
	labels := template.Labels
	if labels == nil {
		labels = make(map[string]string, 2)
	}
	labels[constant.AppInstanceLabelKey] = csSet.Name
	labels[constant.KBManagedByKey] = ConsensusSetKind
	template.Labels = labels
	stsBuilder.AddLabels(constant.AppInstanceLabelKey, csSet.Name).
		AddLabels(constant.KBManagedByKey, ConsensusSetKind).
		AddMatchLabel(constant.AppInstanceLabelKey, csSet.Name).
		AddMatchLabel(constant.KBManagedByKey, ConsensusSetKind).
		SetServiceName(headLessSvc.Name).
		SetReplicas(csSet.Spec.Replicas).
		SetMinReadySeconds(10).
		SetPodManagementPolicy(apps.ParallelPodManagement).
		SetVolumeClaimTemplates(csSet.Spec.VolumeClaimTemplates...).
		SetTemplate(template).
		SetUpdateStrategyType(apps.OnDeleteStatefulSetStrategyType)
	sts := stsBuilder.GetObject()

	envData := buildEnvConfigData(*csSet)
	envConfig := builder.NewConfigMapBuilder(csSet.Namespace, csSet.Name+"-env").
		AddLabels(constant.AppInstanceLabelKey, csSet.Name).
		AddLabels(constant.KBManagedByKey, ConsensusSetKind).
		SetData(envData).GetObject()

	// put all objects into the dag
	vertices := make([]*model.ObjectVertex, 0)
	svcVertex := &model.ObjectVertex{Obj: svc}
	headlessSvcVertex := &model.ObjectVertex{Obj: headLessSvc}
	stsVertex := &model.ObjectVertex{Obj: sts}
	envConfigVertex := &model.ObjectVertex{Obj: envConfig}
	vertices = append(vertices, svcVertex, headlessSvcVertex, stsVertex, envConfigVertex)
	for _, vertex := range vertices {
		if err := controllerutil.SetOwnership(csSet, vertex.Obj, model.GetScheme(), CSSetFinalizerName); err != nil {
			return err
		}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
	}
	dag.Connect(stsVertex, svcVertex)
	dag.Connect(stsVertex, headlessSvcVertex)
	dag.Connect(stsVertex, envConfigVertex)

	// read cache snapshot
	oldSnapshot, err := model.ReadCacheSnapshot(ctx, csSet, ownedKinds()...)
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

func getHeadlessSvcName(set workloads.ConsensusSet) string {
	return strings.Join([]string{set.Name, "headless"}, "-")
}

func buildEnvConfigData(set workloads.ConsensusSet) map[string]string {
	envData := map[string]string{}

	prefix := constant.KBPrefix + "_" + strings.ToUpper(set.Name) + "_"
	prefix = strings.ReplaceAll(prefix, "-", "_")
	svcName := getHeadlessSvcName(set)
	envData[prefix+"N"] = strconv.Itoa(int(set.Spec.Replicas))
	for i := 0; i < int(set.Spec.Replicas); i++ {
		hostNameTplKey := prefix + strconv.Itoa(i) + "_HOSTNAME"
		hostNameTplValue := set.Name + "-" + strconv.Itoa(i)
		envData[hostNameTplKey] = fmt.Sprintf("%s.%s", hostNameTplValue, svcName)
	}

	// build consensus env from set.status
	podName := set.Status.Leader.PodName
	if podName != "" && podName != DefaultPodName {
		envData[prefix+"LEADER"] = podName
	}
	followers := ""
	for _, follower := range set.Status.Followers {
		podName = follower.PodName
		if podName == "" || podName == DefaultPodName {
			continue
		}
		if len(followers) > 0 {
			followers += ","
		}
		followers += podName
	}
	if followers != "" {
		envData[prefix+"FOLLOWERS"] = followers
	}

	// set owner uid to let pod know if the owner is recreated
	envData[prefix+"OWNER_UID"] = string(set.UID)

	return envData
}

var _ graph.Transformer = &ObjectGenerationTransformer{}
