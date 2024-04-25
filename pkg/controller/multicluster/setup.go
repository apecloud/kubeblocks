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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func Setup(scheme *runtime.Scheme, cfg *rest.Config, cli client.Client, kubeConfig, contexts string) (Manager, error) {
	if len(contexts) == 0 {
		return nil, nil
	}
	mcc, err := newClientNCache(scheme, kubeConfig, contexts)
	if err != nil {
		return nil, err
	}
	for k, c := range mcc {
		if isSameContextWithControl(cfg, c) {
			cc := mcc[k]
			// reset the cache and use default cli of control cluster
			cc.cache = nil
			cc.client = cli
			mcc[k] = cc
		}
	}

	clients := func() map[string]client.Client {
		m := make(map[string]client.Client)
		for _, c := range mcc {
			m[c.context] = c.client
		}
		return m
	}
	caches := func() map[string]cache.Cache {
		m := make(map[string]cache.Cache)
		for _, c := range mcc {
			m[c.context] = c.cache
		}
		return m
	}

	return &manager{
		cli:    NewClient(cli, clients()),
		caches: caches(),
	}, nil
}

// isSameContextWithControl checks whether the context is the same as the control cluster.
func isSameContextWithControl(cfg *rest.Config, mcc multiClusterContext) bool {
	return cfg.Host == mcc.id
}

func newClientNCache(scheme *runtime.Scheme, kubeConfig, contexts string) (map[string]multiClusterContext, error) {
	mcc := make(map[string]multiClusterContext, 0)
	for _, context := range strings.Split(contexts, ",") {
		cc, err := newClientNCache4Context(scheme, kubeConfig, context)
		if err != nil {
			return nil, err
		}
		if cc != nil {
			mcc[context] = *cc
		}
	}
	return mcc, nil
}

func newClientNCache4Context(scheme *runtime.Scheme, kubeConfig, context string) (*multiClusterContext, error) {
	if len(context) == 0 {
		return nil, nil
	}

	config, err := getConfigWithContext(kubeConfig, context)
	if err != nil {
		return nil, fmt.Errorf("unable to get kubeconfig for context %s: %s", context, err.Error())
	}
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	clientOpts, err := clientOptions(scheme, context, config)
	if err != nil {
		return nil, err
	}

	cli, err := client.New(config, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to create Client for context %s: %s", context, err.Error())
	}
	cache, err := cache.New(config, cacheOptions(clientOpts))
	if err != nil {
		return nil, fmt.Errorf("unable to create Cache for context %s: %s", context, err.Error())
	}
	return &multiClusterContext{
		context: context,
		id:      config.Host,
		cache:   cache,
		client:  cli,
	}, nil
}

func getConfigWithContext(kubeConfig, context string) (*rest.Config, error) {
	if len(kubeConfig) == 0 {
		return config.GetConfigWithContext(context)
	}
	return getConfigWithContextFromSpecified(kubeConfig, context)
}

func getConfigWithContextFromSpecified(kubeConfig, context string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfig},
		&clientcmd.ConfigOverrides{
			ClusterInfo: clientcmdapi.Cluster{
				Server: "",
			},
			CurrentContext: context,
		}).ClientConfig()
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
			DisableFor:   intctrlutil.GetUncachedObjects(),
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
