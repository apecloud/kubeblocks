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

package testutil

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/apecloud/kubeblocks/internal/constant"
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

var ErrUninitError = fmt.Errorf("cli uninitialized error")

const (
	envExistingClusterType = "EXISTING_CLUSTER_TYPE"
	envUseExistingCluster  = "USE_EXISTING_CLUSTER"
)

func init() {
	viper.AutomaticEnv()
	viper.SetDefault("EventuallyTimeout", time.Second*10)
	viper.SetDefault("EventuallyPollingInterval", time.Millisecond)
	viper.SetDefault("ConsistentlyDuration", time.Second*3)
	viper.SetDefault("ConsistentlyPollingInterval", time.Millisecond)
	viper.SetDefault("ClearResourceTimeout", time.Second*10)
	viper.SetDefault("ClearResourcePollingInterval", time.Millisecond)
}

// NewDefaultTestContext creates default test context, if provided namespace optional arg, a namespace
// will be created if not exist
func NewDefaultTestContext(ctx context.Context, cli client.Client, testEnv *envtest.Environment, namespace ...string) TestContext {
	if cli == nil {
		panic("missing required cli arg")
	}
	t := TestContext{
		TestObjLabelKey:                    "kubeblocks.io/test",
		DefaultNamespace:                   "default",
		DefaultEventuallyTimeout:           viper.GetDuration("EventuallyTimeout"),
		DefaultEventuallyPollingInterval:   viper.GetDuration("EventuallyPollingInterval"),
		DefaultConsistentlyDuration:        viper.GetDuration("ConsistentlyDuration"),
		DefaultConsistentlyPollingInterval: viper.GetDuration("ConsistentlyPollingInterval"),
		ClearResourceTimeout:               viper.GetDuration("ClearResourceTimeout"),
		ClearResourcePollingInterval:       viper.GetDuration("ClearResourcePollingInterval"),
		Ctx:                                ctx,
		Cli:                                cli,
		TestEnv:                            testEnv,
	}
	t.CreateObj = func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
		l := obj.GetLabels()
		if l == nil {
			l = map[string]string{}
		}
		l[t.TestObjLabelKey] = "true"
		obj.SetLabels(l)
		return t.Cli.Create(ctx, obj, opts...)
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

	if len(namespace) > 0 && len(namespace[0]) > 0 && namespace[0] != "default" {
		t.DefaultNamespace = namespace[0]
		err := t.CreateNamespace()
		gomega.Expect(client.IgnoreAlreadyExists(err)).To(gomega.Not(gomega.HaveOccurred()))
	}
	return t
}

func (testCtx TestContext) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return testCtx.CreateObj(ctx, obj, opts...)
}

func (testCtx TestContext) GetNamespaceKey() client.ObjectKey {
	return client.ObjectKey{
		Name: testCtx.DefaultNamespace,
	}
}

func (testCtx TestContext) GetNamespaceObj() corev1.Namespace {
	return corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testCtx.DefaultNamespace,
		},
	}
}

func (testCtx TestContext) CreateNamespace() error {
	if testCtx.DefaultNamespace == "default" {
		return nil
	}
	namespace := testCtx.GetNamespaceObj()
	return testCtx.Create(testCtx.Ctx, &namespace)
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

// SetKubeServerVersionWithDistro provides "_KUBE_SERVER_INFO" viper settings helper function.
func SetKubeServerVersionWithDistro(major, minor, patch, distro string) {
	ver := version.Info{
		Major:      major,
		Minor:      minor,
		GitVersion: fmt.Sprintf("v%s.%s.%s+%s", major, minor, patch, distro),
	}
	viper.Set(constant.CfgKeyServerInfo, ver)
}

// SetKubeServerVersion provides "_KUBE_SERVER_INFO" viper settings helper function.
func SetKubeServerVersion(major, minor, patch string) {
	ver := version.Info{
		Major:      major,
		Minor:      minor,
		GitVersion: fmt.Sprintf("v%s.%s.%s", major, minor, patch),
	}
	viper.Set(constant.CfgKeyServerInfo, ver)
}
