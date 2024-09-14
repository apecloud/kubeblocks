/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/apis/workloads/legacy"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// componentWorkloadUpgradeTransformer upgrades the underlying workload from the legacy RSM API to the InstanceSet API.
type componentWorkloadUpgradeTransformer struct{}

var _ graph.Transformer = &componentWorkloadUpgradeTransformer{}

func (t *componentWorkloadUpgradeTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component
	synthesizeComp := transCtx.SynthesizeComponent
	compDef := transCtx.CompDef

	if model.IsObjectDeleting(comp) {
		return nil
	}

	var parent *model.ObjectVertex
	legacyFound := false

	// update pod & pvc & svc labels
	objectList := []client.ObjectList{&corev1.PersistentVolumeClaimList{}, &corev1.PodList{}, &corev1.ServiceList{}, &corev1.ConfigMapList{}}
	ml := constant.GetComponentWellKnownLabels(synthesizeComp.ClusterName, synthesizeComp.Name)
	inNS := client.InNamespace(comp.Namespace)
	defaultHeadlessSvc := constant.GenerateDefaultComponentHeadlessServiceName(synthesizeComp.ClusterName, synthesizeComp.Name)
	envCM := instanceset.GetEnvConfigMapName(constant.GenerateClusterComponentName(synthesizeComp.ClusterName, synthesizeComp.Name))
	var revision string
	for _, list := range objectList {
		if err := graphCli.List(transCtx.Context, list, client.MatchingLabels(ml), inNS); err == nil {
			items := reflect.ValueOf(list).Elem().FieldByName("Items")
			l := items.Len()
			for i := 0; i < l; i++ {
				object := items.Index(i).Addr().Interface().(client.Object)
				if _, ok := object.GetLabels()[instanceset.WorkloadsManagedByLabelKey]; ok {
					continue
				}
				_, isSvc := object.(*corev1.Service)
				if isSvc && object.GetName() != defaultHeadlessSvc {
					continue
				}
				_, isCM := object.(*corev1.ConfigMap)
				if isCM && object.GetName() != envCM {
					continue
				}
				legacyFound = true

				// fix labels
				object.GetLabels()[instanceset.WorkloadsManagedByLabelKey] = workloads.Kind
				object.GetLabels()[instanceset.WorkloadsInstanceLabelKey] = comp.Name

				// fix labels, ownerReference and finalizer of Service and ConfigMap:
				// assume all the OwnerReferences and Finalizers were set by the KubeBlocks.
				// set them to empty and the InstanceSet Controller will fix them.
				if isSvc || isCM {
					object.SetOwnerReferences([]metav1.OwnerReference{})
					object.SetFinalizers([]string{})
					delete(object.GetLabels(), constant.AppManagedByLabelKey)
					delete(object.GetLabels(), constant.AppNameLabelKey)
					delete(object.GetLabels(), constant.AppComponentLabelKey)
					delete(object.GetLabels(), constant.AppInstanceLabelKey)
					delete(object.GetLabels(), constant.KBAppComponentLabelKey)
				}

				// fix revision of Pods
				if _, ok := object.(*corev1.Pod); ok {
					if revision == "" {
						revision, err = buildRevision(synthesizeComp, compDef)
						if err != nil {
							return err
						}
					}
					object.GetLabels()[appsv1.ControllerRevisionHashLabelKey] = revision
				}
				parent = graphCli.Do(dag, nil, object, model.ActionUpdatePtr(), parent)
			}
		}
	}

	// remove the StatefulSet object if found
	sts := &appsv1.StatefulSet{}
	if err := graphCli.Get(transCtx.Context, client.ObjectKeyFromObject(comp), sts); err == nil {
		legacyFound = true
		parent = graphCli.Do(dag, nil, sts, model.ActionDeletePtr(), parent, model.WithPropagationPolicy(client.PropagationPolicy(metav1.DeletePropagationOrphan)))
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	// remove the RSM object if found
	exists, err := legacyCRDExists(transCtx.Context, graphCli)
	if err != nil {
		return err
	}
	if exists {
		rsm := &legacy.ReplicatedStateMachine{}
		if err := graphCli.Get(transCtx.Context, client.ObjectKeyFromObject(comp), rsm); err == nil {
			legacyFound = true
			graphCli.Do(dag, nil, rsm, model.ActionDeletePtr(), parent, model.WithPropagationPolicy(client.PropagationPolicy(metav1.DeletePropagationOrphan)))
		} else if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if legacyFound {
		// set status.observedGeneration to zero to trigger a creation reconciliation loop of the component controller.
		comp.Status.ObservedGeneration = 0
		return graph.ErrPrematureStop
	}
	return nil
}

func legacyCRDExists(ctx context.Context, cli model.GraphClient) (bool, error) {
	crdName := "replicatedstatemachines.workloads.kubeblocks.io"
	crd := &apiextv1.CustomResourceDefinition{}
	err := cli.Get(ctx, client.ObjectKey{Name: crdName}, crd)
	if err == nil {
		return true, nil
	}
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

func buildRevision(synthesizeComp *component.SynthesizedComponent, componentDef *kbappsv1.ComponentDefinition) (string, error) {
	buildPodSpecVolumeMounts(synthesizeComp)
	its, err := factory.BuildInstanceSet(synthesizeComp, componentDef)
	if err != nil {
		return "", err
	}
	template := instanceset.BuildPodTemplate(its, instanceset.GetEnvConfigMapName(its.Name))
	return instanceset.BuildInstanceTemplateRevision(template, its)
}
