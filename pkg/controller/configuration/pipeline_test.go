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
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	configurationv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("ConfigurationPipelineTest", func() {
	const testConfigFile = "postgresql.conf"

	var clusterObj *appsv1.Cluster
	var componentObj *appsv1.Component
	var compDefObj *appsv1.ComponentDefinition
	var synthesizedComponent *component.SynthesizedComponent
	var configMapObj *corev1.ConfigMap
	var configConstraint *appsv1beta1.ConfigConstraint
	var configurationObj *configurationv1alpha1.ComponentParameter
	var k8sMockClient *testutil.K8sClientMockHelper

	mockAPIResource := func(lazyFetcher testutil.Getter) {
		k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
			[]client.Object{
				compDefObj,
				clusterObj,
				clusterObj,
				configMapObj,
				configConstraint,
				configurationObj,
			}, lazyFetcher), testutil.WithAnyTimes()))
		k8sMockClient.MockCreateMethod(testutil.WithCreateReturned(testutil.WithCreatedSucceedResult(), testutil.WithAnyTimes()))
		k8sMockClient.MockPatchMethod(testutil.WithPatchReturned(func(obj client.Object, patch client.Patch) error {
			switch v := obj.(type) {
			case *configurationv1alpha1.ComponentParameter:
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
				case *configurationv1alpha1.ComponentParameter:
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
		clusterObj, compDefObj, _ = newAllFieldsClusterObj(nil, false)
		componentObj = newAllFieldsComponent(clusterObj)
		synthesizedComponent = newAllFieldsSynthesizedComponent(compDefObj, clusterObj)
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
		configConstraint = &appsv1beta1.ConfigConstraint{
			ObjectMeta: metav1.ObjectMeta{
				Name: mysqlConfigConstraintName,
			},
			Spec: appsv1beta1.ConfigConstraintSpec{
				FileFormatConfig: &appsv1beta1.FileFormatConfig{
					Format: appsv1beta1.Properties,
				},
			}}
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("ConfigPipelineTest", func() {
		It("NormalTest", func() {
			By("mock configSpec keys")
			synthesizedComponent.ConfigTemplates[0].Keys = []string{testConfigFile}

			By("create configuration resource")
			createPipeline := NewReloadActionBuilderHelper(ReconcileCtx{
				ResourceCtx: &ResourceCtx{
					Client:        k8sMockClient.Client(),
					Context:       ctx,
					Namespace:     testCtx.DefaultNamespace,
					ClusterName:   clusterName,
					ComponentName: mysqlCompName,
				},
				Cluster:              clusterObj,
				Component:            componentObj,
				SynthesizedComponent: synthesizedComponent,
				PodSpec:              synthesizedComponent.PodSpec,
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
				UpdateConfigurationForTest().
				Configuration().
				InitConfigRelatedObject().
				UpdatePodVolumes().
				BuildConfigManagerSidecar().
				Complete()
			Expect(err).Should(Succeed())

			By("update configuration resource for mocking reconfiguring")
			item := configurationObj.Spec.ConfigItemDetails[0]
			item.ConfigFileParams = map[string]configurationv1alpha1.ParametersInFile{
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
				ResourceCtx:          createPipeline.ResourceCtx,
				Cluster:              clusterObj,
				Component:            componentObj,
				SynthesizedComponent: synthesizedComponent,
				PodSpec:              synthesizedComponent.PodSpec,
			}, item, &configurationObj.Status.ConfigurationItemStatus[0], nil)

			By("update configuration resource")
			err = reconcileTask.InitConfigSpec().
				Configuration().
				ConfigMap(configTemplateName).
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
			// reconcileTask.item.Version = "v2"
			err = reconcileTask.InitConfigSpec().
				Configuration().
				ConfigMap(configTemplateName).
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

func (p *reloadActionBuilderHelper) UpdateConfigurationForTest() *reloadActionBuilderHelper {
	buildConfiguration := func() (err error) {
		expectedConfiguration := p.createConfiguration()
		if intctrlutil.SetControllerReference(p.ctx.Component, expectedConfiguration) != nil {
			return
		}
		_, _ = UpdateConfigPayload(&expectedConfiguration.Spec, p.ctx.SynthesizedComponent)

		existingConfiguration := configurationv1alpha1.ComponentParameter{}
		err = p.ResourceFetcher.Client.Get(p.Context, client.ObjectKeyFromObject(expectedConfiguration), &existingConfiguration)
		switch {
		case err == nil:
			return p.updateConfiguration(expectedConfiguration, &existingConfiguration)
		case apierrors.IsNotFound(err):
			return p.ResourceFetcher.Client.Create(p.Context, expectedConfiguration)
		default:
			return err
		}
	}
	return p.Wrap(buildConfiguration)
}

func (p *reloadActionBuilderHelper) createConfiguration() *configurationv1alpha1.ComponentParameter {
	builder := builder.NewConfigurationBuilder(p.Namespace,
		cfgcore.GenerateComponentConfigurationName(p.ClusterName, p.ComponentName),
	)
	for _, template := range p.ctx.SynthesizedComponent.ConfigTemplates {
		builder.AddConfigurationItem(template)
	}
	return builder.Component(p.ComponentName).
		ClusterRef(p.ClusterName).
		AddLabelsInMap(constant.GetComponentWellKnownLabels(p.ClusterName, p.ComponentName)).
		GetObject()
}

func (p *reloadActionBuilderHelper) updateConfiguration(expected *configurationv1alpha1.ComponentParameter, existing *configurationv1alpha1.ComponentParameter) error {
	fromMap := func(items []configurationv1alpha1.ConfigTemplateItemDetail) *cfgutil.Sets {
		sets := cfgutil.NewSet()
		for _, item := range items {
			sets.Add(item.Name)
		}
		return sets
	}

	updateConfigSpec := func(item configurationv1alpha1.ConfigTemplateItemDetail) configurationv1alpha1.ConfigTemplateItemDetail {
		if newItem := intctrlutil.GetConfigurationItem(&expected.Spec, item.Name); newItem != nil {
			item.ConfigSpec = newItem.ConfigSpec
		}
		return item
	}

	oldSets := fromMap(existing.Spec.ConfigItemDetails)
	newSets := fromMap(expected.Spec.ConfigItemDetails)

	addSets := cfgutil.Difference(newSets, oldSets)
	delSets := cfgutil.Difference(oldSets, newSets)

	newConfigItems := make([]configurationv1alpha1.ConfigTemplateItemDetail, 0)
	for _, item := range existing.Spec.ConfigItemDetails {
		if !delSets.InArray(item.Name) {
			newConfigItems = append(newConfigItems, updateConfigSpec(item))
		}
	}
	for _, item := range expected.Spec.ConfigItemDetails {
		if addSets.InArray(item.Name) {
			newConfigItems = append(newConfigItems, item)
		}
	}

	patch := client.MergeFrom(existing)
	updated := existing.DeepCopy()
	updated.Spec.ConfigItemDetails = newConfigItems
	return p.Client.Patch(p.Context, updated, patch)
}
