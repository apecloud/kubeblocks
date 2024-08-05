/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Prepare Test", func() {

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
		clusterDefName = "test-clusterdef"
		clusterName    = "test-cluster"

		mysqlClusterCompDefName = "mysql-cluster-comp-def"
		mysqlCompDefName        = "mysql-comp-def"
		mysqlCompName           = "mysql"
	)
	var (
		clusterDefObj  *appsv1alpha1.ClusterDefinition
		compDefObj     *appsv1alpha1.ComponentDefinition
		cluster        *appsv1alpha1.Cluster
		comp           *appsv1alpha1.Component
		configSpecName string
	)

	Context("create cluster with component and component definition API, testing render configuration", func() {
		createAllTypesClusterDef := func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, mysqlClusterCompDefName).
				Create(&testCtx).GetObject()

			By("Create a componentDefinition obj")
			compDefObj = testapps.NewComponentDefinitionFactory(mysqlCompDefName).
				WithRandomName().
				SetDefaultSpec().
				AddConfigs(testapps.DefaultCompDefConfigs).
				AddScripts(testapps.DefaultCompDefScripts).
				AddVolumeMounts("mysql", []corev1.VolumeMount{{Name: testapps.DefaultConfigSpecVolumeName, MountPath: "/mnt/config"}}).
				Create(&testCtx).
				GetObject()
		}

		BeforeEach(func() {
			createAllTypesClusterDef()

			testapps.CreateCustomizedObj(&testCtx, "config/envfrom-config.yaml", &corev1.ConfigMap{}, testCtx.UseDefaultNamespace())
			tpl := testapps.CreateCustomizedObj(&testCtx, "config/envfrom-constraint.yaml", &appsv1beta1.ConfigConstraint{})
			configSpecName = tpl.Name

			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name).
				AddComponentV2(mysqlCompName, compDefObj.Name).
				SetReplicas(1).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				Create(&testCtx).
				GetObject()

			comp, _ = component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
			comp.SetUID("test-uid")
		})

		It("render configuration should success", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			synthesizeComp, err := component.BuildSynthesizedComponentWrapper(reqCtx, testCtx.Cli, cluster, &cluster.Spec.ComponentSpecs[0])
			Expect(err).Should(Succeed())
			Expect(synthesizeComp.PodSpec).ShouldNot(BeNil())
			resCtx := &configuration.ResourceCtx{
				Context:       testCtx.Ctx,
				Client:        testCtx.Cli,
				Namespace:     synthesizeComp.Namespace,
				ClusterName:   synthesizeComp.ClusterName,
				ComponentName: synthesizeComp.Name,
			}
			err = RenderConfigNScriptFiles(resCtx, cluster, comp, synthesizeComp, synthesizeComp.PodSpec, nil)
			Expect(err).Should(Succeed())
			Expect(configuration.CheckEnvFrom(&synthesizeComp.PodSpec.Containers[0], cfgcore.GenerateEnvFromName(cfgcore.GetComponentCfgName(cluster.Name, synthesizeComp.Name, configSpecName)))).Should(BeFalse())
			// TODO(xingran): add more test cases
			// Expect(len(synthesizeComp.PodSpec.Containers) >= 3).Should(BeTrue())
		})
	})
})
