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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func Setup(scheme *runtime.Scheme, cli client.Client, kubeContexts string) (Manager, error) {
	if len(kubeContexts) == 0 {
		return nil, nil
	}
	clients, caches, err := newClientsNCaches(scheme, kubeContexts)
	if err != nil {
		return nil, err
	}
	return &manager{
		cli:    NewClient(cli, clients),
		caches: caches,
	}, nil
}

func newClientsNCaches(scheme *runtime.Scheme, kubeContexts string) (map[string]client.Client, map[string]cache.Cache, error) {
	clients := make(map[string]client.Client)
	caches := make(map[string]cache.Cache)
	for _, ctx := range strings.Split(kubeContexts, ",") {
		cli, cache, err := newClientNCache4Context(scheme, ctx)
		if err != nil {
			return nil, nil, err
		}
		if cli != nil && cache != nil {
			clients[ctx] = cli
			caches[ctx] = cache
		}
	}
	return clients, caches, nil
}

func newClientNCache4Context(scheme *runtime.Scheme, ctx string) (client.Client, cache.Cache, error) {
	if len(ctx) == 0 {
		return nil, nil, nil
	}

	config, err := config.GetConfigWithContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get kubeconfig for context %s: %s", ctx, err.Error())
	}
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	clientOpts, err := clientOptions(scheme, ctx, config)
	if err != nil {
		return nil, nil, err
	}

	cli, err := client.New(config, clientOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create Client for context %s: %s", ctx, err.Error())
	}
	cache, err := cache.New(config, cacheOptions(clientOpts))
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create Cache for context %s: %s", ctx, err.Error())
	}
	return cli, cache, nil
}

func clientOptions(scheme *runtime.Scheme, ctx string, config *rest.Config) (client.Options, error) {
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return client.Options{}, fmt.Errorf("unable to create HTTP client for context %s: %s", ctx, err.Error())
	}

	mapper, err := apiutil.NewDynamicRESTMapper(config, httpClient)
	if err != nil {
		return client.Options{}, fmt.Errorf("failed to get API Group-Resources for context %s: %s", ctx, err.Error())
	}

	return client.Options{
		Scheme:     scheme,
		HTTPClient: httpClient,
		Mapper:     mapper,
		Cache: &client.CacheOptions{
			Unstructured: false,
			DisableFor:   []client.Object{},
		},
	}, nil
}

func cacheOptions(opts client.Options) cache.Options {
	return cache.Options{
		HTTPClient: opts.HTTPClient,
		Scheme:     opts.Scheme,
		Mapper:     opts.Mapper,
	}
}
