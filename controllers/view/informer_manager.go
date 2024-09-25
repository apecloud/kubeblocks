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
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type InformerManager interface {
	SetContext(ctx context.Context)
	Start() error
	Watch(watcher client.Object, watched schema.GroupVersionKind) error
	UnWatch(watcher client.Object, watched schema.GroupVersionKind) error
}

type informerManager struct {
	once sync.Once

	eventChan chan event.GenericEvent

	informerRefCounter map[schema.GroupVersionKind]sets.Set[model.GVKNObjKey]
	refCounterLock     sync.Mutex

	cache cache.Cache
	ctx   context.Context

	handler handler.EventHandler

	// Queue is an listeningQueue that listens for events from Informers and adds object keys to
	// the Queue for processing
	queue workqueue.RateLimitingInterface

	scheme *runtime.Scheme
}

func (m *informerManager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *informerManager) Start() error {
	m.once.Do(func() {
		go func() {
			for m.processNextWorkItem() {
			}
		}()
	})

	return nil
}

func (m *informerManager) Watch(watcher client.Object, watched schema.GroupVersionKind) error {
	m.refCounterLock.Lock()
	defer m.refCounterLock.Unlock()

	watchers, ok := m.informerRefCounter[watched]
	if !ok {
		watchers = sets.New[model.GVKNObjKey]()
		m.informerRefCounter[watched] = watchers
	}
	watcherRef, err := getObjectRef(watcher, m.scheme)
	if err != nil {
		return err
	}
	if watchers.Has(*watcherRef) {
		return nil
	}
	if err := m.createInformer(watched); err != nil {
		return nil
	}
	watchers.Insert(*watcherRef)
	return nil
}

func (m *informerManager) UnWatch(watcher client.Object, watched schema.GroupVersionKind) error {
	m.refCounterLock.Lock()
	defer m.refCounterLock.Unlock()

	watchers, ok := m.informerRefCounter[watched]
	if !ok {
		return nil
	}
	watcherRef, err := getObjectRef(watcher, m.scheme)
	if err != nil {
		return err
	}
	watchers.Delete(*watcherRef)
	if watchers.Len() == 0 {
		if err := m.deleteInformer(watched); err != nil {
			return err
		}
	}
	return nil
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the reconcileHandler.
func (m *informerManager) processNextWorkItem() bool {
	obj, shutdown := m.queue.Get()
	if shutdown {
		// Stop working
		return false
	}

	defer m.queue.Done(obj)
	switch o := obj.(type) {
	case event.CreateEvent:
		m.eventChan <- event.GenericEvent{Object: o.Object}
	case event.UpdateEvent:
		m.eventChan <- event.GenericEvent{Object: o.ObjectNew}
	case event.DeleteEvent:
		m.eventChan <- event.GenericEvent{Object: o.Object}
	case event.GenericEvent:
		m.eventChan <- o
	}

	return true
}

func (m *informerManager) createInformer(gvk schema.GroupVersionKind) error {
	o, err := m.scheme.New(gvk)
	if err != nil {
		return err
	}
	obj, ok := o.(client.Object)
	if !ok {
		return fmt.Errorf("can't find object of type %s", gvk)
	}
	src := source.Kind(m.cache, obj)
	return src.Start(m.ctx, m.handler, m.queue)
}

func (m *informerManager) deleteInformer(gvk schema.GroupVersionKind) error {
	// Can't call m.cache.RemoveInformer() here, as m.cache is shared with all other controllers in the same Manager,
	// they may still need the informer.
	return nil
}

type eventProxy struct{}

func (e *eventProxy) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	q.Add(evt)
}

func (e *eventProxy) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	q.Add(evt)
}

func (e *eventProxy) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	q.Add(evt)
}

func (e *eventProxy) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	q.Add(evt)
}

func NewInformerManager(cache cache.Cache, scheme *runtime.Scheme, eventChan chan event.GenericEvent) InformerManager {
	return &informerManager{
		cache:              cache,
		scheme:             scheme,
		eventChan:          eventChan,
		handler:            &eventProxy{},
		informerRefCounter: make(map[schema.GroupVersionKind]sets.Set[model.GVKNObjKey]),
		queue: workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{
			Name: "informer-manager",
		}),
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
	parseGVK := func(ot viewv1.ObjectType) error {
		gvk, err := objectTypeToGVK(&ot)
		if err != nil {
			return err
		}
		gvks.Insert(*gvk)
		return nil
	}
	for _, rule := range KBOwnershipRules {
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
			if err := r.manager.UnWatch(v, gvk); err != nil {
				return kubebuilderx.Commit, err
			}
		}
	} else {
		for gvk, _ := range gvks {
			if err := r.manager.Watch(v, gvk); err != nil {
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
var _ handler.EventHandler = &eventProxy{}
