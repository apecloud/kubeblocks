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
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"strconv"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type viewCalculator struct {
	ctx    context.Context
	cli    client.Reader
	store  ObjectStore
	scheme *runtime.Scheme
}

func (c *viewCalculator) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (c *viewCalculator) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	view, _ := tree.GetRoot().(*viewv1.ReconciliationView)
	viewDef := tree.List(&viewv1.ReconciliationViewDefinition{})[0].(*viewv1.ReconciliationViewDefinition)
	objs := tree.List(&corev1.ConfigMap{})
	var i18nResource *corev1.ConfigMap
	if len(objs) > 0 {
		i18nResource, _ = objs[0].(*corev1.ConfigMap)
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
	if err := c.cli.Get(c.ctx, objectKey, root); err != nil {
		return kubebuilderx.Commit, err
	}
	newObjectMap := make(map[model.GVKNObjKey]client.Object)
	newObjectSet := sets.New[model.GVKNObjKey]()
	waitingList := list.New()
	waitingList.PushFront(root)
	for waitingList.Len() > 0 {
		e := waitingList.Front()
		waitingList.Remove(e)
		obj, _ := e.Value.(client.Object)
		objKey, err := getObjectRef(obj, c.scheme)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		newObjectSet.Insert(*objKey)
		newObjectMap[*objKey] = obj

		secondaries, err := c.getSecondaryObjectsOf(obj, viewDef.Spec.OwnershipRules)
		if err != nil {
			return kubebuilderx.Commit, err
		}
		for _, secondary := range secondaries {
			waitingList.PushBack(secondary)
		}
	}

	// build old object set from view.status.currentObjectTree
	oldObjectSet, oldObjectMap, err := c.getAllObjectsFrom(view.Status.CurrentObjectTree)
	if err != nil {
		return kubebuilderx.Commit, err
	}

	// calculate createSet, deleteSet and updateSet
	createSet := newObjectSet.Difference(oldObjectSet)
	updateSet := newObjectSet.Intersection(oldObjectSet)
	deleteSet := oldObjectSet.Difference(newObjectSet)

	// build view progress from three sets.
	var viewSlice []viewv1.ObjectChange
	// for createSet, build objectChange.description by reading i18n of the corresponding object type.
	changes := buildChanges(createSet, oldObjectMap, newObjectMap, viewv1.ObjectCreationType, i18nResource, viewDef.Spec.Locale, view.Spec.Locale)
	viewSlice = append(viewSlice, changes...)
	// for updateSet, read old version from object store by object type and resource version, calculate the diff, render the objectChange.description
	changes = buildChanges(updateSet, oldObjectMap, newObjectMap, viewv1.ObjectUpdateType, i18nResource, viewDef.Spec.Locale, view.Spec.Locale)
	viewSlice = append(viewSlice, changes...)
	// for deleteSet, build objectChange.description by reading i18n of the corresponding object type.
	changes = buildChanges(deleteSet, oldObjectMap, newObjectMap, viewv1.ObjectDeletionType, i18nResource, viewDef.Spec.Locale, view.Spec.Locale)
	viewSlice = append(viewSlice, changes...)

	// sort the view progress by resource version.
	slices.SortStableFunc(viewSlice, func(a, b viewv1.ObjectChange) bool {
		return a.Revision < b.Revision
	})

	// concat it to view.status.view
	view.Status.View = append(view.Status.View, viewSlice...)

	// save new version objects to object store.
	for _, object := range newObjectMap {
		if err = c.store.Insert(object, view); err != nil {
			return kubebuilderx.Commit, err
		}
	}

	// update view.status.currentObjectTree
	view.Status.CurrentObjectTree, err = getObjectTreeWithRevision(root, viewDef.Spec.OwnershipRules, c.store, parseRevision(root.ResourceVersion), c.scheme)
	if err != nil {
		return kubebuilderx.Commit, err
	}

	// update view summary
	initialObjectSet, initialObjectMap, err := c.getAllObjectsFrom(view.Status.InitialObjectTree)
	if err != nil {
		return kubebuilderx.Commit, err
	}
	createSet = newObjectSet.Difference(initialObjectSet)
	updateSet = newObjectSet.Intersection(initialObjectSet)
	deleteSet = initialObjectSet.Difference(newObjectSet)
	summaryMap := make(map[viewv1.ObjectType]*viewv1.ObjectSummary)
	doCount := func(s sets.Set[model.GVKNObjKey], summaryUpdater func(objectRef *model.GVKNObjKey, summary *viewv1.ObjectSummary)) {
		for objectRef := range s {
			key := *objectRefToType(&objectRef)
			summary, ok := summaryMap[key]
			if !ok {
				summary = &viewv1.ObjectSummary{
					Type:  key,
					Total: 0,
				}
				summaryMap[key] = summary
			}
			if summary.ChangeSummary == nil {
				summary.ChangeSummary = &viewv1.ObjectChangeSummary{}
			}
			summaryUpdater(&objectRef, summary)
		}
	}
	doCount(createSet, func(_ *model.GVKNObjKey, summary *viewv1.ObjectSummary) {
		summary.Total += 1
		if summary.ChangeSummary.Added == nil {
			summary.ChangeSummary.Added = pointer.Int32(0)
		}
		*summary.ChangeSummary.Added += 1
	})
	doCount(updateSet, func(objectRef *model.GVKNObjKey, summary *viewv1.ObjectSummary) {
		initialObj, _ := initialObjectMap[*objectRef]
		newObj, _ := newObjectMap[*objectRef]
		summary.Total += 1
		if initialObj != nil && newObj != nil && initialObj.GetResourceVersion() == newObj.GetResourceVersion() {
			return
		}
		if summary.ChangeSummary.Updated == nil {
			summary.ChangeSummary.Updated = pointer.Int32(0)
		}
		*summary.ChangeSummary.Updated += 1
	})
	doCount(deleteSet, func(_ *model.GVKNObjKey,summary *viewv1.ObjectSummary) {
		if summary.ChangeSummary.Deleted == nil {
			summary.ChangeSummary.Deleted = pointer.Int32(0)
		}
		*summary.ChangeSummary.Deleted += 1
	})
	var objectSummaries []viewv1.ObjectSummary
	for _, summary := range summaryMap {
		objectSummaries = append(objectSummaries, *summary)
	}
	slices.SortStableFunc(objectSummaries, func(a, b viewv1.ObjectSummary) bool {
		if a.Type.APIVersion != b.Type.APIVersion {
			return a.Type.APIVersion < b.Type.APIVersion
		}
		return a.Type.Kind < b.Type.Kind
	})
	view.Status.ViewSummary.ObjectSummaries = objectSummaries

	return kubebuilderx.Continue, nil
}

