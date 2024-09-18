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

package view

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/event"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type InformerManager interface {
	Watch(watcher, watched schema.GroupVersionKind) error
	UnWatch(watcher, watched schema.GroupVersionKind) error
}

type informerManager struct {
	eventChan chan event.GenericEvent

	informerRefCounter map[schema.GroupVersionKind]sets.Set[schema.GroupVersionKind]
	refCounterLock     sync.Mutex
}

func (m *informerManager) Watch(watcher, watched schema.GroupVersionKind) error {
	m.refCounterLock.Lock()
	defer m.refCounterLock.Unlock()

	watchers, ok := m.informerRefCounter[watched]
	if !ok {
		watchers = sets.New[schema.GroupVersionKind]()
	}
	if watchers.Has(watcher) {
		return nil
	}
	if err := m.createInformer(watched); err != nil {
		return nil
	}
	watchers.Insert(watcher)
	return nil
}

func (m *informerManager) UnWatch(watcher, watched schema.GroupVersionKind) error {
	m.refCounterLock.Lock()
	defer m.refCounterLock.Unlock()

	watchers, ok := m.informerRefCounter[watched]
	if !ok {
		return nil
	}
	watchers.Delete(watcher)
	if watchers.Len() == 0 {
		if err := m.deleteInformer(watched); err != nil {
			return err
		}
	}
	return nil
}

func (m *informerManager) createInformer(gvk schema.GroupVersionKind) error {
	//TODO implement me
	panic("implement me")
}

func (m *informerManager) deleteInformer(gvk schema.GroupVersionKind) error {
	//TODO implement me
	panic("implement me")
}

func NewInformerManager(eventChan chan event.GenericEvent) InformerManager {
	return &informerManager{
		eventChan:          eventChan,
		informerRefCounter: make(map[schema.GroupVersionKind]sets.Set[schema.GroupVersionKind]),
	}
}

type informerManagerReconciler struct {
	manager InformerManager
}

func (r *informerManagerReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	return kubebuilderx.ConditionSatisfied
}

func (r *informerManagerReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	gvks := sets.New[schema.GroupVersionKind]()
	o, _ := tree.Get(&viewv1.ReconciliationViewDefinition{})
	viewDef, _ := o.(*viewv1.ReconciliationViewDefinition)
	parseGVK := func(ot viewv1.ObjectType) error {
		gv, err := schema.ParseGroupVersion(ot.APIVersion)
		if err != nil {
			return err
		}
		gvks.Insert(gv.WithKind(ot.Kind))
		return nil
	}
	for _, rule := range viewDef.Spec.OwnershipRules {
		if err := parseGVK(rule.Primary); err != nil {
			return kubebuilderx.Commit, err
		}
		for _, resource := range rule.OwnedResources {
			if err := parseGVK(resource.Secondary); err != nil {
				return kubebuilderx.Commit, err
			}
		}
	}
	v, _ := tree.GetRoot().(*viewv1.ReconciliationView)
	if model.IsObjectDeleting(tree.GetRoot()) {
		for gvk, _ := range gvks {
			if err := r.manager.UnWatch(v.GetObjectKind().GroupVersionKind(), gvk); err != nil {
				return kubebuilderx.Commit, err
			}
		}
	} else {
		for gvk, _ := range gvks {
			if err := r.manager.Watch(v.GetObjectKind().GroupVersionKind(), gvk); err != nil {
				return kubebuilderx.Commit, err
			}
		}
	}

	return kubebuilderx.Continue, nil
}

func updateInformerManager(manager InformerManager) kubebuilderx.Reconciler {
	return &informerManagerReconciler{manager: manager}
}

var _ InformerManager = &informerManager{}
var _ kubebuilderx.Reconciler = &informerManagerReconciler{}
