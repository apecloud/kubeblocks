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

package apps

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("object rbac transformer test.", func() {
	const compDefName = "test-compdef"
	const clusterName = "test-cluster"
	const compName = "default"
	var serviceAccountName = constant.GenerateDefaultServiceAccountName(clusterName)

	var transCtx graph.TransformContext
	var dag *graph.DAG
	var graphCli model.GraphClient
	var transformer graph.Transformer
	var cluster *appsv1.Cluster
	var compDefObj *appsv1.ComponentDefinition
	var compObj *appsv1.Component
	var saKey types.NamespacedName
	var allSettings map[string]interface{}

	BeforeEach(func() {
		By("Create a component definition")
		compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			Create(&testCtx).
			GetObject()

		By("Creating a cluster")
		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(1).
			SetServiceAccountName(serviceAccountName).
			GetObject()

		By("Creating a component")
		fullCompName := constant.GenerateClusterComponentName(cluster.Name, compName)
		compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefName).
			AddLabels(constant.AppInstanceLabelKey, cluster.Name).
			AddLabels(constant.KBAppClusterUIDLabelKey, string(cluster.UID)).
			SetReplicas(1).
			SetServiceAccountName(serviceAccountName).
			GetObject()

		saKey = types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      serviceAccountName,
		}

		graphCli = model.NewGraphClient(k8sClient)

		synthesizedComponent, err := component.BuildSynthesizedComponent(ctx, k8sClient, compDefObj, compObj, cluster)
		Expect(err).Should(Succeed())

		transCtx = &componentTransformContext{
			Context:             ctx,
			Client:              graphCli,
			EventRecorder:       nil,
			Logger:              logger,
			Cluster:             cluster,
			CompDef:             compDefObj,
			Component:           compObj,
			ComponentOrig:       compObj.DeepCopy(),
			SynthesizeComponent: synthesizedComponent,
		}

		dag = mockDAG(graphCli, cluster)
		transformer = &componentRBACTransformer{}
		allSettings = viper.AllSettings()
		viper.SetDefault(constant.EnableRBACManager, true)
	})

	AfterEach(func() {
		viper.SetDefault(constant.EnableRBACManager, false)
		if allSettings != nil {
			Expect(viper.MergeConfigMap(allSettings)).ShouldNot(HaveOccurred())
			allSettings = nil
		}
	})

	disableVolumeProtection := func() {
		for i := range compDefObj.Spec.Volumes {
			compDefObj.Spec.Volumes[i].HighWatermark = 0
		}
	}

	enableVolumeProtection := func() {
		for i := range compDefObj.Spec.Volumes {
			compDefObj.Spec.Volumes[i].HighWatermark = 85
		}
	}

	Context("transformer rbac manager", func() {
		It("create serviceaccount, rolebinding if not exist", func() {
			disableVolumeProtection()

			Eventually(testapps.CheckObjExists(&testCtx, saKey,
				&corev1.ServiceAccount{}, false)).Should(Succeed())
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())

			serviceAccount := factory.BuildServiceAccount(cluster, serviceAccountName)
			roleBinding := factory.BuildRoleBinding(cluster, serviceAccount.Name)

			dagExpected := mockDAG(graphCli, cluster)
			graphCli.Create(dagExpected, serviceAccount)
			graphCli.Create(dagExpected, roleBinding)
			graphCli.DependOn(dagExpected, roleBinding, serviceAccount)
			itsList := graphCli.FindAll(dagExpected, &workloads.InstanceSet{})
			for i := range itsList {
				graphCli.DependOn(dagExpected, itsList[i], serviceAccount)
			}
			Expect(dag.Equals(dagExpected, model.DefaultLess)).Should(BeTrue())
		})

		It("create clusterrolebinding if volumeprotection enabled", func() {
			enableVolumeProtection()

			Eventually(testapps.CheckObjExists(&testCtx, saKey,
				&corev1.ServiceAccount{}, false)).Should(Succeed())
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())

			serviceAccount := factory.BuildServiceAccount(cluster, serviceAccountName)
			roleBinding := factory.BuildRoleBinding(cluster, serviceAccount.Name)
			clusterRoleBinding := factory.BuildClusterRoleBinding(cluster, serviceAccount.Name)

			dagExpected := mockDAG(graphCli, cluster)
			graphCli.Create(dagExpected, serviceAccount)
			graphCli.Create(dagExpected, roleBinding)
			graphCli.Create(dagExpected, clusterRoleBinding)
			graphCli.DependOn(dagExpected, roleBinding, clusterRoleBinding)
			graphCli.DependOn(dagExpected, clusterRoleBinding, serviceAccount)
			itsList := graphCli.FindAll(dagExpected, &workloads.InstanceSet{})
			for i := range itsList {
				graphCli.DependOn(dagExpected, itsList[i], serviceAccount)
			}
			Expect(dag.Equals(dagExpected, model.DefaultLess)).Should(BeTrue())
		})
	})
})

func mockDAG(graphCli model.GraphClient, cluster *appsv1.Cluster) *graph.DAG {
	d := graph.NewDAG()
	graphCli.Root(d, cluster, cluster, model.ActionStatusPtr())
	its := &workloads.InstanceSet{}
	graphCli.Create(d, its)
	return d
}
