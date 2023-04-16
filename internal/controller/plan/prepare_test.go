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

const (
	mysqlCompDefName = "replicasets"
	mysqlCompName    = "mysql"
	nginxCompDefName = "nginx"
	nginxCompName    = "nginx"
	redisCompDefName = "replicasets"
	redisCompName    = "redis"
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
		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatelessNginxComponent, nginxCompDefName).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(nginxCompDefName).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(nginxCompDefName, nginxCompName).
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
			Expect(PrepareComponentResources(reqCtx, testCtx.Cli, task)).Should(Succeed())

			resources := *task.Resources
			Expect(len(resources)).Should(Equal(4))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("Deployment"))
			Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("Service"))
		})
	})

	Context("with Stateful workload and without config template", func() {
		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompDefName).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
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
			Expect(PrepareComponentResources(reqCtx, testCtx.Cli, task)).Should(Succeed())

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
		BeforeEach(func() {
			cm := testapps.CreateCustomizedObj(&testCtx, "config/config-template.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			cfgTpl := testapps.CreateCustomizedObj(&testCtx, "config/config-constraint.yaml",
				&appsv1alpha1.ConfigConstraint{})

			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				AddConfigTemplate(cm.Name, cm.Name, cfgTpl.Name, testCtx.DefaultNamespace, "mysql-config").
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompDefName).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
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
			Expect(PrepareComponentResources(reqCtx, testCtx.Cli, task)).Should(Succeed())

			resources := *task.Resources
			Expect(len(resources)).Should(Equal(5))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("StatefulSet"))
		})
	})

	Context("with Stateful workload and with config template and with config volume mount", func() {
		BeforeEach(func() {
			cm := testapps.CreateCustomizedObj(&testCtx, "config/config-template.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			cfgTpl := testapps.CreateCustomizedObj(&testCtx, "config/config-constraint.yaml",
				&appsv1alpha1.ConfigConstraint{})

			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				AddConfigTemplate(cm.Name, cm.Name, cfgTpl.Name, testCtx.DefaultNamespace, "mysql-config").
				AddContainerVolumeMounts("mysql", []corev1.VolumeMount{{Name: "mysql-config", MountPath: "/mnt/config"}}).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompDefName).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
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
			Expect(PrepareComponentResources(reqCtx, testCtx.Cli, task)).Should(Succeed())

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
		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, mysqlCompDefName).
				AddComponentDef(testapps.StatelessNginxComponent, nginxCompDefName).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompDefName).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponent(nginxCompDefName).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
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
			Expect(PrepareComponentResources(reqCtx, testCtx.Cli, task)).Should(Succeed())

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
		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, redisCompDefName).
				AddComponentDef(testapps.StatelessNginxComponent, nginxCompDefName).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(redisCompDefName).
				AddContainerShort("redis", testapps.DefaultRedisImageName).
				AddComponent(nginxCompDefName).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(redisCompName, redisCompDefName).
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
			Expect(PrepareComponentResources(reqCtx, testCtx.Cli, task)).Should(Succeed())

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
		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, redisCompDefName).
				AddComponentDef(testapps.StatelessNginxComponent, nginxCompDefName).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(redisCompDefName).
				AddContainerShort("redis", testapps.DefaultRedisImageName).
				AddComponent(nginxCompDefName).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(redisCompName, redisCompDefName).
				SetReplicas(2).
				SetPrimaryIndex(0).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
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
			Expect(PrepareComponentResources(reqCtx, testCtx.Cli, task)).Should(Succeed())

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
