/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	v1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeComponentParameters implements ComponentParameterInterface
type FakeComponentParameters struct {
	Fake *FakeParametersV1alpha1
	ns   string
}

var componentparametersResource = v1alpha1.SchemeGroupVersion.WithResource("componentparameters")

var componentparametersKind = v1alpha1.SchemeGroupVersion.WithKind("ComponentParameter")

// Get takes name of the componentParameter, and returns the corresponding componentParameter object, and an error if there is any.
func (c *FakeComponentParameters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ComponentParameter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(componentparametersResource, c.ns, name), &v1alpha1.ComponentParameter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentParameter), err
}

// List takes label and field selectors, and returns the list of ComponentParameters that match those selectors.
func (c *FakeComponentParameters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ComponentParameterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(componentparametersResource, componentparametersKind, c.ns, opts), &v1alpha1.ComponentParameterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ComponentParameterList{ListMeta: obj.(*v1alpha1.ComponentParameterList).ListMeta}
	for _, item := range obj.(*v1alpha1.ComponentParameterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested componentParameters.
func (c *FakeComponentParameters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(componentparametersResource, c.ns, opts))

}

// Create takes the representation of a componentParameter and creates it.  Returns the server's representation of the componentParameter, and an error, if there is any.
func (c *FakeComponentParameters) Create(ctx context.Context, componentParameter *v1alpha1.ComponentParameter, opts v1.CreateOptions) (result *v1alpha1.ComponentParameter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(componentparametersResource, c.ns, componentParameter), &v1alpha1.ComponentParameter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentParameter), err
}

// Update takes the representation of a componentParameter and updates it. Returns the server's representation of the componentParameter, and an error, if there is any.
func (c *FakeComponentParameters) Update(ctx context.Context, componentParameter *v1alpha1.ComponentParameter, opts v1.UpdateOptions) (result *v1alpha1.ComponentParameter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(componentparametersResource, c.ns, componentParameter), &v1alpha1.ComponentParameter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentParameter), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeComponentParameters) UpdateStatus(ctx context.Context, componentParameter *v1alpha1.ComponentParameter, opts v1.UpdateOptions) (*v1alpha1.ComponentParameter, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(componentparametersResource, "status", c.ns, componentParameter), &v1alpha1.ComponentParameter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentParameter), err
}

// Delete takes name of the componentParameter and deletes it. Returns an error if one occurs.
func (c *FakeComponentParameters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(componentparametersResource, c.ns, name, opts), &v1alpha1.ComponentParameter{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeComponentParameters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(componentparametersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ComponentParameterList{})
	return err
}

// Patch applies the patch and returns the patched componentParameter.
func (c *FakeComponentParameters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ComponentParameter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(componentparametersResource, c.ns, name, pt, data, subresources...), &v1alpha1.ComponentParameter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ComponentParameter), err
}
