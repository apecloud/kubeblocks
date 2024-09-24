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
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

type ObjectTreeRootFinder interface {
	GetEventChannel() chan event.GenericEvent
	GetEventHandler() handler.EventHandler
}

type rootFinder struct {
	client.Client
	logger    logr.Logger
	eventChan chan event.GenericEvent
}

func (f *rootFinder) GetEventChannel() chan event.GenericEvent {
	return f.eventChan
}

func (f *rootFinder) GetEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(f.findRoots)
}

// findRoots finds the root(s) object of the 'object' by the object tree.
// The basic idea is, find the parent(s) of the object based on ownership rules defined in view definition,
// and do this recursively until find all the root object(s).
func (f *rootFinder) findRoots(ctx context.Context, object client.Object) []reconcile.Request {
	waitingList := list.New()
	waitingList.PushFront(object)

	var roots []client.Object
	viewDefList := &viewv1.ReconciliationViewDefinitionList{}
	if err := f.List(ctx, viewDefList); err != nil {
		f.logger.Error(err, "list reconciliation view definition failed")
		return nil
	}
	var primaryTypeList []viewv1.ObjectType
	var ownershipRuleList []viewv1.OwnershipRule
	for _, viewDef := range viewDefList.Items {
		// build the ownership hierarchy as a DAG
		dag := graph.NewDAG()
		for _, rule := range viewDef.Spec.OwnershipRules {
			dag.AddVertex(rule.Primary)
			for _, resource := range rule.OwnedResources {
				dag.AddVertex(resource.Secondary)
				dag.Connect(rule.Primary, resource.Secondary)
			}
			ownershipRuleList = append(ownershipRuleList, rule)
		}
		// DAG should be valid(one and only one root without cycle)
		if err := dag.Validate(); err != nil {
			f.logger.Error(err, "invalid spec.ownershipRules in view definition %s", viewDef.Name)
			return nil
		}
		primaryTypeList = append(primaryTypeList, dag.Root().(viewv1.ObjectType))
	}

	for waitingList.Len() > 0 {
		e := waitingList.Front()
		waitingList.Remove(e)
		obj, _ := e.Value.(client.Object)
		objGVK, err := apiutil.GVKForObject(obj, f.Scheme())
		if err != nil {
			f.logger.Error(err, "get GVK of %s/%s failed", obj.GetNamespace(), obj.GetName())
			return nil
		}
		found := false
		for _, primaryType := range primaryTypeList {
			gvk, err := objectTypeToGVK(&primaryType)
			if err != nil {
				f.logger.Error(err, "convert objectType %s to GVK failed", primaryType)
				return nil
			}
			if objGVK == *gvk {
				roots = append(roots, obj)
				found = true
			}
		}
		if found {
			continue
		}
		for _, rule := range ownershipRuleList {
			for _, resource := range rule.OwnedResources {
				gvk, err := objectTypeToGVK(&resource.Secondary)
				if err != nil {
					f.logger.Error(err, "convert objectType %s to GVK failed", resource.Secondary)
					return nil
				}
				if objGVK != *gvk {
					continue
				}
				primaryGVK, err := objectTypeToGVK(&rule.Primary)
				if err != nil {
					f.logger.Error(err, "convert objectType %s to GVK failed", rule.Primary)
					return nil
				}
				objectList, err := getObjectsByGVK(ctx, f, primaryGVK)
				if err != nil {
					f.logger.Error(err, "getObjectsByGVK for GVK %s failed", primaryGVK)
					return nil
				}
				for _, owner := range objectList {
					if ownedBy(owner, obj, resource) {
						waitingList.PushBack(owner)
					}
				}
			}
		}
	}

	result := sets.New[reconcile.Request]()
	for _, root := range roots {
		result.Insert(reconcile.Request{NamespacedName: client.ObjectKeyFromObject(root)})
	}

	return result.UnsortedList()
}

func ownedBy(owner client.Object, obj client.Object, ownedResource viewv1.OwnedResource) bool {
	if ownedResource.Criteria.SelectorCriteria != nil {
		labels, err := parseSelector(owner, ownedResource.Criteria.SelectorCriteria.Path)
		if err != nil {
			return false
		}
		if len(labels) > 0 && isSubset(labels, obj.GetLabels()) {
			return true
		}
		return false
	}

	if ownedResource.Criteria.LabelCriteria != nil {
		labels := parseLabels(owner, ownedResource.Criteria.LabelCriteria)
		if len(labels) > 0 && isSubset(labels, obj.GetLabels()) {
			return true
		}
		return false
	}

	// TODO(free6om): handle builtin

	return false
}

func parseLabels(obj client.Object, criteria map[string]string) map[string]string {
	labels := make(map[string]string, len(criteria))
	for k, v := range criteria {
		value := strings.ReplaceAll(v, "$(primary.name)", obj.GetName())
		labels[k] = value
	}
	return labels
}

// parseSelector checks if a field exists in the object and returns it if it's a metav1.LabelSelector
func parseSelector(obj client.Object, fieldPath string) (map[string]string, error) {
	// Convert client.Object to unstructured to handle dynamic fields
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	// Use the field path to find the field
	pathParts := strings.Split(fieldPath, ".")
	current := unstructuredObj
	for _, part := range pathParts {
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil, fmt.Errorf("field '%s' does not exist", fieldPath)
		}
	}

	// Attempt to convert the final field to a LabelSelector
	// TODO(free6om): handle metav1.LabelSelector
	//labelSelector := &metav1.LabelSelector{}
	labelSelector := make(map[string]string)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(current, labelSelector); err != nil {
		return nil, fmt.Errorf("failed to parse as LabelSelector: %w", err)
	}

	return labelSelector, nil
}

// isSubset checks if map1 is a subset of map2
func isSubset(map1, map2 map[string]string) bool {
	for key, value := range map1 {
		// Check if the key exists in map2 and if the values match
		if v, exists := map2[key]; !exists || v != value {
			return false
		}
	}
	return true
}

func NewObjectTreeRootFinder(cli client.Client) ObjectTreeRootFinder {
	logger := log.FromContext(context.Background()).WithName("ObjectTreeRootFinder")
	return &rootFinder{
		Client:    cli,
		logger:    logger,
		eventChan: make(chan event.GenericEvent),
	}
}

var _ ObjectTreeRootFinder = &rootFinder{}
