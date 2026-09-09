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
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("Automatic resource rendering", func() {
	BeforeEach(cleanEnv)
	AfterEach(cleanEnv)

	waitFinished := func(comp *appsv1.Component, content string, afterGeneration ...int64) int64 {
		key := client.ObjectKey{Namespace: comp.Namespace, Name: core.GenerateComponentConfigurationName(clusterName, defaultCompName)}
		var generation int64
		Eventually(func(g Gomega) {
			cp := &parametersv1alpha1.ComponentParameter{}
			g.Expect(testCtx.Cli.Get(testCtx.Ctx, key, cp)).To(Succeed())
			if len(afterGeneration) > 0 {
				g.Expect(cp.Generation).To(BeNumerically(">", afterGeneration[0]))
			}
			g.Expect(cp.Status.ObservedGeneration).To(Equal(cp.Generation))
			item := parameters.GetItemStatus(&cp.Status, configSpecName)
			g.Expect(item).NotTo(BeNil())
			g.Expect(item.Phase).To(Equal(parametersv1alpha1.CFinishedPhase))
			g.Expect(item.UpdateRevision).To(Equal(strconv.FormatInt(cp.Generation, 10)))
			cm := &corev1.ConfigMap{}
			cmKey := client.ObjectKey{Namespace: comp.Namespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
			g.Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
			g.Expect(cm.Data[testparameters.MysqlConfigFile]).To(ContainSubstring(content))
			g.Expect(cm.Annotations[constant.ConfigurationRevision]).To(Equal(item.UpdateRevision))
			generation = cp.Generation
		}).Should(Succeed())
		return generation
	}

	It("rerenders storage vars and built-in functions without trigger declarations", func() {
		_, _, _, comp, _ := mockReconcileResource(mockResourceConfiguration{
			Vars: []appsv1.EnvVar{storageVar("")},
			Template: func(cm *corev1.ConfigMap) {
				cm.Data[testparameters.MysqlConfigFile] = strings.ReplaceAll(cm.Data[testparameters.MysqlConfigFile], "server-id=1", "server-id={{ div (int64 $.DISK) 1073741824 }}")
				cm.Data[envTestFileKey] = `capacity={{ getComponentPVCSizeByName $.component "data" }}`
			},
		})
		waitFinished(comp, "server-id=0")
		expand := func(size string) {
			// This suite runs parameter controllers; emulate the Component update that
			// the apps controller normally propagates from a VolumeExpansion operation.
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(c *appsv1.Component) {
				c.Spec.VolumeClaimTemplates = resourceTestComponent("db", "db").Spec.VolumeClaimTemplates
				c.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(size)
			})()).To(Succeed())
		}
		expand("2Gi")
		first := waitFinished(comp, "server-id=2")
		cm := &corev1.ConfigMap{}
		cmKey := client.ObjectKey{Namespace: comp.Namespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
		Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
		Expect(cm.Data[envTestFileKey]).To(Equal("capacity=2147483648"))
		expand("3Gi")
		second := waitFinished(comp, "server-id=3")
		Expect(second).To(BeNumerically(">", first))
		expand("3072Mi")
		Consistently(func() int64 {
			cp := &parametersv1alpha1.ComponentParameter{}
			Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKey{Namespace: comp.Namespace, Name: comp.Name}, cp)).To(Succeed())
			return cp.Generation
		}, time.Second*2, time.Millisecond*100).Should(Equal(second))
	})

	It("preserves explicit parameters through storage changes", func() {
		_, _, _, comp, _ := mockReconcileResource(mockResourceConfiguration{
			Vars: []appsv1.EnvVar{storageVar("")},
			Template: func(cm *corev1.ConfigMap) {
				cm.Data[testparameters.MysqlConfigFile] = strings.ReplaceAll(cm.Data[testparameters.MysqlConfigFile], "server-id=1", "server-id={{ div (int64 $.DISK) 1073741824 }}")
			},
		})
		waitFinished(comp, "server-id=0")
		expand := func(size string) {
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(c *appsv1.Component) {
				c.Spec.VolumeClaimTemplates = resourceTestComponent("db", "db").Spec.VolumeClaimTemplates
				c.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(size)
			})()).To(Succeed())
		}
		expand("2Gi")
		waitFinished(comp, "server-id=2")
		By("preserve a user override through subsequent expansions")
		override := "9"
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(cp *parametersv1alpha1.ComponentParameter) {
			item := parameters.GetConfigTemplateItem(&cp.Spec, configSpecName)
			item.ConfigFileParams = map[string]parametersv1alpha1.ParametersInFile{testparameters.MysqlConfigFile: {Parameters: map[string]*string{"server-id": &override}}}
		})()).To(Succeed())
		before := waitFinished(comp, "server-id=9")
		cm := &corev1.ConfigMap{}
		cmKey := client.ObjectKey{Namespace: comp.Namespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
		Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
		data := cm.DeepCopy().Data
		expand("3Gi")
		after := waitFinished(comp, "server-id=9", before)
		Expect(after).To(BeNumerically(">", before))
		Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
		Expect(cm.Data).To(Equal(data))
		Expect(fmt.Sprint(after)).To(Equal(cm.Annotations[constant.ConfigurationRevision]))
	})
	It("refreshes sharding payload when only Cluster shard count changes", func() {
		_, _, _, comp, _ := mockReconcileResource()
		waitFinished(comp, "server-id=1")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(c *appsv1.Component) {
			c.Labels[constant.KBAppShardingNameLabelKey] = shardingCompName
		})()).To(Succeed())
		waitShards := func(shards string) int64 {
			var generation int64
			Eventually(func(g Gomega) {
				cp := &parametersv1alpha1.ComponentParameter{}
				g.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), cp)).To(Succeed())
				item := parameters.GetConfigTemplateItem(&cp.Spec, configSpecName)
				g.Expect(item).NotTo(BeNil())
				g.Expect(string(item.Payload[constant.ShardingPayload])).To(ContainSubstring(`"shards":"` + shards + `"`))
				g.Expect(cp.Status.ObservedGeneration).To(Equal(cp.Generation))
				generation = cp.Generation
			}).Should(Succeed())
			return generation
		}
		before := waitShards("5")
		Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), comp)).To(Succeed())
		componentGeneration := comp.Generation
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKey{Namespace: comp.Namespace, Name: clusterName}, func(c *appsv1.Cluster) {
			c.Spec.Shardings[0].Shards = 6
		})()).To(Succeed())
		Expect(waitShards("6")).To(BeNumerically(">", before))
		Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), comp)).To(Succeed())
		Expect(comp.Generation).To(Equal(componentGeneration))
	})

})
