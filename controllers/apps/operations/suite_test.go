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

package operations

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var k8sManager ctrl.Manager
var testCtx testutil.TestContext
var eventRecorder record.EventRecorder

const (
	statelessComp = "stateless"
	statefulComp  = "stateful"
	consensusComp = "consensus"
)

func init() {
	viper.AutomaticEnv()
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Operation Controller Suite")
}

var _ = BeforeSuite(func() {
	if viper.GetBool("ENABLE_DEBUG_LOG") {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
			o.TimeEncoder = zapcore.ISO8601TimeEncoder
		}))
	}

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = appsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// run reconcile
	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                scheme.Scheme,
		MetricsBindAddress:    "0",
		ClientDisableCacheFor: intctrlutil.GetUncachedObjects(),
	})
	Expect(err).ToNot(HaveOccurred())

	eventRecorder = k8sManager.GetEventRecorderFor("event-controller")
	err = (&k8score.PersistentVolumeClaimReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("pvc-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	appsv1alpha1.RegisterWebhookManager(k8sManager)

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

// initOperationsResources inits the operations resources.
func initOperationsResources(clusterDefinitionName,
	clusterVersionName,
	clusterName string) (*OpsResource, *appsv1alpha1.ClusterDefinition, *appsv1alpha1.Cluster) {
	clusterDef, _, clusterObject := testapps.InitClusterWithHybridComps(&testCtx, clusterDefinitionName,
		clusterVersionName, clusterName, statelessComp, statefulComp, consensusComp)
	opsRes := &OpsResource{
		Cluster:  clusterObject,
		Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
	}
	By("mock cluster is Running and the status operations")
	Expect(testapps.ChangeObjStatus(&testCtx, clusterObject, func() {
		clusterObject.Status.Phase = appsv1alpha1.RunningClusterPhase
		clusterObject.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
			consensusComp: {
				Phase: appsv1alpha1.RunningClusterCompPhase,
			},
			statelessComp: {
				Phase: appsv1alpha1.RunningClusterCompPhase,
			},
			statefulComp: {
				Phase: appsv1alpha1.RunningClusterCompPhase,
			},
		}
	})).Should(Succeed())
	opsRes.Cluster = clusterObject
	return opsRes, clusterDef, clusterObject
}

func initConsensusPods(ctx context.Context, cli client.Client, opsRes *OpsResource, clusterName string) []corev1.Pod {
	// mock the pods of consensusSet component
	testapps.MockConsensusComponentPods(&testCtx, nil, clusterName, consensusComp)
	podList, err := util.GetComponentPodList(ctx, cli, *opsRes.Cluster, consensusComp)
	Expect(err).Should(Succeed())
	// the opsRequest will use startTime to check some condition.
	// if there is no sleep for 1 second, unstable error may occur.
	time.Sleep(time.Second)
	return podList.Items
}

func mockComponentIsOperating(cluster *appsv1alpha1.Cluster, expectPhase appsv1alpha1.ClusterComponentPhase, compNames ...string) {
	Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
		for _, v := range compNames {
			compStatus := cluster.Status.Components[v]
			compStatus.Phase = expectPhase
			cluster.Status.Components[v] = compStatus
		}
	})).Should(Succeed())
}
