/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package instanceset2

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func NewAssistantObjectReconciler() kubebuilderx.Reconciler {
	return &assistantObjectReconciler{}
}

type assistantObjectReconciler struct{}

var _ kubebuilderx.Reconciler = &assistantObjectReconciler{}

func (a *assistantObjectReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (a *assistantObjectReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	var (
		objects []client.Object
		its, _  = tree.GetRoot().(*workloads.InstanceSet)
	)

	// do not reconcile the default headless service if shouldCloneInstanceAssistantObjects is true
	if shouldCloneInstanceAssistantObjects(its) {
		return kubebuilderx.Continue, nil
	}

	if !its.Spec.DisableDefaultHeadlessService {
		labels := getMatchLabels(its.Name)
		headlessSelectors := getHeadlessSvcSelector(its)
		headLessSvc := buildHeadlessSvc(*its, labels, headlessSelectors)
		objects = append(objects, headLessSvc)
	}
	for _, object := range objects {
		if err := intctrlutil.SetOwnership(its, object, model.GetScheme(), finalizer); err != nil {
			return kubebuilderx.Continue, err
		}
	}

	// compute create/update/delete set
	newSnapshot := make(map[model.GVKNObjKey]client.Object)
	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		newSnapshot[*name] = object
	}
	oldSnapshot := make(map[model.GVKNObjKey]client.Object)
	svcList := tree.List(&corev1.Service{})
	for _, objectList := range [][]client.Object{svcList} {
		for _, object := range objectList {
			name, err := model.GetGVKName(object)
			if err != nil {
				return kubebuilderx.Continue, err
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
			return kubebuilderx.Continue, err
		}
	}
	for name := range updateSet {
		oldObj := oldSnapshot[name]
		newObj := copyAndMerge(oldObj, newSnapshot[name])
		if err := tree.Update(newObj); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	for name := range deleteSet {
		if err := tree.Delete(oldSnapshot[name]); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	return kubebuilderx.Continue, nil
}

func getHeadlessSvcSelector(its *workloads.InstanceSet) map[string]string {
	selectors := make(map[string]string)
	for k, v := range its.Spec.Selector.MatchLabels {
		selectors[k] = v
	}
	selectors[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseStable
	return selectors
}

func buildHeadlessSvc(its workloads.InstanceSet, labels, selectors map[string]string) *corev1.Service {
	hdlBuilder := builder.NewHeadlessServiceBuilder(its.Namespace, getHeadlessSvcName(its.Name)).
		AddLabelsInMap(labels).
		AddSelectorsInMap(selectors).
		SetPublishNotReadyAddresses(true)

	portNames := sets.New[string]()
	for _, container := range its.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			servicePort := corev1.ServicePort{
				Protocol: port.Protocol,
				Port:     port.ContainerPort,
			}
			switch {
			case len(port.Name) > 0 && !portNames.Has(port.Name):
				portNames.Insert(port.Name)
				servicePort.Name = port.Name
			default:
				servicePort.Name = fmt.Sprintf("%s-%d", strings.ToLower(string(port.Protocol)), port.ContainerPort)
			}
			hdlBuilder.AddPorts(servicePort)
		}
	}
	return hdlBuilder.GetObject()
}
