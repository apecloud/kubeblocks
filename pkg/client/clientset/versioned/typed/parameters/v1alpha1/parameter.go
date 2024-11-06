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

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	scheme "github.com/apecloud/kubeblocks/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ParametersGetter has a method to return a ParameterInterface.
// A group's client should implement this interface.
type ParametersGetter interface {
	Parameters(namespace string) ParameterInterface
}

// ParameterInterface has methods to work with Parameter resources.
type ParameterInterface interface {
	Create(ctx context.Context, parameter *v1alpha1.Parameter, opts v1.CreateOptions) (*v1alpha1.Parameter, error)
	Update(ctx context.Context, parameter *v1alpha1.Parameter, opts v1.UpdateOptions) (*v1alpha1.Parameter, error)
	UpdateStatus(ctx context.Context, parameter *v1alpha1.Parameter, opts v1.UpdateOptions) (*v1alpha1.Parameter, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.Parameter, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ParameterList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Parameter, err error)
	ParameterExpansion
}

// parameters implements ParameterInterface
type parameters struct {
	client rest.Interface
	ns     string
}

// newParameters returns a Parameters
func newParameters(c *ParametersV1alpha1Client, namespace string) *parameters {
	return &parameters{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the parameter, and returns the corresponding parameter object, and an error if there is any.
func (c *parameters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Parameter, err error) {
	result = &v1alpha1.Parameter{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("parameters").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Parameters that match those selectors.
func (c *parameters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ParameterList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ParameterList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("parameters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested parameters.
func (c *parameters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("parameters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a parameter and creates it.  Returns the server's representation of the parameter, and an error, if there is any.
func (c *parameters) Create(ctx context.Context, parameter *v1alpha1.Parameter, opts v1.CreateOptions) (result *v1alpha1.Parameter, err error) {
	result = &v1alpha1.Parameter{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("parameters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(parameter).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a parameter and updates it. Returns the server's representation of the parameter, and an error, if there is any.
func (c *parameters) Update(ctx context.Context, parameter *v1alpha1.Parameter, opts v1.UpdateOptions) (result *v1alpha1.Parameter, err error) {
	result = &v1alpha1.Parameter{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("parameters").
		Name(parameter.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(parameter).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *parameters) UpdateStatus(ctx context.Context, parameter *v1alpha1.Parameter, opts v1.UpdateOptions) (result *v1alpha1.Parameter, err error) {
	result = &v1alpha1.Parameter{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("parameters").
		Name(parameter.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(parameter).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the parameter and deletes it. Returns an error if one occurs.
func (c *parameters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("parameters").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *parameters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("parameters").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched parameter.
func (c *parameters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Parameter, err error) {
	result = &v1alpha1.Parameter{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("parameters").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}