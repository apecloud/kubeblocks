/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("ComponentParameterGenerator Controller", func() {

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initTestResource := func() *appsv1.Component {
		By("Create a config template obj")
		configmap := testparameters.NewComponentTemplateFactory(configSpecName, testCtx.DefaultNamespace).
			Create(&testCtx).
			GetObject()

		By("Create a parameters definition obj")
		paramsDef := testparameters.NewParametersDefinitionFactory(paramsDefName).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
		})()).Should(Succeed())

		By("Create a component definition obj and mock to available")
		compDefObj := testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, true).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
			obj.Status.Phase = appsv1.AvailablePhase
		})()).Should(Succeed())
		By("Bind the parameters definition directly to the component template")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Spec.ComponentDef = compDefObj.GetName()
			obj.Spec.TemplateName = configSpecName
			obj.Spec.FileFormatConfig = &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Ini,
				FormatterAction: parametersv1alpha1.FormatterAction{
					IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
				},
			}
		})()).Should(Succeed())

		By("Create a custom template cm")
		tplKey := testapps.GetRandomizedKey(testCtx.DefaultNamespace, "custom-tpl")
		tpl := testparameters.NewComponentTemplateFactory(tplKey.Name, testCtx.DefaultNamespace).
			AddConfigFile(testparameters.MysqlConfigFile, "abcde=1234").
			Create(&testCtx).
			GetObject()

		cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			AddComponent(defaultCompName, compDefObj.GetName()).
			AddAnnotations(constant.LegacyConfigManagerRequiredAnnotationKey, "true").
			GetObject()
		Expect(parametersv1alpha1.SetInitialParameters(cluster, parametersv1alpha1.InitialParameters{
			defaultCompName: {
				Assignments: map[string]*string{
					"innodb_buffer_pool_size": pointer.String("1024M"),
					"max_connections":         pointer.String("100"),
				},
				Templates: map[string]parametersv1alpha1.ConfigTemplateExtension{
					configSpecName: {
						TemplateRef: tpl.Name,
						Namespace:   tpl.Namespace,
						Policy:      parametersv1alpha1.ReplacePolicy,
					},
				},
			},
		})).Should(Succeed())
		Expect(testCtx.CreateObj(testCtx.Ctx, cluster)).Should(Succeed())

		By("Create a component obj")
		fullCompName := constant.GenerateClusterComponentName(clusterName, defaultCompName)
		compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefObj.Name).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
			SetUID(types.UID("test-uid")).
			SetReplicas(1).
			SetResources(corev1.ResourceRequirements{Limits: corev1.ResourceList{"memory": resource.MustParse("2Gi")}}).
			Create(&testCtx).
			GetObject()

		testapps.NewInstanceSetFactory(testCtx.DefaultNamespace, constant.GenerateWorkloadNamePattern(clusterName, defaultCompName), clusterName, defaultCompName).
			AddContainer(corev1.Container{
				Name: "config-manager",
				Ports: []corev1.ContainerPort{{
					Name:          "config-manager",
					ContainerPort: 9901,
				}},
			}).
			Create(&testCtx)
		return compObj
	}

	Context("Generate ComponentParameter", func() {
		It("Should reconcile success", func() {
			component := initTestResource()
			parameterKey := types.NamespacedName{
				Namespace: component.Namespace,
				Name:      parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}

			Eventually(testapps.CheckObj(&testCtx, parameterKey, func(g Gomega, parameter *parametersv1alpha1.ComponentParameter) {
				g.Expect(parameter.Spec.Initial).ShouldNot(BeNil())
				g.Expect(parameter.Spec.Initial.Assignments).Should(HaveKeyWithValue("innodb_buffer_pool_size", pointer.String("1024M")))
				g.Expect(parameter.Spec.Initial.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("100")))
				g.Expect(parameter.Spec.Initial.Templates).Should(HaveKey(configSpecName))
				item := parameters.GetConfigTemplateItem(&parameter.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.ConfigFileParams).Should(HaveKey(testparameters.MysqlConfigFile))
				g.Expect(item.ConfigFileParams[testparameters.MysqlConfigFile].Parameters).Should(HaveKeyWithValue("innodb_buffer_pool_size", pointer.String("1024M")))
				g.Expect(item.ConfigFileParams[testparameters.MysqlConfigFile].Parameters).Should(HaveKeyWithValue("max_connections", pointer.String("100")))
			})).Should(Succeed())
		})

		It("does not reapply init template after runtime updates", func() {
			component := initTestResource()
			parameterKey := types.NamespacedName{
				Namespace: component.Namespace,
				Name:      parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}

			Eventually(testapps.CheckObj(&testCtx, parameterKey, func(g Gomega, parameter *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&parameter.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.CustomTemplates).ShouldNot(BeNil())
			})).Should(Succeed())

			By("update the ComponentParameter with a runtime custom template")
			runtimeTplKey := testapps.GetRandomizedKey(testCtx.DefaultNamespace, "runtime-tpl")
			runtimeTpl := testparameters.NewComponentTemplateFactory(runtimeTplKey.Name, testCtx.DefaultNamespace).
				AddConfigFile(testparameters.MysqlConfigFile, "runtime=1").
				Create(&testCtx).
				GetObject()

			Expect(testapps.GetAndChangeObj(&testCtx, parameterKey, func(parameter *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&parameter.Spec, configSpecName)
				Expect(item).ShouldNot(BeNil())
				item.CustomTemplates = &parametersv1alpha1.ConfigTemplateExtension{
					TemplateRef: runtimeTpl.Name,
					Namespace:   runtimeTpl.Namespace,
					Policy:      parametersv1alpha1.ReplacePolicy,
				}
			})()).Should(Succeed())

			By("touch the component to trigger ComponentDrivenParameter reconciliation")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(component), func(comp *appsv1.Component) {
				if comp.Annotations == nil {
					comp.Annotations = map[string]string{}
				}
				comp.Annotations["parameters.kubeblocks.io/runtime-template-test"] = "true"
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, parameterKey, func(g Gomega, parameter *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&parameter.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.CustomTemplates).ShouldNot(BeNil())
				g.Expect(item.CustomTemplates.TemplateRef).Should(Equal(runtimeTpl.Name))
				g.Expect(item.CustomTemplates.Namespace).Should(Equal(runtimeTpl.Namespace))
				g.Expect(parameter.Spec.Initial).ShouldNot(BeNil())
				g.Expect(parameter.Spec.Initial.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("100")))
				g.Expect(parameter.Spec.Initial.Templates).Should(HaveKey(configSpecName))
			})).Should(Succeed())
		})

		It("ignores legacy custom-template component annotation", func() {
			component := initTestResource()
			parameterKey := types.NamespacedName{
				Namespace: component.Namespace,
				Name:      parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}

			legacyTplKey := testapps.GetRandomizedKey(testCtx.DefaultNamespace, "legacy-custom-tpl")
			legacyTpl := testparameters.NewComponentTemplateFactory(legacyTplKey.Name, testCtx.DefaultNamespace).
				AddConfigFile(testparameters.MysqlConfigFile, "legacy=1").
				Create(&testCtx).
				GetObject()
			legacyAnnotation, err := json.Marshal(map[string]parametersv1alpha1.ConfigTemplateExtension{
				configSpecName: {
					TemplateRef: legacyTpl.Name,
					Namespace:   legacyTpl.Namespace,
					Policy:      parametersv1alpha1.ReplacePolicy,
				},
			})
			Expect(err).ShouldNot(HaveOccurred())

			By("set the legacy component annotation and trigger reconcile")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(component), func(comp *appsv1.Component) {
				if comp.Spec.Annotations == nil {
					comp.Spec.Annotations = map[string]string{}
				}
				comp.Spec.Annotations["config.kubeblocks.io/custom-template"] = string(legacyAnnotation)
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, parameterKey, func(g Gomega, parameter *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&parameter.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.CustomTemplates).ShouldNot(BeNil())
				g.Expect(item.CustomTemplates.TemplateRef).ShouldNot(Equal(legacyTpl.Name))
			})).Should(Succeed())
		})
	})

	Context("Resolve init parameters from cluster annotation", func() {
		It("returns error when cluster annotation payload is invalid", func() {
			cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				AddComponent(defaultCompName, compDefName).
				AddAnnotations("config.kubeblocks.io/init-parameters", "{invalid-json").
				Create(&testCtx).
				GetObject()
			comp := &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      constant.GenerateClusterComponentName(cluster.Name, defaultCompName),
					Labels: map[string]string{
						constant.AppInstanceLabelKey: cluster.Name,
					},
				},
			}

			scheme := runtime.NewScheme()
			Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
			fakeReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
			_, err := resolveInitialParameters(intctrlutil.RequestCtx{Ctx: context.Background()}, fakeReader, comp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("invalid cluster initialization payload"))
		})
	})

	Context("No matching ParametersDefinition", func() {
		It("NPE test", func() {
			initTestResource()

			By("Create a component definition obj without matching ParametersDefinition")
			key := testapps.GetRandomizedKey(testCtx.DefaultNamespace, compDefName)
			compDefObj := testapps.NewComponentDefinitionFactory(key.Name).
				WithRandomName().
				SetDefaultSpec().
				AddConfigTemplate(configSpecName, configSpecName, testCtx.DefaultNamespace, configVolumeName, true).
				Create(&testCtx).
				GetObject()
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
				obj.Status.Phase = appsv1.AvailablePhase
			})()).Should(Succeed())

			By("Create a component obj depends on the new cmpd")
			key = testapps.GetRandomizedKey(testCtx.DefaultNamespace, defaultCompName)
			compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, key.Name, compDefObj.Name).
				AddLabels(constant.AppInstanceLabelKey, clusterName).
				AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
				SetUID(types.UID("test-uid")).
				SetReplicas(1).
				Create(&testCtx).
				GetObject()

			Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(compObj), &appsv1.Component{}, true)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(compObj), &parametersv1alpha1.ComponentParameter{}, false)).Should(Succeed())
		})
	})
})

