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

package configuration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("ConfigurationOperatorTest", func() {

	var clusterObj *appsv1alpha1.Cluster
	var clusterVersionObj *appsv1alpha1.ClusterVersion
	var clusterDefObj *appsv1alpha1.ClusterDefinition
	var clusterComponent *component.SynthesizedComponent
	var configMapObj *corev1.ConfigMap
	var scriptsObj *corev1.ConfigMap
	var configConstraint *appsv1alpha1.ConfigConstraint
	var configurationObj *appsv1alpha1.Configuration
	var k8sMockClient *testutil.K8sClientMockHelper

	mockStatefulSet := func() *appsv1.StatefulSet {
		envConfig := factory.BuildEnvConfig(clusterObj, clusterComponent)
		stsObj, err := factory.BuildSts(intctrlutil.RequestCtx{
			Ctx: ctx,
			Log: logger,
		}, clusterObj, clusterComponent, envConfig.Name)
		Expect(err).Should(Succeed())
		return stsObj
	}

	createConfigReconcileTask := func() *configOperator {
		task := NewConfigReconcileTask(&intctrlutil.ResourceCtx{
			Client:        k8sMockClient.Client(),
			Context:       ctx,
			Namespace:     testCtx.DefaultNamespace,
			ClusterName:   clusterName,
			ComponentName: mysqlCompName,
		},
			clusterObj,
			clusterVersionObj,
			clusterComponent,
			mockStatefulSet(),
			clusterComponent.PodSpec,
			nil)
		return task
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		k8sMockClient = testutil.NewK8sMockClient()
		clusterObj, clusterDefObj, clusterVersionObj, _ = newAllFieldsClusterObj(nil, nil, false)
		clusterComponent = newAllFieldsComponent(clusterDefObj, clusterVersionObj)
		configMapObj = testapps.NewConfigMap("default", mysqlConfigName,
			testapps.SetConfigMapData("test", "test"))
		scriptsObj = testapps.NewConfigMap("default", mysqlScriptsConfigName,
			testapps.SetConfigMapData("script.sh", "echo \"hello\""))
		configurationObj = builder.NewConfigurationBuilder(testCtx.DefaultNamespace,
			cfgcore.GenerateComponentConfigurationName(clusterName, mysqlCompName)).
			ClusterRef(clusterName).
			Component(mysqlCompName).
			GetObject()
		configConstraint = &appsv1alpha1.ConfigConstraint{
			ObjectMeta: metav1.ObjectMeta{
				Name: mysqlConfigConstraintName,
			},
			Spec: appsv1alpha1.ConfigConstraintSpec{
				ReloadOptions: &appsv1alpha1.ReloadOptions{
					ShellTrigger: &appsv1alpha1.ShellTrigger{
						Command: []string{"echo", "hello"},
						Sync:    cfgutil.ToPointer(true),
					},
				},
				FormatterConfig: &appsv1alpha1.FormatterConfig{
					Format: appsv1alpha1.Ini,
					FormatterOptions: appsv1alpha1.FormatterOptions{
						IniConfig: &appsv1alpha1.IniConfig{
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
					clusterDefObj,
					clusterVersionObj,
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
					clusterDefObj,
					clusterVersionObj,
					clusterObj,
					clusterObj,
				},
			), testutil.WithAnyTimes()))

			clusterComponent.ConfigTemplates = nil
			clusterComponent.ScriptTemplates = nil
			Expect(createConfigReconcileTask().Reconcile()).Should(Succeed())
		})

	})

})
