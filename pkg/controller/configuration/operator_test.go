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

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("ConfigurationOperatorTest", func() {
	var clusterObj *appsv1alpha1.Cluster
	var compDefObj *appsv1.ComponentDefinition
	var componentObj *appsv1.Component
	var synthesizedComponent *component.SynthesizedComponent
	var configMapObj *corev1.ConfigMap
	var scriptsObj *corev1.ConfigMap
	var configConstraint *appsv1beta1.ConfigConstraint
	var configurationObj *appsv1alpha1.Configuration
	var k8sMockClient *testutil.K8sClientMockHelper

	createConfigReconcileTask := func() *configOperator {
		task := NewConfigReconcileTask(&ResourceCtx{
			Client:        k8sMockClient.Client(),
			Context:       ctx,
			Namespace:     testCtx.DefaultNamespace,
			ClusterName:   clusterName,
			ComponentName: mysqlCompName,
		},
			clusterObj,
			componentObj,
			synthesizedComponent,
			synthesizedComponent.PodSpec,
			nil)
		return task
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		k8sMockClient = testutil.NewK8sMockClient()
		clusterObj, compDefObj, _ = newAllFieldsClusterObj(nil, false)
		synthesizedComponent = newAllFieldsSynthesizedComponent(compDefObj, clusterObj)
		componentObj = newAllFieldsComponent(clusterObj)
		configMapObj = testapps.NewConfigMap("default", mysqlConfigName,
			testapps.SetConfigMapData("test", "test"))
		scriptsObj = testapps.NewConfigMap("default", mysqlScriptsTemplateName,
			testapps.SetConfigMapData("script.sh", "echo \"hello\""))
		configurationObj = builder.NewConfigurationBuilder(testCtx.DefaultNamespace,
			cfgcore.GenerateComponentConfigurationName(clusterName, mysqlCompName)).
			ClusterRef(clusterName).
			Component(mysqlCompName).
			GetObject()
		configConstraint = &appsv1beta1.ConfigConstraint{
			ObjectMeta: metav1.ObjectMeta{
				Name: mysqlConfigConstraintName,
			},
			Spec: appsv1beta1.ConfigConstraintSpec{
				ReloadAction: &appsv1beta1.ReloadAction{
					ShellTrigger: &appsv1beta1.ShellTrigger{
						Command: []string{"echo", "hello"},
						Sync:    cfgutil.ToPointer(true),
					},
				},
				FileFormatConfig: &appsv1beta1.FileFormatConfig{
					Format: appsv1beta1.Ini,
					FormatterAction: appsv1beta1.FormatterAction{
						IniConfig: &appsv1beta1.IniConfig{
							SectionName: "mysqld",
						},
					},
				},
			},
		}
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("ConfigOperatorTest", func() {
		It("NormalTest", func() {
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
				[]client.Object{
					compDefObj,
					clusterObj,
					clusterObj,
					scriptsObj,
					configMapObj,
					configConstraint,
					configurationObj,
				},
			), testutil.WithAnyTimes()))
			k8sMockClient.MockCreateMethod(testutil.WithCreateReturned(testutil.WithCreatedSucceedResult(), testutil.WithAnyTimes()))
			k8sMockClient.MockPatchMethod(testutil.WithPatchReturned(func(obj client.Object, patch client.Patch) error {
				switch v := obj.(type) {
				case *appsv1alpha1.Configuration:
					if client.ObjectKeyFromObject(obj) == client.ObjectKeyFromObject(configurationObj) {
						configurationObj.Spec = *v.Spec.DeepCopy()
						configurationObj.Status = *v.Status.DeepCopy()
					}
				}
				return nil
			}))
			k8sMockClient.MockStatusMethod().
				EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil)
			// DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			//	return nil
			// })

			Expect(createConfigReconcileTask().Reconcile()).Should(Succeed())
		})

		It("EmptyConfigSpecTest", func() {

			k8sMockClient.MockCreateMethod(testutil.WithCreateReturned(testutil.WithCreatedSucceedResult(), testutil.WithTimes(1)))
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
				[]client.Object{
					compDefObj,
					clusterObj,
					clusterObj,
				},
			), testutil.WithAnyTimes()))

			synthesizedComponent.ConfigTemplates = nil
			synthesizedComponent.ScriptTemplates = nil
			Expect(createConfigReconcileTask().Reconcile()).Should(Succeed())
		})

	})

})
