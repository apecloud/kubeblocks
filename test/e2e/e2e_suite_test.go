/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
var provider string
var region string
var secretID string
var secretKey string

func init() {
	viper.AutomaticEnv()
	flag.StringVar(&version, "VERSION", "", "kubeblocks test version")
	flag.StringVar(&provider, "PROVIDER", "", "kubeblocks test cloud-provider")
	flag.StringVar(&region, "REGION", "", "kubeblocks test region")
	flag.StringVar(&secretID, "SECRET_ID", "", "cloud-provider SECRET_ID")
	flag.StringVar(&secretKey, "SECRET_KEY", "", "cloud-provider SECRET_KEY")
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
	if len(provider) > 0 && len(region) > 0 && len(secretID) > 0 && len(secretKey) > 0 {
		Provider = provider
		Region = region
		SecretID = secretID
		SecretKey = secretKey
	}
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
	var _ = Describe("KubeBlocks playground init", PlaygroundInit)

	var _ = Describe("KubeBlocks uninstall", UninstallKubeblocks)

	var _ = Describe("Check healthy Kubernetes cluster status", EnvCheckTest)

	var _ = Describe("KubeBlocks operator installation", InstallationTest)

	var _ = Describe("KubeBlocks smoke test run", SmokeTest)

	var _ = Describe("KubeBlocks operator uninstallation", UninstallationTest)

	var _ = Describe("Check environment has been cleaned", EnvGotCleanedTest)

	var _ = Describe("KubeBlocks playground destroy", PlaygroundDestroy)
})
