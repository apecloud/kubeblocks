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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/controllers/parameters/reconfigure"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/operations"
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
			configMap.Name = "reload-config"
			cluster := &appsv1.Cluster{}
			componentSpec := &appsv1.ClusterComponentSpec{Name: defaultCompName}
			comp := &appsv1.Component{}
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
					ComponentObj:  comp,
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
			Expect(task.Ctx.ConfigMapName).Should(Equal(configMap.Name))
			Expect(task.Ctx.ConfigTemplate).Should(Equal(*templateSpec))
			Expect(task.Ctx.ConfigHash).ShouldNot(BeNil())
			Expect(*task.Ctx.ConfigHash).Should(Equal(*computeTargetConfigHash(nil, configHashData)))
			Expect(task.Ctx.Cluster).Should(BeIdenticalTo(cluster))
			Expect(task.Ctx.ClusterComponent).Should(BeIdenticalTo(componentSpec))
			Expect(task.Ctx.Component).Should(BeIdenticalTo(comp))
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
				g.Expect(cluster.Spec.GetComponentByName(defaultCompName).Configs).Should(HaveLen(1))
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
				g.Expect(cluster.Spec.GetComponentByName(defaultCompName).Configs).Should(HaveLen(1))
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

		It("keeps changed shard config targets and completion independent", func() {
			pdKey := client.ObjectKey{Name: paramsDefName}
			Expect(testapps.GetAndChangeObj(&testCtx, pdKey, func(pd *parametersv1alpha1.ParametersDefinition) {
				pd.Spec.ReloadAction = nil
				pd.Spec.DynamicParameters = []string{"max_connections"}
			})()).Should(Succeed())
			definition := &appsv1.ComponentDefinition{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: synthesizedComp.CompDefName}, definition)).Should(Succeed())
			configSpec := definition.Spec.Configs[0].DeepCopy()
			configSpec.Reconfigure = &appsv1.Action{Exec: &appsv1.ExecAction{Command: []string{"reload"}}}
			oldData, err := json.Marshal(map[string]string{testparameters.MysqlConfigFile: "[mysqld]\nmax_connections=100\n"})
			Expect(err).Should(Succeed())
			recorder := record.NewFakeRecorder(20)
			reconciler := &ReconfigureReconciler{Client: k8sClient, Recorder: recorder}
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx, Log: logr.Discard(), Recorder: recorder}
			components := make([]*appsv1.Component, 2)
			sets := make([]*workloads.InstanceSet, 2)
			configMaps := make([]*corev1.ConfigMap, 2)
			hashes := make([]string, 2)
			checksums := make([]string, 2)
			for i := range components {
				name := fmt.Sprintf("%s-%c", shardingCompName, 'a'+i)
				fullName := constant.GenerateClusterComponentName(clusterObj.Name, name)
				components[i] = testapps.NewComponentFactory(clusterObj.Namespace, fullName, definition.Name).
					AddLabels(constant.KBAppShardingNameLabelKey, shardingCompName).
					SetReplicas(int32(i + 1)).Create(&testCtx).GetObject()
				sets[i] = mockCreateITSObject(clusterObj.Namespace, fullName, clusterObj.Name, name)
				configMaps[i] = testparameters.NewComponentTemplateFactory(
					parameterscore.GetComponentCfgName(clusterObj.Name, name, configSpecName), clusterObj.Namespace).
					AddLabels(constant.AppInstanceLabelKey, clusterObj.Name, constant.KBAppComponentLabelKey, name).
					AddAnnotations(constant.ConfigurationRevision, "2", constant.LastAppliedConfigAnnotationKey, string(oldData)).
					Create(&testCtx).GetObject()
				configMaps[i].Data = map[string]string{testparameters.MysqlConfigFile: fmt.Sprintf("[mysqld]\nmax_connections=%d\n", 200+i*100)}
				Expect(k8sClient.Update(ctx, configMaps[i])).Should(Succeed())
				hashes[i] = *computeTargetConfigHash(nil, configMaps[i].Data)
				_, err := reconciler.sync(newReconcileContext(reqCtx, k8sClient, configMaps[i]), configSpec, false)
				Expect(err).Should(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(components[i]), components[i])).Should(Succeed())
				Expect(components[i].Spec.Configs).Should(HaveLen(1))
				target := components[i].Spec.Configs[0]
				Expect(target.ConfigMap.Name).Should(Equal(configMaps[i].Name))
				Expect(target.ConfigHash).Should(HaveValue(Equal(hashes[i])))
				Expect(target.Restart).Should(HaveValue(BeFalse()))
				Expect(target.Reconfigure).Should(HaveValue(BeTrue()))
				Expect(target.ReconfigureAction).Should(BeNil())
				Expect(target.ReconfigureArgs).Should(Equal([][]string{{"max_connections", fmt.Sprint(200 + i*100)}}))
				Expect(target.Variables).Should(HaveKey("KB_CONFIG_FILES_UPDATED"))
				checksums[i] = target.Variables["KB_CONFIG_FILES_UPDATED"]
			}
			Expect(hashes[0]).ShouldNot(Equal(hashes[1]))
			Expect(checksums[0]).ShouldNot(Equal(checksums[1]))
			currentCluster := &appsv1.Cluster{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObj), currentCluster)).Should(Succeed())
			Expect(currentCluster.Spec).Should(Equal(clusterObj.Spec))

			observe := func(index int, observedHashes ...string) {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sets[index]), sets[index])).Should(Succeed())
				sets[index].Status.InstanceStatus = nil
				for ordinal, hash := range observedHashes {
					sets[index].Status.InstanceStatus = append(sets[index].Status.InstanceStatus, workloads.InstanceStatus{
						PodName: fmt.Sprintf("%s-%d", sets[index].Name, ordinal),
						Configs: []workloads.InstanceConfigStatus{{Name: configSpecName, ConfigHash: ptr.To(hash)}},
					})
				}
				Expect(k8sClient.Status().Update(ctx, sets[index])).Should(Succeed())
			}
			reconcilePhase := func(index int, phase parametersv1alpha1.ParameterPhase, count int32) {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configMaps[index]), configMaps[index])).Should(Succeed())
				_, err := reconciler.sync(newReconcileContext(reqCtx, k8sClient, configMaps[index]), configSpec, false)
				Expect(err).Should(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configMaps[index]), configMaps[index])).Should(Succeed())
				result := parseResult(configMaps[index].Annotations[parameterscore.GenerateRevisionPhaseKey("2")], "2")
				Expect(result.Phase).Should(Equal(phase))
				Expect(result.SucceedCount).Should(Equal(count))
				Expect(result.ExpectedCount).Should(Equal(components[index].Spec.Replicas))
			}
			observe(0, hashes[0])
			observe(1, hashes[0], hashes[1])
			reconcilePhase(0, parametersv1alpha1.CFinishedPhase, 1)
			reconcilePhase(1, parametersv1alpha1.CUpgradingPhase, 1)
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configMaps[0]), configMaps[0])).Should(Succeed())
			Expect(parseResult(configMaps[0].Annotations[parameterscore.GenerateRevisionPhaseKey("2")], "2").Phase).
				Should(Equal(parametersv1alpha1.CFinishedPhase))
			observe(1, hashes[1], hashes[1])
			reconcilePhase(1, parametersv1alpha1.CFinishedPhase, 2)
		})
	})

	Context("restored configuration target recovery", func() {
		for _, tc := range []struct {
			name, restored string
			recover        bool
			conflict       bool
			newRevision    bool
			finished       bool
			stopped        bool
			noRuntime      bool
			unbound        bool
			foreign        bool
			foreignActual  bool
			currentActual  bool
			replicas       int32
		}{
			{name: "restored content", restored: "enabled=ON\n", recover: true},
			{name: "same name from different source semantic no-op", restored: "# other source\nenabled=ON\n", foreign: true},
			{name: "restored unbound source", restored: "enabled=ON\n", recover: true, unbound: true},
			{name: "ordinary component replica override", restored: "enabled=ON\n", recover: true, replicas: 2},
			{name: "completed ordinary target stale while actual component hash is current", restored: "enabled=ON\n", recover: true, finished: true, currentActual: true},
			{name: "unbound ordinary target from different actual source", restored: "# other source\nenabled=ON\n", foreignActual: true},
			{name: "restored semantics", restored: "# restored\nenabled=ON\n", recover: true},
			{name: "stable formatting change", restored: "# reformatted\nenabled=ON\n"},
			{name: "cluster conflict", restored: "enabled=ON\n", recover: true, conflict: true},
			{name: "newer revision", restored: "# restored\nenabled=ON\n", recover: true, newRevision: true},
			{name: "completed stopped configuration", restored: "enabled=ON\n", finished: true, stopped: true, noRuntime: true},
			{name: "completed unavailable configuration", restored: "enabled=ON\n", finished: true, noRuntime: true},
			{name: "completed stale configuration", restored: "enabled=ON\n", finished: true, recover: true},
		} {
			It(tc.name, func() {
				ctx := context.Background()
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, parametersv1alpha1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme} {
					Expect(add(scheme)).Should(Succeed())
				}
				const namespace, clusterName, componentName, configName = "default", "mysql", "mysql", "mysql-config"
				applied, err := json.Marshal(map[string]string{"my.cnf": "enabled=ON\n"})
				Expect(err).Should(Succeed())
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: parameterscore.GetComponentCfgName(clusterName, componentName, configName), Namespace: namespace,
						Labels: map[string]string{
							constant.AppInstanceLabelKey: clusterName, constant.KBAppComponentLabelKey: componentName,
							constant.CMConfigurationTemplateNameLabelKey: configName,
							constant.CMConfigurationTypeLabelKey:         constant.ConfigInstanceType,
							constant.CMConfigurationSpecProviderLabelKey: configName,
						},
						Annotations: map[string]string{constant.LastAppliedConfigAnnotationKey: string(applied), constant.ConfigurationRevision: "3"},
					},
					Data: map[string]string{"my.cnf": tc.restored},
				}
				failedHash := computeTargetConfigHash(nil, map[string]string{"my.cnf": "enabled=nonsense\n"})
				appliedHash := computeTargetConfigHash(nil, map[string]string{"my.cnf": "enabled=ON\n"})
				targetHash := appliedHash
				targetValue := "ON"
				if tc.recover {
					targetHash = failedHash
					targetValue = "nonsense"
				}
				cluster := &appsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace},
					Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
						Name: componentName, Replicas: 1,
						Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To(configName), ConfigHash: targetHash,
							Reconfigure: ptr.To(true), ReconfigureArgs: [][]string{{"enabled", targetValue}}}},
					}}},
				}
				otherConfig := appsv1.ClusterComponentConfig{Name: ptr.To("other"), ConfigHash: ptr.To("other-hash"),
					Reconfigure: ptr.To(true), ReconfigureArgs: [][]string{{"other", "value"}}}
				cluster.Spec.ComponentSpecs[0].Configs = append(cluster.Spec.ComponentSpecs[0].Configs, otherConfig)
				template := appsv1.ComponentFileTemplate{Name: configName, Template: "template", Namespace: namespace, ExternalManaged: ptr.To(true)}
				cp := &parametersv1alpha1.ComponentParameter{
					ObjectMeta: metav1.ObjectMeta{Name: parameterscore.GenerateComponentConfigurationName(clusterName, componentName), Namespace: namespace, Generation: 3, Finalizers: []string{constant.ConfigFinalizerName}},
					Spec: parametersv1alpha1.ComponentParameterSpec{ClusterName: clusterName, ComponentName: componentName,
						ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{Name: configName, ConfigSpec: &template}}},
					Status: parametersv1alpha1.ComponentParameterStatus{ConfigurationItemStatus: []parametersv1alpha1.ConfigTemplateItemDetailStatus{{Name: configName, Phase: parametersv1alpha1.CRunningPhase, UpdateRevision: "3"}}},
				}
				item, err := json.Marshal(cp.Spec.ConfigItemDetails[0])
				Expect(err).Should(Succeed())
				cm.Annotations[constant.ConfigAppliedVersionAnnotationKey] = string(item)
				cm.Annotations[constant.ParametersAppliedComponentGenerationKey] = "1"
				component := &appsv1.Component{
					ObjectMeta: metav1.ObjectMeta{Name: constant.GenerateClusterComponentName(clusterName, componentName), Namespace: namespace, Generation: 1,
						Labels: constant.GetCompLabels(clusterName, componentName), Annotations: map[string]string{constant.KBAppClusterUIDKey: "cluster-uid"}},
					Spec: appsv1.ComponentSpec{CompDef: "mysql", Replicas: 1, Configs: []appsv1.ClusterComponentConfig{*cluster.Spec.ComponentSpecs[0].Configs[0].DeepCopy()}},
				}
				cluster.Spec.ComponentSpecs[0].Configs[0].Variables = map[string]string{"user-variable": "kept", constant.ConfigFilesUpdated: "old-checksum"}
				component.Spec.Configs[0].ConfigMap = &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name}}
				if tc.foreign {
					cluster.Spec.ComponentSpecs[0].Configs[0].ConfigMap = &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "another-configmap"}}
				}
				if tc.foreignActual {
					component.Spec.Configs[0].ConfigMap.Name = "another-configmap"
				}
				if tc.currentActual {
					component.Spec.Configs[0].ConfigHash = appliedHash
				}
				if tc.unbound {
					component.Spec.Configs[0].ConfigMap = nil
				}
				if tc.replicas > 0 {
					component.Spec.Replicas = tc.replicas
				}
				component.Spec.Configs[0].Variables = map[string]string{"user-variable": "kept", constant.ConfigFilesUpdated: "old-checksum"}
				definition := &appsv1.ComponentDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
					Spec:       appsv1.ComponentDefinitionSpec{Configs: []appsv1.ComponentFileTemplate{template}},
					Status:     appsv1.ComponentDefinitionStatus{Phase: appsv1.AvailablePhase},
				}
				pd := &parametersv1alpha1.ParametersDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
					Spec: parametersv1alpha1.ParametersDefinitionSpec{ComponentDef: "mysql", TemplateName: configName, FileName: "my.cnf",
						FileFormatConfig: &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Ini}},
					Status: parametersv1alpha1.ParametersDefinitionStatus{Phase: parametersv1alpha1.PDAvailablePhase},
				}
				its := &workloads.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{Name: constant.GenerateWorkloadNamePattern(clusterName, componentName), Namespace: namespace},
					Spec:       workloads.InstanceSetSpec{Replicas: ptr.To(int32(1)), Configs: []workloads.ConfigTemplate{{Name: configName, ConfigHash: targetHash}}},
					Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
						Configs: []workloads.InstanceConfigStatus{{Name: configName, ConfigHash: appliedHash}},
					}}},
				}
				if tc.stopped {
					cluster.Spec.ComponentSpecs[0].Stop = ptr.To(true)
					its.Spec.Stop = ptr.To(true)
				}
				if tc.noRuntime {
					its.Status.InstanceStatus = nil
				}
				if tc.finished {
					finished := unReconciled(parametersv1alpha1.CFinishedPhase, "", "already applied")
					finished.Revision = "3"
					data, err := json.Marshal(finished)
					Expect(err).Should(Succeed())
					cm.Annotations[parameterscore.GenerateRevisionPhaseKey("3")] = string(data)
					cp.Status.Phase = parametersv1alpha1.CFinishedPhase
					cp.Status.ObservedGeneration = cp.Generation
					cp.Status.ConfigurationItemStatus[0].Phase = parametersv1alpha1.CFinishedPhase
				}
				ops := &opsv1alpha1.OpsRequest{
					ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: namespace},
					Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: clusterName, Type: opsv1alpha1.ReconfiguringType},
					Status:     opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase},
				}
				ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: componentName}}}
				tpl := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: template.Template, Namespace: namespace}, Data: cm.Data}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(cp, ops, its).
					WithObjects(cm, cluster, cp, component, definition, pd, its, ops, tpl).Build()
				reconciler := &ReconfigureReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(10)}
				originalComponent := component.DeepCopy()
				if tc.foreign || tc.foreignActual {
					originalCluster := cluster.DeepCopy()
					original := component.DeepCopy()
					_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
					Expect(err).Should(Succeed())
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(component), component)).Should(Succeed())
					Expect(component.Spec).Should(Equal(original.Spec))
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
					Expect(cluster.Spec).Should(Equal(originalCluster.Spec))
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).Should(Succeed())
					Expect(parseResult(cm.Annotations[parameterscore.GenerateRevisionPhaseKey("3")], "3").Phase).Should(Equal(parametersv1alpha1.CFinishedPhase))
					return
				}
				if tc.noRuntime {
					originalSpec := cluster.DeepCopy().Spec
					for range 2 {
						result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
						Expect(err).Should(Succeed())
						Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).Should(Succeed())
						status := parametersv1alpha1.ConfigTemplateItemDetailStatus{UpdateRevision: "3"}
						Expect(syncStatus(cm, &status)).Should(Succeed())
						Expect(status.Phase).Should(Equal(parametersv1alpha1.CFinishedPhase))
						Expect(result.RequeueAfter).Should(BeZero())
						cpReconciler := &ComponentParameterReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(10)}
						_, err = cpReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cp)})
						Expect(err).Should(Succeed())
						Expect(cli.Get(ctx, client.ObjectKeyFromObject(cp), cp)).Should(Succeed())
						Expect(cp.Status.Phase).Should(Equal(parametersv1alpha1.CFinishedPhase))
						Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
						Expect(cluster.Spec).Should(Equal(originalSpec))
						Expect(cm.Annotations).Should(HaveKeyWithValue(constant.LastAppliedConfigAnnotationKey, string(applied)))
					}
					return
				}
				if tc.conflict {
					conflicted := false
					reconciler.Client = interceptor.NewClient(cli, interceptor.Funcs{
						Update: func(ctx context.Context, underlying client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							if _, ok := obj.(*appsv1.Cluster); ok && !conflicted {
								conflicted = true
								concurrent := &appsv1.Cluster{}
								if err := underlying.Get(ctx, client.ObjectKeyFromObject(obj), concurrent); err != nil {
									return err
								}
								if concurrent.Annotations == nil {
									concurrent.Annotations = map[string]string{}
								}
								concurrent.Annotations["concurrent"] = "kept"
								if err := underlying.Update(ctx, concurrent); err != nil {
									return err
								}
							}
							return underlying.Update(ctx, obj, opts...)
						},
					})
					result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
					Expect(err).Should(Succeed())
					Expect(result.RequeueAfter).Should(BeNumerically(">", 0))
					Expect(conflicted).Should(BeTrue())
				}
				_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
				Expect(err).Should(Succeed())
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).Should(Succeed())
				status := parametersv1alpha1.ConfigTemplateItemDetailStatus{UpdateRevision: "3"}
				Expect(syncStatus(cm, &status)).Should(Succeed())
				Expect(status.Phase).ShouldNot(Equal(parametersv1alpha1.CFinishedPhase))
				Expect(parseResult(cm.Annotations[parameterscore.GenerateRevisionPhaseKey("3")], "3").ExpectedCount).Should(Equal(cluster.Spec.ComponentSpecs[0].Replicas))
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(component), component)).Should(Succeed())
				Expect(component.Spec).Should(Equal(originalComponent.Spec))
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
				cfg := cluster.Spec.ComponentSpecs[0].Configs[0]
				Expect(cluster.Spec.ComponentSpecs[0].Configs[1]).Should(Equal(otherConfig))
				if tc.conflict {
					Expect(cluster.Annotations).Should(HaveKeyWithValue("concurrent", "kept"))
				}
				if tc.recover {
					Expect(cfg.ConfigHash).ShouldNot(Equal(failedHash))
				}
				Expect(cfg.ConfigHash).Should(Equal(computeTargetConfigHash(nil, cm.Data)))
				if !tc.recover {
					_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
					Expect(err).Should(Succeed())
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
					Expect(ptr.Deref(cluster.Spec.ComponentSpecs[0].Configs[0].Restart, false)).Should(BeFalse())
				}
				Expect(cfg.Restart).Should(HaveValue(BeFalse()))
				Expect(cfg.Reconfigure).Should(HaveValue(BeFalse()))
				Expect(cfg.ReconfigureArgs).Should(BeNil())
				Expect(cfg.ReconfigureAction).Should(BeNil())
				Expect(cfg.ConfigMap).Should(BeNil())
				Expect(cfg.Variables).Should(Equal(map[string]string{"user-variable": "kept"}))
				checkCompletion := func(finished bool) {
					GinkgoHelper()
					_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
					Expect(err).Should(Succeed())
					cpReconciler := &ComponentParameterReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(10)}
					_, err = cpReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cp)})
					Expect(err).Should(Succeed())
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(cp), cp)).Should(Succeed())
					Expect(cp.Status.Phase == parametersv1alpha1.CFinishedPhase).Should(Equal(finished))
					resource := &operations.OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(10)}
					phase, _, err := operations.GetOpsManager().OpsMap[opsv1alpha1.ReconfiguringType].OpsHandler.ReconcileAction(
						intctrlutil.RequestCtx{Ctx: ctx}, cli, resource)
					Expect(err).Should(Succeed())
					Expect(phase == opsv1alpha1.OpsSucceedPhase).Should(Equal(finished))
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(ops), ops)).Should(Succeed())
					Expect(ops.Status.Phase).Should(Equal(phase))
				}
				checkCompletion(false)
				its.Spec.Configs = []workloads.ConfigTemplate{{Name: configName, ConfigHash: cfg.ConfigHash}}
				Expect(cli.Update(ctx, its)).Should(Succeed())
				if tc.restored != "enabled=ON\n" {
					checkCompletion(false)
				}
				its.Status.InstanceStatus[0].Configs[0].ConfigHash = cfg.ConfigHash
				Expect(cli.Status().Update(ctx, its)).Should(Succeed())
				if tc.newRevision {
					changed := false
					reconciler.Client = interceptor.NewClient(cli, interceptor.Funcs{
						Patch: func(ctx context.Context, underlying client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
							if _, ok := obj.(*corev1.ConfigMap); ok && !changed {
								changed = true
								latest := &corev1.ConfigMap{}
								if err := underlying.Get(ctx, client.ObjectKeyFromObject(obj), latest); err != nil {
									return err
								}
								latest.Data["my.cnf"] = "enabled=OFF\n"
								latest.Annotations[constant.ConfigurationRevision] = "4"
								if err := underlying.Update(ctx, latest); err != nil {
									return err
								}
							}
							return underlying.Patch(ctx, obj, patch, opts...)
						},
					})
					result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
					Expect(err).Should(Succeed())
					Expect(result.RequeueAfter).Should(BeNumerically(">", 0))
					Expect(changed).Should(BeTrue())
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).Should(Succeed())
					Expect(cm.Annotations).Should(HaveKeyWithValue(constant.LastAppliedConfigAnnotationKey, string(applied)))
					_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
					Expect(err).Should(Succeed())
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
					Expect(cluster.Spec.ComponentSpecs[0].Configs[0].ConfigHash).Should(Equal(computeTargetConfigHash(nil, cm.Data)))
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(component), component)).Should(Succeed())
					Expect(component.Spec).Should(Equal(originalComponent.Spec))
					return
				}
				checkCompletion(true)
			})
		}
	})

	Context("unchanged shard configurations", func() {
		const namespace, clusterName, shardingName, configName = "default", "mysql", "db", "mysql-config"
		var (
			ctx        context.Context
			cli        client.Client
			cluster    *appsv1.Cluster
			configMaps []*corev1.ConfigMap
			sets       []*workloads.InstanceSet
			components []*appsv1.Component
			reconciler *ReconfigureReconciler
		)

		setPhase := func(cm *corev1.ConfigMap, phase parametersv1alpha1.ParameterPhase) {
			GinkgoHelper()
			result := unReconciled(phase, "", "existing configuration operation")
			result.Revision = "3"
			data, err := json.Marshal(result)
			Expect(err).Should(Succeed())
			cm.Annotations[parameterscore.GenerateRevisionPhaseKey("3")] = string(data)
		}

		BeforeEach(func() {
			ctx = context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, parametersv1alpha1.AddToScheme, workloads.AddToScheme} {
				Expect(add(scheme)).Should(Succeed())
			}
			previous := map[string]string{"my.cnf": "enabled=ON\n"}
			applied, err := json.Marshal(previous)
			Expect(err).Should(Succeed())
			sharedHash := computeTargetConfigHash(nil, previous)
			template := appsv1.ComponentFileTemplate{Name: configName, Template: "template", Namespace: namespace, ExternalManaged: ptr.To(true)}
			cluster = &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace},
				Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
					Name: shardingName, Shards: 2,
					Template: appsv1.ClusterComponentSpec{ComponentDef: "mysql", Replicas: 1,
						Configs: []appsv1.ClusterComponentConfig{
							{Name: ptr.To(configName), ConfigHash: sharedHash, Restart: ptr.To(false),
								Reconfigure: ptr.To(true), ReconfigureArgs: [][]string{{"enabled", "ON"}},
								Variables:         map[string]string{"user-variable": "kept", "KB_CONFIG_FILES_UPDATED": "/config/my.cnf:checksum"},
								ReconfigureAction: &appsv1.Action{Exec: &appsv1.ExecAction{Command: []string{"reload"}}}},
							{Name: ptr.To("other"), ConfigHash: ptr.To("other-hash"), Restart: ptr.To(true)},
						}},
				}}},
			}
			definition := &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
				Spec:   appsv1.ComponentDefinitionSpec{Configs: []appsv1.ComponentFileTemplate{template}},
				Status: appsv1.ComponentDefinitionStatus{Phase: appsv1.AvailablePhase}}
			pd := &parametersv1alpha1.ParametersDefinition{ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
				Spec: parametersv1alpha1.ParametersDefinitionSpec{ComponentDef: "mysql", TemplateName: configName, FileName: "my.cnf",
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Ini}},
				Status: parametersv1alpha1.ParametersDefinitionStatus{Phase: parametersv1alpha1.PDAvailablePhase}}
			source := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: template.Template, Namespace: namespace},
				Data: map[string]string{"my.cnf": "enabled=ON\n# {{ $.component.name }}\n"}}
			objects := []client.Object{cluster, definition, pd, source}
			configMaps, sets, components = nil, nil, nil
			for _, name := range []string{"db-aaa", "db-bbb"} {
				item := parametersv1alpha1.ConfigTemplateItemDetail{Name: configName, ConfigSpec: &template}
				itemData, err := json.Marshal(item)
				Expect(err).Should(Succeed())
				labels := constant.GetCompLabels(clusterName, name, map[string]string{constant.KBAppShardingNameLabelKey: shardingName})
				cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
					Name: parameterscore.GetComponentCfgName(clusterName, name, configName), Namespace: namespace,
					Labels: map[string]string{constant.AppInstanceLabelKey: clusterName, constant.KBAppComponentLabelKey: name,
						constant.KBAppShardingNameLabelKey: shardingName, constant.CMConfigurationTemplateNameLabelKey: configName,
						constant.CMConfigurationTypeLabelKey: constant.ConfigInstanceType, constant.CMConfigurationSpecProviderLabelKey: configName},
					Annotations: map[string]string{constant.LastAppliedConfigAnnotationKey: string(applied), constant.ConfigurationRevision: "3",
						constant.ConfigAppliedVersionAnnotationKey: string(itemData), constant.ParametersAppliedComponentGenerationKey: "1"}},
					Data: map[string]string{"my.cnf": "enabled=ON\n# " + name + "\n"}}
				setPhase(cm, parametersv1alpha1.CUpgradingPhase)
				cp := &parametersv1alpha1.ComponentParameter{ObjectMeta: metav1.ObjectMeta{
					Name: parameterscore.GenerateComponentConfigurationName(clusterName, name), Namespace: namespace, Generation: 3},
					Spec: parametersv1alpha1.ComponentParameterSpec{ClusterName: clusterName, ComponentName: name,
						ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{item}}}
				comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
					Name: constant.GenerateClusterComponentName(clusterName, name), Namespace: namespace, Generation: 1, Labels: labels,
					Annotations: map[string]string{constant.KBAppClusterUIDKey: "cluster-uid"}},
					Spec: appsv1.ComponentSpec{CompDef: "mysql", Replicas: 1, Configs: []appsv1.ClusterComponentConfig{
						{Name: ptr.To(configName), ConfigHash: sharedHash,
							ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name}}},
							Restart:                      ptr.To(true), Reconfigure: ptr.To(true), ReconfigureArgs: [][]string{{"enabled", "old"}}},
						{Name: ptr.To("other"), ConfigHash: ptr.To("other-hash"), Restart: ptr.To(true)},
					}}}
				its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{
					Name: constant.GenerateWorkloadNamePattern(clusterName, name), Namespace: namespace, Labels: labels},
					Spec: workloads.InstanceSetSpec{Replicas: ptr.To(int32(1)), Configs: []workloads.ConfigTemplate{{Name: configName, ConfigHash: sharedHash}}},
					Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{PodName: name + "-0",
						DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent,
						Configs: []workloads.InstanceConfigStatus{{Name: configName, ConfigHash: sharedHash}}}}}}
				objects = append(objects, cm, cp, comp, its)
				configMaps = append(configMaps, cm)
				sets = append(sets, its)
				components = append(components, comp)
			}
			cli = fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(sets[0], sets[1]).WithObjects(objects...).Build()
			reconciler = &ReconfigureReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
		})

		reconcileFinished := func(index int, original *appsv1.Cluster) {
			GinkgoHelper()
			cm := configMaps[index]
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
			Expect(err).Should(Succeed())
			if result.RequeueAfter > 0 {
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(components[index]), components[index])).Should(Succeed())
				target := components[index].Spec.Configs[0]
				Expect(target.ConfigHash).Should(Equal(computeTargetConfigHash(nil, cm.Data)))
				Expect(target.ConfigMap.Name).Should(Equal(cm.Name))
				Expect(target.Restart).Should(HaveValue(BeFalse()))
				Expect(target.Reconfigure).Should(HaveValue(BeFalse()))
				Expect(target.ReconfigureArgs).Should(BeNil())
				Expect(components[index].Spec.Configs[1].ConfigHash).Should(HaveValue(Equal("other-hash")))
				sets[index].Spec.Configs[0].ConfigHash = target.ConfigHash
				Expect(cli.Update(ctx, sets[index])).Should(Succeed())
				sets[index].Status.InstanceStatus[0].Configs[0].ConfigHash = target.ConfigHash
				Expect(cli.Status().Update(ctx, sets[index])).Should(Succeed())
				result, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
				Expect(err).Should(Succeed())
			}
			Expect(result).Should(Equal(ctrl.Result{}))
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).Should(Succeed())
			status := parametersv1alpha1.ConfigTemplateItemDetailStatus{UpdateRevision: "3"}
			Expect(syncStatus(cm, &status)).Should(Succeed())
			Expect(status.Phase).Should(Equal(parametersv1alpha1.CFinishedPhase))
			applied, err := json.Marshal(cm.Data)
			Expect(err).Should(Succeed())
			Expect(cm.Annotations).Should(HaveKeyWithValue(constant.LastAppliedConfigAnnotationKey, string(applied)))
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
			Expect(cluster.Spec).Should(Equal(original.Spec))
			Expect(cluster.ResourceVersion).Should(Equal(original.ResourceVersion))
		}

		It("recovers alternating semantic no-ops into independent shard targets", func() {
			original := cluster.DeepCopy()
			Expect(computeTargetConfigHash(nil, configMaps[0].Data)).ShouldNot(Equal(computeTargetConfigHash(nil, configMaps[1].Data)))
			reconcileFinished(0, original)
			reconcileFinished(1, original)
			for _, index := range []int{0, 1, 0, 1, 0, 1} {
				reconcileFinished(index, original)
			}
		})

		It("waits for the actual shard replica count", func() {
			original := cluster.DeepCopy()
			components[0].Spec.Replicas = 2
			Expect(cli.Update(ctx, components[0])).Should(Succeed())
			cm := configMaps[0]
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
			Expect(err).Should(Succeed())
			Expect(result.RequeueAfter).Should(BeNumerically(">", 0))
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).Should(Succeed())
			Expect(parseResult(cm.Annotations[parameterscore.GenerateRevisionPhaseKey("3")], "3").ExpectedCount).Should(Equal(int32(2)))
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(components[0]), components[0])).Should(Succeed())
			hash := components[0].Spec.Configs[0].ConfigHash
			sets[0].Spec.Configs[0].ConfigHash = hash
			Expect(cli.Update(ctx, sets[0])).Should(Succeed())
			sets[0].Status.InstanceStatus[0].Configs[0].ConfigHash = hash
			Expect(cli.Status().Update(ctx, sets[0])).Should(Succeed())
			result, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
			Expect(err).Should(Succeed())
			Expect(result.RequeueAfter).Should(BeNumerically(">", 0))
			sets[0].Status.InstanceStatus = append(sets[0].Status.InstanceStatus, *sets[0].Status.InstanceStatus[0].DeepCopy())
			Expect(cli.Status().Update(ctx, sets[0])).Should(Succeed())
			reconcileFinished(0, original)
		})

		It("preserves shards with different applied parameter values", func() {
			for index, value := range []string{"1", "2"} {
				cm := configMaps[index]
				baseline := map[string]string{"my.cnf": "server_id=" + value + "\nenabled=ON\n"}
				applied, err := json.Marshal(baseline)
				Expect(err).Should(Succeed())
				cm.Annotations[constant.LastAppliedConfigAnnotationKey] = string(applied)
				cm.Data["my.cnf"] = baseline["my.cnf"] + "# shard-specific formatting\n"
				Expect(cli.Update(ctx, cm)).Should(Succeed())
			}
			original := cluster.DeepCopy()
			reconcileFinished(0, original)
			reconcileFinished(1, original)
			for _, index := range []int{1, 0, 1, 0, 1, 0} {
				reconcileFinished(index, original)
			}
		})

		It("keeps completed shard configurations finished with absent and stopped runtimes", func() {
			for index, cm := range configMaps {
				applied, err := json.Marshal(cm.Data)
				Expect(err).Should(Succeed())
				cm.Annotations[constant.LastAppliedConfigAnnotationKey] = string(applied)
				setPhase(cm, parametersv1alpha1.CFinishedPhase)
				Expect(cli.Update(ctx, cm)).Should(Succeed())
				components[index].Spec.Configs[0].ConfigHash = computeTargetConfigHash(nil, cm.Data)
				Expect(cli.Update(ctx, components[index])).Should(Succeed())
				sets[index].Status.InstanceStatus = nil
				Expect(cli.Status().Update(ctx, sets[index])).Should(Succeed())
			}
			original := cluster.DeepCopy()
			for _, index := range []int{0, 1, 0, 1} {
				reconcileFinished(index, original)
			}
			cluster.Spec.Shardings[0].Template.Stop = ptr.To(true)
			Expect(cli.Update(ctx, cluster)).Should(Succeed())
			for _, its := range sets {
				its.Spec.Stop = ptr.To(true)
				Expect(cli.Update(ctx, its)).Should(Succeed())
			}
			original = cluster.DeepCopy()
			for _, index := range []int{1, 0, 1, 0} {
				reconcileFinished(index, original)
			}
		})

		It("preserves a sibling shard's pending parameter update when reconciling unchanged content", func() {
			configMaps[1].Data["my.cnf"] = "enabled=OFF\n# db-bbb\n"
			Expect(cli.Update(ctx, configMaps[1])).Should(Succeed())
			config := &components[1].Spec.Configs[0]
			config.ConfigHash = computeTargetConfigHash(nil, configMaps[1].Data)
			config.Restart = ptr.To(true)
			config.ReconfigureArgs = [][]string{{"enabled", "OFF"}}
			Expect(cli.Update(ctx, components[1])).Should(Succeed())
			sibling := components[1].DeepCopy()
			original := cluster.DeepCopy()
			for range 3 {
				reconcileFinished(0, original)
			}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(configMaps[1]), configMaps[1])).Should(Succeed())
			status := parametersv1alpha1.ConfigTemplateItemDetailStatus{UpdateRevision: "3"}
			Expect(syncStatus(configMaps[1], &status)).Should(Succeed())
			Expect(status.Phase).Should(Equal(parametersv1alpha1.CUpgradingPhase))
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(components[1]), components[1])).Should(Succeed())
			Expect(components[1].Spec).Should(Equal(sibling.Spec))
		})

		for _, unavailable := range []string{"Cluster", "Component"} {
			for _, applied := range []bool{false, true} {
				It(fmt.Sprintf("retries initialized shard configuration without %s (applied=%t)", unavailable, applied), func() {
					cm := configMaps[0]
					if applied {
						cm.Data = map[string]string{"my.cnf": "enabled=ON\n"}
						Expect(cli.Update(ctx, cm)).Should(Succeed())
					}
					lastApplied := cm.Annotations[constant.LastAppliedConfigAnnotationKey]
					var missing client.Object = cluster.DeepCopy()
					if unavailable == "Component" {
						missing = components[0].DeepCopy()
					}
					Expect(cli.Delete(ctx, missing)).Should(Succeed())
					_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
					Expect(err).Should(HaveOccurred())
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).Should(Succeed())
					Expect(cm.Annotations[constant.LastAppliedConfigAnnotationKey]).Should(Equal(lastApplied))
					Expect(parseResult(cm.Annotations[parameterscore.GenerateRevisionPhaseKey("3")], "3").Phase).
						Should(Equal(parametersv1alpha1.CUpgradingPhase))
					missing.SetResourceVersion("")
					Expect(cli.Create(ctx, missing)).Should(Succeed())
					_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
					Expect(err).Should(Succeed())
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(components[0]), components[0])).Should(Succeed())
					Expect(components[0].Spec.Configs[0].ConfigHash).Should(Equal(computeTargetConfigHash(nil, cm.Data)))
				})
			}
		}

		for _, unavailable := range []string{"InstanceSet", "ComponentParameter", "ComponentDefinition"} {
			It("keeps applied shard configurations finished without "+unavailable, func() {
				for index, cm := range configMaps {
					applied, err := json.Marshal(cm.Data)
					Expect(err).Should(Succeed())
					cm.Annotations[constant.LastAppliedConfigAnnotationKey] = string(applied)
					setPhase(cm, parametersv1alpha1.CFinishedPhase)
					Expect(cli.Update(ctx, cm)).Should(Succeed())
					components[index].Spec.Configs[0].ConfigHash = computeTargetConfigHash(nil, cm.Data)
					Expect(cli.Update(ctx, components[index])).Should(Succeed())
				}
				original := cluster.DeepCopy()
				switch unavailable {
				case "InstanceSet":
					Expect(cli.Delete(ctx, sets[0])).Should(Succeed())
				case "ComponentParameter":
					cp := &parametersv1alpha1.ComponentParameter{ObjectMeta: metav1.ObjectMeta{
						Name: parameterscore.GenerateComponentConfigurationName(clusterName, "db-aaa"), Namespace: namespace}}
					Expect(cli.Delete(ctx, cp)).Should(Succeed())
				case "ComponentDefinition":
					definition := &appsv1.ComponentDefinition{}
					Expect(cli.Get(ctx, client.ObjectKey{Name: "mysql"}, definition)).Should(Succeed())
					definition.Status.Phase = appsv1.UnavailablePhase
					Expect(cli.Update(ctx, definition)).Should(Succeed())
				}
				for _, index := range []int{0, 1, 0, 1} {
					reconcileFinished(index, original)
				}
				components[0].Spec.Configs[0].ConfigHash = ptr.To("stale-target")
				Expect(cli.Update(ctx, components[0])).Should(Succeed())
				result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(configMaps[0])})
				Expect(err != nil || result.RequeueAfter > 0).Should(BeTrue(), "stale targets must not use completed metadata to skip missing dependencies")
			})
		}
	})

	Context("applied configurations without an independent target", func() {
		for _, name := range []string{"absent target", "target without a hash", "target from a different source", "initial configuration"} {
			It("preserves applied metadata for "+name, func() {
				ctx := context.Background()
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, parametersv1alpha1.AddToScheme} {
					Expect(add(scheme)).Should(Succeed())
				}
				data := map[string]string{"my.cnf": "enabled=ON\n"}
				applied, err := json.Marshal(data)
				Expect(err).Should(Succeed())
				cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "mysql-config", Namespace: "default",
					Labels: map[string]string{constant.AppInstanceLabelKey: "mysql", constant.KBAppComponentLabelKey: "mysql",
						constant.CMConfigurationTemplateNameLabelKey: "mysql-config", constant.CMConfigurationTypeLabelKey: constant.ConfigInstanceType,
						constant.CMConfigurationSpecProviderLabelKey: "mysql-config", constant.CMInsConfigurationHashLabelKey: "previous-label"},
					Annotations: map[string]string{constant.LastAppliedConfigAnnotationKey: string(applied), constant.ConfigurationRevision: "3",
						constant.UpgradePolicyAnnotationKey: "keep-policy"}}, Data: data}
				cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "mysql", Namespace: "default"},
					Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql", Replicas: 1,
						Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To("other"), ConfigHash: ptr.To("other-hash")}}}}}}
				component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "mysql-mysql", Namespace: "default"},
					Spec: appsv1.ComponentSpec{Replicas: 1, Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To("other"), ConfigHash: ptr.To("other-hash")}}}}
				switch name {
				case "target without a hash":
					cluster.Spec.ComponentSpecs[0].Configs = append(cluster.Spec.ComponentSpecs[0].Configs, appsv1.ClusterComponentConfig{Name: ptr.To("mysql-config")})
				case "target from a different source":
					cluster.Spec.ComponentSpecs[0].Configs = append(cluster.Spec.ComponentSpecs[0].Configs, appsv1.ClusterComponentConfig{Name: ptr.To("mysql-config"),
						ConfigHash: ptr.To("unrelated-hash"), Restart: ptr.To(true),
						ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "another-configmap"}}}})
				case "initial configuration":
					delete(cm.Annotations, constant.LastAppliedConfigAnnotationKey)
				}
				labels := cm.DeepCopy().Labels
				if name == "initial configuration" {
					labels[constant.CMInsConfigurationHashLabelKey] = *computeTargetConfigHash(nil, cm.Data)
				}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, cluster, component).Build()
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
				original := cluster.DeepCopy()
				originalComponent := component.DeepCopy()
				if name == "initial configuration" {
					Expect(cli.Delete(ctx, component)).Should(Succeed())
				}
				r := &ReconfigureReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(10)}
				result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cm)})
				Expect(err).Should(Succeed())
				Expect(result).Should(Equal(ctrl.Result{}))
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).Should(Succeed())
				status := parametersv1alpha1.ConfigTemplateItemDetailStatus{UpdateRevision: "3"}
				Expect(syncStatus(cm, &status)).Should(Succeed())
				Expect(status.Phase).Should(Equal(parametersv1alpha1.CFinishedPhase))
				Expect(cm.Labels).Should(Equal(labels))
				Expect(cm.Annotations).Should(HaveKeyWithValue(constant.LastAppliedConfigAnnotationKey, string(applied)))
				if name == "initial configuration" {
					Expect(cm.Annotations).ShouldNot(HaveKey(constant.UpgradePolicyAnnotationKey))
				} else {
					Expect(cm.Annotations).Should(HaveKeyWithValue(constant.UpgradePolicyAnnotationKey, "keep-policy"))
				}
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
				Expect(cluster.Spec).Should(Equal(original.Spec))
				Expect(cluster.ResourceVersion).Should(Equal(original.ResourceVersion))
				if name != "initial configuration" {
					Expect(cli.Get(ctx, client.ObjectKeyFromObject(component), component)).Should(Succeed())
					Expect(component.Spec).Should(Equal(originalComponent.Spec))
				}
			})
		}
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
