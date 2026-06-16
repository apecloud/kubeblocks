/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/controllers/parameters/reconfigure"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("Reconfigure Controller", func() {
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("reconfigure policy", func() {
		It("classifies reload action predicates", func() {
			Expect(isValidReloadPolicy(parametersv1alpha1.ReloadAction{})).Should(BeFalse())
			Expect(isValidReloadPolicy(parametersv1alpha1.ReloadAction{
				AutoTrigger: &parametersv1alpha1.AutoTrigger{ProcessName: "mysql"},
			})).Should(BeTrue())
			Expect(isValidReloadPolicy(parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{Command: []string{"reload"}},
			})).Should(BeTrue())

			Expect(isAutoReload(nil)).Should(BeFalse())
			Expect(isAutoReload(&parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{Command: []string{"reload"}},
			})).Should(BeFalse())
			Expect(isAutoReload(&parametersv1alpha1.ReloadAction{
				AutoTrigger: &parametersv1alpha1.AutoTrigger{ProcessName: "mysql"},
			})).Should(BeTrue())
		})

		It("enables sync shell triggers only for synchronous shell actions", func() {
			reconciler := &ReconfigureReconciler{}
			Expect(reconciler.enableSyncTrigger(nil)).Should(BeFalse())
			Expect(reconciler.enableSyncTrigger(&parametersv1alpha1.ReloadAction{
				AutoTrigger: &parametersv1alpha1.AutoTrigger{ProcessName: "mysql"},
			})).Should(BeFalse())
			Expect(reconciler.enableSyncTrigger(&parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{Command: []string{"reload"}},
			})).Should(BeFalse())
			Expect(reconciler.enableSyncTrigger(&parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{Command: []string{"reload"}, Sync: ptr.To(false)},
			})).Should(BeFalse())
			Expect(reconciler.enableSyncTrigger(&parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{Command: []string{"reload"}, Sync: ptr.To(true)},
			})).Should(BeTrue())
		})

		It("builds reload tasks with resolved config context", func() {
			configHashData := map[string]string{
				testparameters.MysqlConfigFile: "max_connections=100",
			}
			configMap := &corev1.ConfigMap{
				Data: configHashData,
			}
			cluster := &appsv1.Cluster{}
			componentSpec := &appsv1.ClusterComponentSpec{Name: defaultCompName}
			its := &workloads.InstanceSet{}
			templateSpec := &appsv1.ComponentFileTemplate{
				Name: configSpecName,
			}
			configDesc := &parametersv1alpha1.ComponentConfigDescription{
				Name: testparameters.MysqlConfigFile,
			}
			pd := &parametersv1alpha1.ParametersDefinition{
				Spec: parametersv1alpha1.ParametersDefinitionSpec{
					DynamicParameters: []string{"max_connections"},
				},
			}
			patch := &parameterscore.ConfigPatchInfo{
				UpdateConfig: map[string][]byte{
					testparameters.MysqlConfigFile: []byte(`{"max_connections":"200"}`),
				},
			}
			rctx := &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{
						ComponentName: defaultCompName,
					},
					ConfigMapObj:  configMap,
					ClusterObj:    cluster,
					ClusterComObj: componentSpec,
				},
				its:       its,
				configMap: configMap,
			}

			task := (&ReconfigureReconciler{}).buildReloadTask(
				reconfigure.SyncDynamicReloadPolicy,
				templateSpec,
				rctx,
				pd,
				configDesc,
				patch,
			)

			Expect(task.Policy).Should(Equal(reconfigure.SyncDynamicReloadPolicy))
			Expect(task.Ctx.ConfigTemplate).Should(Equal(*templateSpec))
			Expect(task.Ctx.ConfigHash).ShouldNot(BeNil())
			Expect(*task.Ctx.ConfigHash).Should(Equal(*computeTargetConfigHash(nil, configHashData)))
			Expect(task.Ctx.Cluster).Should(BeIdenticalTo(cluster))
			Expect(task.Ctx.ClusterComponent).Should(BeIdenticalTo(componentSpec))
			Expect(task.Ctx.ITS).Should(BeIdenticalTo(its))
			Expect(task.Ctx.ConfigDescription).Should(BeIdenticalTo(configDesc))
			Expect(task.Ctx.ParametersDef).Should(BeIdenticalTo(&pd.Spec))
			Expect(task.Ctx.Patch).Should(BeIdenticalTo(patch))
		})

		It("falls back to restart tasks when reload tasks are not applicable", func() {
			reconciler := &ReconfigureReconciler{}
			templateSpec := &appsv1.ComponentFileTemplate{Name: configSpecName}
			rctx := &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{},
				},
				configMap: &corev1.ConfigMap{
					Data: map[string]string{testparameters.MysqlConfigFile: "max_connections=100"},
				},
				configDescs: []parametersv1alpha1.ComponentConfigDescription{{
					Name: testparameters.MysqlConfigFile,
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
						Format: parametersv1alpha1.Ini,
						FormatterAction: parametersv1alpha1.FormatterAction{
							IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
						},
					},
				}},
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{},
			}

			tasks, err := reconciler.buildReconfigureTasks(templateSpec, rctx, nil, false)
			Expect(err).Should(Succeed())
			Expect(tasks).Should(HaveLen(1))
			Expect(tasks[0].Policy).Should(Equal(reconfigure.RestartPolicy))

			tasks, err = reconciler.buildReconfigureTasks(templateSpec, rctx, &parameterscore.ConfigPatchInfo{
				UpdateConfig: map[string][]byte{
					testparameters.MysqlConfigFile: []byte(`{"mysqld":{"max_connections":"200"}}`),
				},
			}, false)
			Expect(err).Should(Succeed())
			Expect(tasks).Should(HaveLen(1))
			Expect(tasks[0].Policy).Should(Equal(reconfigure.RestartPolicy))
		})

		It("builds dynamic reload tasks from supported config patches", func() {
			reconciler := &ReconfigureReconciler{}
			templateSpec := &appsv1.ComponentFileTemplate{
				Name: configSpecName,
				Reconfigure: &appsv1.Action{
					Exec: &appsv1.ExecAction{Command: []string{"reload"}},
				},
			}
			rctx := &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{},
				},
				configMap: &corev1.ConfigMap{
					Data: map[string]string{testparameters.MysqlConfigFile: "max_connections=100"},
				},
				configDescs: []parametersv1alpha1.ComponentConfigDescription{{
					Name: testparameters.MysqlConfigFile,
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
						Format: parametersv1alpha1.Ini,
						FormatterAction: parametersv1alpha1.FormatterAction{
							IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
						},
					},
				}},
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					testparameters.MysqlConfigFile: {
						Spec: parametersv1alpha1.ParametersDefinitionSpec{
							DynamicParameters: []string{"max_connections"},
						},
					},
				},
			}

			tasks, err := reconciler.buildReconfigureTasks(templateSpec, rctx, &parameterscore.ConfigPatchInfo{
				UpdateConfig: map[string][]byte{
					testparameters.MysqlConfigFile: []byte(`{"mysqld":{"max_connections":"200"}}`),
				},
			}, false)
			Expect(err).Should(Succeed())
			Expect(tasks).Should(HaveLen(1))
			Expect(tasks[0].Policy).Should(Equal(reconfigure.SyncDynamicReloadPolicy))
			Expect(tasks[0].Ctx.ConfigDescription.Name).Should(Equal(testparameters.MysqlConfigFile))
		})

		It("applies failed result options", func() {
			status := reconfigure.Status{
				Status:        reconfigure.StatusFailed,
				Reason:        "original",
				ExpectedCount: 2,
				SucceedCount:  1,
			}

			retryResult := reconciled(status, string(reconfigure.SyncDynamicReloadPolicy), parametersv1alpha1.CFailedPhase, withFailed(nil, true))
			Expect(retryResult.Retry).Should(BeTrue())
			Expect(retryResult.Failed).Should(BeFalse())
			Expect(retryResult.Message).Should(Equal("original"))

			pauseResult := reconciled(status, string(reconfigure.SyncDynamicReloadPolicy), parametersv1alpha1.CFailedAndPausePhase, withFailed(errors.New("reload failed"), false))
			Expect(pauseResult.Retry).Should(BeFalse())
			Expect(pauseResult.Failed).Should(BeTrue())
			Expect(pauseResult.Message).Should(Equal("reload failed"))
		})
	})

	Context("reconfigure", func() {
		var (
			configmap                    *corev1.ConfigMap
			clusterObj                   *appsv1.Cluster
			synthesizedComp              *component.SynthesizedComponent
			clusterKey, compParameterKey types.NamespacedName
		)

		BeforeEach(func() {
			configmap, clusterObj, _, synthesizedComp, _ = mockReconcileResource()

			clusterKey = client.ObjectKeyFromObject(clusterObj)

			compParameterKey = types.NamespacedName{
				Namespace: synthesizedComp.Namespace,
				Name:      parameterscore.GenerateComponentConfigurationName(synthesizedComp.ClusterName, synthesizedComp.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, compParameterKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
				g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
				g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(2)))
			})).Should(Succeed())
		})

		It("compute config hash", func() {
			By("compute hash for initial configuration")
			initialHash := computeTargetConfigHash(nil, configmap.Data)
			Expect(initialHash).NotTo(BeNil())
			Expect(*initialHash).NotTo(BeEmpty())

			By("compute hash again for same data should give same result")
			sameHash := computeTargetConfigHash(nil, configmap.Data)
			Expect(sameHash).NotTo(BeNil())
			Expect(*sameHash).To(Equal(*initialHash))

			By("compute hash for different data should give different result")
			modifiedData := make(map[string]string)
			for k, v := range configmap.Data {
				modifiedData[k] = v
			}
			modifiedData["new_key"] = "new_value"
			differentHash := computeTargetConfigHash(nil, modifiedData)
			Expect(differentHash).NotTo(BeNil())
			Expect(*differentHash).NotTo(BeEmpty())
			Expect(*differentHash).NotTo(Equal(*initialHash))
		})

		It("submit changes to cluster", func() {
			By("submit changes through component parameter")
			Eventually(testapps.GetAndChangeObj(&testCtx, compParameterKey, func(compParameter *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&compParameter.Spec, configSpecName)
				item.ConfigFileParams = map[string]parametersv1alpha1.ParametersInFile{
					testparameters.MysqlConfigFile: {
						Parameters: map[string]*string{
							"innodb_buffer_pool_size": ptr.To("1024M"),
							"max_connections":         ptr.To("100"),
						},
					},
				}
			})).Should(Succeed())

			expectedHash := waitRenderedConfigHash(
				testCtx.DefaultNamespace, synthesizedComp.ClusterName, synthesizedComp.Name, configSpecName,
				"innodb_buffer_pool_size=1024M", "max_connections=100",
			)

			By("verify changes submit to cluster")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				for _, comp := range cluster.Spec.ComponentSpecs {
					for _, config := range comp.Configs {
						// g.Expect(config.Variables).Should(HaveKeyWithValue("innodb_buffer_pool_size", "1024M"))
						// g.Expect(config.Variables).Should(HaveKeyWithValue("max_connections", "100"))
						g.Expect(config.Variables).Should(BeNil())
						g.Expect(config.ConfigHash).ShouldNot(BeNil())
						g.Expect(*config.ConfigHash).Should(Equal(expectedHash))
						g.Expect(config.Restart).ShouldNot(BeNil())
						g.Expect(*config.Restart).Should(BeTrue())
						g.Expect(config.Reconfigure).ShouldNot(BeNil())
						g.Expect(*config.Reconfigure).Should(BeFalse())
						g.Expect(config.ReconfigureAction).Should(BeNil())
					}
				}
			})).Should(Succeed())
		})

		It("restart", func() {
			By("mock parameters definition")
			pdKey := types.NamespacedName{
				Namespace: "",
				Name:      paramsDefName,
			}
			Expect(testapps.GetAndChangeObj(&testCtx, pdKey, func(pd *parametersv1alpha1.ParametersDefinition) {
				pd.Spec.ReloadAction = nil // restart
			})()).Should(Succeed())

			By("submit changes through component parameter")
			Eventually(testapps.GetAndChangeObj(&testCtx, compParameterKey, func(compParameter *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&compParameter.Spec, configSpecName)
				item.ConfigFileParams = map[string]parametersv1alpha1.ParametersInFile{
					testparameters.MysqlConfigFile: {
						Parameters: map[string]*string{
							"innodb_buffer_pool_size": ptr.To("1024M"),
							"max_connections":         ptr.To("100"),
						},
					},
				}
			})).Should(Succeed())

			expectedHash := waitRenderedConfigHash(
				testCtx.DefaultNamespace, synthesizedComp.ClusterName, synthesizedComp.Name, configSpecName,
				"innodb_buffer_pool_size=1024M", "max_connections=100",
			)

			By("verify changes submit to cluster")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				for _, comp := range cluster.Spec.ComponentSpecs {
					for _, config := range comp.Configs {
						// g.Expect(config.Variables).Should(HaveKeyWithValue("innodb_buffer_pool_size", "1024M"))
						// g.Expect(config.Variables).Should(HaveKeyWithValue("max_connections", "100"))
						g.Expect(config.Variables).Should(BeNil())
						g.Expect(config.ConfigHash).ShouldNot(BeNil())
						g.Expect(*config.ConfigHash).Should(Equal(expectedHash))
						g.Expect(config.Restart).ShouldNot(BeNil())
						g.Expect(*config.Restart).Should(BeTrue())
						g.Expect(config.Reconfigure).ShouldNot(BeNil())
						g.Expect(*config.Reconfigure).Should(BeFalse())
						g.Expect(config.ReconfigureAction).Should(BeNil())
					}
				}
			})).Should(Succeed())
		})
	})

	Context("phase", func() {
		// TODO: impl
	})
})

