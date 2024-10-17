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

package trace

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

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
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func objectTypeToGVK(objectType *tracev1.ObjectType) (*schema.GroupVersionKind, error) {
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

func objectReferenceToType(objectRef *corev1.ObjectReference) *tracev1.ObjectType {
	if objectRef == nil {
		return nil
	}
	return &tracev1.ObjectType{
		APIVersion: objectRef.APIVersion,
		Kind:       objectRef.Kind,
	}
}

func objectReferenceToRef(reference *corev1.ObjectReference) *model.GVKNObjKey {
	if reference == nil {
		return nil
	}
	return &model.GVKNObjKey{
		GroupVersionKind: reference.GroupVersionKind(),
		ObjectKey: client.ObjectKey{
			Namespace: reference.Namespace,
			Name:      reference.Name,
		},
	}
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

func objectRefToType(objectRef *model.GVKNObjKey) *tracev1.ObjectType {
	if objectRef == nil {
		return nil
	}
	return &tracev1.ObjectType{
		APIVersion: objectRef.GroupVersionKind.GroupVersion().String(),
		Kind:       objectRef.Kind,
	}
}

func objectType(apiVersion, kind string) tracev1.ObjectType {
	return tracev1.ObjectType{
		APIVersion: apiVersion,
		Kind:       kind,
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

func getObjectReferenceString(n *tracev1.ObjectTreeNode) string {
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

func getObjectTreeFromCache(ctx context.Context, cli client.Client, primary client.Object, ownershipRules []OwnershipRule) (*tracev1.ObjectTreeNode, error) {
	if primary == nil {
		return nil, nil
	}

	// primary tree node
	reference, err := getObjectReference(primary, cli.Scheme())
	if err != nil {
		return nil, err
	}
	tree := &tracev1.ObjectTreeNode{
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
		sort.SliceStable(tree.Secondaries, func(i, j int) bool {
			return getObjectReferenceString(tree.Secondaries[i]) < getObjectReferenceString(tree.Secondaries[j])
		})
	}

	return tree, nil
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
	// find matched rules
	rules, err := findMatchedRules(obj, ownershipRules, cli.Scheme())
	if err != nil {
		return nil, err
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
	if criteria.Validation != "" && criteria.Validation != NoValidation {
		opts.matchOwner = &matchOwner{
			ownerUID:   primary.GetUID(),
			controller: criteria.Validation == ControllerValidation,
		}
	}
	return opts, nil
}

// parseSelector checks if a field exists in the object and returns it if it's a metav1.LabelSelector
func parseSelector(obj client.Object, fieldPath string) (map[string]string, error) {
	selectorField, err := parseField(obj, fieldPath)
	if err != nil {
		return nil, err
	}
	// Attempt to convert the final field to a LabelSelector
	// TODO(free6om): handle metav1.LabelSelector
	// labelSelector := &metav1.LabelSelector{}
	labelSelector := make(map[string]string)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(selectorField, &labelSelector); err != nil {
		return nil, fmt.Errorf("failed to parse as LabelSelector: %w", err)
	}

	return labelSelector, nil
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

func buildObjectSummaries(initialObjectMap, newObjectMap map[model.GVKNObjKey]client.Object) []tracev1.ObjectSummary {
	initialObjectSet, newObjectSet := sets.KeySet(initialObjectMap), sets.KeySet(newObjectMap)
	createSet := newObjectSet.Difference(initialObjectSet)
	updateSet := newObjectSet.Intersection(initialObjectSet)
	deleteSet := initialObjectSet.Difference(newObjectSet)
	summaryMap := make(map[tracev1.ObjectType]*tracev1.ObjectSummary)
	doCount := func(s sets.Set[model.GVKNObjKey], summaryUpdater func(objectRef *model.GVKNObjKey, summary *tracev1.ObjectSummary)) {
		for objectRef := range s {
			key := *objectRefToType(&objectRef)
			summary, ok := summaryMap[key]
			if !ok {
				summary = &tracev1.ObjectSummary{
					ObjectType: key,
					Total:      0,
				}
				summaryMap[key] = summary
			}
			if summary.ChangeSummary == nil {
				summary.ChangeSummary = &tracev1.ObjectChangeSummary{}
			}
			summaryUpdater(&objectRef, summary)
		}
	}
	doCount(createSet, func(_ *model.GVKNObjKey, summary *tracev1.ObjectSummary) {
		summary.Total += 1
		if summary.ChangeSummary.Added == nil {
			summary.ChangeSummary.Added = pointer.Int32(0)
		}
		*summary.ChangeSummary.Added += 1
	})
	doCount(updateSet, func(objectRef *model.GVKNObjKey, summary *tracev1.ObjectSummary) {
		initialObj := initialObjectMap[*objectRef]
		newObj := newObjectMap[*objectRef]
		summary.Total += 1
		if initialObj != nil && newObj != nil && initialObj.GetResourceVersion() == newObj.GetResourceVersion() {
			return
		}
		if summary.ChangeSummary.Updated == nil {
			summary.ChangeSummary.Updated = pointer.Int32(0)
		}
		*summary.ChangeSummary.Updated += 1
	})
	doCount(deleteSet, func(_ *model.GVKNObjKey, summary *tracev1.ObjectSummary) {
		if summary.ChangeSummary.Deleted == nil {
			summary.ChangeSummary.Deleted = pointer.Int32(0)
		}
		*summary.ChangeSummary.Deleted += 1
	})

	var objectSummaries []tracev1.ObjectSummary
	for _, summary := range summaryMap {
		objectSummaries = append(objectSummaries, *summary)
	}
	sort.SliceStable(objectSummaries, func(i, j int) bool {
		a, b := &objectSummaries[i], &objectSummaries[j]
		if a.ObjectType.APIVersion != b.ObjectType.APIVersion {
			return a.ObjectType.APIVersion < b.ObjectType.APIVersion
		}
		return a.ObjectType.Kind < b.ObjectType.Kind
	})

	return objectSummaries
}

func buildChanges(oldObjectMap, newObjectMap map[model.GVKNObjKey]client.Object,
	descriptionFormat func(client.Object, client.Object, tracev1.ObjectChangeType, *schema.GroupVersionKind) (string, *string)) []tracev1.ObjectChange {
	// calculate createSet, deleteSet and updateSet
	newObjectSet := sets.KeySet(newObjectMap)
	oldObjectSet := sets.KeySet(oldObjectMap)
	createSet := newObjectSet.Difference(oldObjectSet)
	updateSet := newObjectSet.Intersection(oldObjectSet)
	deleteSet := oldObjectSet.Difference(newObjectSet)

	// build new slice of reconciliation changes from last round calculation
	var changes []tracev1.ObjectChange
	allObjectMap := map[tracev1.ObjectChangeType]sets.Set[model.GVKNObjKey]{
		tracev1.ObjectCreationType: createSet,
		tracev1.ObjectUpdateType:   updateSet,
		tracev1.ObjectDeletionType: deleteSet,
	}
	for _, changeType := range []tracev1.ObjectChangeType{tracev1.ObjectCreationType, tracev1.ObjectUpdateType, tracev1.ObjectDeletionType} {
		changeSet := allObjectMap[changeType]
		for key := range changeSet {
			var oldObj, newObj client.Object
			if oldObjectMap != nil {
				oldObj = oldObjectMap[key]
			}
			if newObjectMap != nil {
				newObj = newObjectMap[key]
			}
			obj := newObj
			if changeType == tracev1.ObjectDeletionType {
				obj = oldObj
			}
			if changeType == tracev1.ObjectUpdateType &&
				(oldObj == nil || newObj == nil || oldObj.GetResourceVersion() == newObj.GetResourceVersion()) {
				continue
			}
			isEvent := isEvent(&key.GroupVersionKind)
			if isEvent && changeType == tracev1.ObjectDeletionType {
				continue
			}
			var (
				ref             *corev1.ObjectReference
				eventAttributes *tracev1.EventAttributes
			)
			if isEvent {
				changeType = tracev1.EventType
				evt, _ := obj.(*corev1.Event)
				ref = &evt.InvolvedObject
				eventAttributes = &tracev1.EventAttributes{
					Name:   evt.Name,
					Type:   evt.Type,
					Reason: evt.Reason,
				}
			} else {
				ref = objectRefToReference(key, obj.GetUID(), obj.GetResourceVersion())
			}
			description, localDescription := descriptionFormat(oldObj, newObj, changeType, &key.GroupVersionKind)
			change := tracev1.ObjectChange{
				ObjectReference:  *ref,
				ChangeType:       changeType,
				EventAttributes:  eventAttributes,
				Revision:         parseRevision(obj.GetResourceVersion()),
				Timestamp:        func() *metav1.Time { t := metav1.Now(); return &t }(),
				Description:      description,
				LocalDescription: localDescription,
			}
			changes = append(changes, change)
		}
	}
	return changes
}

func buildDescriptionFormatter(i18nResources *corev1.ConfigMap, defaultLocale string, locale *string) func(client.Object, client.Object, tracev1.ObjectChangeType, *schema.GroupVersionKind) (string, *string) {
	return func(oldObj client.Object, newObj client.Object, changeType tracev1.ObjectChangeType, gvk *schema.GroupVersionKind) (string, *string) {
		description := formatDescription(oldObj, newObj, changeType, gvk, i18nResources, &defaultLocale)
		localDescription := formatDescription(oldObj, newObj, changeType, gvk, i18nResources, locale)
		return *description, localDescription
	}
}

func formatDescription(oldObj, newObj client.Object, changeType tracev1.ObjectChangeType, gvk *schema.GroupVersionKind, i18nResource *corev1.ConfigMap, locale *string) *string {
	if locale == nil {
		return nil
	}
	defaultStr := pointer.String(string(changeType))
	if oldObj == nil && newObj == nil {
		return defaultStr
	}
	if err := defaultResourcesManager.ParseRaw(i18nResource); err != nil {
		return defaultStr
	}
	obj := newObj
	if obj == nil {
		obj = oldObj
	}
	var (
		key        string
		needFormat bool
	)
	if isEvent(gvk) {
		evt, _ := obj.(*corev1.Event)
		defaultStr = pointer.String(evt.Message)
		key = evt.Message
	} else {
		key = fmt.Sprintf("%s/%s/%s", gvk.GroupVersion().String(), gvk.Kind, changeType)
		needFormat = true
	}
	formatString := defaultResourcesManager.GetFormatString(key, *locale)
	if len(formatString) == 0 {
		return defaultStr
	}
	result := formatString
	if needFormat {
		result = fmt.Sprintf(formatString, obj.GetNamespace(), obj.GetName())
	}
	return pointer.String(result)
}

func isEvent(gvk *schema.GroupVersionKind) bool {
	return *gvk == eventGVK
}

func getObjectsFromTree(tree *tracev1.ObjectTreeNode, store ObjectRevisionStore, scheme *runtime.Scheme) (map[model.GVKNObjKey]client.Object, error) {
	if tree == nil {
		return nil, nil
	}
	objectRef := objectReferenceToRef(&tree.Primary)
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

func getObjectTreeWithRevision(primary client.Object, ownershipRules []OwnershipRule, store ObjectRevisionStore, revision int64, scheme *runtime.Scheme) (*tracev1.ObjectTreeNode, error) {
	// find matched rules
	matchedRules, err := findMatchedRules(primary, ownershipRules, scheme)
	if err != nil {
		return nil, err
	}

	reference, err := getObjectReference(primary, scheme)
	if err != nil {
		return nil, err
	}
	tree := &tracev1.ObjectTreeNode{
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
		sort.SliceStable(tree.Secondaries, func(i, j int) bool {
			return getObjectReferenceString(tree.Secondaries[i]) < getObjectReferenceString(tree.Secondaries[j])
		})
	}

	return tree, nil
}

func findMatchedRules(obj client.Object, ownershipRules []OwnershipRule, scheme *runtime.Scheme) ([]*OwnershipRule, error) {
	targetGVK, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return nil, err
	}
	var matchedRules []*OwnershipRule
	for i := range ownershipRules {
		rule := &ownershipRules[i]
		gvk, err := objectTypeToGVK(&rule.Primary)
		if err != nil {
			return nil, err
		}

		if *gvk == targetGVK {
			matchedRules = append(matchedRules, rule)
		}
	}
	return matchedRules, nil
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

func deleteUnusedRevisions(store ObjectRevisionStore, changes []tracev1.ObjectChange, reference client.Object) {
	for _, change := range changes {
		objectRef := objectReferenceToRef(&change.ObjectReference)
		if change.ChangeType == tracev1.EventType {
			objectRef.GroupVersionKind = eventGVK
			objectRef.Name = change.EventAttributes.Name
		}
		store.Delete(objectRef, reference, change.Revision)
	}
}
