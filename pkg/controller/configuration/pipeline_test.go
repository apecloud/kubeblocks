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
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("ConfigurationPipelineTest", func() {

	const testConfigFile = "postgresql.conf"

	var clusterObj *appsv1alpha1.Cluster
	var clusterVersionObj *appsv1alpha1.ClusterVersion
	var clusterDefObj *appsv1alpha1.ClusterDefinition
	var clusterComponent *component.SynthesizedComponent
	var configMapObj *corev1.ConfigMap
	var configConstraint *appsv1alpha1.ConfigConstraint
	var configurationObj *appsv1alpha1.Configuration
	var k8sMockClient *testutil.K8sClientMockHelper

	mockAPIResource := func(lazyFetcher testutil.Getter) {
		k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
			[]client.Object{
				clusterDefObj,
				clusterVersionObj,
				clusterObj,
				clusterObj,
				configMapObj,
				configConstraint,
				configurationObj,
			}, lazyFetcher), testutil.WithAnyTimes()))
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
		}, testutil.WithAnyTimes()))
		k8sMockClient.MockStatusMethod().
			EXPECT().
			Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				switch v := obj.(type) {
				case *appsv1alpha1.Configuration:
					if client.ObjectKeyFromObject(obj) == client.ObjectKeyFromObject(configurationObj) {
						configurationObj.Status = *v.Status.DeepCopy()
					}
				}
				return nil
			}).AnyTimes()
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		k8sMockClient = testutil.NewK8sMockClient()
		clusterObj, clusterDefObj, clusterVersionObj, _ = newAllFieldsClusterObj(nil, nil, false)
		clusterComponent = newAllFieldsComponent(clusterDefObj, clusterVersionObj, clusterObj)
		configMapObj = testapps.NewConfigMap("default", mysqlConfigName,
			testapps.SetConfigMapData(testConfigFile, `
bgwriter_delay = '200ms'
bgwriter_flush_after = '64'
bgwriter_lru_maxpages = '1000'
bgwriter_lru_multiplier = '10.0'
bytea_output = 'hex'
check_function_bodies = 'True'
checkpoint_completion_target = '0.9'
checkpoint_flush_after = '32'
checkpoint_timeout = '15min'
max_connections = '1000'
`))
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
				FormatterConfig: &appsv1alpha1.FormatterConfig{
					Format: appsv1alpha1.Properties,
				},
			}}
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("ConfigPipelineTest", func() {
		It("NormalTest", func() {
			By("mock configSpec keys")
			clusterComponent.ConfigTemplates[0].Keys = []string{testConfigFile}

			By("create configuration resource")
			createPipeline := NewCreatePipeline(ReconcileCtx{
				ResourceCtx: &intctrlutil.ResourceCtx{
					Client:        k8sMockClient.Client(),
					Context:       ctx,
					Namespace:     testCtx.DefaultNamespace,
					ClusterName:   clusterName,
					ComponentName: mysqlCompName,
				},
				Cluster:   clusterObj,
				Component: clusterComponent,
				PodSpec:   clusterComponent.PodSpec,
			})

			By("mock api resource for configuration")
			mockAPIResource(func(key client.ObjectKey, obj client.Object) (bool, error) {
				switch obj.(type) {
				case *corev1.ConfigMap:
					for _, renderedObj := range createPipeline.renderWrapper.renderedObjs {
						if client.ObjectKeyFromObject(renderedObj) == key {
							testutil.SetGetReturnedObject(obj, renderedObj)
							return true, nil
						}
					}
				}
				return false, nil
			})

			err := createPipeline.Prepare().
				UpdateConfiguration(). // reconcile Configuration
				Configuration().       // sync Configuration
				CreateConfigTemplate().
				UpdatePodVolumes().
				BuildConfigManagerSidecar().
				UpdateConfigRelatedObject().
				UpdateConfigurationStatus().
				Complete()
			Expect(err).Should(Succeed())

			By("update configuration resource for mocking reconfiguring")
			item := configurationObj.Spec.ConfigItemDetails[0]
			item.ConfigFileParams = map[string]appsv1alpha1.ConfigParams{
				testConfigFile: {
					Parameters: map[string]*string{
						"max_connections": cfgutil.ToPointer("2000"),
					},
				},
				"other.conf": {
					Content: cfgutil.ToPointer(`for test`),
				},
			}
			reconcileTask := NewReconcilePipeline(ReconcileCtx{
				ResourceCtx: createPipeline.ResourceCtx,
				Cluster:     clusterObj,
				Component:   clusterComponent,
				PodSpec:     clusterComponent.PodSpec,
			}, item, &configurationObj.Status.ConfigurationItemStatus[0], nil)

			By("update configuration resource")
			err = reconcileTask.InitConfigSpec().
				Configuration().
				ConfigMap(configSpecName).
				ConfigConstraints(reconcileTask.ConfigSpec().ConfigConstraintRef).
				PrepareForTemplate().
				RerenderTemplate().
				ApplyParameters().
				UpdateConfigVersion(strconv.FormatInt(reconcileTask.ConfigurationObj.GetGeneration(), 10)).
				Sync().
				SyncStatus().
				Complete()
			Expect(err).Should(Succeed())

			By("rerender configuration template")
			reconcileTask.item.Version = "v2"
			err = reconcileTask.InitConfigSpec().
				Configuration().
				ConfigMap(configSpecName).
				ConfigConstraints(reconcileTask.ConfigSpec().ConfigConstraintRef).
				PrepareForTemplate().
				RerenderTemplate().
				ApplyParameters().
				UpdateConfigVersion(strconv.FormatInt(reconcileTask.ConfigurationObj.GetGeneration(), 10)).
				Sync().
				SyncStatus().
				Complete()
			Expect(err).Should(Succeed())
		})
	})

})
