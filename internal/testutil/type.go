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

package testutil

import (
	"context"
	"os"
	"time"

	"github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type TestContext struct {
	Ctx                                context.Context
	Cli                                client.Client
	TestEnv                            *envtest.Environment
	TestObjLabelKey                    string
	DefaultNamespace                   string
	DefaultEventuallyTimeout           time.Duration
	DefaultEventuallyPollingInterval   time.Duration
	DefaultConsistentlyDuration        time.Duration
	DefaultConsistentlyPollingInterval time.Duration
	ClearResourceTimeout               time.Duration
	ClearResourcePollingInterval       time.Duration
	CreateObj                          func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	CheckedCreateObj                   func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
}

const (
	envExistingClusterType = "EXISTING_CLUSTER_TYPE"

	envUseExistingCluster = "USE_EXISTING_CLUSTER"
)

func NewDefaultTestContext(ctx context.Context, cli client.Client, testEnv *envtest.Environment) TestContext {
	t := TestContext{
		TestObjLabelKey:                    "kubeblocks.io/test",
		DefaultNamespace:                   "default",
		DefaultEventuallyTimeout:           time.Second * 10,
		DefaultEventuallyPollingInterval:   time.Second,
		DefaultConsistentlyDuration:        time.Second * 3,
		DefaultConsistentlyPollingInterval: time.Second,
		ClearResourceTimeout:               time.Second * 60,
		ClearResourcePollingInterval:       time.Second,
	}
	t.Ctx = ctx
	t.Cli = cli
	t.TestEnv = testEnv
	t.CreateObj = func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
		l := obj.GetLabels()
		if l == nil {
			l = map[string]string{}
		}
		l[t.TestObjLabelKey] = "true"
		obj.SetLabels(l)
		return cli.Create(ctx, obj, opts...)
	}

	t.CheckedCreateObj = func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
		if err := t.CreateObj(ctx, obj, opts...); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}

	gomega.SetDefaultEventuallyTimeout(t.DefaultEventuallyTimeout)
	gomega.SetDefaultEventuallyPollingInterval(t.DefaultEventuallyPollingInterval)
	gomega.SetDefaultConsistentlyDuration(t.DefaultConsistentlyDuration)
	gomega.SetDefaultConsistentlyPollingInterval(t.DefaultConsistentlyPollingInterval)

	return t
}

func (testCtx TestContext) GetRandomStr() string {
	seq, _ := password.Generate(6, 2, 0, true, true)
	return seq
}

func (testCtx TestContext) UsingExistingCluster() bool {
	if testCtx.TestEnv == nil || testCtx.TestEnv.UseExistingCluster == nil {
		return os.Getenv(envUseExistingCluster) == "true"
	}
	return *testCtx.TestEnv.UseExistingCluster
}

func (testCtx TestContext) GetWebhookHostExternalName() string {
	var (
		minikubeType = "minikube"
		minikubeHost = "host.minikube.internal"
		k3dType      = "k3d"
		k3dHost      = "host.k3d.internal"
	)
	clusterType := os.Getenv(envExistingClusterType)
	if !testCtx.UsingExistingCluster() {
		return ""
	}
	switch clusterType {
	case minikubeType:
		return minikubeHost
	case k3dType:
		return k3dHost
	default:
		return ""
	}
}
