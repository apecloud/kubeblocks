/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func NewClient(global client.Client, workers map[string]client.Client) client.Client {
	ctx := multiClientContext{
		global:  global,
		workers: workers,
	}
	return &multiClient{
		multiClientReader:                 multiClientReader{ctx},
		multiClientWriter:                 multiClientWriter{ctx},
		multiStatusClient:                 multiStatusClient{ctx},
		multiSubResourceClientConstructor: multiSubResourceClientConstructor{ctx},
		ctx:                               ctx,
	}
}

type multiClientContext struct {
	global  client.Client
	workers map[string]client.Client
}

type multiClient struct {
	multiClientReader
	multiClientWriter
	multiStatusClient
	multiSubResourceClientConstructor

	ctx multiClientContext
}

var _ client.Client = &multiClient{}

func (c *multiClient) Scheme() *runtime.Scheme {
	return c.ctx.global.Scheme()
}

func (c *multiClient) RESTMapper() meta.RESTMapper {
	return c.ctx.global.RESTMapper()
}

func (c *multiClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.ctx.global.GroupVersionKindFor(obj)
}

func (c *multiClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.ctx.global.IsObjectNamespaced(obj)
}

type multiClientReader struct {
	ctx multiClientContext
}

var _ client.Reader = &multiClientReader{}

func (c *multiClientReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	request := func(cli client.Client) error {
		return cli.Get(ctx, key, obj, opts...)
	}
	return anyOf(c.ctx, ctx, obj, request, opts)
}

func (c *multiClientReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	items := reflect.ValueOf(list).Elem().FieldByName("Items")
	if !items.IsValid() {
		return fmt.Errorf("ObjectList has no Items field: %s", list.GetObjectKind().GroupVersionKind().String())
	}

	objects := reflect.MakeSlice(items.Type(), 0, 0)
	request := func(cli client.Client) error {
		if err := cli.List(ctx, list, opts...); err != nil {
			return err
		}
		objs := reflect.ValueOf(list).Elem().FieldByName("Items")
		if !objs.IsZero() {
			for i := 0; i < objs.Len(); i++ {
				objects = reflect.Append(objects, objs.Index(i))
			}
		}
		return nil
	}
	err := allOf(c.ctx, ctx, nil, request, opts)
	if objects.Len() != 0 {
		items.Set(objects)
	}
	return err
}

type multiClientWriter struct {
	ctx multiClientContext
}

var _ client.Writer = &multiClientWriter{}

func (c *multiClientWriter) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	request := func(cli client.Client) error {
		o, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cli.Create(ctx, o, opts...)
	}
	return allOf(c.ctx, ctx, obj, request, opts)
}

func (c *multiClientWriter) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	request := func(cli client.Client) error {
		return cli.Delete(ctx, obj, opts...)
	}
	return allOf(c.ctx, ctx, obj, request, opts)
}

func (c *multiClientWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	request := func(cli client.Client) error {
		o, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cli.Update(ctx, o, opts...)
	}
	return allOf(c.ctx, ctx, obj, request, opts)
}

func (c *multiClientWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	request := func(cli client.Client) error {
		o, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cli.Patch(ctx, o, patch, opts...)
	}
	return allOf(c.ctx, ctx, obj, request, opts)
}

func (c *multiClientWriter) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	request := func(cli client.Client) error {
		return cli.DeleteAllOf(ctx, obj, opts...)
	}
	return allOf(c.ctx, ctx, obj, request, opts)
}

type multiStatusClient struct {
	ctx multiClientContext
}

var _ client.StatusClient = &multiStatusClient{}

func (c *multiStatusClient) Status() client.SubResourceWriter {
	return &multiSubResourceWriter{
		ctx: c.ctx,
	}
}

type multiSubResourceClientConstructor struct {
	ctx multiClientContext
}

var _ client.SubResourceClientConstructor = &multiSubResourceClientConstructor{}

func (c *multiSubResourceClientConstructor) SubResource(subResource string) client.SubResourceClient {
	return &multiSubResourceClient{
		multiSubResourceReader: multiSubResourceReader{
			ctx:         c.ctx,
			subResource: subResource,
		},
		multiSubResourceWriter: multiSubResourceWriter{
			ctx: c.ctx,
		},
	}
}

