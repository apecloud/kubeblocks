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

package dataprotection

import (
	"context"
	"go/build"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	vsv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"go.uber.org/zap/zapcore"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	testclocks "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/testutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var testCtx testutil.TestContext
var fakeClock *testclocks.FakeClock

func init() {
	viper.AutomaticEnv()
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Data Protection Controller Suite")
}

var _ = BeforeSuite(func() {
	if viper.GetBool("ENABLE_DEBUG_LOG") {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
			o.TimeEncoder = zapcore.ISO8601TimeEncoder
		}))
	}
	reconcileInterval = time.Millisecond

	ctx, cancel = context.WithCancel(context.TODO())

	viper.SetDefault(constant.CfgKeyCtrlrMgrNS, "default")
	viper.SetDefault(constant.KBToolsImage, "apecloud/kubeblocks:latest")

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			// use dependent external CRDs.
			// resolved by ref: https://github.com/operator-framework/operator-sdk/issues/4434#issuecomment-786794418
			filepath.Join(build.Default.GOPATH, "pkg", "mod", "github.com", "kubernetes-csi/external-snapshotter/",
				"client/v6@v6.2.0", "config", "crd"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := scheme.Scheme

	err = vsv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = vsv1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = appsv1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = dpv1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = storagev1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	uncachedObjects := []client.Object{
		&dpv1alpha1.ActionSet{},
		&dpv1alpha1.BackupPolicy{},
		&dpv1alpha1.BackupSchedule{},
		&dpv1alpha1.BackupRepo{},
		&dpv1alpha1.Backup{},
		&dpv1alpha1.Restore{},
		&vsv1.VolumeSnapshot{},
		&vsv1beta1.VolumeSnapshot{},
		&batchv1.Job{},
		&batchv1.CronJob{},
	}
	// run reconcile
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                scheme,
		MetricsBindAddress:    "0",
		ClientDisableCacheFor: uncachedObjects,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&BackupReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("backup-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&BackupScheduleReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("backup-schedule-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&BackupPolicyReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("backup-policy-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ActionSetReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("actionset-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&BackupRepoReconciler{
		Client:     k8sManager.GetClient(),
		Scheme:     k8sManager.GetScheme(),
		Recorder:   k8sManager.GetEventRecorderFor("backup-repo-controller"),
		RestConfig: k8sManager.GetConfig(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&RestoreReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("restore-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&VolumePopulatorReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("volume-populate-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = mockGCReconciler(k8sManager).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	testCtx = testutil.NewDefaultTestContext(ctx, k8sClient, testEnv)

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func mockGCReconciler(mgr ctrl.Manager) *GCReconciler {
	fakeClock = testclocks.NewFakeClock(time.Now())
	return &GCReconciler{
		Client:    mgr.GetClient(),
		Recorder:  mgr.GetEventRecorderFor("gc-controller"),
		clock:     fakeClock,
		frequency: time.Duration(1) * time.Second,
	}
}
