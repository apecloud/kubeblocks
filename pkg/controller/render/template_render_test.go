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

package render

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

const (
	ns                 = "default"
	compDefName        = "test-compdef"
	clusterName        = "test-cluster"
	configTemplateName = "test-config-template"
	mysqlCompName      = "mysql"
	mysqlConfigName    = "mysql-component-config"
	configVolumeName   = "mysql-config"
)

var _ = Describe("TemplateWrapperTest", func() {
	var mockK8sCli *testutil.K8sClientMockHelper
	var clusterObj *appsv1.Cluster
	var componentObj *appsv1.Component
	var compDefObj *appsv1.ComponentDefinition
	var clusterComponent *component.SynthesizedComponent
	var configMapObj *corev1.ConfigMap

	renderTemplate := func(tpls []appsv1.ComponentTemplateSpec) error {
		_, err := RenderTemplate(&ResourceCtx{
			Context:       ctx,
			Client:        mockK8sCli.Client(),
			ClusterName:   clusterName,
			ComponentName: mysqlCompName,
		}, clusterObj, clusterComponent, componentObj, nil, tpls)
		return err
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		mockK8sCli = testutil.NewK8sMockClient()

		configMapObj = testparameters.NewComponentTemplateFactory(configTemplateName, ns).
			GetObject()

		compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			AddConfigTemplate(mysqlConfigName, configMapObj.Name, ns, configVolumeName).
			GetObject()

		clusterObj = testapps.NewClusterFactory(ns, clusterName, "").
			AddComponent(mysqlCompName, compDefObj.GetName()).
			GetObject()

		fullCompName := constant.GenerateClusterComponentName(clusterName, mysqlCompName)
		componentObj = testapps.NewComponentFactory(ns, fullCompName, compDefObj.Name).
			AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			SetUID(types.UID(fmt.Sprintf("%s-%s", clusterObj.Name, "test-uid"))).
			SetReplicas(1).
			GetObject()

		clusterComponent, _ = component.BuildSynthesizedComponent(ctx, mockK8sCli.Client(), compDefObj, componentObj, clusterObj)
	})

	AfterEach(func() {
		DeferCleanup(mockK8sCli.Finish)
	})

	Context("TestConfigSpec", func() {
		It("TestConfigSpec without template", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{}), testutil.WithAnyTimes()))

			Expect(renderTemplate(clusterComponent.ConfigTemplates)).ShouldNot(Succeed())
		})

		It("TestConfigSpec with exist configmap", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				configMapObj,
			}), testutil.WithAnyTimes()))

			Expect(renderTemplate(clusterComponent.ConfigTemplates)).Should(Succeed())
		})
	})
})