func buildChanges(objKeySet sets.Set[model.GVKNObjKey], oldObjectMap, newObjectMap map[model.GVKNObjKey]client.Object, changeType viewv1.ObjectChangeType, i18nResource *corev1.ConfigMap, defaultLocale, locale *string) []viewv1.ObjectChange {
	var changes []viewv1.ObjectChange
	for key := range objKeySet {
		var oldObj, newObj client.Object
		if oldObjectMap != nil {
			oldObj = oldObjectMap[key]
		}
		if newObjectMap != nil {
			newObj = newObjectMap[key]
		}
		obj := newObj
		if changeType == viewv1.ObjectDeletionType {
			obj = oldObj
		}
		if changeType == viewv1.ObjectUpdateType &&
			(oldObj == nil || newObj == nil  || oldObj.GetResourceVersion() == newObj.GetResourceVersion()) {
			continue
		}
		change := viewv1.ObjectChange{
			ObjectReference: *objectRefToReference(key, obj.GetUID(), obj.GetResourceVersion()),
			ChangeType:      changeType,
			// TODO(free6om): EventAttributes
			// TODO(free6om): State
			Revision:    parseRevision(obj.GetResourceVersion()),
			Timestamp:   func() *metav1.Time { t := metav1.Now(); return &t }(),
			Description: formatDescription(oldObj, newObj, changeType, i18nResource, getStringWithDefault(defaultLocale, "en")),
		}
		if locale != nil {
			change.LocalDescription = pointer.String(formatDescription(oldObj, newObj, changeType, i18nResource, *locale))
		}
		changes = append(changes, change)
	}
	return changes
}

