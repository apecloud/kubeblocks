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
	"fmt"
	"os"
	"time"

	"github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/viper"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/version"
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
	envUseExistingCluster  = "USE_EXISTING_CLUSTER"
)

func init() {
	viper.AutomaticEnv()
	viper.SetDefault("EventuallyTimeout", time.Second*10)
	viper.SetDefault("EventuallyPollingInterval", time.Second*1)
	viper.SetDefault("ConsistentlyDuration", time.Second*3)
	viper.SetDefault("ConsistentlyPollingInterval", time.Second*1)
	viper.SetDefault("ClearResourceTimeout", time.Second*60)
	viper.SetDefault("ClearResourcePollingInterval", time.Second*1)
}

func NewDefaultTestContext(ctx context.Context, cli client.Client, testEnv *envtest.Environment) TestContext {
	t := TestContext{
		TestObjLabelKey:                    "kubeblocks.io/test",
		DefaultNamespace:                   "default",
		DefaultEventuallyTimeout:           viper.GetDuration("EventuallyTimeout"),
		DefaultEventuallyPollingInterval:   viper.GetDuration("EventuallyPollingInterval"),
		DefaultConsistentlyDuration:        viper.GetDuration("ConsistentlyDuration"),
		DefaultConsistentlyPollingInterval: viper.GetDuration("ConsistentlyPollingInterval"),
		ClearResourceTimeout:               viper.GetDuration("ClearResourceTimeout"),
		ClearResourcePollingInterval:       viper.GetDuration("ClearResourcePollingInterval"),
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
		return viper.GetBool(envUseExistingCluster)
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

func (testCtx TestContext) UseDefaultNamespace() func(client.Object) {
	return func(obj client.Object) {
		obj.SetNamespace(testCtx.DefaultNamespace)
	}
}

// SetKubeServerVersionWithDistro provide "_KUBE_SERVER_INFO" viper settings helper function.
func SetKubeServerVersionWithDistro(major, minor, patch, distro string) {
	ver := version.Info{
		Major:      major,
		Minor:      minor,
		GitVersion: fmt.Sprintf("v%s.%s.%s+%s", major, minor, patch, distro),
	}
	viper.Set("_KUBE_SERVER_INFO", ver)
}

// SetKubeServerVersion provide "_KUBE_SERVER_INFO" viper settings helper function.
func SetKubeServerVersion(major, minor, patch string) {
	ver := version.Info{
		Major:      major,
		Minor:      minor,
		GitVersion: fmt.Sprintf("v%s.%s.%s", major, minor, patch),
	}
	viper.Set("_KUBE_SERVER_INFO", ver)
}
