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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func objectTypeToGVK(objectType *viewv1.ObjectType) (*schema.GroupVersionKind, error) {
	if objectType == nil {
		return nil, nil
	}
	gv, err := schema.ParseGroupVersion(objectType.APIVersion)
	if err != nil {
		return nil, err
	}
	gvk := gv.WithKind(objectType.Kind)
	return &gvk, nil
}

func objectReferenceToType(objectRef *corev1.ObjectReference) *viewv1.ObjectType {
	return &viewv1.ObjectType{
		APIVersion: objectRef.APIVersion,
		Kind:       objectRef.Kind,
	}
}

func objectReferenceToRef(reference *corev1.ObjectReference) (*model.GVKNObjKey, error) {
	if reference == nil {
		return nil, nil
	}
	gv, err := schema.ParseGroupVersion(reference.APIVersion)
	if err != nil {
		return nil, err
	}
	gvk := gv.WithKind(reference.Kind)
	return &model.GVKNObjKey{
		GroupVersionKind: gvk,
		ObjectKey: client.ObjectKey{
			Namespace: reference.Namespace,
			Name:      reference.Name,
		},
	}, nil
}

func objectRefToReference(objectRef model.GVKNObjKey, uid types.UID, resourceVersion string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion:      objectRef.GroupVersionKind.GroupVersion().String(),
		Kind:            objectRef.Kind,
		Namespace:       objectRef.Namespace,
		Name:            objectRef.Name,
		UID:             uid,
		ResourceVersion: resourceVersion,
	}
}

func objectRefToType(objectRef *model.GVKNObjKey) *viewv1.ObjectType {
	return &viewv1.ObjectType{
		APIVersion: objectRef.GroupVersionKind.GroupVersion().String(),
		Kind:       objectRef.Kind,
	}
}

func getObjectRef(object client.Object, scheme *runtime.Scheme) (*model.GVKNObjKey, error) {
	gvk, err := apiutil.GVKForObject(object, scheme)
	if err != nil {
		return nil, err
	}
	return &model.GVKNObjKey{
		GroupVersionKind: gvk,
		ObjectKey:        client.ObjectKeyFromObject(object),
	}, nil
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

// getObjectsByGVK gets all objects of a specific GVK.
// why not merge matchingFields into opts:
// fields.Selector needs the underlying cache to build an Indexer on the specific field, which is too heavy.
func getObjectsByGVK(ctx context.Context, cli client.Client, gvk *schema.GroupVersionKind, opts *queryOptions) ([]client.Object, error) {
	runtimeObjectList, err := cli.Scheme().New(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	})
	if err != nil {
		return nil, err
	}
	objectList, ok := runtimeObjectList.(client.ObjectList)
	if !ok {
		return nil, fmt.Errorf("list object is not a client.ObjectList for GVK %s", gvk)
	}
	var ml client.MatchingLabels
	if opts != nil && opts.matchLabels != nil {
		ml = opts.matchLabels
	}
	if err = cli.List(ctx, objectList, ml); err != nil {
		return nil, err
	}
	runtimeObjects, err := meta.ExtractList(objectList)
	if err != nil {
		return nil, err
	}
	var objects []client.Object
	listOptions := &client.ListOptions{}
	if opts != nil && opts.matchFields != nil {
		opts.matchFields.ApplyToList(listOptions)
	}
	for _, object := range runtimeObjects {
		o, ok := object.(client.Object)
		if !ok {
			return nil, fmt.Errorf("object is not a client.Object for GVK %s", gvk)
		}
		if listOptions.FieldSelector != nil && !listOptions.FieldSelector.Matches(fields.Set{"metadata.name": o.GetName()}) {
			continue
		}
		if opts != nil && opts.matchOwner != nil && !matchOwnerOf(opts.matchOwner, o) {
			continue
		}
		objects = append(objects, o)
	}

	return objects, nil
}

func matchOwnerOf(owner *matchOwner, o client.Object) bool {
	for _, ref := range o.GetOwnerReferences() {
		if ref.UID == owner.ownerUID {
			if !owner.controller {
				return true
			}
			return ref.Controller != nil && *ref.Controller
		}
	}
	return false
}

func parseRevision(revisionStr string) int64 {
	revision, err := strconv.ParseInt(revisionStr, 10, 64)
	if err != nil {
		revision = 0
	}
	return revision
}

