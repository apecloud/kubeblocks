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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var (
	scheme *runtime.Scheme
)

func Setup(scheme *runtime.Scheme, cfg *rest.Config, cli client.Client, kubeConfig, contexts, disabledContexts string) (Manager, error) {
	if len(contexts) == 0 {
		return nil, nil
	}

	mcc, err := newClientNCache(scheme, kubeConfig, contexts, disabledContexts)
	if err != nil {
		return nil, err
	}
	for k, c := range mcc {
		if isSameContextWithControl(cfg, c) {
			cc := mcc[k]
			if isUnavailableClient(cc.client) {
				return nil, fmt.Errorf("control cluster %s is disabled", cc.context)
			}
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
	setupScheme(scheme)
	return &manager{
		cli:    NewClient(cli, clients()),
		caches: caches(),
	}, nil
}

func setupScheme(s *runtime.Scheme) {
	scheme = s
}

// isSameContextWithControl checks whether the context is the same as the control cluster.
func isSameContextWithControl(cfg *rest.Config, mcc multiClusterContext) bool {
	return cfg.Host == mcc.id
}

func newClientNCache(scheme *runtime.Scheme, kubeConfig, contexts, disabledContexts string) (map[string]multiClusterContext, error) {
	merged := map[string]bool{}
	for _, c := range strings.Split(contexts, ",") {
		merged[c] = false
	}
	if len(disabledContexts) > 0 {
		for _, c := range strings.Split(disabledContexts, ",") {
			if _, ok := merged[c]; !ok {
				return nil, fmt.Errorf("disabled context %s is not in contexts", c)
			}
			merged[c] = true
		}
	}

	mcc := make(map[string]multiClusterContext)
	for context, disabled := range merged {
		cc, err := newClientNCache4Context(scheme, kubeConfig, context, disabled)
		if err != nil {
			return nil, err
		}
		if cc != nil {
			mcc[context] = *cc
		}
	}
	return mcc, nil
}

func newClientNCache4Context(scheme *runtime.Scheme, kubeConfig, context string, disabled bool) (*multiClusterContext, error) {
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

	var cli client.Client
	var cache cache.Cache
	if !disabled {
		cli, cache, err = createClientNCache(scheme, config, context)
	} else {
		cli, cache, err = createUnavailableClientNCache(scheme, config, context)
	}
	if err != nil {
		return nil, err
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

func createClientNCache(scheme *runtime.Scheme, config *rest.Config, context string) (client.Client, cache.Cache, error) {
	clientOpts, err := clientOptions(scheme, context, config)
	if err != nil {
		return nil, nil, err
	}
	cli, err := client.New(config, clientOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create Client for context %s: %s", context, err.Error())
	}
	cache, err := cache.New(config, cacheOptions(clientOpts))
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create Cache for context %s: %s", context, err.Error())
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
			DisableFor:   getUncachedObjects(),
		},
	}, nil
}

// getUncachedObjects returns a list of K8s objects, for these object types,
// and their list types, client.Reader will read directly from the API server instead
// of the cache, which may not be up-to-date.
// see sigs.k8s.io/controller-runtime/pkg/client/split.go to understand how client
// works with this UncachedObjects filter.
func getUncachedObjects() []client.Object {
	// client-side read cache reduces the number of requests processed in the API server,
	// which is good for performance. However, it can sometimes lead to obscure issues,
	// most notably lacking read-after-write consistency, i.e. reading a value immediately
	// after updating it may miss to see the changes.
	// while in most cases this problem can be mitigated by retrying later in an idempotent
	// manner, there are some cases where it cannot, for example if a decision is to be made
	// that has side-effect operations such as returning an error message to the user
	// (in webhook) or deleting certain resources (in controllerutil.HandleCRDeletion).
	// additionally, retry loops cause unnecessary delays when reconciliations are processed.
	// for the sake of performance, now only the objects created by the end-user is listed here,
	// to solve the two problems mentioned above.
	// consider carefully before adding new objects to this list.
	return []client.Object{
		// avoid to cache potential large data objects
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&appsv1.Cluster{},
		&appsv1alpha1.Configuration{},
	}
}

func cacheOptions(opts client.Options) cache.Options {
	return cache.Options{
		HTTPClient: opts.HTTPClient,
		Scheme:     opts.Scheme,
		Mapper:     opts.Mapper,
	}
}

func createUnavailableClientNCache(_ *runtime.Scheme, _ *rest.Config, context string) (client.Client, cache.Cache, error) {
	return newUnavailableClient(context), nil, nil
}
