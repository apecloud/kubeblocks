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

package instanceset

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// assistantObjectReconciler manages non-workload objects, such as Service, ConfigMap, etc.
type assistantObjectReconciler struct{}

func NewAssistantObjectReconciler() kubebuilderx.Reconciler {
	return &assistantObjectReconciler{}
}

func (a *assistantObjectReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	return kubebuilderx.ResultSatisfied
}

func (a *assistantObjectReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)

	// generate objects by current spec
	labels := getMatchLabels(its.Name)
	selectors := getSvcSelector(its, false)
	headlessSelectors := getSvcSelector(its, true)

	svc := buildSvc(*its, labels, selectors)
	headLessSvc := buildHeadlessSvc(*its, labels, headlessSelectors)
	envConfig, err := buildEnvConfigMap(*its, labels)
	if err != nil {
		return nil, err
	}
	var objects []client.Object
	if svc != nil {
		objects = append(objects, svc)
	}
	objects = append(objects, headLessSvc, envConfig)
	for _, object := range objects {
		if err := intctrlutil.SetOwnership(its, object, model.GetScheme(), finalizer); err != nil {
			return nil, err
		}
	}
	// compute create/update/delete set
	newSnapshot := make(map[model.GVKNObjKey]client.Object)
	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return nil, err
		}
		newSnapshot[*name] = object
	}
	oldSnapshot := make(map[model.GVKNObjKey]client.Object)
	svcList := tree.List(&corev1.Service{})
	cmList := tree.List(&corev1.ConfigMap{})
	cmListFiltered, err := filterTemplate(cmList, its.Annotations)
	if err != nil {
		return nil, err
	}
	for _, objectList := range [][]client.Object{svcList, cmListFiltered} {
		for _, object := range objectList {
			name, err := model.GetGVKName(object)
			if err != nil {
				return nil, err
			}
			oldSnapshot[*name] = object
		}
	}
	// now compute the diff between old and target snapshot and generate the plan
	oldNameSet := sets.KeySet(oldSnapshot)
	newNameSet := sets.KeySet(newSnapshot)

	createSet := newNameSet.Difference(oldNameSet)
	updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)
	for name := range createSet {
		if err := tree.Add(newSnapshot[name]); err != nil {
			return nil, err
		}
	}
	for name := range updateSet {
		oldObj := oldSnapshot[name]
		newObj := copyAndMerge(oldObj, newSnapshot[name])
		if err := tree.Update(newObj); err != nil {
			return nil, err
		}
	}
	for name := range deleteSet {
		if err := tree.Delete(oldSnapshot[name]); err != nil {
			return nil, err
		}
	}
	return tree, nil
}

func filterTemplate(cmList []client.Object, annotations map[string]string) ([]client.Object, error) {
	templateMap, err := getInstanceTemplateMap(annotations)
	if err != nil {
		return nil, err
	}
	isTemplate := func(cm client.Object) bool {
		for _, name := range templateMap {
			if name == cm.GetName() {
				return true
			}
		}
		return false
	}
	var cmListFiltered []client.Object
	for _, cm := range cmList {
		if isTemplate(cm) {
			continue
		}
		cmListFiltered = append(cmListFiltered, cm)
	}
	return cmListFiltered, nil
}

var _ kubebuilderx.Reconciler = &assistantObjectReconciler{}