type multiSubResourceClient struct {
	multiSubResourceReader
	multiSubResourceWriter
}

var _ client.SubResourceClient = &multiSubResourceClient{}

type multiSubResourceReader struct {
	ctx         multiClientContext
	subResource string
}

var _ client.SubResourceReader = &multiSubResourceReader{}

func (c *multiSubResourceReader) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	request := func(cli client.Client) error {
		return cli.SubResource(c.subResource).Get(ctx, obj, subResource, opts...)
	}
	return anyOf(c.ctx, ctx, obj, request, opts)
}

type multiSubResourceWriter struct {
	ctx multiClientContext
}

var _ client.SubResourceWriter = &multiSubResourceWriter{}

func (c *multiSubResourceWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	request := func(cli client.Client) error {
		o, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cli.Status().Create(ctx, o, subResource, opts...)
	}
	return allOf(c.ctx, ctx, obj, request, opts)
}

func (c *multiSubResourceWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	request := func(cli client.Client) error {
		o, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cli.Status().Update(ctx, o, opts...)
	}
	return allOf(c.ctx, ctx, obj, request, opts)
}

func (c *multiSubResourceWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	request := func(cli client.Client) error {
		o, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cli.Status().Patch(ctx, o, patch, opts...)
	}
	return allOf(c.ctx, ctx, obj, request, opts)
}

func allOf(mctx multiClientContext, ctx context.Context, obj client.Object, request func(cli client.Client) error, opts any) error {
	var err error
	for _, cli := range clients(mctx, ctx, obj, opts) {
		if e := request(cli); e != nil {
			if err == nil {
				err = e
			}
		}
	}
	return err
}

func anyOf(mctx multiClientContext, ctx context.Context, obj client.Object, request func(cli client.Client) error, opts any) error {
	var err error
	for _, cli := range clients(mctx, ctx, obj, opts) {
		if err = request(cli); err == nil {
			return nil
		}
	}
	return err
}

func clients(mctx multiClientContext, ctx context.Context, obj client.Object, opts any) []client.Client {
	// has no worker k8s clusters
	if len(mctx.workers) == 0 {
		return []client.Client{mctx.global}
	}

	o := hasClientOption(opts)
	if o == nil {
		return []client.Client{mctx.global}
	}

	if o.global {
		return []client.Client{mctx.global}
	}

	if o.unspecified {
		return maps.Values(mctx.workers)
	}

	if o.universal {
		return append([]client.Client{mctx.global}, workerClients(mctx, fromContext(ctx))...)
	}

	if o.oneshot {
		workers := fromContext(ctx)
		if len(workers) > 0 {
			workers = workers[:1] // always to use first worker k8s cluster
		}
		return workerClients(mctx, workers)
	}

	return workerClients(mctx, fromContextNObject(ctx, obj))
}

func hasClientOption(opts any) *ClientOption {
	value := reflect.ValueOf(opts)
	if !value.IsValid() || value.IsZero() {
		return nil
	}
	for i := 0; i < value.Len(); i++ {
		if o, ok := value.Index(i).Interface().(*ClientOption); ok {
			return o
		}
	}
	return nil
}

func workerClients(mctx multiClientContext, workers []string) []client.Client {
	l := make([]client.Client, 0)
	for _, c := range workers {
		if cli, ok := mctx.workers[c]; ok {
			l = append(l, cli)
		}
	}
	return l
}

func fromContextNObject(ctx context.Context, obj client.Object) []string {
	p1, p2 := fromContext(ctx), fromObject(obj)
	switch {
	case p1 == nil:
		return p2
	case p2 == nil:
		return p1
	default:
		s1, s2 := sets.New(p1...), sets.New(p2...)
		// if !s1.IsSuperset(s2) {
		//	panic("runtime error")
		// }
		// return p2
		return sets.List(s1.Intersection(s2))
	}
}

func fromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	p, err := FromContext(ctx)
	if err != nil {
		return nil
	}
	return strings.Split(p, ",")
}

func fromObject(obj client.Object) []string {
	if obj == nil || obj.GetAnnotations() == nil {
		return nil
	}
	p, ok := obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey]
	if !ok {
		return nil
	}
	return strings.Split(p, ",")
}
