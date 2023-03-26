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

package loadbalancer

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/apecloud/kubeblocks/internal/testutil"
)

func init() {
	viper.AutomaticEnv()
}

func TestLoadbalancer(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Loadbalancer Suite")
}

var (
	cfg                *rest.Config
	k8sClient          client.Client
	testEnv            *envtest.Environment
	ctx                context.Context
	cancel             context.CancelFunc
	endpointController *EndpointController
	serviceController  *ServiceController
	logger             = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
		o.TimeEncoder = zapcore.ISO8601TimeEncoder
	})
	testCtx testutil.TestContext
)

var _ = BeforeSuite(func() {
	if viper.GetBool("ENABLE_DEBUG_LOG") {
		logf.SetLogger(logger)
	}

	var err error

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).ShouldNot(HaveOccurred())

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{}
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// run reconcile
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                scheme.Scheme,
		MetricsBindAddress:    "0",
		ClientDisableCacheFor: []client.Object{&corev1.Service{}, &corev1.Endpoints{}, &corev1.Pod{}},
	})
	Expect(err).ToNot(HaveOccurred())

	endpointController = &EndpointController{
		logger:   logger,
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("LoadBalancer"),
	}
	err = endpointController.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	Expect(endpointController).NotTo(BeNil())

	serviceController = &ServiceController{
		logger:   logger,
		cache:    make(map[string]*FloatingIP),
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("LoadBalancer"),
	}
	err = serviceController.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	Expect(serviceController).NotTo(BeNil())

	testCtx = testutil.NewDefaultTestContext(ctx, k8sClient, testEnv)

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run controller")
	}()

	k8sManager.GetCache().WaitForCacheSync(ctx)
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
