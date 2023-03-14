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

package plan

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Cluster Controller", func() {

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResources(&testCtx, generics.ConfigConstraintSignature, ml)

		// namespaced
		testapps.ClearResources(&testCtx, generics.ConfigMapSignature, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	const (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		clusterName        = "test-cluster"
	)
	var (
		clusterDef     *appsv1alpha1.ClusterDefinition
		clusterVersion *appsv1alpha1.ClusterVersion
		cluster        *appsv1alpha1.Cluster
	)

	Context("with Deployment workload", func() {
		const (
			nginxCompType = "nginx"
			nginxCompName = "nginx"
		)
		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatelessNginxComponent, nginxCompType).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(nginxCompType).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(nginxCompType, nginxCompName).
				GetObject()
		})

		It("should construct env, headless service, deployment and external service objects", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			task := types.InitReconcileTask(clusterDef, clusterVersion, cluster, component)
			Expect(PrepareStatelessComponentWorkloads(reqCtx, testCtx.Cli, task)).Should(Succeed())

			resources := *task.Resources
			Expect(len(resources)).Should(Equal(4))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("Deployment"))
			Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("Service"))
		})
	})

	Context("with Stateful workload and without config template", func() {
		const (
			mysqlCompType = "replicasets"
			mysqlCompName = "mysql"
		)
		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompType).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()
			pvcSpec := testapps.NewPVC("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompType).
				AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
				GetObject()
		})

		It("should construct env, default ClusterIP service, headless service and statefuset objects and should not render config template", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0],
			)
			task := types.InitReconcileTask(clusterDef, clusterVersion, cluster, component)
			Expect(PrepareStatefulComponentWorkloads(reqCtx, testCtx.Cli, task)).Should(Succeed())

			resources := *task.Resources
			Expect(len(resources)).Should(Equal(4))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("StatefulSet"))

			container := clusterDef.Spec.ComponentDefs[0].PodSpec.Containers[0]
			sts := resources[2].(*appsv1.StatefulSet)
			Expect(len(sts.Spec.Template.Spec.Volumes)).Should(Equal(len(container.VolumeMounts)))
		})
	})

	Context("with Stateful workload and with config template", func() {
		const (
			mysqlCompType = "replicasets"
			mysqlCompName = "mysql"
		)
		BeforeEach(func() {
			cm := testapps.CreateCustomizedObj(&testCtx, "config/configcm.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			cfgTpl := testapps.CreateCustomizedObj(&testCtx, "config/configtpl.yaml",
				&appsv1alpha1.ConfigConstraint{})

			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				AddConfigTemplate(cm.Name, cm.Name, cfgTpl.Name, testCtx.DefaultNamespace, "mysql-config", nil).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompType).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()
			pvcSpec := testapps.NewPVC("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompType).
				AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
				GetObject()
		})

		It("should render config template", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			task := types.InitReconcileTask(clusterDef, clusterVersion, cluster, component)
			Expect(PrepareStatefulComponentWorkloads(reqCtx, testCtx.Cli, task)).Should(Succeed())

			resources := *task.Resources
			Expect(len(resources)).Should(Equal(5))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("StatefulSet"))
		})
	})

	Context("with Stateful workload and with config template and with config volume mount", func() {
		const (
			mysqlCompType = "replicasets"
			mysqlCompName = "mysql"
		)
		BeforeEach(func() {
			cm := testapps.CreateCustomizedObj(&testCtx, "config/configcm.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			cfgTpl := testapps.CreateCustomizedObj(&testCtx, "config/configtpl.yaml",
				&appsv1alpha1.ConfigConstraint{})

			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				AddConfigTemplate(cm.Name, cm.Name, cfgTpl.Name, testCtx.DefaultNamespace, "mysql-config", nil).
				AddContainerVolumeMounts("mysql", []corev1.VolumeMount{{Name: "mysql-config", MountPath: "/mnt/config"}}).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompType).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()
			pvcSpec := testapps.NewPVC("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompType).
				AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
				GetObject()
		})

		It("should add config manager sidecar container", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			task := types.InitReconcileTask(clusterDef, clusterVersion, cluster, component)
			Expect(PrepareStatefulComponentWorkloads(reqCtx, testCtx.Cli, task)).Should(Succeed())

			resources := *task.Resources
			Expect(len(resources)).Should(Equal(5))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("StatefulSet"))

			originPodSpec := clusterDef.Spec.ComponentDefs[0].PodSpec
			Expect(len(originPodSpec.Containers)).Should(Equal(1))

			sts := resources[3].(*appsv1.StatefulSet)
			podSpec := sts.Spec.Template.Spec
			Expect(len(podSpec.Containers)).Should(Equal(2))
		})
	})

	// for test GetContainerWithVolumeMount
	Context("with Consensus workload and with external service", func() {
		const (
			mysqlCompType = "replicasets"
			mysqlCompName = "mysql"
			nginxCompType = "proxy"
		)

		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ConsensusMySQLComponent, mysqlCompType).
				AddComponent(testapps.StatelessNginxComponent, nginxCompType).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompType).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponent(nginxCompType).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			pvcSpec := testapps.NewPVC("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompType).
				AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
				GetObject()
		})

		It("should construct env, headless service, statefuset and external service objects", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			task := types.InitReconcileTask(clusterDef, clusterVersion, cluster, component)
			Expect(PrepareConsensusComponentWorkloads(reqCtx, testCtx.Cli, task)).Should(Succeed())

			resources := *task.Resources
			Expect(len(resources)).Should(Equal(4))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("StatefulSet"))
			Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("Service"))
		})
	})

	// for test GetContainerWithVolumeMount
	Context("with Replications workload without pvc", func() {
		const (
			redisCompType = "replicasets"
			redisCompName = "redis"
			nginxCompType = "proxy"
		)

		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ReplicationRedisComponent, redisCompType).
				AddComponent(testapps.StatelessNginxComponent, nginxCompType).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(redisCompType).
				AddContainerShort("redis", testapps.DefaultRedisImageName).
				AddComponent(nginxCompType).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(redisCompName, redisCompType).
				SetReplicas(2).
				SetPrimaryIndex(0).
				GetObject()
		})

		It("should construct env, headless service, statefuset objects for each replica, besides an external service object", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			task := types.InitReconcileTask(clusterDef, clusterVersion, cluster, component)
			Expect(PrepareReplicationComponentWorkloads(reqCtx, testCtx.Cli, task)).Should(Succeed())

			resources := *task.Resources
			Expect(len(resources)).Should(Equal(7))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("StatefulSet"))
			Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[4]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[5]).String()).Should(ContainSubstring("StatefulSet"))
			Expect(reflect.TypeOf(resources[6]).String()).Should(ContainSubstring("Service"))
		})
	})

	// for test GetContainerWithVolumeMount
	Context("with Replications workload with pvc", func() {
		const (
			redisCompType = "replicasets"
			redisCompName = "redis"
			nginxCompType = "proxy"
		)

		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ReplicationRedisComponent, redisCompType).
				AddComponent(testapps.StatelessNginxComponent, nginxCompType).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(redisCompType).
				AddContainerShort("redis", testapps.DefaultRedisImageName).
				AddComponent(nginxCompType).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			pvcSpec := testapps.NewPVC("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(redisCompName, redisCompType).
				SetReplicas(2).
				SetPrimaryIndex(0).
				AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
				GetObject()
		})

		It("should construct pvc objects for each replica", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			task := types.InitReconcileTask(clusterDef, clusterVersion, cluster, component)
			Expect(PrepareReplicationComponentWorkloads(reqCtx, testCtx.Cli, task)).Should(Succeed())

			resources := *task.Resources
			Expect(len(resources)).Should(Equal(9))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("PersistentVolumeClaim"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("StatefulSet"))
			Expect(reflect.TypeOf(resources[4]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[5]).String()).Should(ContainSubstring("PersistentVolumeClaim"))
			Expect(reflect.TypeOf(resources[6]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[7]).String()).Should(ContainSubstring("StatefulSet"))
			Expect(reflect.TypeOf(resources[8]).String()).Should(ContainSubstring("Service"))
		})
	})

})
