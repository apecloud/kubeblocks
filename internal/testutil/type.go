/*
Copyright ApeCloud Inc.

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

	"github.com/sethvargo/go-password/password"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestContext struct {
	TestObjLabelKey  string
	DefaultNamespace string
	CreateObj        func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	CheckedCreateObj func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
}

const (
	envExistingClusterType = "EXISTING_CLUSTER_TYPE"

	envUseExistingCluster = "USE_EXISTING_CLUSTER"
)

func NewDefaultTestContext(cli client.Client) TestContext {
	t := TestContext{
		TestObjLabelKey:  "kubeblocks.io/test",
		DefaultNamespace: "default",
	}

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
	return t
}

func (testCtx TestContext) GetRandomStr() string {
	seq, _ := password.Generate(6, 2, 0, true, true)
	return seq
}

func (testCtx TestContext) UsingExistingCluster() bool {
	return os.Getenv(envUseExistingCluster) == "true"
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
