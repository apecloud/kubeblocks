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

	corev1 "k8s.io/api/core/v1"
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
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type InformerManager interface {
	Start(context.Context) error
}

type informerManager struct {
	once sync.Once

	eventChan chan event.GenericEvent

	informerSet     sets.Set[schema.GroupVersionKind]
	informerSetLock sync.Mutex

	cache cache.Cache
	cli   client.Client
	ctx   context.Context

	handler handler.EventHandler

	// Queue is an listeningQueue that listens for events from Informers and adds object keys to
	// the Queue for processing
	queue workqueue.RateLimitingInterface

	scheme *runtime.Scheme
}

func (m *informerManager) Start(ctx context.Context) error {
	m.ctx = ctx

	if err := m.watchKubeBlocksRelatedResources(); err != nil {
		return err
	}

	m.once.Do(func() {
		go func() {
			for m.processNextWorkItem() {
			}
		}()
	})

	return nil
}

func (m *informerManager) watch(resource schema.GroupVersionKind) error {
	m.informerSetLock.Lock()
	defer m.informerSetLock.Unlock()

	if _, ok := m.informerSet[resource]; ok {
		return nil
	}
	if err := m.createInformer(resource); err != nil {
		return err
	}
	m.informerSet.Insert(resource)

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

	var object client.Object
	switch o := obj.(type) {
	case event.CreateEvent:
		object = o.Object
	case event.UpdateEvent:
		object = o.ObjectNew
	case event.DeleteEvent:
		object = o.Object
	case event.GenericEvent:
		object = o.Object
	}
	// get involved object if 'object' is an Event
	if evt, ok := object.(*corev1.Event); ok {
		ro, err := m.scheme.New(evt.InvolvedObject.GroupVersionKind())
		if err != nil {
			return false
		}
		object, _ = ro.(client.Object)
		err = m.cli.Get(context.Background(), client.ObjectKey{Namespace: evt.InvolvedObject.Namespace, Name: evt.InvolvedObject.Name}, object)
		if err != nil {
			return false
		}
	}
	if object != nil {
		m.eventChan <- event.GenericEvent{Object: object}
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

func NewInformerManager(cli client.Client, cache cache.Cache, scheme *runtime.Scheme, eventChan chan event.GenericEvent) InformerManager {
	return &informerManager{
		cli:         cli,
		cache:       cache,
		scheme:      scheme,
		eventChan:   eventChan,
		handler:     &eventProxy{},
		informerSet: sets.New[schema.GroupVersionKind](),
		queue: workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{
			Name: "informer-manager",
		}),
	}
}

func (m *informerManager) watchKubeBlocksRelatedResources() error {
	gvks := sets.New[schema.GroupVersionKind]()
	parseGVK := func(ot *viewv1.ObjectType) error {
		gvk, err := objectTypeToGVK(ot)
		if err != nil {
			return err
		}
		gvks.Insert(*gvk)
		return nil
	}
	// watch corev1.Event
	if err := parseGVK(&viewv1.ObjectType{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       constant.EventKind,
	}); err != nil {
		return err
	}
	for _, rule := range kbOwnershipRules {
		if err := parseGVK(&rule.Primary); err != nil {
			return err
		}
		for _, resource := range rule.OwnedResources {
			if err := parseGVK(&resource.Secondary); err != nil {
				return err
			}
		}
	}
	for gvk := range gvks {
		if err := m.watch(gvk); err != nil {
			return err
		}
	}
	return nil
}

var _ InformerManager = &informerManager{}
var _ handler.EventHandler = &eventProxy{}