func getStringWithDefault(ptr *string, defaultStr string) string {
	if ptr != nil && len(*ptr) > 0 {
		return *ptr
	}
	return defaultStr
}

func formatDescription(oldObj, newObj client.Object, changeType viewv1.ObjectChangeType, i18nResource *corev1.ConfigMap, locale string) string {
	if locale == "" {
		return ""
	}
	// TODO(free6om): finish me
	// TODO(free6om): handle nil oldObj(that lost after controller restarted)
	return string(changeType)
}

func parseRevision(revisionStr string) int64 {
	revision, err := strconv.ParseInt(revisionStr, 10, 64)
	if err != nil {
		revision = 0
	}
	return revision
}

func (c *viewCalculator) getSecondaryObjectsOf(obj client.Object, ownershipRules []viewv1.OwnershipRule) ([]client.Object, error) {
	objGVK, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return nil, err
	}
	// find matched rules
	var rules []viewv1.OwnershipRule
	for _, rule := range ownershipRules {
		gvk, err := objectTypeToGVK(&rule.Primary)
		if err != nil {
			return nil, err
		}
		if *gvk == objGVK {
			rules = append(rules, rule)
		}
	}

	// get secondary objects
	var secondaries []client.Object
	for _, rule := range rules {
		for _, ownedResource := range rule.OwnedResources {
			gvk, err := objectTypeToGVK(&ownedResource.Secondary)
			if err != nil {
				return nil, err
			}
			ml, err := parseMatchingLabels(obj, &ownedResource.Criteria)
			if err != nil {
				return nil, err
			}
			objects, err := getObjectsByGVK(c.ctx, c.cli, c.scheme, gvk, ml)
			if err != nil {
				return nil, err
			}
			secondaries = append(secondaries, objects...)
		}
	}

	return secondaries, nil
}

func (c *viewCalculator) getAllObjectsFrom(tree *viewv1.ObjectTreeNode) (sets.Set[model.GVKNObjKey], map[model.GVKNObjKey]client.Object, error) {
	if tree == nil {
		return nil, nil, nil
	}
	objectRef, err := objectReferenceToRef(&tree.Primary)
	if err != nil {
		return nil, nil, err
	}
	revision := parseRevision(tree.Primary.ResourceVersion)
	obj, err := c.store.Get(objectRef, revision)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}
	objectRefSet := sets.New(*objectRef)
	objectMap := make(map[model.GVKNObjKey]client.Object)
	if obj != nil {
		objectMap[*objectRef] = obj

	}
	for _, treeNode := range tree.Secondaries {
		secondaryRefSet, secondaryMap, err := c.getAllObjectsFrom(treeNode)
		if err != nil {
			return nil, nil, err
		}
		objectRefSet.Insert(secondaryRefSet.UnsortedList()...)
		for key, object := range secondaryMap {
			objectMap[key] = object
		}
	}
	return objectRefSet, objectMap, nil
}

func parseMatchingLabels(obj client.Object, criteria *viewv1.OwnershipCriteria) (client.MatchingLabels, error) {
	if criteria.SelectorCriteria != nil {
		return parseSelector(obj, criteria.SelectorCriteria.Path)
	}
	if criteria.LabelCriteria != nil {
		return parseLabels(obj, criteria.LabelCriteria), nil
	}
	return nil, fmt.Errorf("parse matching labels failed")
}

// getObjectReference creates a corev1.ObjectReference from a client.Object
func getObjectReference(obj client.Object, scheme *runtime.Scheme) (*corev1.ObjectReference, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return nil, err
	}

	return &corev1.ObjectReference{
		APIVersion:      gvk.GroupVersion().String(),
		Kind:            gvk.Kind,
		Namespace:       obj.GetNamespace(),
		Name:            obj.GetName(),
		UID:             obj.GetUID(),
		ResourceVersion: obj.GetResourceVersion(),
	}, nil
}

func viewCalculation(ctx context.Context, cli client.Client, store ObjectStore, scheme *runtime.Scheme) kubebuilderx.Reconciler {
	return &viewCalculator{
		ctx:    ctx,
		cli:    cli,
		store:  store,
		scheme: scheme,
	}
}

var _ kubebuilderx.Reconciler = &viewCalculator{}