func parseField(obj client.Object, fieldPath string) (map[string]interface{}, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	// Use the field path to find the field
	pathParts := strings.Split(fieldPath, ".")
	current := unstructuredObj
	for i := 0; i < len(pathParts); i++ {
		part := pathParts[i]
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil, fmt.Errorf("field '%s' does not exist", fieldPath)
		}
	}
	return current, nil
}

// parseSelector checks if a field exists in the object and returns it if it's a metav1.LabelSelector
func parseSelector(obj client.Object, fieldPath string) (map[string]string, error) {
	selectorField, err := parseField(obj, fieldPath)
	if err != nil {
		return nil, err
	}
	// Attempt to convert the final field to a LabelSelector
	// TODO(free6om): handle metav1.LabelSelector
	//labelSelector := &metav1.LabelSelector{}
	labelSelector := make(map[string]string)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(selectorField, labelSelector); err != nil {
		return nil, fmt.Errorf("failed to parse as LabelSelector: %w", err)
	}

	return labelSelector, nil
}

func getObjectTreeFromCache(ctx context.Context, cli client.Client, primary client.Object, ownershipRules []OwnershipRule) (*viewv1.ObjectTreeNode, error) {
	if primary == nil {
		return nil, nil
	}

	// primary tree node
	reference, err := getObjectReference(primary, cli.Scheme())
	if err != nil {
		return nil, err
	}
	tree := &viewv1.ObjectTreeNode{
		Primary: *reference,
	}

	// secondary tree nodes
	// find matched rules
	primaryGVK, err := apiutil.GVKForObject(primary, cli.Scheme())
	if err != nil {
		return nil, err
	}
	var matchedRules []OwnershipRule
	for i := range ownershipRules {
		rule := ownershipRules[i]
		gvk, err := objectTypeToGVK(&rule.Primary)
		if err != nil {
			return nil, err
		}
		if *gvk == primaryGVK {
			matchedRules = append(matchedRules, rule)
		}
	}
	// build subtree
	secondaries, err := getSecondaryObjects(ctx, cli, primary, matchedRules)
	if err != nil {
		return nil, err
	}
	for _, secondary := range secondaries {
		subTree, err := getObjectTreeFromCache(ctx, cli, secondary, ownershipRules)
		if err != nil {
			return nil, err
		}
		tree.Secondaries = append(tree.Secondaries, subTree)
		slices.SortStableFunc(tree.Secondaries, func(a, b *viewv1.ObjectTreeNode) bool {
			return getObjectReferenceString(a) < getObjectReferenceString(b)
		})
	}

	return tree, nil
}

func getObjectReferenceString(n *viewv1.ObjectTreeNode) string {
	if n == nil {
		return "nil"
	}
	return strings.Join([]string{
		n.Primary.Kind,
		n.Primary.Namespace,
		n.Primary.Name,
		n.Primary.APIVersion,
	}, "")
}

func getObjectsFromCache(ctx context.Context, cli client.Client, root *kbappsv1.Cluster, ownershipRules []OwnershipRule) (map[model.GVKNObjKey]client.Object, error) {
	objectMap := make(map[model.GVKNObjKey]client.Object)
	waitingList := list.New()
	waitingList.PushFront(root)
	for waitingList.Len() > 0 {
		e := waitingList.Front()
		waitingList.Remove(e)
		obj, _ := e.Value.(client.Object)
		objKey, err := getObjectRef(obj, cli.Scheme())
		if err != nil {
			return nil, err
		}
		objectMap[*objKey] = obj

		secondaries, err := getSecondaryObjects(ctx, cli, obj, ownershipRules)
		if err != nil {
			return nil, err
		}
		for _, secondary := range secondaries {
			waitingList.PushBack(secondary)
		}
	}
	return objectMap, nil
}

func getSecondaryObjects(ctx context.Context, cli client.Client, obj client.Object, ownershipRules []OwnershipRule) ([]client.Object, error) {
	objGVK, err := apiutil.GVKForObject(obj, cli.Scheme())
	if err != nil {
		return nil, err
	}
	// find matched rules
	var rules []OwnershipRule
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
			opts, err := parseQueryOptions(obj, &ownedResource.Criteria)
			if err != nil {
				return nil, err
			}
			objects, err := getObjectsByGVK(ctx, cli, gvk, opts)
			if err != nil {
				return nil, err
			}
			secondaries = append(secondaries, objects...)
		}
	}

	return secondaries, nil
}

