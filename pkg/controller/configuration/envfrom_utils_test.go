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

package configuration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("ConfigEnvFrom test", func() {
	const (
		compDefName   = "test-compdef"
		clusterName   = "test-cluster"
		mysqlCompName = "mysql"
	)

	var (
		compDef *appsv1.ComponentDefinition
		cluster *appsv1.Cluster

		k8sMockClient *testutil.K8sClientMockHelper
		origCMObject  *corev1.ConfigMap
		configRender  *parametersv1alpha1.ParameterDrivenConfigRender
	)

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()

		cm := testparameters.NewComponentTemplateFactory("", testCtx.DefaultNamespace).
			WithRandomName().
			AddConfigFile("env-file", `
dbStorage_rocksDB_writeBufferSizeMB=8
dbStorage_rocksDB_sstSizeInMB=64
dbStorage_rocksDB_blockSize=65536
dbStorage_rocksDB_bloomFilterBitsPerKey=10
dbStorage_rocksDB_numLevels=-1
dbStorage_rocksDB_numFilesInLevel0=4
dbStorage_rocksDB_maxSizeInLevel1MB=256
`).GetObject()

		configRender = testparameters.NewParametersDrivenConfigFactory("").
			WithRandomName().
			SetConfigDescription("env-file", cm.Name,
				parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Properties}).
			GetObject()

		compDef = testapps.NewComponentDefinitionFactory(compDefName).
			SetDefaultSpec().
			AddConfigTemplate(cm.Name, cm.Name, testCtx.DefaultNamespace, "mysql-config").
			GetObject()

		pvcSpec := testapps.NewPVCSpec("1Gi")
		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			AddComponent(mysqlCompName, compDef.Name).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			GetObject()

		origCMObject = cm.DeepCopy()
		origCMObject.Name = core.GetComponentCfgName(clusterName, mysqlCompName, cm.Name)

		_ = cluster
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("test config template inject envfrom", func() {
		It("should inject success", func() {
			comp, err := component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
			Expect(err).Should(Succeed())

			synthesizeComp, err := component.BuildSynthesizedComponent(ctx, testCtx.Cli, compDef, comp, cluster)
			Expect(err).Should(Succeed())

			podSpec := &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: testapps.DefaultMySQLContainerName,
					},
				},
			}
			desc := intctrlutil.GetComponentConfigDescription(&configRender.Spec, "env-file")
			desc.InjectEnvTo = []string{testapps.DefaultMySQLContainerName}
			objs, err := InjectTemplateEnvFrom(synthesizeComp, podSpec, configRender, []*corev1.ConfigMap{origCMObject})
			Expect(err).Should(Succeed())
			Expect(objs).Should(HaveLen(1))
			Expect(generics.FindFunc(podSpec.Containers[0].EnvFrom, func(source corev1.EnvFromSource) bool {
				return source.ConfigMapRef.Name == objs[0].Name
			})).Should(HaveLen(1))
		})

		// It("should SyncEnvSourceObject success", func() {
		// 	configSpec := compDef.Spec.Configs[0]
		// 	configSpec.Keys = []string{"env-config"}
		//
		// 	comp, err := component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
		// 	Expect(err).Should(Succeed())
		//
		// 	synthesizeComp, err := component.BuildSynthesizedComponent(ctx, testCtx.Cli, compDef, comp, cluster)
		// 	Expect(err).Should(Succeed())
		//
		// 	cmObj := origCMObject.DeepCopy()
		// 	cmObj.SetName(core.GenerateEnvFromName(origCMObject.Name))
		// 	k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
		// 		cmObj,
		// 		configConstraint,
		// 	}), testutil.WithAnyTimes()))
		// 	k8sMockClient.MockUpdateMethod(testutil.WithFailed(core.MakeError("failed to patch"), testutil.WithTimes(1)),
		// 		testutil.WithSucceed(), testutil.WithAnyTimes())
		//
		// 	Expect(SyncEnvSourceObject(configSpec, origCMObject, &configConstraint.Spec, k8sMockClient.Client(), ctx, cluster, synthesizeComp)).ShouldNot(Succeed())
		// 	Expect(SyncEnvSourceObject(configSpec, origCMObject, &configConstraint.Spec, k8sMockClient.Client(), ctx, cluster, synthesizeComp)).Should(Succeed())
		// })
		//
		// It("SyncEnvSourceObject abnormal test", func() {
		// 	comp, err := component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
		// 	Expect(err).Should(Succeed())
		//
		// 	synthesizeComp, err := component.BuildSynthesizedComponent(ctx, testCtx.Cli, compDef, comp, cluster)
		// 	Expect(err).Should(Succeed())
		//
		// 	configSpec := compDef.Spec.Configs[0]
		// 	configSpec.InjectEnvTo = nil
		// 	Expect(SyncEnvSourceObject(configSpec, origCMObject, &configConstraint.Spec, k8sMockClient.Client(), ctx, cluster, synthesizeComp)).Should(Succeed())
		//
		// 	configSpec.InjectEnvTo = nil
		// 	cmObj := origCMObject.DeepCopy()
		// 	cmObj.SetName(core.GenerateEnvFromName(origCMObject.Name))
		// 	k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
		// 		cmObj,
		// 		configConstraint,
		// 	}), testutil.WithAnyTimes()))
		// 	k8sMockClient.MockUpdateMethod(testutil.WithSucceed(testutil.WithAnyTimes()))
		//
		// 	configSpec = compDef.Spec.Configs[0]
		// 	configSpec.Keys = []string{"env-config", "not-exist"}
		// 	Expect(SyncEnvSourceObject(configSpec, origCMObject, &configConstraint.Spec, k8sMockClient.Client(), ctx, cluster, synthesizeComp)).Should(Succeed())
		// })
	})
})
