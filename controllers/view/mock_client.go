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
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type mockClient struct {
	realClient client.Client
	store      map[model.GVKNObjKey]client.Object
}

func (c *mockClient) Get(ctx context.Context, objKey client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) Status() client.SubResourceWriter {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) SubResource(subResource string) client.SubResourceClient {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) Scheme() *runtime.Scheme {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) RESTMapper() meta.RESTMapper {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	//TODO implement me
	panic("implement me")
}

func (c *mockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func newMockClient(realClient client.Client, store map[model.GVKNObjKey]client.Object) client.Client {
	return &mockClient{
		realClient: realClient,
		store:      store,
	}
}

var _ client.Client = &mockClient{}