func parseQueryOptions(primary client.Object, criteria *OwnershipCriteria) (*queryOptions, error) {
	opts := &queryOptions{}
	if criteria.SelectorCriteria != nil {
		ml, err := parseSelector(primary, criteria.SelectorCriteria.Path)
		if err != nil {
			return nil, err
		}
		opts.matchLabels = ml
	}
	if criteria.LabelCriteria != nil {
		labels := make(map[string]string, len(criteria.LabelCriteria))
		for k, v := range criteria.LabelCriteria {
			value := strings.ReplaceAll(v, "$(primary.name)", primary.GetName())
			value = strings.ReplaceAll(value, "$(primary)", primary.GetLabels()[k])
			labels[k] = value
		}
		opts.matchLabels = labels
	}
	if criteria.SpecifiedNameCriteria != nil {
		fieldMap, err := flattenObject(primary)
		if err != nil {
			return nil, err
		}
		name, ok := fieldMap[criteria.SpecifiedNameCriteria.Path]
		if ok {
			opts.matchFields = client.MatchingFields{"metadata.name": name}
		}
	}
	if criteria.Validation != NoValidation {
		opts.matchOwner = &matchOwner{
			ownerUID:   primary.GetUID(),
			controller: criteria.Validation == ControllerValidation,
		}
	}
	return opts, nil
}

func specMapToJSON(spec interface{}) []byte {
	// Convert the spec map to JSON for the patch functions
	specJSON, _ := json.Marshal(spec)
	return specJSON
}

// convertToMap converts a client.Object to a map respecting JSON tags.
func convertToMap(obj client.Object) (map[string]interface{}, error) {
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var objMap map[string]interface{}
	if err := json.Unmarshal(objBytes, &objMap); err != nil {
		return nil, err
	}
	return objMap, nil
}

// flattenJSON flattens a nested JSON object into a single-level map.
func flattenJSON(data map[string]interface{}, prefix string, flatMap map[string]string) {
	for key, value := range data {
		newKey := key
		if prefix != "" {
			newKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			flattenJSON(v, newKey, flatMap)
		case []interface{}:
			for i, item := range v {
				flattenJSON(map[string]interface{}{fmt.Sprintf("%d", i): item}, newKey, flatMap)
			}
		default:
			flatMap[newKey] = fmt.Sprintf("%v", v)
		}
	}
}

// flattenObject converts a client.Object to a flattened map.
func flattenObject(obj client.Object) (map[string]string, error) {
	objMap, err := convertToMap(obj)
	if err != nil {
		return nil, err
	}

	flatMap := make(map[string]string)
	flattenJSON(objMap, "", flatMap)
	return flatMap, nil
}

func objectType(apiVersion, kind string) viewv1.ObjectType {
	return viewv1.ObjectType{
		APIVersion: apiVersion,
		Kind:       kind,
	}
}

