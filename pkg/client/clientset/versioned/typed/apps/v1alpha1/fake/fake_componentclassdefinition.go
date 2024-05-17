/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeComponentClassDefinitions implements ComponentClassDefinitionInterface
type FakeComponentClassDefinitions struct {
	Fake *FakeAppsV1alpha1
}

var componentclassdefinitionsResource = v1alpha1.SchemeGroupVersion.WithResource("componentclassdefinitions")

var componentclassdefinitionsKind = v1alpha1.SchemeGroupVersion.WithKind("ComponentClassDefinition")

// Get takes name of the componentClassDefinition, and returns the corresponding componentClassDefinition object, and an error if there is any.
func (c *FakeComponentClassDefinitions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ComponentClassDefinition, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(componentclassdefinitionsResource, name), &v1alpha1.ComponentClassDefinition{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentClassDefinition), err
}

// List takes label and field selectors, and returns the list of ComponentClassDefinitions that match those selectors.
func (c *FakeComponentClassDefinitions) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ComponentClassDefinitionList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(componentclassdefinitionsResource, componentclassdefinitionsKind, opts), &v1alpha1.ComponentClassDefinitionList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ComponentClassDefinitionList{ListMeta: obj.(*v1alpha1.ComponentClassDefinitionList).ListMeta}
	for _, item := range obj.(*v1alpha1.ComponentClassDefinitionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested componentClassDefinitions.
func (c *FakeComponentClassDefinitions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(componentclassdefinitionsResource, opts))
}

// Create takes the representation of a componentClassDefinition and creates it.  Returns the server's representation of the componentClassDefinition, and an error, if there is any.
func (c *FakeComponentClassDefinitions) Create(ctx context.Context, componentClassDefinition *v1alpha1.ComponentClassDefinition, opts v1.CreateOptions) (result *v1alpha1.ComponentClassDefinition, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(componentclassdefinitionsResource, componentClassDefinition), &v1alpha1.ComponentClassDefinition{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentClassDefinition), err
}

// Update takes the representation of a componentClassDefinition and updates it. Returns the server's representation of the componentClassDefinition, and an error, if there is any.
func (c *FakeComponentClassDefinitions) Update(ctx context.Context, componentClassDefinition *v1alpha1.ComponentClassDefinition, opts v1.UpdateOptions) (result *v1alpha1.ComponentClassDefinition, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(componentclassdefinitionsResource, componentClassDefinition), &v1alpha1.ComponentClassDefinition{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentClassDefinition), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeComponentClassDefinitions) UpdateStatus(ctx context.Context, componentClassDefinition *v1alpha1.ComponentClassDefinition, opts v1.UpdateOptions) (*v1alpha1.ComponentClassDefinition, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(componentclassdefinitionsResource, "status", componentClassDefinition), &v1alpha1.ComponentClassDefinition{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentClassDefinition), err
}

// Delete takes name of the componentClassDefinition and deletes it. Returns an error if one occurs.
func (c *FakeComponentClassDefinitions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(componentclassdefinitionsResource, name, opts), &v1alpha1.ComponentClassDefinition{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeComponentClassDefinitions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(componentclassdefinitionsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ComponentClassDefinitionList{})
	return err
}

// Patch applies the patch and returns the patched componentClassDefinition.
func (c *FakeComponentClassDefinitions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ComponentClassDefinition, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(componentclassdefinitionsResource, name, pt, data, subresources...), &v1alpha1.ComponentClassDefinition{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentClassDefinition), err
}