func TestValidateLegacyReloadActionSupport(t *testing.T) {
	newParamsDef := func(name string, withReload bool) *parametersv1alpha1.ParametersDefinition {
		pd := &parametersv1alpha1.ParametersDefinition{}
		pd.Name = name
		if withReload {
			pd.Spec.ReloadAction = &parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{
					Command: []string{"bash", "-c", "reload"},
				},
			}
		}
		return pd
	}
	newITS := func(containers ...corev1.Container) *workloads.InstanceSet {
		return &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: containers,
					},
				},
			},
		}
	}

	tests := []struct {
		name    string
		rctx    *reconcileContext
		patch   *parameterscore.ConfigPatchInfo
		wantErr string
	}{
		{
			name: "allow config without reload action",
			rctx: &reconcileContext{
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					"my.cnf": newParamsDef("mysql", false),
				},
			},
			patch: &parameterscore.ConfigPatchInfo{UpdateConfig: map[string][]byte{"my.cnf": []byte(`{}`)}},
		},
		{
			name: "allow existing instance with legacy config manager",
			rctx: &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{ComponentName: "mysql"},
				},
				its: newITS(corev1.Container{
					Name: "config-manager",
					Ports: []corev1.ContainerPort{{
						Name:          "config-manager",
						ContainerPort: 9901,
					}},
				}),
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					"my.cnf": newParamsDef("mysql-params", true),
				},
			},
			patch: &parameterscore.ConfigPatchInfo{UpdateConfig: map[string][]byte{"my.cnf": []byte(`{}`)}},
		},
		{
			name: "reject new instance with legacy reload action",
			rctx: &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{ComponentName: "mysql"},
				},
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					"my.cnf": newParamsDef("mysql-params", true),
				},
			},
			patch:   &parameterscore.ConfigPatchInfo{UpdateConfig: map[string][]byte{"my.cnf": []byte(`{}`)}},
			wantErr: "legacy reloadAction requires an existing instanceSet with config-manager injected",
		},
		{
			name: "reject legacy config manager without port",
			rctx: &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{ComponentName: "mysql"},
				},
				its: newITS(corev1.Container{
					Name: "config-manager",
				}),
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					"my.cnf": newParamsDef("mysql-params", true),
				},
			},
			patch:   &parameterscore.ConfigPatchInfo{UpdateConfig: map[string][]byte{"my.cnf": []byte(`{}`)}},
			wantErr: "legacy config-manager container has no reachable port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLegacyReloadActionSupport(tt.rctx, tt.patch)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidateLegacyReloadActionSupportWithClusterAnnotation(t *testing.T) {
	newParamsDef := func(name string, withReload bool) *parametersv1alpha1.ParametersDefinition {
		pd := &parametersv1alpha1.ParametersDefinition{}
		pd.Name = name
		if withReload {
			pd.Spec.ReloadAction = &parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{
					Command: []string{"bash", "-c", "reload"},
				},
			}
		}
		return pd
	}
	newITS := func(containers ...corev1.Container) *workloads.InstanceSet {
		return &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: containers,
					},
				},
			},
		}
	}
	newCluster := func(annotationValue string) *appsv1.Cluster {
		cluster := &appsv1.Cluster{}
		if annotationValue == "" {
			return cluster
		}
		cluster.Annotations = map[string]string{
			constant.LegacyConfigManagerRequiredAnnotationKey: annotationValue,
		}
		return cluster
	}

	tests := []struct {
		name    string
		rctx    *reconcileContext
		patch   *parameterscore.ConfigPatchInfo
		wantErr string
	}{
		{
			name: "allow config without reload action",
			rctx: &reconcileContext{
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					"my.cnf": newParamsDef("mysql", false),
				},
			},
			patch: &parameterscore.ConfigPatchInfo{UpdateConfig: map[string][]byte{"my.cnf": []byte(`{}`)}},
		},
		{
			name: "allow existing instance with legacy config manager and cluster marker",
			rctx: &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{ComponentName: "mysql"},
					ClusterObj:  newCluster("true"),
				},
				its: newITS(corev1.Container{
					Name: "config-manager",
					Ports: []corev1.ContainerPort{{
						Name:          "config-manager",
						ContainerPort: 9901,
					}},
				}),
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					"my.cnf": newParamsDef("mysql-params", true),
				},
			},
			patch: &parameterscore.ConfigPatchInfo{UpdateConfig: map[string][]byte{"my.cnf": []byte(`{}`)}},
		},
		{
			name: "reject when cluster annotation is not enabled",
			rctx: &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{ComponentName: "mysql"},
					ClusterObj:  newCluster("false"),
				},
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					"my.cnf": newParamsDef("mysql-params", true),
				},
			},
			patch:   &parameterscore.ConfigPatchInfo{UpdateConfig: map[string][]byte{"my.cnf": []byte(`{}`)}},
			wantErr: `cluster annotation "parameters.kubeblocks.io/legacy-config-manager-required" is not enabled`,
		},
		{
			name: "reject legacy config manager without port",
			rctx: &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{ComponentName: "mysql"},
					ClusterObj:  newCluster("true"),
				},
				its: newITS(corev1.Container{
					Name: "config-manager",
				}),
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					"my.cnf": newParamsDef("mysql-params", true),
				},
			},
			patch:   &parameterscore.ConfigPatchInfo{UpdateConfig: map[string][]byte{"my.cnf": []byte(`{}`)}},
			wantErr: "legacy config-manager container has no reachable port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLegacyReloadActionSupport(tt.rctx, tt.patch)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidateLegacyReloadActionSupportWithUnknownClusterAnnotation(t *testing.T) {
	newParamsDef := func(name string) *parametersv1alpha1.ParametersDefinition {
		pd := &parametersv1alpha1.ParametersDefinition{}
		pd.Name = name
		pd.Spec.ReloadAction = &parametersv1alpha1.ReloadAction{
			ShellTrigger: &parametersv1alpha1.ShellTrigger{
				Command: []string{"bash", "-c", "reload"},
			},
		}
		return pd
	}
	newITS := func(containers ...corev1.Container) *workloads.InstanceSet {
		return &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: containers,
					},
				},
			},
		}
	}

	tests := []struct {
		name    string
		its     *workloads.InstanceSet
		wantErr string
	}{
		{
			name: "allow existing runtime during upgrade race when annotation is missing",
			its: newITS(corev1.Container{
				Name: "config-manager",
				Ports: []corev1.ContainerPort{{
					Name:          "config-manager",
					ContainerPort: 9901,
				}},
			}),
		},
		{
			name:    "reject when annotation is missing and runtime is absent",
			wantErr: "legacy reloadAction requires an existing instanceSet with config-manager injected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rctx := &reconcileContext{
				ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
					ResourceCtx: &render.ResourceCtx{ComponentName: "mysql"},
					ClusterObj:  &appsv1.Cluster{},
				},
				its: tt.its,
				parametersDefs: map[string]*parametersv1alpha1.ParametersDefinition{
					"my.cnf": newParamsDef("mysql-params"),
				},
			}
			err := validateLegacyReloadActionSupport(rctx, &parameterscore.ConfigPatchInfo{UpdateConfig: map[string][]byte{"my.cnf": []byte(`{}`)}})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestNeedRestartAllowsTemplateReconfigure(t *testing.T) {
	patch := &parameterscore.ConfigPatchInfo{
		UpdateConfig: map[string][]byte{
			"my.cnf": []byte(`{"mysqld":{"binlog_expire_logs_seconds":432000}}`),
		},
	}
	paramsDefs := map[string]*parametersv1alpha1.ParametersDefinition{
		"my.cnf": {
			Spec: parametersv1alpha1.ParametersDefinitionSpec{},
		},
	}
	configSpec := &appsv1.ComponentFileTemplate{
		Name: "mysql-replication-config",
		Reconfigure: &appsv1.Action{
			Exec: &appsv1.ExecAction{Command: []string{"bash", "-c", "reload"}},
		},
	}

	if got := needRestart(paramsDefs, patch, configSpec); got {
		t.Fatalf("expected template reconfigure action to avoid forced restart")
	}
}

