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
	"slices"
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

func NewHeadlessServiceReconciler() kubebuilderx.Reconciler {
	return &headlessServiceReconciler{}
}

type headlessServiceReconciler struct{}

var _ kubebuilderx.Reconciler = &headlessServiceReconciler{}

func (r *headlessServiceReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *headlessServiceReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	var headlessService *corev1.Service
	if !its.Spec.DisableDefaultHeadlessService {
		labels := getMatchLabels(its.Name)
		headlessSelectors := getHeadlessSvcSelector(its)
		headlessService = buildHeadlessSvc(*its, labels, headlessSelectors)
	}
	if headlessService != nil {
		if err := intctrlutil.SetOwnership(its, headlessService, model.GetScheme(), finalizer); err != nil {
			return kubebuilderx.Continue, err
		}
	}

	oldHeadlessService, err := tree.Get(buildHeadlessSvc(*its, nil, nil))
	if err != nil {
		return kubebuilderx.Continue, err
	}

	skipToReconcileOpt := kubebuilderx.SkipToReconcile(shouldCloneInstanceAssistantObjects(its))
	if oldHeadlessService == nil && headlessService != nil {
		if err := tree.AddWithOption(headlessService, skipToReconcileOpt); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	if oldHeadlessService != nil && headlessService != nil {
		newObj := copyAndMerge(oldHeadlessService, headlessService)
		if err := tree.Update(newObj, skipToReconcileOpt); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	if oldHeadlessService != nil && headlessService == nil {
		if err := tree.DeleteWithOption(oldHeadlessService, skipToReconcileOpt); err != nil {
			return kubebuilderx.Continue, err
		}
	}

	if headlessService != nil {
		r.addHeadlessService(its, headlessService)
	} else {
		r.deleteHeadlessService(its, oldHeadlessService)
	}

	return kubebuilderx.Continue, nil
}

func (r *headlessServiceReconciler) addHeadlessService(its *workloads.InstanceSet, svc *corev1.Service) {
	if shouldCloneInstanceAssistantObjects(its) && svc != nil {
		if its.Spec.InstanceAssistantObjects == nil {
			its.Spec.InstanceAssistantObjects = make([]corev1.ObjectReference, 0)
		}
		gvk, _ := model.GetGVKName(svc)
		its.Spec.InstanceAssistantObjects = append(its.Spec.InstanceAssistantObjects,
			corev1.ObjectReference{
				Kind:      gvk.Kind,
				Namespace: gvk.Namespace,
				Name:      gvk.Name,
			})
	}
}

func (r *headlessServiceReconciler) deleteHeadlessService(its *workloads.InstanceSet, obj client.Object) {
	var svc *corev1.Service
	if obj != nil {
		svc = obj.(*corev1.Service)
	}
	if svc != nil {
		gvk, _ := model.GetGVKName(svc)
		its.Spec.InstanceAssistantObjects = slices.DeleteFunc(its.Spec.InstanceAssistantObjects,
			func(o corev1.ObjectReference) bool {
				return o.Kind == gvk.Kind && o.Namespace == gvk.Namespace && o.Name == gvk.Name
			})
	}
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
