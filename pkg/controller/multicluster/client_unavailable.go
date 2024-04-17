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

type unavailableClient struct {
	unavailableClientReader
	unavailableClientWriter
	unavailableStatusClient
	unavailableSubResourceClientConstructor

	mctx mcontext
}

var _ client.Client = &unavailableClient{}

func (c *unavailableClient) Scheme() *runtime.Scheme {
	return c.mctx.control.Scheme()
}

func (c *unavailableClient) RESTMapper() meta.RESTMapper {
	return c.mctx.control.RESTMapper()
}

func (c *unavailableClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.mctx.control.GroupVersionKindFor(obj)
}

func (c *unavailableClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.mctx.control.IsObjectNamespaced(obj)
}

type unavailableClientReader struct{}

var _ client.Reader = &unavailableClientReader{}

func (c *unavailableClientReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return NewUnavailableError("Get")
}

func (c *unavailableClientReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return NewUnavailableError("List")
}

type unavailableClientWriter struct{}

var _ client.Writer = &unavailableClientWriter{}

func (c *unavailableClientWriter) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return NewUnavailableError("Create")
}

func (c *unavailableClientWriter) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return NewUnavailableError("Delete")
}

func (c *unavailableClientWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return NewUnavailableError("Delete")
}

func (c *unavailableClientWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return NewUnavailableError("Patch")
}

func (c *unavailableClientWriter) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return NewUnavailableError("DeleteAllOf")
}

type unavailableStatusClient struct{}

var _ client.StatusClient = &unavailableStatusClient{}

func (c *unavailableStatusClient) Status() client.SubResourceWriter {
	return &unavailableSubResourceWriter{}
}

type unavailableSubResourceClientConstructor struct{}

var _ client.SubResourceClientConstructor = &unavailableSubResourceClientConstructor{}

func (c *unavailableSubResourceClientConstructor) SubResource(subResource string) client.SubResourceClient {
	return &unavailableSubResourceClient{
		unavailableSubResourceReader: unavailableSubResourceReader{},
		unavailableSubResourceWriter: unavailableSubResourceWriter{},
	}
}

type unavailableSubResourceClient struct {
	unavailableSubResourceReader
	unavailableSubResourceWriter
}

var _ client.SubResourceClient = &unavailableSubResourceClient{}

type unavailableSubResourceReader struct{}

var _ client.SubResourceReader = &unavailableSubResourceReader{}

func (c *unavailableSubResourceReader) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	return NewUnavailableError("Get")
}

type unavailableSubResourceWriter struct{}

var _ client.SubResourceWriter = &unavailableSubResourceWriter{}

func (c *unavailableSubResourceWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return NewUnavailableError("Create")
}

func (c *unavailableSubResourceWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return NewUnavailableError("Update")
}

func (c *unavailableSubResourceWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return NewUnavailableError("Patch")
}
