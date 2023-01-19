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

package dbaas

import (
	"context"
	"go/build"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	//+kubebuilder:scaffold:imports

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var testCtx testutil.TestContext
var clusterRecorder record.EventRecorder
var systemAccountReconciler *SystemAccountReconciler

func init() {
	viper.AutomaticEnv()
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	if viper.GetBool("ENABLE_DEBUG_LOG") {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
			o.TimeEncoder = zapcore.ISO8601TimeEncoder
		}))
	}

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	var flag = false
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases"),
			// use dependent external CRDs.
			// resolved by ref: https://github.com/operator-framework/operator-sdk/issues/4434#issuecomment-786794418
			filepath.Join(build.Default.GOPATH, "pkg", "mod", "github.com", "kubernetes-csi/external-snapshotter/",
				"client/v6@v6.0.1", "config", "crd")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    &flag,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = dbaasv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = dataprotectionv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = snapshotv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// run reconcile
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	viper.SetDefault("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs")
	viper.SetDefault("VOLUMESNAPSHOT", false)
	viper.SetDefault("KUBEBLOCKS_IMAGE", "apecloud/kubeblocks:latest")
	viper.SetDefault("PROBE_SERVICE_PORT", 3501)
	viper.SetDefault("PROBE_SERVICE_LOG_LEVEL", "info")

	clusterRecorder = k8sManager.GetEventRecorderFor("db-cluster-controller")
	err = (&ClusterReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: clusterRecorder,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ClusterDefinitionReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("cluster-definition-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ClusterVersionReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("cluster-version-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&OpsRequestReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("ops-request-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&components.StatefulSetReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("stateful-set-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&components.DeploymentReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("deployment-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&k8score.StorageClassReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("storage-class-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&k8score.EventReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("event-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	// add SystemAccountReconciler
	systemAccountReconciler = &SystemAccountReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("system-account-controller"),
	}
	err = systemAccountReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	testCtx = testutil.NewDefaultTestContext(k8sManager.GetClient())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// Helper functions to change fields in the desired state and status of resources.
// Each helper is a wrapper of k8sClient.Patch.
// Example:
// changeSpec(key, func(clusterDef *dbaasv1alpha1.ClusterDefinition) {
//		// modify clusterDef
// })

func changeSpec[T intctrlutil.Object, PT intctrlutil.PObject[T]](namespacedName types.NamespacedName,
	action func(pobj PT)) error {
	var obj T
	pobj := PT(&obj)
	if err := k8sClient.Get(ctx, namespacedName, pobj); err != nil {
		return err
	}
	patch := client.MergeFrom(PT(pobj.DeepCopy()))
	action(pobj)
	if err := k8sClient.Patch(ctx, pobj, patch); err != nil {
		return err
	}
	return nil
}

func changeStatus[T intctrlutil.Object, PT intctrlutil.PObject[T]](namespacedName types.NamespacedName,
	action func(pobj PT)) error {
	var obj T
	pobj := PT(&obj)
	if err := k8sClient.Get(ctx, namespacedName, pobj); err != nil {
		return err
	}
	patch := client.MergeFrom(PT(pobj.DeepCopy()))
	action(pobj)
	if err := k8sClient.Status().Patch(ctx, pobj, patch); err != nil {
		return err
	}
	return nil
}

// Helper functions to check fields of resources when writing unit tests.
// Each helper returns a Gomega assertion function, which should be passed into
// Eventually() or Consistently() as the first parameter.
// Example:
// Eventually(checkObj(key, func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
//   g.Expect(..).To(BeTrue()) // do some check
// })).Should(Succeed())

func checkObj[T intctrlutil.Object, PT intctrlutil.PObject[T]](namespacedName types.NamespacedName,
	check func(g Gomega, pobj PT)) func(g Gomega) {
	return func(g Gomega) {
		var obj T
		pobj := PT(&obj)
		g.Expect(k8sClient.Get(ctx, namespacedName, pobj)).To(Succeed())
		check(g, pobj)
	}
}

func clearResources[T intctrlutil.Object, PT intctrlutil.PObject[T],
	L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L], Traits intctrlutil.ObjListTraits[T, L]](
	ctx context.Context, _ func(T, L, Traits), opts ...client.ListOption) {
	const cleanTimeout = time.Second * 60
	const cleanInterval = time.Second

	var (
		obj     T
		objList L
		traits  Traits
	)
	listOptions := &client.ListOptions{}
	for _, opt := range opts {
		opt.ApplyToList(listOptions)
	}
	deleteAllOfOpts := &client.DeleteAllOfOptions{}
	listOptions.ApplyToList(&deleteAllOfOpts.ListOptions)
	Expect(k8sClient.DeleteAllOf(ctx, PT(&obj), deleteAllOfOpts)).Should(Succeed())

	Eventually(func(g Gomega) {
		g.Expect(k8sClient.List(ctx, PL(&objList), opts...)).Should(Succeed())
		g.Expect(len(traits.GetItems(&objList))).Should(BeEquivalentTo(0))
	}, cleanTimeout, cleanInterval).Should(Succeed())
}
