/*
Copyright ApeCloud, Inc.

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

package main

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// noneClient is a client.Client that does nothing.
// It is used to prevent the template engine requests to the API server.
// Avoid nil type client
type noneClient struct {
}

func (n noneClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return apierrors.NewServiceUnavailable("pod secondary not support client request.")
}

func (n noneClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return apierrors.NewServiceUnavailable("pod secondary not support client request.")
}

var nclient = &noneClient{}
