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

package hook

import (
	"context"

	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"

	restclient "k8s.io/client-go/rest"

	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

type UpgradeMetaContext struct {
	context.Context
	Client    *versioned.Clientset
	K8sClient *kubernetes.Clientset
	CRDClient *clientset.Clientset

	CRDPath   string
	Version   string
	Namespace string
}

type UpgradeContext struct {
	UpgradeMetaContext

	From *Version
	To   Version
}

type Version struct {
}

// ContextHandler is the interface for a "chunk" of reconciliation. It either
// returns, often by adjusting the current key's place in the queue (i.e. via
// requeue or done) or calls another handler in the chain.
type ContextHandler interface {
	Handle(*UpgradeContext) error
}

// ContextHandlerFunc is a function type that implements ContextHandler
type ContextHandlerFunc func(ctx *UpgradeContext) error

func (f ContextHandlerFunc) Handle(ctx *UpgradeContext) error {
	return f(ctx)
}

type Workflow []ContextHandlerFunc

func (w Workflow) Do(ctx *UpgradeContext) error {
	for _, handler := range w {
		if err := handler(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (w Workflow) AddStage(stage ContextHandler) Workflow {
	return append(w, func(ctx *UpgradeContext) error {
		return stage.Handle(ctx)
	})
}

func NewUpgradeWorkflow() Workflow {
	return Workflow{}
}

func NewUpgradeContext(ctx context.Context, config *restclient.Config, version string, crd string, ns string) *UpgradeContext {
	client, err := versioned.NewForConfig(config)
	CheckErr(err)
	k8sClient, err := kubernetes.NewForConfig(config)
	CheckErr(err)
	apiextensions, err := clientset.NewForConfig(config)
	CheckErr(err)

	upgradeMeta := UpgradeMetaContext{
		Context:   ctx,
		Client:    client,
		CRDClient: apiextensions,
		K8sClient: k8sClient,
		Version:   version,
		CRDPath:   crd,
		Namespace: ns,
	}
	return &UpgradeContext{UpgradeMetaContext: upgradeMeta,
		To: *NewVersion(version),
	}
}

func NewVersion(v string) *Version {
	return &Version{}
}
