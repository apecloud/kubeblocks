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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
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
		testapps.ClearResources(&testCtx, generics.ParamConfigRendererSignature, ml)
		testapps.ClearResources(&testCtx, generics.ParametersDefinitionSignature, ml)
		testapps.ClearResources(&testCtx, generics.ComponentDefinitionSignature, ml)

		// namespaced
		testapps.ClearResources(&testCtx, generics.ConfigMapSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ComponentSignature, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	const (
		compDefName   = "test-compdef"
		clusterName   = "test-cluster"
		mysqlCompName = "mysql"
		paramsDefName = "mysql-params-def"
		pdcrName      = "mysql-pdcr"
		envFileName   = "test"
	)

	var (
		compDefObj *appsv1.ComponentDefinition
		cluster    *appsv1.Cluster
		comp       *appsv1.Component
	)

	Context("create cluster with component and component definition API, testing render configuration", func() {
		createAllTypesClusterDef := func() {
			By("Create a componentDefinition obj")
			compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
				WithRandomName().
				SetDefaultSpec().
				AddConfigs(testapps.DefaultCompDefConfigs).
				AddScripts(testapps.DefaultCompDefScripts).
				AddVolumeMounts("mysql", []corev1.VolumeMount{{Name: testapps.DefaultConfigSpecVolumeName, MountPath: "/mnt/config"}}).
				Create(&testCtx).
				GetObject()
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
				obj.Status.Phase = appsv1.AvailablePhase
			})()).Should(Succeed())
		}

		BeforeEach(func() {
			createAllTypesClusterDef()

			parametersDef := testparameters.NewParametersDefinitionFactory(paramsDefName).
				SetConfigFile(envFileName).
				Schema("").
				Create(&testCtx).
				GetObject()
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(parametersDef), func(obj *parametersv1alpha1.ParametersDefinition) {
				obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
			})()).Should(Succeed())

			configRender := testparameters.NewParamConfigRendererFactory(pdcrName).
				SetConfigDescription(envFileName, testapps.DefaultConfigSpecName, parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Properties}).
				SetComponentDefinition(compDefObj.Name).
				SetParametersDefs(paramsDefName).
				Create(&testCtx).
				GetObject()

			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(configRender), func(obj *parametersv1alpha1.ParamConfigRenderer) {
				obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
			})()).Should(Succeed())

			testparameters.NewComponentTemplateFactory(testapps.DefaultConfigSpecTplRef, testCtx.DefaultNamespace).
				AddConfigFile(envFileName, `
dbStorage_rocksDB_writeBufferSizeMB=8
dbStorage_rocksDB_sstSizeInMB=64
dbStorage_rocksDB_blockSize=65536
dbStorage_rocksDB_bloomFilterBitsPerKey=10
dbStorage_rocksDB_numLevels=-1
dbStorage_rocksDB_numFilesInLevel0=4
dbStorage_rocksDB_maxSizeInLevel1MB=256
`).
				Create(&testCtx)

			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				AddComponent(mysqlCompName, compDefObj.Name).
				SetReplicas(1).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				Create(&testCtx).
				GetObject()

			var err error
			comp, err = component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
			Expect(err).Should(Succeed())
			comp.SetUID("test-uid")
			Expect(testCtx.CreateObj(ctx, comp)).Should(Succeed())
		})

		It("render configuration should success", func() {
			synthesizeComp, err := component.BuildSynthesizedComponent(ctx, testCtx.Cli, compDefObj, comp, cluster)
			Expect(err).Should(Succeed())
			Expect(synthesizeComp.PodSpec).ShouldNot(BeNil())
			resCtx := &render.ResourceCtx{
				Context:       testCtx.Ctx,
				Client:        testCtx.Cli,
				Namespace:     synthesizeComp.Namespace,
				ClusterName:   synthesizeComp.ClusterName,
				ComponentName: synthesizeComp.Name,
			}
			err = RenderConfigNScriptFiles(resCtx, cluster, comp, synthesizeComp, synthesizeComp.PodSpec, nil)
			Expect(err).Should(Succeed())
		})
	})
})
