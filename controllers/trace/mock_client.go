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
	"context"
	"encoding/json"
	"fmt"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
)

type mockClient struct {
	realClient        client.Client
	subResourceClient client.SubResourceWriter
	store             ChangeCaptureStore
	managedGVK        sets.Set[schema.GroupVersionKind]
}

type mockSubResourceClient struct {
	store  ChangeCaptureStore
	scheme *runtime.Scheme
}

func (c *mockClient) Get(ctx context.Context, objKey client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	objectRef, err := getObjectRef(obj, c.realClient.Scheme())
	if err != nil {
		return err
	}
	objectRef.ObjectKey = objKey
	res := c.store.Get(objectRef)
	if res == nil {
		return c.realClient.Get(ctx, objKey, obj, opts...)
	}
	return copyObj(res, obj)
}

func (c *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	// Get the GVK for the list type
	gvk, err := apiutil.GVKForObject(list, c.realClient.Scheme())
	if err != nil {
		return fmt.Errorf("failed to get GVK for list: %w", err)
	}
	gvk.Kind, _ = strings.CutSuffix(gvk.Kind, "List")

	if !c.managedGVK.Has(gvk) {
		return c.realClient.List(ctx, list, opts...)
	}

	// Get the objects of the same GVK from the store
	objects := c.store.List(&gvk)

	// Iterate over stored objects and add them to the list
	items, err := meta.ExtractList(list)
	if err != nil {
		return fmt.Errorf("failed to extract list: %w", err)
	}

	// Extract namespace and other options from opts
	listOptions := &client.ListOptions{}
	listOptions.ApplyOptions(opts)

	for _, obj := range objects {
		if listOptions.Namespace != "" && obj.GetNamespace() != listOptions.Namespace {
			continue
		}
		if listOptions.LabelSelector != nil && !listOptions.LabelSelector.Matches(labels.Set(obj.GetLabels())) {
			continue
		}
		items = append(items, obj.DeepCopyObject())
	}

	// Set the items back to the list
	if err := meta.SetList(list, items); err != nil {
		return fmt.Errorf("failed to set list: %w", err)
	}

	return nil
}

func (c *mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	objectRef, err := getObjectRef(obj, c.realClient.Scheme())
	if err != nil {
		return err
	}
	o := c.store.Get(objectRef)
	if o != nil {
		return apierrors.NewAlreadyExists(objectRef.GroupVersion().WithResource(objectRef.Kind).GroupResource(), fmt.Sprintf("%s/%s", objectRef.Namespace, objectRef.Name))
	}
	obj.SetGeneration(1)
	return c.store.Insert(obj)
}

func (c *mockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	objectRef, err := getObjectRef(obj, c.realClient.Scheme())
	if err != nil {
		return err
	}
	object := c.store.Get(objectRef)
	if object == nil {
		return nil
	}
	if object.GetDeletionTimestamp() == nil {
		ts := metav1.Now()
		object.SetDeletionTimestamp(&ts)
		return c.store.Update(object)
	}
	return c.store.Delete(obj)
}

func (c *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	objectRef, err := getObjectRef(obj, c.realClient.Scheme())
	if err != nil {
		return err
	}
	oldObj := c.store.Get(objectRef)
	if oldObj == nil {
		return apierrors.NewNotFound(objectRef.GroupVersion().WithResource(objectRef.Kind).GroupResource(), fmt.Sprintf("%s/%s", objectRef.Namespace, objectRef.Name))
	}
	metaChanged := checkMetadata(oldObj, obj)
	specChanged := increaseGeneration(oldObj, obj)
	if metaChanged || specChanged {
		return c.store.Update(obj)
	}
	return nil
}

func (c *mockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return doPatch(obj, patch, c.store, c.realClient.Scheme(), true)
}

func doPatch(obj client.Object, patch client.Patch, store ChangeCaptureStore, scheme *runtime.Scheme, checkGeneration bool) error {
	objectRef, err := getObjectRef(obj, scheme)
	if err != nil {
		return err
	}
	oldObj := store.Get(objectRef)
	if oldObj == nil {
		return apierrors.NewNotFound(objectRef.GroupVersion().WithResource(objectRef.Kind).GroupResource(), fmt.Sprintf("%s/%s", objectRef.Namespace, objectRef.Name))
	}
	patchData, err := patch.Data(obj)
	if err != nil {
		return err
	}
	newObj := oldObj.DeepCopyObject().(client.Object)
	if err = applyPatch(newObj, patch.Type(), patchData); err != nil {
		return err
	}
	metaChanged := checkMetadata(oldObj, newObj)
	specChanged := false
	statusChanged := false
	if checkGeneration {
		specChanged = increaseGeneration(oldObj, newObj)
	} else {
		statusChanged = checkStatus(oldObj, newObj)
	}
	if metaChanged || specChanged || statusChanged {
		return store.Update(newObj)
	}
	return nil
}

