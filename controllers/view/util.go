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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
func getObjectsByGVK(ctx context.Context, cli client.Client, gvk *schema.GroupVersionKind, matchingFields []client.MatchingFields, opts ...client.ListOption) ([]client.Object, error) {
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
	if err = cli.List(ctx, objectList, opts...); err != nil {
		return nil, err
	}
	runtimeObjects, err := meta.ExtractList(objectList)
	if err != nil {
		return nil, err
	}
	var objects []client.Object
	listOptions := &client.ListOptions{}
	for _, mf := range matchingFields {
		mf.ApplyToList(listOptions)
	}
	for _, object := range runtimeObjects {
		o, ok := object.(client.Object)
		if !ok {
			return nil, fmt.Errorf("object is not a client.Object for GVK %s", gvk)
		}
		if listOptions.FieldSelector != nil && !listOptions.FieldSelector.Matches(fields.Set{"metadata.name": o.GetName()}) {
			continue
		}
		objects = append(objects, o)
	}

	return objects, nil
}

func parseRevision(revisionStr string) int64 {
	revision, err := strconv.ParseInt(revisionStr, 10, 64)
	if err != nil {
		revision = 0
	}
	return revision
}

func parseListOptions(obj client.Object, criteria *OwnershipCriteria) ([]client.ListOption, error) {
	opts, err := parseMatchingLabels(obj, criteria)
	if err != nil {
		return nil, err
	}
	mfs, err := parseMatchingFields(obj, criteria)
	if err != nil {
		return nil, err
	}
	for _, mf := range mfs {
		opts = append(opts, mf)
	}
	return opts, nil
}

func parseMatchingLabels(obj client.Object, criteria *OwnershipCriteria) ([]client.ListOption, error) {
	var opts []client.ListOption
	if criteria.SelectorCriteria != nil {
		ml, err := parseSelector(obj, criteria.SelectorCriteria.Path)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.MatchingLabels(ml))
	}
	if criteria.LabelCriteria != nil {
		ml := parseLabels(obj, criteria.LabelCriteria)
		opts = append(opts, client.MatchingLabels(ml))
	}
	return opts, nil
}

func parseMatchingFields(obj client.Object, criteria *OwnershipCriteria) ([]client.MatchingFields, error) {
	if criteria.SpecifiedNameCriteria != nil {
		fieldMap, err := flattenObject(obj)
		if err != nil {
			return nil, err
		}
		name, ok := fieldMap[criteria.SpecifiedNameCriteria.Path]
		if ok {
			return []client.MatchingFields{{"metadata.name": name}}, nil
		}
	}
	return nil, nil
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
	secondaries, err := getSecondaryObjectsOf(ctx, cli, primary, matchedRules)
	if err != nil {
		return nil, err
	}
	for _, secondary := range secondaries {
		subTree, err := getObjectTreeFromCache(ctx, cli, secondary, ownershipRules)
		if err != nil {
			return nil, err
		}
		tree.Secondaries = append(tree.Secondaries, subTree)
	}

	return tree, nil
}

func getObjectsFromCache(ctx context.Context, cli client.Client, root *appsv1alpha1.Cluster, ownershipRules []OwnershipRule) (sets.Set[model.GVKNObjKey], map[model.GVKNObjKey]client.Object, error) {
	objectMap := make(map[model.GVKNObjKey]client.Object)
	objectSet := sets.New[model.GVKNObjKey]()
	waitingList := list.New()
	waitingList.PushFront(root)
	for waitingList.Len() > 0 {
		e := waitingList.Front()
		waitingList.Remove(e)
		obj, _ := e.Value.(client.Object)
		objKey, err := getObjectRef(obj, cli.Scheme())
		if err != nil {
			return nil, nil, err
		}
		objectSet.Insert(*objKey)
		objectMap[*objKey] = obj

		secondaries, err := getSecondaryObjectsOf(ctx, cli, obj, ownershipRules)
		if err != nil {
			return nil, nil, err
		}
		for _, secondary := range secondaries {
			waitingList.PushBack(secondary)
		}
	}
	return objectSet, objectMap, nil
}

func getSecondaryObjectsOf(ctx context.Context, cli client.Client, obj client.Object, ownershipRules []OwnershipRule) ([]client.Object, error) {
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
			opts, err := parseMatchingLabels(obj, &ownedResource.Criteria)
			if err != nil {
				return nil, err
			}
			mf, err := parseMatchingFields(obj, &ownedResource.Criteria)
			if err != nil {
				return nil, err
			}
			objects, err := getObjectsByGVK(ctx, cli, gvk, mf, opts...)
			if err != nil {
				return nil, err
			}
			secondaries = append(secondaries, objects...)
		}
	}

	return secondaries, nil
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
