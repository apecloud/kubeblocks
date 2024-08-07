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
	"github.com/vmware-tanzu/velero/test"
	. "github.com/vmware-tanzu/velero/test/util/k8s"

	"github.com/onsi/ginkgo/v2/reporters"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
	. "github.com/apecloud/kubeblocks/test/e2e"
	. "github.com/apecloud/kubeblocks/test/e2e/envcheck"
	. "github.com/apecloud/kubeblocks/test/e2e/installation"
	. "github.com/apecloud/kubeblocks/test/e2e/testdata/smoketest"
	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

var cfg *rest.Config
var testEnv *envtest.Environment
var TC *TestClient
var version string
var provider string
var region string
var secretID string
var secretKey string
var initEnv bool
var testType string
var skipCase string
var configType string

func init() {
	viper.AutomaticEnv()
	flag.StringVar(&version, "VERSION", "", "kubeblocks test version")
	flag.StringVar(&provider, "PROVIDER", "", "kubeblocks test cloud-provider")
	flag.StringVar(&region, "REGION", "", "kubeblocks test region")
	flag.StringVar(&secretID, "SECRET_ID", "", "cloud-provider SECRET_ID")
	flag.StringVar(&secretKey, "SECRET_KEY", "", "cloud-provider SECRET_KEY")
	flag.BoolVar(&initEnv, "INIT_ENV", false, "cloud-provider INIT_ENV")
	flag.StringVar(&testType, "TEST_TYPE", "", "test type")
	flag.StringVar(&skipCase, "SKIP_CASE", "", "skip not execute cases")
	flag.StringVar(&configType, "CONFIG_TYPE", "", "test config")
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
	tcDefault, err = NewTestClient(test.VeleroCfg.DefaultCluster)
	test.VeleroCfg.DefaultClient = &tcDefault
	test.VeleroCfg.ClientToInstallVelero = test.VeleroCfg.DefaultClient
	if err != nil {
		return err
	}

	if test.VeleroCfg.DefaultCluster != "" {
		err = KubectlConfigUseContext(context.Background(), test.VeleroCfg.DefaultCluster)
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
	}
	Version = version
	InitEnv = initEnv
	TestType = testType
	ConfigType = configType
	log.Println("TestType is ：" + TestType)
	SkipCase = skipCase
	TestResults = make([]Result, 0)
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
})

var _ = AfterSuite(func() {
	By("delete helm release in e2e-test environment")
	CheckedUninstallHelmRelease()
	if testEnv != nil {
		By("removed installed CRDs in e2e-test environment")
		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})

var _ = Describe("e2e test", func() {
	if initEnv {
		Cancel()
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
		var _ = Describe("KubeBlocks playground init", PlaygroundInit)

		var _ = Describe("KubeBlocks uninstall", UninstallKubeblocks)

		var _ = Describe("Check healthy Kubernetes cluster status", EnvCheckTest)
	}

	var kubeblocks string

	if initEnv == false && len(version) > 0 {
		It("check kbcli exist or not-exist", func() {
			kbcli := e2eutil.CheckCommand("kbcli", "/usr/local/bin")
			Expect(kbcli).Should(BeTrue())
			kubeblocks = e2eutil.ExecCommand("kbcli version | grep KubeBlocks " +
				"| (grep \"$1\" || true) | awk '{print $2}'")
		})
		It("check kubeblocks exist or not-exist", func() {
			log.Println("kubeblocks : " + kubeblocks)
			Expect(kubeblocks).ShouldNot(BeEmpty())
			if len(kubeblocks) == 0 {
				var _ = Describe("KubeBlocks operator installation", InstallationTest)
			}
		})
	}

	var _ = Describe("Configure running e2e information", Config)

	var _ = Describe("KubeBlocks smoke test run", SmokeTest)

	var _ = Describe("Delete e2e config resources", DeleteConfig)

	if initEnv == false {
		if len(kubeblocks) > 0 {
			var _ = Describe("KubeBlocks operator uninstallation", UninstallationTest)
		}
	}

	if initEnv {
		var _ = Describe("KubeBlocks playground destroy", PlaygroundDestroy)
		var _ = Describe("Check environment has been cleaned", EnvGotCleanedTest)
	}

	var _ = Describe("show test report", AnalyzeE2eReport)

	if initEnv {
		var _ = Describe("save test report to s3", UploadReport)
	}
})
