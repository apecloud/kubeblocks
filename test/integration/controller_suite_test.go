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

package appstest

import (
	"context"
	"go/build"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	dpctrl "github.com/apecloud/kubeblocks/controllers/dataprotection"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	cliutil "github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var dynamicClient dynamic.Interface
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var testCtx testutil.TestContext
var clusterRecorder record.EventRecorder
var logger logr.Logger

func init() {
	viper.AutomaticEnv()
	// viper.Set("ENABLE_DEBUG_LOG", "true")
}

func TestIntegrationController(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	RegisterFailHandler(Fail)

	RunSpecs(t, "Integration Test Suite")
}

// GetConsensusRoleCountMap gets a role:count map from a consensusSet cluster
func GetConsensusRoleCountMap(testCtx testutil.TestContext, k8sClient client.Client, cluster *appsv1alpha1.Cluster) (roleCountMap map[string]int) {
	clusterkey := client.ObjectKeyFromObject(cluster)
	stsList := &appsv1.StatefulSetList{}
	err := testCtx.Cli.List(testCtx.Ctx, stsList, client.MatchingLabels{
		constant.AppInstanceLabelKey: clusterkey.Name,
	}, client.InNamespace(clusterkey.Namespace))

	roleCountMap = make(map[string]int)
	roleCountMap["leader"] = 0
	roleCountMap["follower"] = 0
	roleCountMap["learner"] = 0

	if err != nil || len(stsList.Items) == 0 {
		return roleCountMap
	}

	sts := stsList.Items[0]
	pods, err := util.GetPodListByStatefulSet(testCtx.Ctx, k8sClient, &sts)

	if err != nil {
		return roleCountMap
	}

	for _, pod := range pods {
		role := pod.Labels[constant.RoleLabelKey]
		roleCountMap[role]++
	}

	return roleCountMap
}

func CreateSimpleConsensusMySQLClusterWithConfig(
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	mysqlConfigTemplatePath,
	mysqlConfigConstraintPath,
	mysqlScriptsPath string) (
	*appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, *appsv1alpha1.Cluster) {
	const mysqlCompName = "mysql"
	const mysqlCompDefName = "mysql"
	const mysqlConfigName = "mysql-component-config"
	const mysqlConfigConstraintName = "mysql8.0-config-constraints"
	const mysqlScriptsConfigName = "apecloud-mysql-scripts"
	const mysqlDataVolumeName = "data"
	const mysqlConfigVolumeName = "mysql-config"
	const mysqlScriptsVolumeName = "scripts"
	const mysqlErrorFilePath = "/data/mysql/log/mysqld-error.log"
	const mysqlGeneralFilePath = "/data/mysql/log/mysqld.log"
	const mysqlSlowlogFilePath = "/data/mysql/log/mysqld-slowquery.log"

	mysqlConsensusType := string(testapps.ConsensusMySQLComponent)

	configmap := testapps.CreateCustomizedObj(&testCtx,
		mysqlConfigTemplatePath, &corev1.ConfigMap{},
		testCtx.UseDefaultNamespace(),
		testapps.WithLabels(
			constant.AppNameLabelKey, clusterName,
			constant.AppInstanceLabelKey, clusterName,
			constant.KBAppComponentLabelKey, mysqlConsensusType,
			constant.CMConfigurationTemplateNameLabelKey, mysqlConfigName,
			constant.CMConfigurationConstraintsNameLabelKey, mysqlConfigConstraintName,
			constant.CMConfigurationSpecProviderLabelKey, mysqlConfigName,
			constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
		))

	_ = testapps.CreateCustomizedObj(&testCtx, mysqlScriptsPath, &corev1.ConfigMap{},
		testapps.WithName(mysqlScriptsConfigName), testCtx.UseDefaultNamespace())

	By("Create a constraint obj")
	constraint := testapps.CreateCustomizedObj(&testCtx,
		mysqlConfigConstraintPath,
		&appsv1alpha1.ConfigConstraint{})

	mysqlVolumeMounts := []corev1.VolumeMount{
		{
			Name:      mysqlConfigVolumeName,
			MountPath: "/opt/mysql",
		},
		{
			Name:      mysqlScriptsVolumeName,
			MountPath: "/scripts",
		},
		{
			Name:      mysqlDataVolumeName,
			MountPath: "/data/mysql",
		},
	}

	By("Create a clusterDefinition obj")
	mode := int32(0755)
	clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
		SetConnectionCredential(map[string]string{"username": "root", "password": ""}, nil).
		AddComponentDef(testapps.ConsensusMySQLComponent, mysqlCompDefName).
		AddConfigTemplate(mysqlConfigName, configmap.Name, constraint.Name,
			testCtx.DefaultNamespace, mysqlConfigVolumeName).
		AddScriptTemplate(mysqlScriptsConfigName, mysqlScriptsConfigName,
			testCtx.DefaultNamespace, mysqlScriptsVolumeName, &mode).
		AddContainerVolumeMounts(testapps.DefaultMySQLContainerName, mysqlVolumeMounts).
		AddLogConfig("error", mysqlErrorFilePath).
		AddLogConfig("general", mysqlGeneralFilePath).
		AddLogConfig("slow", mysqlSlowlogFilePath).
		AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(mysqlConfigName), configmap.Name,
			cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
		AddContainerEnv(testapps.DefaultMySQLContainerName, corev1.EnvVar{Name: "MYSQL_ALLOW_EMPTY_PASSWORD", Value: "yes"}).
		AddContainerEnv(testapps.DefaultMySQLContainerName, corev1.EnvVar{Name: "CLUSTER_START_INDEX", Value: "1"}).
		AddContainerEnv(testapps.DefaultMySQLContainerName, corev1.EnvVar{Name: "CLUSTER_ID", Value: "1"}).
		Create(&testCtx).GetObject()

	By("Create a clusterVersion obj")
	clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
		AddComponentVersion(mysqlCompDefName).
		AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
		AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(mysqlConfigName), configmap.Name,
			cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
		Create(&testCtx).GetObject()

	By("Creating a cluster")
	pvcSpec := appsv1alpha1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
		},
	}
	clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
		clusterDefObj.Name, clusterVersionObj.Name).
		AddComponent(mysqlCompName, mysqlCompDefName).
		SetReplicas(3).
		SetEnabledLogs("error", "general", "slow").
		AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
		Create(&testCtx).GetObject()

	return clusterDefObj, clusterVersionObj, clusterObj
}