func TestResolveReconfigurePolicyUsesTemplateReconfigureForDynamicUpdate(t *testing.T) {
	r := &ReconfigureReconciler{}
	pd := &parametersv1alpha1.ParametersDefinitionSpec{
		DynamicParameters: []string{"binlog_expire_logs_seconds"},
	}
	configSpec := &appsv1.ComponentFileTemplate{
		Name: "mysql-replication-config",
		Reconfigure: &appsv1.Action{
			Exec: &appsv1.ExecAction{Command: []string{"bash", "-c", "reload"}},
		},
	}

	policy, err := r.resolveReconfigurePolicy(
		`{"mysqld":{"binlog_expire_logs_seconds":432000}}`,
		&parametersv1alpha1.FileFormatConfig{
			Format: parametersv1alpha1.Ini,
			FormatterAction: parametersv1alpha1.FormatterAction{
				IniConfig: &parametersv1alpha1.IniConfig{
					SectionName: "mysqld",
				},
			},
		},
		pd,
		configSpec,
	)
	if err != nil {
		t.Fatalf("resolveReconfigurePolicy returned error: %v", err)
	}
	if policy != reconfigure.SyncDynamicReloadPolicy {
		t.Fatalf("expected %q, got %q", reconfigure.SyncDynamicReloadPolicy, policy)
	}
}

