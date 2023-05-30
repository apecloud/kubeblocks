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

package plan

import (
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
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

// buildComponentResources generate all necessary sub-resources objects used in component,
// like Secret, ConfigMap, Service, StatefulSet, Deployment, Volume, PodDisruptionBudget etc.
func buildComponentResources(reqCtx intctrlutil.RequestCtx, cli client.Client,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) ([]client.Object, error) {
	resources := make([]client.Object, 0)
	workloadProcessor := func(customSetup func(*corev1.ConfigMap) (client.Object, error)) error {
		envConfig, err := builder.BuildEnvConfigLow(reqCtx, cli, cluster, component)
		if err != nil {
			return err
		}
		resources = append(resources, envConfig)

		workload, err := customSetup(envConfig)
		if err != nil {
			return err
		}

		defer func() {
			// workload object should be appended last
			resources = append(resources, workload)
		}()

		svc, err := builder.BuildHeadlessSvcLow(cluster, component)
		if err != nil {
			return err
		}
		resources = append(resources, svc)

		var podSpec *corev1.PodSpec
		sts, ok := workload.(*appsv1.StatefulSet)
		if ok {
			podSpec = &sts.Spec.Template.Spec
		} else {
			deploy, ok := workload.(*appsv1.Deployment)
			if ok {
				podSpec = &deploy.Spec.Template.Spec
			}
		}
		if podSpec == nil {
			return nil
		}

		defer func() {
			for _, cc := range []*[]corev1.Container{
				&podSpec.Containers,
				&podSpec.InitContainers,
			} {
				volumes := podSpec.Volumes
				for _, c := range *cc {
					for _, v := range c.VolumeMounts {
						// if persistence is not found, add emptyDir pod.spec.volumes[]
						volumes, _ = intctrlutil.CreateOrUpdateVolume(volumes, v.Name, func(volumeName string) corev1.Volume {
							return corev1.Volume{
								Name: v.Name,
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							}
						}, nil)
					}
				}
				podSpec.Volumes = volumes
			}
		}()

		// render config template
		configs, err := RenderConfigNScriptFiles(clusterVer, cluster, component, workload, podSpec, nil, reqCtx.Ctx, cli)
		if err != nil {
			return err
		}
		if configs != nil {
			resources = append(resources, configs...)
		}
		// end render config

		//// tls certs secret volume and volumeMount
		// if err := updateTLSVolumeAndVolumeMount(podSpec, cluster.Name, *component); err != nil {
		//	return err
		// }
		return nil
	}

	// pre-condition check
	// if component.WorkloadType == appsv1alpha1.Replication {
	//	// get the number of existing pods under the current component
	//	var existPodList = &corev1.PodList{}
	//	if err := componentutil.GetObjectListByComponentName(reqCtx.Ctx, cli, *cluster, existPodList, component.Name); err != nil {
	//		return nil, err
	//	}
	//
	//	// If the Pods already exists, check whether there is an HA switching and the HA process is prioritized to handle.
	//	// TODO: (xingran) After refactoring, HA switching will be handled in the replicationSet controller.
	//	if len(existPodList.Items) > 0 {
	//		primaryIndexChanged, _, err := replication.CheckPrimaryIndexChanged(reqCtx.Ctx, cli, cluster,
	//			component.Name, component.GetPrimaryIndex())
	//		if err != nil {
	//			return nil, err
	//		}
	//		if primaryIndexChanged {
	//			if err := replication.HandleReplicationSetHASwitch(reqCtx.Ctx, cli, cluster,
	//				componentutil.GetClusterComponentSpecByName(*cluster, component.Name)); err != nil {
	//				return nil, err
	//			}
	//		}
	//	}
	// }

	// TODO: may add a PDB transform to Create/Update/Delete.
	// if no these handle, the cluster controller will occur an error during reconciling.
	// conditional build PodDisruptionBudget
	if component.MinAvailable != nil {
		pdb, err := builder.BuildPDBLow(cluster, component)
		if err != nil {
			return nil, err
		}
		resources = append(resources, pdb)
	} else {
		panic("this shouldn't happen")
	}

	svcList, err := builder.BuildSvcListWithCustomAttributes(cluster, component, func(svc *corev1.Service) {
		switch component.WorkloadType {
		case appsv1alpha1.Consensus:
			addLeaderSelectorLabels(svc, component)
		case appsv1alpha1.Replication:
			svc.Spec.Selector[constant.RoleLabelKey] = "primary"
		}
	})
	if err != nil {
		return nil, err
	}
	for _, svc := range svcList {
		resources = append(resources, svc)
	}

	// REVIEW/TODO:
	// - need higher level abstraction handling
	// - or move this module to part operator controller handling
	switch component.WorkloadType {
	case appsv1alpha1.Stateless:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildDeployLow(reqCtx, cluster, component)
			}); err != nil {
			return nil, err
		}
	case appsv1alpha1.Stateful, appsv1alpha1.Consensus, appsv1alpha1.Replication:
		if err := workloadProcessor(
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return builder.BuildStsLow(reqCtx, cluster, component, envConfig.Name)
			}); err != nil {
			return nil, err
		}
	}

	return resources, nil
}