var _ = BeforeSuite(func() {
	if viper.GetBool("ENABLE_DEBUG_LOG") {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
			o.TimeEncoder = zapcore.ISO8601TimeEncoder
		}))
	}

	ctx, cancel = context.WithCancel(context.TODO())
	logger = logf.FromContext(ctx).WithValues()
	logger.Info("logger start")

	By("bootstrapping test environment")
	var flag = true
	testEnv = &envtest.Environment{
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

	err = appsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = dataprotectionv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = snapshotv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	dynamicClient, err = cliutil.NewFactory().DynamicClient()
	Expect(err).NotTo(HaveOccurred())
	Expect(dynamicClient).NotTo(BeNil())

	// run reconcile
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                scheme.Scheme,
		MetricsBindAddress:    "0",
		ClientDisableCacheFor: intctrlutil.GetUncachedObjects(),
	})
	Expect(err).ToNot(HaveOccurred())

	viper.SetDefault("CERT_DIR", "/tmp/k8s-webhook-server/serving-certs")
	viper.SetDefault("VOLUMESNAPSHOT", false)
	viper.SetDefault(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	viper.SetDefault("PROBE_SERVICE_PORT", 3501)
	viper.SetDefault("PROBE_SERVICE_LOG_LEVEL", "info")

	clusterRecorder = k8sManager.GetEventRecorderFor("db-cluster-controller")
	err = (&apps.ClusterReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: clusterRecorder,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&apps.ClusterDefinitionReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("cluster-definition-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&apps.ClusterVersionReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("cluster-version-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&apps.OpsRequestReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("ops-request-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&apps.SystemAccountReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("system-account-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = apps.NewStatefulSetReconciler(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = apps.NewDeploymentReconciler(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&k8score.EventReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("event-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&dpctrl.BackupReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("backup-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&dpctrl.BackupPolicyReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("backup-policy-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&dpctrl.BackupToolReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("backup-tool-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&dpctrl.RestoreJobReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("restore-job-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&dpctrl.CronJobReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("cronjob-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	// pulling docker images is slow
	viper.SetDefault("EventuallyTimeout", time.Second*300)
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
