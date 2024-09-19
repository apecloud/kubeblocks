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
	"container/list"
	"context"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

type viewCalculator struct {
	ctx context.Context
	cli client.Reader
}

func (c *viewCalculator) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (c *viewCalculator) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	view, _ := tree.GetRoot().(*viewv1.ReconciliationView)
	o, err := tree.Get(&viewv1.ReconciliationViewDefinition{})
	if err != nil {
		return kubebuilderx.Commit, err
	}
	viewDef, _ := o.(*viewv1.ReconciliationViewDefinition)
	o, err = tree.Get(&corev1.ConfigMap{})
	if err != nil {
		return kubebuilderx.Commit, err
	}
	var i18nResource *corev1.ConfigMap
	if o != nil {
		i18nResource, _ = o.(*corev1.ConfigMap)
	}

	// build new object set from cache
	root := &appsv1alpha1.Cluster{}
	objectKey := client.ObjectKeyFromObject(view)
	if view.Spec.TargetObject != nil {
		objectKey = client.ObjectKey{
			Namespace: view.Spec.TargetObject.Namespace,
			Name:      view.Spec.TargetObject.Name,
		}
	}
	if err = c.cli.Get(c.ctx, objectKey, root); err != nil {
		return kubebuilderx.Commit, err
	}
	newObjectSet := sets.New[corev1.ObjectReference]()
	waitingList := list.New()
	waitingList.PushFront(root)
	for waitingList.Len() > 0 {
		e := waitingList.Front()
		waitingList.Remove(e)
		obj, _ := e.Value.(client.Object)
		objRef, err := getObjectRef(c.scheme, obj)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		newObjectSet.Insert(objRef)

		secondaries, err := c.getSecondaryObjectsOf(obj, viewDef.Spec.OwnershipRules)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		for _, secondary := range secondaries {
			waitingList.PushBack(secondary)
		}
	}

	// build old object set from view.status.currentObjectTree
	oldObjectSet := c.getAllObjectsFrom(view.Status.CurrentObjectTree)

	// calculate createSet, deleteSet and updateSet
	createSet := newObjectSet.Difference(oldObjectSet)
	updateSet := newObjectSet.Intersection(oldObjectSet)
	deleteSet := oldObjectSet.Difference(newObjectSet)

	// build view progress from three sets.
	var viewSlice []viewv1.ObjectChange
	// for createSet, build objectChange.description by reading i18n of the corresponding object type.
	for key, _ := range createSet {
		change := viewv1.ObjectChange{
			ObjectReference: key,
			ChangeType:      viewv1.ObjectCreationType,
			// TODO(free6om): EventAttributes
			// TODO(free6om): State
			// TODO(free6om): remove? Revision: key.ResourceVersion,
			Timestamp: func() *metav1.Time {t := metav1.Now(); return &t}(),
			Description: formatDescription(key, viewv1.ObjectCreationType, i18nResource, viewDef.Spec.Locale),
			LocalDescription: formatDescription(key, viewv1.ObjectCreationType, i18nResource, view.Spec.Locale),
		}
	}
	// for updateSet, read old version from object store by object type and resource version, calculate the diff, render the objectChange.description
	// for deleteSet, build objectChange.description by reading i18n of the corresponding object type.
	// sort the view progress by resource version.
	// concat it to view.status.view
	// save new version objects to object store.
}

func viewCalculation(ctx context.Context, cli client.Client) kubebuilderx.Reconciler {
	return &viewCalculator{
		ctx: ctx,
		cli: cli,
	}
}

var _ kubebuilderx.Reconciler = &viewCalculator{}