// TODO multi roles with same accessMode support
func addLeaderSelectorLabels(service *corev1.Service, component *component.SynthesizedComponent) {
	leader := component.ConsensusSpec.Leader
	if len(leader.Name) > 0 {
		service.Spec.Selector[constant.RoleLabelKey] = leader.Name
	}
}

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
				AddComponentVersion(nginxCompDefName).
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
			component, err := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())

			resources, err := buildComponentResources(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, component)
			Expect(err).Should(Succeed())

			expects := []string{
				"PodDisruptionBudget",
				"Service",
				"ConfigMap",
				"Service",
				"Deployment",
			}
			Expect(resources).Should(HaveLen(len(expects)))
			for i, v := range expects {
				Expect(reflect.TypeOf(resources[i]).String()).Should(ContainSubstring(v), fmt.Sprintf("failed at idx %d", i))
			}
		})
	})

	Context("with Stateful workload and without config template", func() {
		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponentVersion(mysqlCompDefName).
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
			component, err := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0],
			)
			Expect(err).Should(Succeed())

			resources, err := buildComponentResources(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, component)
			Expect(err).Should(Succeed())

			expects := []string{
				"PodDisruptionBudget",
				"Service",
				"ConfigMap",
				"Service",
				"StatefulSet",
			}
			Expect(resources).Should(HaveLen(len(expects)))
			for i, v := range expects {
				Expect(reflect.TypeOf(resources[i]).String()).Should(ContainSubstring(v), fmt.Sprintf("failed at idx %d", i))
				if v == "StatefulSet" {
					container := clusterDef.Spec.ComponentDefs[0].PodSpec.Containers[0]
					sts := resources[i].(*appsv1.StatefulSet)
					Expect(len(sts.Spec.Template.Spec.Volumes)).Should(Equal(len(container.VolumeMounts)))
				}
			}
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
				AddComponentVersion(mysqlCompDefName).
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
			component, err := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())

			resources, err := buildComponentResources(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, component)
			Expect(err).Should(Succeed())

			expects := []string{
				"PodDisruptionBudget",
				"Service",
				"ConfigMap",
				"Service",
				"ConfigMap",
				"StatefulSet",
			}
			Expect(resources).Should(HaveLen(len(expects)))
			for i, v := range expects {
				Expect(reflect.TypeOf(resources[i]).String()).Should(ContainSubstring(v), fmt.Sprintf("failed at idx %d", i))
			}
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
				AddComponentVersion(mysqlCompDefName).
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
			component, err := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())

			resources, err := buildComponentResources(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, component)
			Expect(err).Should(Succeed())

			expects := []string{
				"PodDisruptionBudget",
				"Service",
				"ConfigMap",
				"Service",
				"ConfigMap",
				"StatefulSet",
			}
			Expect(resources).Should(HaveLen(len(expects)))
			for i, v := range expects {
				Expect(reflect.TypeOf(resources[i]).String()).Should(ContainSubstring(v), fmt.Sprintf("failed at idx %d", i))
				if v == "StatefulSet" {
					sts := resources[i].(*appsv1.StatefulSet)
					podSpec := sts.Spec.Template.Spec
					Expect(len(podSpec.Containers)).Should(Equal(2))
				}
			}
			originPodSpec := clusterDef.Spec.ComponentDefs[0].PodSpec
			Expect(len(originPodSpec.Containers)).Should(Equal(1))
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
				AddComponentVersion(mysqlCompDefName).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponentVersion(nginxCompDefName).
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
			component, err := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			resources, err := buildComponentResources(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, component)
			Expect(err).Should(Succeed())
			expects := []string{
				"PodDisruptionBudget",
				"Service",
				"ConfigMap",
				"Service",
				"StatefulSet",
			}
			Expect(resources).Should(HaveLen(len(expects)))
			for i, v := range expects {
				Expect(reflect.TypeOf(resources[i]).String()).Should(ContainSubstring(v), fmt.Sprintf("failed at idx %d", i))
			}
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
				AddComponentVersion(redisCompDefName).
				AddContainerShort("redis", testapps.DefaultRedisImageName).
				AddComponentVersion(nginxCompDefName).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(redisCompName, redisCompDefName).
				SetReplicas(2).
				GetObject()
		})

		It("should construct env, headless service, statefuset object, besides an external service object", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component, err := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())

			resources, err := buildComponentResources(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, component)
			Expect(err).Should(Succeed())

			// REVIEW: (free6om)
			//  missing connection credential, TLS secret objs check?
			Expect(resources).Should(HaveLen(5))
			Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("PodDisruptionBudget"))
			Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("ConfigMap"))
			Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("Service"))
			Expect(reflect.TypeOf(resources[4]).String()).Should(ContainSubstring("StatefulSet"))
		})
	})

	// TODO: (free6om)
	//  uncomment following test case until pre-provisoned PVC work begin
	// // for test GetContainerWithVolumeMount
	// Context("with Replications workload with pvc", func() {
	// 	var (
	// 		clusterDef     *appsv1alpha1.ClusterDefinition
	// 		clusterVersion *appsv1alpha1.ClusterVersion
	// 		cluster        *appsv1alpha1.Cluster
	// 	)
	//
	// 	BeforeEach(func() {
	// 		clusterDef = testapps.NewClusterDefFactory(clusterDefName).
	// 			AddComponentDef(testapps.ReplicationRedisComponent, redisCompDefName).
	// 			AddComponentDef(testapps.StatelessNginxComponent, nginxCompDefName).
	// 			GetObject()
	// 		clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
	// 			AddComponentVersion(redisCompDefName).
	// 			AddContainerShort("redis", testapps.DefaultRedisImageName).
	// 			AddComponentVersion(nginxCompDefName).
	// 			AddContainerShort("nginx", testapps.NginxImage).
	// 			GetObject()
	// 		pvcSpec := testapps.NewPVCSpec("1Gi")
	// 		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
	// 			clusterDef.Name, clusterVersion.Name).
	// 			AddComponentVersion(redisCompName, redisCompDefName).
	// 			SetReplicas(2).
	// 			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
	// 			GetObject()
	// 	})
	//
	// 	It("should construct pvc objects for each replica", func() {
	// 		reqCtx := intctrlutil.RequestCtx{
	// 			Ctx: ctx,
	// 			Log: logger,
	// 		}
	// 		component := component.BuildComponent(
	// 			reqCtx,
	// 			*cluster,
	// 			*clusterDef,
	// 			clusterDef.Spec.ComponentDefs[0],
	// 			cluster.Spec.ComponentSpecs[0],
	// 			&clusterVersion.Spec.ComponentVersions[0])
	// 		task := types.InitReconcileTask(clusterDef, clusterVersion, cluster, component)
	// 		Expect(PrepareComponentResources(reqCtx, testCtx.Cli, task)).Should(Succeed())
	//
	// 		resources := *task.Resources
	// 		Expect(resources).Should(HaveLen(6))
	// 		Expect(reflect.TypeOf(resources[0]).String()).Should(ContainSubstring("ConfigMap"))
	// 		Expect(reflect.TypeOf(resources[1]).String()).Should(ContainSubstring("Service"))
	// 		Expect(reflect.TypeOf(resources[2]).String()).Should(ContainSubstring("PersistentVolumeClaim"))
	// 		Expect(reflect.TypeOf(resources[3]).String()).Should(ContainSubstring("PersistentVolumeClaim"))
	// 		Expect(reflect.TypeOf(resources[4]).String()).Should(ContainSubstring("StatefulSet"))
	// 		Expect(reflect.TypeOf(resources[5]).String()).Should(ContainSubstring("Service"))
	// 	})
	// })

})