func TestResolveLegacyConfigManagerRequirement(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = parametersv1alpha1.AddToScheme(scheme)
	_ = workloads.AddToScheme(scheme)

	clusterName := "test-cluster"
	namespace := "default"
	buildObjects := func(withReload, withITS bool) []client.Object {
		compName := "mysql"
		fullCompName := constant.GenerateClusterComponentName(clusterName, compName)
		cmpdName := "mysql-cmpd"
		paramsDefName := "mysql-params"
		objects := []client.Object{
			&appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: cmpdName},
				Spec:       appsv1.ComponentDefinitionSpec{ServiceVersion: "8.0"},
				Status:     appsv1.ComponentDefinitionStatus{Phase: appsv1.AvailablePhase},
			},
			&parametersv1alpha1.ParametersDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: paramsDefName},
				Spec: parametersv1alpha1.ParametersDefinitionSpec{
					ComponentDef: cmpdName,
					FileName:     "my.cnf",
					TemplateName: "mysql-config",
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
						Format: parametersv1alpha1.Ini,
						FormatterAction: parametersv1alpha1.FormatterAction{
							IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
						},
					},
				},
				Status: parametersv1alpha1.ParametersDefinitionStatus{Phase: parametersv1alpha1.PDAvailablePhase},
			},
			&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      fullCompName,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: clusterName,
					},
				},
				Spec: appsv1.ComponentSpec{CompDef: cmpdName},
			},
		}
		if withReload {
			objects[1].(*parametersv1alpha1.ParametersDefinition).Spec.ReloadAction = &parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{Command: []string{"bash", "-c", "reload"}},
			}
		}
		if withITS {
			objects = append(objects, &workloads.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      constant.GenerateWorkloadNamePattern(clusterName, compName),
				},
				Spec: workloads.InstanceSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name: "config-manager",
								Ports: []corev1.ContainerPort{{
									Name:          "config-manager",
									ContainerPort: 9901,
								}},
							}},
						},
					},
				},
			})
		}
		return objects
	}

	tests := []struct {
		name       string
		withReload bool
		withITS    bool
		want       bool
	}{
		{name: "no legacy action", withReload: false, withITS: true, want: false},
		{name: "legacy action without existing workload", withReload: true, withITS: false, want: false},
		{name: "legacy action with existing workload", withReload: true, withITS: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(buildObjects(tt.withReload, tt.withITS)...).Build()
			comp := &appsv1.Component{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: constant.GenerateClusterComponentName(clusterName, "mysql")}, comp); err != nil {
				t.Fatalf("get component: %v", err)
			}
			got, err := resolveLegacyConfigManagerRequirement(context.Background(), cli, comp)
			if err != nil {
				t.Fatalf("resolveLegacyConfigManagerRequirement() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveLegacyConfigManagerRequirement() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSyncLegacyConfigManagerRequirement(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = parametersv1alpha1.AddToScheme(scheme)
	_ = workloads.AddToScheme(scheme)

	clusterName := "test-cluster"
	namespace := "default"
	cmpdName := "mysql-cmpd"
	paramsDefName := "mysql-params"

	newComponent := func(name string) *appsv1.Component {
		return &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      constant.GenerateClusterComponentName(clusterName, name),
				Labels: map[string]string{
					constant.AppInstanceLabelKey: clusterName,
				},
			},
			Spec: appsv1.ComponentSpec{CompDef: cmpdName},
		}
	}
	newITS := func(name string) *workloads.InstanceSet {
		return &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      constant.GenerateWorkloadNamePattern(clusterName, name),
			},
			Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: "config-manager",
							Ports: []corev1.ContainerPort{{
								Name:          "config-manager",
								ContainerPort: 9901,
							}},
						}},
					},
				},
			},
		}
	}

	baseObjects := []client.Object{
		&appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: cmpdName},
			Status:     appsv1.ComponentDefinitionStatus{Phase: appsv1.AvailablePhase},
		},
		&parametersv1alpha1.ParametersDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: paramsDefName},
			Spec: parametersv1alpha1.ParametersDefinitionSpec{
				ComponentDef: cmpdName,
				FileName:     "my.cnf",
				TemplateName: "mysql-config",
				FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
					Format: parametersv1alpha1.Ini,
					FormatterAction: parametersv1alpha1.FormatterAction{
						IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
					},
				},
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{Command: []string{"bash", "-c", "reload"}},
				},
			},
			Status: parametersv1alpha1.ParametersDefinitionStatus{Phase: parametersv1alpha1.PDAvailablePhase},
		},
	}

	tests := []struct {
		name          string
		objects       []client.Object
		currentComp   *appsv1.Component
		currentReq    bool
		wantRequired  bool
		wantAnnoValue string
	}{
		{
			name: "set annotation when current component has legacy workload",
			objects: append(append([]client.Object{}, baseObjects...),
				&appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName}},
				newComponent("mysql"),
				newITS("mysql"),
			),
			currentComp:   newComponent("mysql"),
			currentReq:    true,
			wantRequired:  true,
			wantAnnoValue: "true",
		},
		{
			name: "set annotation false when no component still requires legacy runtime",
			objects: append(append([]client.Object{}, baseObjects...),
				func() *appsv1.Cluster {
					cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName}}
					cluster.Annotations = map[string]string{constant.LegacyConfigManagerRequiredAnnotationKey: "true"}
					return cluster
				}(),
				newComponent("mysql"),
			),
			currentComp:   newComponent("mysql"),
			currentReq:    false,
			wantRequired:  false,
			wantAnnoValue: "false",
		},
		{
			name: "write annotation false even when the key is initially missing",
			objects: append(append([]client.Object{}, baseObjects...),
				&appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName}},
				newComponent("mysql"),
			),
			currentComp:   newComponent("mysql"),
			currentReq:    false,
			wantRequired:  false,
			wantAnnoValue: "false",
		},
		{
			name: "keep annotation when another component still requires legacy runtime",
			objects: append(append([]client.Object{}, baseObjects...),
				func() *appsv1.Cluster {
					cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName}}
					cluster.Annotations = map[string]string{constant.LegacyConfigManagerRequiredAnnotationKey: "true"}
					return cluster
				}(),
				newComponent("mysql"),
				newComponent("mysql2"),
				newITS("mysql2"),
			),
			currentComp:   newComponent("mysql"),
			currentReq:    false,
			wantRequired:  true,
			wantAnnoValue: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()
			r := &ComponentDrivenParameterReconciler{Client: cli}
			reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}
			if err := r.syncLegacyConfigManagerRequirement(reqCtx, tt.currentComp, tt.currentReq); err != nil {
				t.Fatalf("syncLegacyConfigManagerRequirement() error = %v", err)
			}
			cluster := &appsv1.Cluster{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: clusterName}, cluster); err != nil {
				t.Fatalf("get cluster: %v", err)
			}
			got, err := parameters.LegacyConfigManagerRequiredForCluster(cluster)
			if err != nil {
				t.Fatalf("LegacyConfigManagerRequiredForCluster() error = %v", err)
			}
			if got != tt.wantRequired {
				t.Fatalf("LegacyConfigManagerRequiredForCluster() = %v, want %v", got, tt.wantRequired)
			}
			if cluster.Annotations[constant.LegacyConfigManagerRequiredAnnotationKey] != tt.wantAnnoValue {
				t.Fatalf("cluster annotation %q = %q, want %q",
					constant.LegacyConfigManagerRequiredAnnotationKey,
					cluster.Annotations[constant.LegacyConfigManagerRequiredAnnotationKey],
					tt.wantAnnoValue)
			}
		})
	}
}