func TestResolveReconfigurePolicyUsesTemplateReconfigureForStaticReloadBeforeRestart(t *testing.T) {
	r := &ReconfigureReconciler{}
	pd := &parametersv1alpha1.ParametersDefinitionSpec{
		ReloadStaticParamsBeforeRestart: ptr.To(true),
	}
	configSpec := &appsv1.ComponentFileTemplate{
		Name: "mysql-replication-config",
		Reconfigure: &appsv1.Action{
			Exec: &appsv1.ExecAction{Command: []string{"bash", "-c", "reload"}},
		},
	}

	policy, err := r.resolveReconfigurePolicy(
		`{"mysqld":{"performance_schema":"ON"}}`,
		&parametersv1alpha1.FileFormatConfig{
			Format: parametersv1alpha1.Ini,
			FormatterAction: parametersv1alpha1.FormatterAction{
				IniConfig: &parametersv1alpha1.IniConfig{
					SectionName: "mysqld",
				},
			},
		},
		pd,
		configSpec,
	)
	if err != nil {
		t.Fatalf("resolveReconfigurePolicy returned error: %v", err)
	}
	if policy != reconfigure.DynamicReloadAndRestartPolicy {
		t.Fatalf("expected %q, got %q", reconfigure.DynamicReloadAndRestartPolicy, policy)
	}
}