func applyPatch(obj client.Object, patchType types.PatchType, patchData []byte) error {
	// Convert the object to JSON
	originalJSON, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal original object: %w", err)
	}

	// Apply the patch
	var patchedJSON []byte
	switch patchType {
	case types.StrategicMergePatchType:
		patchedJSON, err = strategicpatch.StrategicMergePatch(originalJSON, patchData, obj)
	case types.MergePatchType:
		patchedJSON, err = jsonpatch.MergePatch(originalJSON, patchData)
	default:
		return fmt.Errorf("unsupported patch type: %s", patchType)
	}
	if err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// Unmarshal the patched JSON back into the object
	return json.Unmarshal(patchedJSON, obj)
}

func checkMetadata(oldObj client.Object, newObj client.Object) bool {
	if !reflect.DeepEqual(oldObj.GetFinalizers(), newObj.GetFinalizers()) ||
		!reflect.DeepEqual(oldObj.GetOwnerReferences(), newObj.GetOwnerReferences()) ||
		!reflect.DeepEqual(oldObj.GetAnnotations(), newObj.GetAnnotations()) ||
		!reflect.DeepEqual(oldObj.GetLabels(), newObj.GetLabels()) {
		return true
	}
	return false
}

func increaseGeneration(oldObj, newObj client.Object) bool {
	oldObjCopy, _ := normalize(oldObj)
	newObjCopy, _ := normalize(newObj)
	if oldObjCopy == nil || newObjCopy == nil {
		return false
	}
	oldSpec, _ := getFieldAsStruct(oldObjCopy, specFieldName)
	newSpec, _ := getFieldAsStruct(newObjCopy, specFieldName)
	if oldSpec == nil || newSpec == nil {
		return false
	}
	diff := cmp.Diff(oldSpec, newSpec)
	if diff == "" {
		return false
	}
	newObj.SetGeneration(newObj.GetGeneration() + 1)
	return true
}

func checkStatus(oldObj client.Object, newObj client.Object) bool {
	oldStatus, _ := getFieldAsStruct(oldObj, statusFieldName)
	newStatus, _ := getFieldAsStruct(newObj, statusFieldName)
	if oldStatus == nil || newStatus == nil {
		return false
	}
	return reflect.DeepEqual(oldStatus, newStatus)
}

func (c *mockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("not implemented")
}

func (c *mockClient) Status() client.SubResourceWriter {
	return c.subResourceClient
}

func (c *mockClient) SubResource(subResource string) client.SubResourceClient {
	panic("not implemented")
}

func (c *mockClient) Scheme() *runtime.Scheme {
	return c.realClient.Scheme()
}

func (c *mockClient) RESTMapper() meta.RESTMapper {
	return c.realClient.RESTMapper()
}

func (c *mockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.realClient.GroupVersionKindFor(obj)
}

func (c *mockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.realClient.IsObjectNamespaced(obj)
}

func (c *mockSubResourceClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	panic("not implemented")
}

func (c *mockSubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	objectRef, err := getObjectRef(obj, c.scheme)
	if err != nil {
		return err
	}
	oldObj := c.store.Get(objectRef)
	if oldObj == nil {
		return apierrors.NewNotFound(objectRef.GroupVersion().WithResource(objectRef.Kind).GroupResource(), fmt.Sprintf("%s/%s", objectRef.Namespace, objectRef.Name))
	}
	if checkMetadata(oldObj, obj) {
		return c.store.Update(obj)
	}
	return nil
}

func (c *mockSubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return doPatch(obj, patch, c.store, c.scheme, false)
}

func newMockClient(realClient client.Client, store ChangeCaptureStore, rules []OwnershipRule) (client.Client, error) {
	managedGVK := sets.New[schema.GroupVersionKind]()
	addToManaged := func(objType *tracev1.ObjectType) error {
		gvk, err := objectTypeToGVK(objType)
		if err != nil {
			return err
		}
		managedGVK.Insert(*gvk)
		return nil
	}
	for _, rule := range rules {
		if err := addToManaged(&rule.Primary); err != nil {
			return nil, err
		}
		for _, ownedResource := range rule.OwnedResources {
			if err := addToManaged(&ownedResource.Secondary); err != nil {
				return nil, err
			}
		}
	}

	return &mockClient{
		realClient: realClient,
		store:      store,
		managedGVK: managedGVK,
		subResourceClient: &mockSubResourceClient{
			store:  store,
			scheme: realClient.Scheme(),
		},
	}, nil
}

func copyObj(src, dst client.Object) error {
	srcJSON, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal src object: %w", err)
	}
	return json.Unmarshal(srcJSON, dst)
}

var _ client.Client = &mockClient{}
var _ client.SubResourceWriter = &mockSubResourceClient{}
