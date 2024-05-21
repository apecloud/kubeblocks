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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

type UpgradeMetaContext struct {
	context.Context

	CRDPath   string
	Version   string
	Namespace string

	CRClient
}

type CRClient struct {
	*dynamic.DynamicClient

	KBClient  *versioned.Clientset
	K8sClient *kubernetes.Clientset
	CRDClient *clientset.Clientset
}

type UpgradeContext struct {
	UpgradeMetaContext

	From *Version
	To   Version

	UpdatedObjects map[schema.GroupVersionResource][]client.Object
}

type Version struct {
	Major int32
	Minor int32
}

// ContextHandler is the interface for a "chunk" of reconciliation. It either
// returns, often by adjusting the current key's place in the queue (i.e. via
// requeue or done) or calls another handler in the chain.
type ContextHandler interface {
	IsSkip(*UpgradeContext) (bool, error)
	Handle(*UpgradeContext) error
}

type BasedHandler struct {
	ContextHandler
}

func (b *BasedHandler) IsSkip(*UpgradeContext) (bool, error) {
	return false, nil
}

// func (b *BasedHandler) Handle(*UpgradeContext) error {
// 	return nil
// }

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
		skip, err := stage.IsSkip(ctx)
		if err != nil {
			return err
		}
		if !skip {
			err = stage.Handle(ctx)
		}
		return err
	})
}

func (w Workflow) WrapStage(stageFn ContextHandlerFunc) Workflow {
	return append(w, stageFn)
}

func NewUpgradeWorkflow() Workflow {
	return Workflow{}
}

func NewUpgradeContext(ctx context.Context, config *restclient.Config, version string, crd string, ns string) *UpgradeContext {
	nVersion, err := NewVersion(version)
	CheckErr(err)

	return &UpgradeContext{
		UpgradeMetaContext: UpgradeMetaContext{
			CRClient: CRClient{
				DynamicClient: dynamic.NewForConfigOrDie(config),
				KBClient:      versioned.NewForConfigOrDie(config),
				K8sClient:     kubernetes.NewForConfigOrDie(config),
				CRDClient:     clientset.NewForConfigOrDie(config),
			},
			Context:   ctx,
			Version:   version,
			CRDPath:   crd,
			Namespace: ns,
		},
		UpdatedObjects: make(map[schema.GroupVersionResource][]client.Object),
		To:             *nVersion,
	}
}