func TestResolveReconfigurePolicyUsesTemplateReconfigureForSplitMixedUpdate(t *testing.T) {
	r := &ReconfigureReconciler{}
	pd := &parametersv1alpha1.ParametersDefinitionSpec{
		DynamicParameters:     []string{"binlog_expire_logs_seconds"},
		MergeReloadAndRestart: ptr.To(false),
	}
	configSpec := &appsv1.ComponentFileTemplate{
		Name: "mysql-replication-config",
		Reconfigure: &appsv1.Action{
			Exec: &appsv1.ExecAction{Command: []string{"bash", "-c", "reload"}},
		},
	}

	policy, err := r.resolveReconfigurePolicy(
		`{"mysqld":{"binlog_expire_logs_seconds":"432000","performance_schema":"ON"}}`,
		&parametersv1alpha1.FileFormatConfig{
			Format: parametersv1alpha1.Ini,
			FormatterAction: parametersv1alpha1.FormatterAction{
				IniConfig: &parametersv1alpha1.IniConfig{
					SectionName: "mysqld",
				},
			},
		},
		pd,
		configSpec,
	)
	if err != nil {
		t.Fatalf("resolveReconfigurePolicy returned error: %v", err)
	}
	if policy != reconfigure.DynamicReloadAndRestartPolicy {
		t.Fatalf("expected %q, got %q", reconfigure.DynamicReloadAndRestartPolicy, policy)
	}
}

