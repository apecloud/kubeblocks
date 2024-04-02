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
	"fmt"
	"reflect"

	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewClient(control client.Client, workers map[string]client.Client) client.Client {
	mctx := mcontext{
		control: control,
		workers: workers,
	}
	return &mclient{
		clientReader:                 clientReader{mctx},
		clientWriter:                 clientWriter{mctx},
		statusClient:                 statusClient{mctx},
		subResourceClientConstructor: subResourceClientConstructor{mctx},
		mctx:                         mctx,
	}
}

type mcontext struct {
	control client.Client            // client for control-plane k8s cluster
	workers map[string]client.Client // clients for data-plane k8s clusters
}

type mclient struct {
	clientReader
	clientWriter
	statusClient
	subResourceClientConstructor

	mctx mcontext
}

var _ client.Client = &mclient{}

func (c *mclient) Scheme() *runtime.Scheme {
	return c.mctx.control.Scheme()
}

func (c *mclient) RESTMapper() meta.RESTMapper {
	return c.mctx.control.RESTMapper()
}

func (c *mclient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.mctx.control.GroupVersionKindFor(obj)
}

func (c *mclient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.mctx.control.IsObjectNamespaced(obj)
}

type clientReader struct {
	mctx mcontext
}

var _ client.Reader = &clientReader{}

func (c *clientReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	request := func(cli client.Client) error {
		return cli.Get(ctx, key, obj, opts...)
	}
	return anyOf(c.mctx, ctx, obj, request, opts)
}

func (c *clientReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	items := reflect.ValueOf(list).Elem().FieldByName("Items")
	if !items.IsValid() {
		return fmt.Errorf("ObjectList has no Items field: %s", list.GetObjectKind().GroupVersionKind().String())
	}

	objects := reflect.MakeSlice(items.Type(), 0, 0)
	request := func(cc contextCli, _ client.Object) error {
		if err := cc.cli.List(ctx, list, opts...); err != nil {
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
	err := allOf(c.mctx, ctx, nil, request, opts)
	if objects.Len() != 0 {
		items.Set(objects)
	}
	return err
}

type clientWriter struct {
	mctx mcontext
}

var _ client.Writer = &clientWriter{}

func (c *clientWriter) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	request := func(cc contextCli, lobj client.Object) error {
		o, ok := lobj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		setPlacementKey(o, cc.context)
		return cc.cli.Create(ctx, o, opts...)
	}
	return allOf(c.mctx, ctx, obj, request, opts)
}

func (c *clientWriter) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	request := func(cc contextCli, _ client.Object) error {
		return cc.cli.Delete(ctx, obj, opts...)
	}
	return allOf(c.mctx, ctx, obj, request, opts)
}

func (c *clientWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	request := func(cc contextCli, lobj client.Object) error {
		o, ok := lobj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cc.cli.Update(ctx, o, opts...)
	}
	return allOf(c.mctx, ctx, obj, request, opts)
}

func (c *clientWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	request := func(cc contextCli, lobj client.Object) error {
		o, ok := lobj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cc.cli.Patch(ctx, o, patch, opts...)
	}
	return allOf(c.mctx, ctx, obj, request, opts)
}

func (c *clientWriter) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	request := func(cc contextCli, _ client.Object) error {
		return cc.cli.DeleteAllOf(ctx, obj, opts...)
	}
	return allOf(c.mctx, ctx, obj, request, opts)
}

type statusClient struct {
	mctx mcontext
}

var _ client.StatusClient = &statusClient{}

func (c *statusClient) Status() client.SubResourceWriter {
	return &subResourceWriter{
		mctx: c.mctx,
	}
}

type subResourceClientConstructor struct {
	mctx mcontext
}

var _ client.SubResourceClientConstructor = &subResourceClientConstructor{}

func (c *subResourceClientConstructor) SubResource(subResource string) client.SubResourceClient {
	return &subResourceClient{
		subResourceReader: subResourceReader{
			mctx:        c.mctx,
			subResource: subResource,
		},
		subResourceWriter: subResourceWriter{
			mctx: c.mctx,
		},
	}
}

type subResourceClient struct {
	subResourceReader
	subResourceWriter
}

var _ client.SubResourceClient = &subResourceClient{}

type subResourceReader struct {
	mctx        mcontext
	subResource string
}

var _ client.SubResourceReader = &subResourceReader{}

func (c *subResourceReader) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	request := func(cli client.Client) error {
		return cli.SubResource(c.subResource).Get(ctx, obj, subResource, opts...)
	}
	return anyOf(c.mctx, ctx, obj, request, opts)
}

type subResourceWriter struct {
	mctx mcontext
}

var _ client.SubResourceWriter = &subResourceWriter{}

func (c *subResourceWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	request := func(cc contextCli, lobj client.Object) error {
		o, ok := lobj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cc.cli.Status().Create(ctx, o, subResource, opts...)
	}
	return allOf(c.mctx, ctx, obj, request, opts)
}

func (c *subResourceWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	request := func(cc contextCli, lobj client.Object) error {
		o, ok := lobj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cc.cli.Status().Update(ctx, o, opts...)
	}
	return allOf(c.mctx, ctx, obj, request, opts)
}

func (c *subResourceWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	request := func(cc contextCli, lobj client.Object) error {
		o, ok := lobj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("not client object: %T", obj)
		}
		return cc.cli.Status().Patch(ctx, o, patch, opts...)
	}
	return allOf(c.mctx, ctx, obj, request, opts)
}

func allOf(mctx mcontext, ctx context.Context, obj client.Object, request func(contextCli, client.Object) error, opts any) error {
	var err error
	for _, cc := range resolvedClients(mctx, ctx, obj, opts) {
		if e := request(cc, obj); e != nil {
			if err == nil {
				err = e
			}
		}
	}
	return err
}

func anyOf(mctx mcontext, ctx context.Context, obj client.Object, request func(client.Client) error, opts any) error {
	var err error
	for _, cc := range resolvedClients(mctx, ctx, obj, opts) {
		if err = request(cc.cli); err == nil {
			return nil
		}
	}
	return err
}

type contextCli struct {
	context string
	cli     client.Client
}

func resolvedClients(mctx mcontext, ctx context.Context, obj client.Object, opts any) []contextCli {
	// has no data-plane k8s clusters
	if len(mctx.workers) == 0 {
		return []contextCli{{"", mctx.control}}
	}

	o := hasClientOption(opts)
	if o == nil {
		return []contextCli{{"", mctx.control}}
	}

	if o.control {
		return []contextCli{{"", mctx.control}}
	}

	if o.unspecified {
		return dataClients(mctx, maps.Keys(mctx.workers))
	}

	if o.universal {
		return append([]contextCli{{"", mctx.control}}, dataClients(mctx, fromContext(ctx))...)
	}

	if o.oneshot {
		workers := fromContext(ctx)
		if len(workers) > 0 {
			workers = workers[:1] // always to use first worker k8s cluster
		}
		return dataClients(mctx, workers)
	}

	return dataClients(mctx, fromContextNObject(ctx, obj))
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

func dataClients(mctx mcontext, workers []string) []contextCli {
	l := make([]contextCli, 0)
	for _, c := range workers {
		if cli, ok := mctx.workers[c]; ok {
			l = append(l, contextCli{c, cli})
		}
	}
	return l
}
