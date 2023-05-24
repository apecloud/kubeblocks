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

package v1alpha1

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	admissionv1 "k8s.io/api/admission/v1"
	admissionregv1 "k8s.io/api/admissionregistration/v1"

	// +kubebuilder:scaffold:imports
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/apecloud/kubeblocks/internal/testutil"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var testCtx testutil.TestContext
var cancel context.CancelFunc

func init() {
	viper.AutomaticEnv()
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhook Suite")
}

var _ = BeforeSuite(func() {

	if viper.GetBool("ENABLE_DEBUG_LOG") {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
			o.TimeEncoder = zapcore.ISO8601TimeEncoder
		}))
	}

	ctx, cancel = context.WithCancel(context.TODO())

	webHookHostExternalName := testCtx.GetWebhookHostExternalName()

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook")},
			// LocalServingHost: "host.docker.internal",
			// LocalServingPort: 28080,
			LocalServingHostExternalName: webHookHostExternalName,
		},
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := runtime.NewScheme()
	err = AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionregv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = storagev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	testCtx = testutil.NewDefaultTestContext(ctx, k8sClient, testEnv)

	// start webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		Host:               webhookInstallOptions.LocalServingHost,
		Port:               webhookInstallOptions.LocalServingPort,
		CertDir:            webhookInstallOptions.LocalServingCertDir,
		LeaderElection:     false,
		MetricsBindAddress: "0",
		ClientDisableCacheFor: []client.Object{
			&storagev1.StorageClass{},
			&ClusterDefinition{},
			&Cluster{},
			&ClusterVersion{},
			&OpsRequest{},
			&ConfigConstraint{},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&ClusterDefinition{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&ClusterVersion{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&Cluster{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&OpsRequest{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	RegisterWebhookManager(mgr)

	testCtx = testutil.NewDefaultTestContext(ctx, k8sClient, testEnv)

	// +kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()
	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())

})

var _ = AfterSuite(func() {

	By("cleanup webhook configuration")
	validatingWebhook := &admissionregv1.ValidatingWebhookConfiguration{}
	validatingWebhook.Name = "validating-webhook-configuration"
	err := k8sClient.Delete(ctx, validatingWebhook)
	Expect(err).NotTo(HaveOccurred())

	mutatingWebhook := &admissionregv1.MutatingWebhookConfiguration{}
	mutatingWebhook.Name = "mutating-webhook-configuration"
	err = k8sClient.Delete(ctx, mutatingWebhook)
	Expect(err).NotTo(HaveOccurred())

	cancel()
	By("tearing down the test environment")
	err = testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
