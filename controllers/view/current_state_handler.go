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
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type viewCalculator struct {
	ctx    context.Context
	cli    client.Client
	scheme *runtime.Scheme
	store  ObjectRevisionStore
}

func (c *viewCalculator) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (c *viewCalculator) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	view, _ := tree.GetRoot().(*viewv1.ReconciliationView)
	objs := tree.List(&corev1.ConfigMap{})
	var i18nResource *corev1.ConfigMap
	if len(objs) > 0 {
		i18nResource, _ = objs[0].(*corev1.ConfigMap)
	}

	root := &kbappsv1.Cluster{}
	objectKey := client.ObjectKeyFromObject(view)
	if view.Spec.TargetObject != nil {
		objectKey = client.ObjectKey{
			Namespace: view.Spec.TargetObject.Namespace,
			Name:      view.Spec.TargetObject.Name,
		}
	}
	if err := c.cli.Get(c.ctx, objectKey, root); err != nil {
		return kubebuilderx.Commit, err
	}

	// handle object changes
	// build new object set from cache
	newObjectMap, err := getObjectsFromCache(c.ctx, c.cli, root, getKBOwnershipRules())
	if err != nil {
		return kubebuilderx.Commit, err
	}
	// build old object set from store
	currentState := &view.Status.CurrentState
	oldObjectMap, err := getObjectsFromTree(currentState.ObjectTree, c.store, c.scheme)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	changes := buildChanges(oldObjectMap, newObjectMap, buildDescriptionFormatter(i18nResource, defaultLocale, view.Spec.Locale))

	// handle event changes
	newEventMap, err := filterEvents(getEventsFromCache(c.ctx, c.cli), newObjectMap)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	oldEventMap, err := filterEvents(getEventsFromStore(c.store), oldObjectMap)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	eventChanges := buildChanges(oldEventMap, newEventMap, buildDescriptionFormatter(i18nResource, defaultLocale, view.Spec.Locale))
	changes = append(changes, eventChanges...)

	if len(changes) == 0 {
		return kubebuilderx.Continue, nil
	}

	// sort the changes by resource version.
	sort.SliceStable(changes, func(i, j int) bool {
		return changes[i].Revision < changes[j].Revision
	})

	// concat it to current changes
	currentState.Changes = append(currentState.Changes, changes...)

	// save new version objects to store
	for _, object := range newObjectMap {
		if err = c.store.Insert(object, view); err != nil {
			return kubebuilderx.Commit, err
		}
	}
	// save new events to store
	for _, object := range newEventMap {
		if err = c.store.Insert(object, view); err != nil {
			return kubebuilderx.Commit, err
		}
	}

	// update current object tree
	currentState.ObjectTree, err = getObjectTreeWithRevision(root, getKBOwnershipRules(), c.store, currentState.Changes[len(currentState.Changes)-1].Revision, c.scheme)
	if err != nil {
		return kubebuilderx.Commit, err
	}

	// update changes summary
	initialObjectMap, err := getObjectsFromTree(view.Status.InitialObjectTree, c.store, c.scheme)
	if err != nil {
		return kubebuilderx.Commit, err
	}

	currentState.Summary.ObjectSummaries = buildObjectSummaries(initialObjectMap, newObjectMap)

	return kubebuilderx.Continue, nil
}

func getEventsFromCache(ctx context.Context, cli client.Client) func() ([]client.Object, error) {
	return func() ([]client.Object, error) {
		eventList := &corev1.EventList{}
		if err := cli.List(ctx, eventList); err != nil {
			return nil, err
		}
		var objects []client.Object
		for i := range eventList.Items {
			objects = append(objects, &eventList.Items[i])
		}
		return objects, nil
	}
}

func getEventsFromStore(store ObjectRevisionStore) func() ([]client.Object, error) {
	return func() ([]client.Object, error) {
		eventRevisionMap := store.List(&eventGVK)
		var objects []client.Object
		for _, revisionMap := range eventRevisionMap {
			revision := int64(-1)
			for rev := range revisionMap {
				if rev > revision {
					revision = rev
				}
			}
			if revision > -1 {
				objects = append(objects, revisionMap[revision])
			}
		}
		return objects, nil
	}
}

func filterEvents(eventLister func() ([]client.Object, error), objectMap map[model.GVKNObjKey]client.Object) (map[model.GVKNObjKey]client.Object, error) {
	eventList, err := eventLister()
	if err != nil {
		return nil, err
	}
	objectRefSet := sets.KeySet(objectMap)
	matchedEventMap := make(map[model.GVKNObjKey]client.Object)
	for i := range eventList {
		event, _ := eventList[i].(*corev1.Event)
		objRef := objectReferenceToRef(&event.InvolvedObject)
		if objectRefSet.Has(*objRef) {
			eventRef := model.GVKNObjKey{
				GroupVersionKind: eventGVK,
				ObjectKey:        client.ObjectKeyFromObject(event),
			}
			matchedEventMap[eventRef] = event
		}
	}
	return matchedEventMap, nil
}

func updateCurrentState(ctx context.Context, cli client.Client, scheme *runtime.Scheme, store ObjectRevisionStore) kubebuilderx.Reconciler {
	return &viewCalculator{
		ctx:    ctx,
		cli:    cli,
		scheme: scheme,
		store:  store,
	}
}

var _ kubebuilderx.Reconciler = &viewCalculator{}