func buildObjectSummaries(initialObjectMap, newObjectMap map[model.GVKNObjKey]client.Object) []viewv1.ObjectSummary {
	initialObjectSet, newObjectSet := sets.KeySet(initialObjectMap), sets.KeySet(newObjectMap)
	createSet := newObjectSet.Difference(initialObjectSet)
	updateSet := newObjectSet.Intersection(initialObjectSet)
	deleteSet := initialObjectSet.Difference(newObjectSet)
	summaryMap := make(map[viewv1.ObjectType]*viewv1.ObjectSummary)
	doCount := func(s sets.Set[model.GVKNObjKey], summaryUpdater func(objectRef *model.GVKNObjKey, summary *viewv1.ObjectSummary)) {
		for objectRef := range s {
			key := *objectRefToType(&objectRef)
			summary, ok := summaryMap[key]
			if !ok {
				summary = &viewv1.ObjectSummary{
					ObjectType: key,
					Total:      0,
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
	doCount(deleteSet, func(_ *model.GVKNObjKey, summary *viewv1.ObjectSummary) {
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
		if a.ObjectType.APIVersion != b.ObjectType.APIVersion {
			return a.ObjectType.APIVersion < b.ObjectType.APIVersion
		}
		return a.ObjectType.Kind < b.ObjectType.Kind
	})

	return objectSummaries
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
			(oldObj == nil || newObj == nil || oldObj.GetResourceVersion() == newObj.GetResourceVersion()) {
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

func getObjectsFromTree(tree *viewv1.ObjectTreeNode, store ObjectRevisionStore, scheme *runtime.Scheme) (map[model.GVKNObjKey]client.Object, error) {
	if tree == nil {
		return nil, nil
	}
	objectRef, err := objectReferenceToRef(&tree.Primary)
	if err != nil {
		return nil, err
	}
	revision := parseRevision(tree.Primary.ResourceVersion)
	obj, err := store.Get(objectRef, revision)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	objectMap := make(map[model.GVKNObjKey]client.Object)
	// cache loss after controller restarted, mock one
	if obj == nil {
		ro, err := scheme.New(objectRef.GroupVersionKind)
		if err != nil {
			return nil, err
		}
		obj, _ = ro.(client.Object)
		obj.SetNamespace(tree.Primary.Namespace)
		obj.SetName(tree.Primary.Name)
		obj.SetResourceVersion(tree.Primary.ResourceVersion)
		obj.SetUID(tree.Primary.UID)
	}
	objectMap[*objectRef] = obj

	for _, treeNode := range tree.Secondaries {
		secondaryMap, err := getObjectsFromTree(treeNode, store, scheme)
		if err != nil {
			return nil, err
		}
		for key, object := range secondaryMap {
			objectMap[key] = object
		}
	}
	return objectMap, nil
}


// TODO(free6om): similar as getSecondaryObjects, refactor and merge them
func getObjectTreeWithRevision(primary client.Object, ownershipRules []OwnershipRule, store ObjectRevisionStore, revision int64, scheme *runtime.Scheme) (*viewv1.ObjectTreeNode, error) {
	// find matched rules
	var matchedRules []*OwnershipRule
	for i := range ownershipRules {
		rule := &ownershipRules[i]
		gvk, err := objectTypeToGVK(&rule.Primary)
		if err != nil {
			return nil, err
		}
		primaryGVK, err := apiutil.GVKForObject(primary, scheme)
		if err != nil {
			return nil, err
		}
		if *gvk == primaryGVK {
			matchedRules = append(matchedRules, rule)
		}
	}

	reference, err := getObjectReference(primary, scheme)
	if err != nil {
		return nil, err
	}
	tree := &viewv1.ObjectTreeNode{
		Primary: *reference,
	}
	// traverse rules, build subtree
	var secondaries []client.Object
	for _, rule := range matchedRules {
		for _, ownedResource := range rule.OwnedResources {
			gvk, err := objectTypeToGVK(&ownedResource.Secondary)
			if err != nil {
				return nil, err
			}
			objects := getObjectsByRevision(gvk, store, revision)
			objects, err = filterByCriteria(primary, objects, &ownedResource.Criteria)
			if err != nil {
				return nil, err
			}
			secondaries = append(secondaries, objects...)
		}
	}
	for _, secondary := range secondaries {
		subTree, err := getObjectTreeWithRevision(secondary, ownershipRules, store, revision, scheme)
		if err != nil {
			return nil, err
		}
		tree.Secondaries = append(tree.Secondaries, subTree)
		slices.SortStableFunc(tree.Secondaries, func(a, b *viewv1.ObjectTreeNode) bool {
			return getObjectReferenceString(a) < getObjectReferenceString(b)
		})
	}

	return tree, nil
}

func filterByCriteria(primary client.Object, objects []client.Object, criteria *OwnershipCriteria) ([]client.Object, error) {
	var matchedObjects []client.Object
	opts, err := parseQueryOptions(primary, criteria)
	if err != nil {
		return nil, err
	}
	for _, object := range objects {
		if opts.match(object) {
			matchedObjects = append(matchedObjects, object)
		}
	}
	return matchedObjects, nil
}

func getObjectsByRevision(gvk *schema.GroupVersionKind, store ObjectRevisionStore, revision int64) []client.Object {
	objectMap := store.List(gvk)
	var matchedObjects []client.Object
	for _, revisionMap := range objectMap {
		rev := int64(-1)
		for r := range revisionMap {
			if rev < r && r <= revision {
				rev = r
			}
		}
		if rev > -1 {
			matchedObjects = append(matchedObjects, revisionMap[rev])
		}
	}
	return matchedObjects
}
