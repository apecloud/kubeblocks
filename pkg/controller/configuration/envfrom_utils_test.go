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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("ConfigEnvFrom test", func() {
	const (
		compDefName   = "test-compdef"
		clusterName   = "test-cluster"
		mysqlCompName = "mysql"
	)

	var (
		compDef *appsv1alpha1.ComponentDefinition
		cluster *appsv1alpha1.Cluster

		k8sMockClient    *testutil.K8sClientMockHelper
		origCMObject     *corev1.ConfigMap
		configConstraint *appsv1beta1.ConfigConstraint
	)

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()

		cm := testapps.NewCustomizedObj("config/envfrom-config.yaml", &corev1.ConfigMap{},
			testCtx.UseDefaultNamespace())

		configConstraint = testapps.NewCustomizedObj("config/envfrom-constraint.yaml",
			&appsv1beta1.ConfigConstraint{})

		compDef = testapps.NewComponentDefinitionFactory(compDefName).
			SetDefaultSpec().
			AddConfigTemplate(cm.Name, cm.Name, configConstraint.Name, testCtx.DefaultNamespace, "mysql-config", testapps.DefaultMySQLContainerName).
			GetObject()

		pvcSpec := testapps.NewPVCSpec("1Gi")
		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			AddComponentV2(mysqlCompName, compDef.Name).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			GetObject()

		origCMObject = cm.DeepCopy()
		origCMObject.Name = core.GetComponentCfgName(clusterName, mysqlCompName, cm.Name)
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("test config template inject envfrom", func() {
		It("should inject success", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			comp, err := component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
			Expect(err).Should(Succeed())

			synthesizeComp, err := component.BuildSynthesizedComponent(reqCtx, testCtx.Cli, cluster, compDef, comp)
			Expect(err).Should(Succeed())

			podSpec := &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: testapps.DefaultMySQLContainerName,
					},
				},
			}
			k8sMockClient.MockGetMethod(
				testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
					origCMObject,
					configConstraint,
				}), testutil.WithAnyTimes()))
			k8sMockClient.MockCreateMethod(
				testutil.WithCreateReturned(testutil.WithCreatedFailedResult(), testutil.WithTimes(1)),
				testutil.WithCreateReturned(testutil.WithCreatedSucceedResult(), testutil.WithAnyTimes()),
			)

			Expect(injectTemplateEnvFrom(cluster, synthesizeComp, podSpec, k8sMockClient.Client(), reqCtx.Ctx, nil)).ShouldNot(Succeed())
			Expect(injectTemplateEnvFrom(cluster, synthesizeComp, podSpec, k8sMockClient.Client(), reqCtx.Ctx, nil)).Should(Succeed())
		})

		It("should SyncEnvConfigmap success", func() {
			configSpec := compDef.Spec.Configs[0]
			configSpec.Keys = []string{"env-config"}

			cmObj := origCMObject.DeepCopy()
			cmObj.SetName(core.GenerateEnvFromName(origCMObject.Name))
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				cmObj,
				configConstraint,
			}), testutil.WithAnyTimes()))
			k8sMockClient.MockPatchMethod(testutil.WithFailed(core.MakeError("failed to patch"), testutil.WithTimes(1)),
				testutil.WithSucceed(), testutil.WithAnyTimes())

			Expect(SyncEnvConfigmap(configSpec, origCMObject, &configConstraint.Spec, k8sMockClient.Client(), ctx)).ShouldNot(Succeed())
			Expect(SyncEnvConfigmap(configSpec, origCMObject, &configConstraint.Spec, k8sMockClient.Client(), ctx)).Should(Succeed())
		})

		It("SyncEnvConfigmap abnormal test", func() {
			configSpec := compDef.Spec.Configs[0]
			configSpec.InjectEnvTo = nil
			Expect(SyncEnvConfigmap(configSpec, origCMObject, &configConstraint.Spec, k8sMockClient.Client(), ctx)).Should(Succeed())

			configSpec.InjectEnvTo = nil
			cmObj := origCMObject.DeepCopy()
			cmObj.SetName(core.GenerateEnvFromName(origCMObject.Name))
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				cmObj,
				configConstraint,
			}), testutil.WithAnyTimes()))
			k8sMockClient.MockPatchMethod(testutil.WithSucceed(testutil.WithAnyTimes()))

			configSpec = compDef.Spec.Configs[0]
			configSpec.Keys = []string{"env-config", "not-exist"}
			Expect(SyncEnvConfigmap(configSpec, origCMObject, &configConstraint.Spec, k8sMockClient.Client(), ctx)).Should(Succeed())
		})
	})
})
