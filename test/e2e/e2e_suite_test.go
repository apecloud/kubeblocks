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

package e2e_test

import (
	"context"
	"flag"
	"fmt"
	"go/build"
	"log"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"

	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/apecloud/kubeblocks/test/e2e"
	. "github.com/apecloud/kubeblocks/test/e2e/envcheck"
	. "github.com/apecloud/kubeblocks/test/e2e/installation"
	. "github.com/apecloud/kubeblocks/test/e2e/testdata/smoketest"
)

var cfg *rest.Config
var testEnv *envtest.Environment
var TC *TestClient
var version string

func init() {
	viper.AutomaticEnv()
	flag.StringVar(&version, "VERSION", "", "kubeblocks test version")
}

func TestE2e(t *testing.T) {
	if err := GetKubeconfigContext(); err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("report.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "E2e Suite", []Reporter{junitReporter})
}

func GetKubeconfigContext() error {
	var err error
	var tcDefault TestClient
	tcDefault, err = NewTestClient(VeleroCfg.DefaultCluster)
	VeleroCfg.DefaultClient = &tcDefault
	VeleroCfg.ClientToInstallVelero = VeleroCfg.DefaultClient
	if err != nil {
		return err
	}

	if VeleroCfg.DefaultCluster != "" {
		err = KubectlConfigUseContext(context.Background(), VeleroCfg.DefaultCluster)
		if err != nil {
			return err
		}
	}
	TC = &tcDefault
	return nil
}

var _ = BeforeSuite(func() {
	if len(version) == 0 {
		log.Println("kubeblocks version is not specified")
		return
	}
	log.Println("kb version:" + version)
	Version = version
	if viper.GetBool("ENABLE_DEBUG_LOG") {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
			o.TimeEncoder = zapcore.ISO8601TimeEncoder
		}))
	}

	Ctx, Cancel = context.WithCancel(context.TODO())
	Logger = logf.FromContext(Ctx).WithValues()
	Logger.Info("logger start")

	K8sClient = TC.Kubebuilder
	CheckNoKubeBlocksCRDs()

	By("bootstrapping e2e-test environment")
	var flag = true
	testEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			CleanUpAfterUse: true,
		},
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases"),
			// use dependent external CRDs.
			// resolved by ref: https://github.com/operator-framework/operator-sdk/issues/4434#issuecomment-786794418
			filepath.Join(build.Default.GOPATH, "pkg", "mod", "github.com", "kubernetes-csi/external-snapshotter/",
				"client/v6@v6.2.0", "config", "crd")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    &flag,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("delete helm release in e2e-test environment")
	CheckedUninstallHelmRelease()
	if testEnv != nil {
		By("removed installed CRDs in e2e-test environment")
		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
	Cancel()
})

var _ = Describe("e2e test", func() {
	var _ = Describe("Check healthy Kubernetes cluster status", EnvCheckTest)

	var _ = Describe("KubeBlocks operator installation", InstallationTest)

	var _ = Describe("KubeBlocks smoke test run", SmokeTest)

	var _ = Describe("KubeBlocks operator uninstallation", UninstallationTest)

	var _ = Describe("Check environment has been cleaned", EnvGotCleanedTest)

	var _ = Describe("KubeBlocks playground test", PlaygroundTest)
})
