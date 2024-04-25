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

package multicluster

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newUnavailableClient(context string) client.Client {
	return &unavailableClient{
		unavailableClientReader:                 unavailableClientReader{context},
		unavailableSubResourceClientConstructor: unavailableSubResourceClientConstructor{context},
		context:                                 context,
	}
}

func isUnavailableClient(c client.Client) bool {
	_, ok := c.(*unavailableClient)
	return ok
}

type unavailableClient struct {
	unavailableClientReader
	unavailableClientWriter
	unavailableStatusClient
	unavailableSubResourceClientConstructor

	context string
}

var _ client.Client = &unavailableClient{}

func (c *unavailableClient) Scheme() *runtime.Scheme {
	return nil
}

func (c *unavailableClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (c *unavailableClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, genericUnavailableError(c.context, obj)
}

func (c *unavailableClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return false, genericUnavailableError(c.context, obj)
}

type unavailableClientReader struct {
	context string
}

var _ client.Reader = &unavailableClientReader{}

func (c *unavailableClientReader) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	return getUnavailableError(c.context, obj)
}

func (c *unavailableClientReader) List(_ context.Context, objList client.ObjectList, _ ...client.ListOption) error {
	return listUnavailableError(c.context, objList)
}

type unavailableClientWriter struct{}

var _ client.Writer = &unavailableClientWriter{}

func (c *unavailableClientWriter) Create(context.Context, client.Object, ...client.CreateOption) error {
	return nil
}

func (c *unavailableClientWriter) Delete(context.Context, client.Object, ...client.DeleteOption) error {
	return nil
}

func (c *unavailableClientWriter) Update(context.Context, client.Object, ...client.UpdateOption) error {
	return nil
}

func (c *unavailableClientWriter) Patch(context.Context, client.Object, client.Patch, ...client.PatchOption) error {
	return nil
}

func (c *unavailableClientWriter) DeleteAllOf(context.Context, client.Object, ...client.DeleteAllOfOption) error {
	return nil
}

type unavailableStatusClient struct{}

var _ client.StatusClient = &unavailableStatusClient{}

func (c *unavailableStatusClient) Status() client.SubResourceWriter {
	return &unavailableSubResourceWriter{}
}

type unavailableSubResourceClientConstructor struct {
	context string
}

var _ client.SubResourceClientConstructor = &unavailableSubResourceClientConstructor{}

func (c *unavailableSubResourceClientConstructor) SubResource(string) client.SubResourceClient {
	return &unavailableSubResourceClient{
		unavailableSubResourceReader: unavailableSubResourceReader{c.context},
	}
}

type unavailableSubResourceClient struct {
	unavailableSubResourceReader
	unavailableSubResourceWriter
}

var _ client.SubResourceClient = &unavailableSubResourceClient{}

type unavailableSubResourceReader struct {
	context string
}

var _ client.SubResourceReader = &unavailableSubResourceReader{}

func (c *unavailableSubResourceReader) Get(_ context.Context, obj client.Object, _ client.Object, _ ...client.SubResourceGetOption) error {
	return getUnavailableError(c.context, obj)
}

type unavailableSubResourceWriter struct{}

var _ client.SubResourceWriter = &unavailableSubResourceWriter{}

func (c *unavailableSubResourceWriter) Create(context.Context, client.Object, client.Object, ...client.SubResourceCreateOption) error {
	return nil
}

func (c *unavailableSubResourceWriter) Update(context.Context, client.Object, ...client.SubResourceUpdateOption) error {
	return nil
}

func (c *unavailableSubResourceWriter) Patch(context.Context, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
	return nil
}
