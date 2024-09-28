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

	"golang.org/x/exp/slices"
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

	// build new object set from cache
	newObjectMap, err := getObjectsFromCache(c.ctx, c.cli, root, kbOwnershipRules)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	newObjectSet := sets.KeySet(newObjectMap)

	// build old object set from store
	currentState := &view.Status.CurrentState
	oldObjectMap, err := getObjectsFromTree(currentState.ObjectTree, c.store, c.scheme)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	oldObjectSet := sets.KeySet(oldObjectMap)

	// calculate createSet, deleteSet and updateSet
	createSet := newObjectSet.Difference(oldObjectSet)
	updateSet := newObjectSet.Intersection(oldObjectSet)
	deleteSet := oldObjectSet.Difference(newObjectSet)

	// build new slice of reconciliation changes from last round calculation
	var changeSlice []viewv1.ObjectChange
	// for createSet, build objectChange.description by reading i18n of the corresponding object type.
	changes := buildChanges(createSet, oldObjectMap, newObjectMap, viewv1.ObjectCreationType, i18nResource, defaultLocale, view.Spec.Locale)
	changeSlice = append(changeSlice, changes...)
	// for updateSet, read old version from object store by object type and resource version, calculate the diff, render the objectChange.description
	changes = buildChanges(updateSet, oldObjectMap, newObjectMap, viewv1.ObjectUpdateType, i18nResource, defaultLocale, view.Spec.Locale)
	changeSlice = append(changeSlice, changes...)
	// for deleteSet, build objectChange.description by reading i18n of the corresponding object type.
	changes = buildChanges(deleteSet, oldObjectMap, newObjectMap, viewv1.ObjectDeletionType, i18nResource, defaultLocale, view.Spec.Locale)
	changeSlice = append(changeSlice, changes...)

	if len(changeSlice) == 0 {
		return kubebuilderx.Continue, nil
	}

	// sort the changes by resource version.
	slices.SortStableFunc(changeSlice, func(a, b viewv1.ObjectChange) bool {
		return a.Revision < b.Revision
	})

	// concat it to current changes
	currentState.Changes = append(currentState.Changes, changeSlice...)

	// save new version objects to store
	for _, object := range newObjectMap {
		if err = c.store.Insert(object, view); err != nil {
			return kubebuilderx.Commit, err
		}
	}

	// update current object tree
	currentState.ObjectTree, err = getObjectTreeWithRevision(root, kbOwnershipRules, c.store, currentState.Changes[len(currentState.Changes)-1].Revision, c.scheme)
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

func viewCalculation(ctx context.Context, cli client.Client, scheme *runtime.Scheme, store ObjectRevisionStore) kubebuilderx.Reconciler {
	return &viewCalculator{
		ctx:    ctx,
		cli:    cli,
		scheme: scheme,
		store:  store,
	}
}

var _ kubebuilderx.Reconciler = &viewCalculator{}