func TestResolveReconfigurePolicyKeepsStaticOnlyUpdateAsRestartWhenMergeIsDisabled(t *testing.T) {
	r := &ReconfigureReconciler{}
	pd := &parametersv1alpha1.ParametersDefinitionSpec{
		DynamicParameters:     []string{"binlog_expire_logs_seconds"},
		MergeReloadAndRestart: ptr.To(false),
	}
	configSpec := &appsv1.ComponentFileTemplate{
		Name: "mysql-replication-config",
		Reconfigure: &appsv1.Action{
			Exec: &appsv1.ExecAction{Command: []string{"bash", "-c", "reload"}},
		},
	}

	policy, err := r.resolveReconfigurePolicy(
		`{"mysqld":{"table_open_cache_instances":"8"}}`,
		&parametersv1alpha1.FileFormatConfig{
			Format: parametersv1alpha1.Ini,
			FormatterAction: parametersv1alpha1.FormatterAction{
				IniConfig: &parametersv1alpha1.IniConfig{
					SectionName: "mysqld",
				},
			},
		},
		pd,
		configSpec,
	)
	if err != nil {
		t.Fatalf("resolveReconfigurePolicy returned error: %v", err)
	}
	if policy != reconfigure.RestartPolicy {
		t.Fatalf("expected %q, got %q", reconfigure.RestartPolicy, policy)
	}
}
