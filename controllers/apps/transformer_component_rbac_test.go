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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	var serviceAccountName = ""

	var transCtx graph.TransformContext
	var dag *graph.DAG
	var graphCli model.GraphClient
	var transformer graph.Transformer
	var cluster *appsv1.Cluster
	var compDefObj *appsv1.ComponentDefinition
	var compObj *appsv1.Component
	var synthesizedComp *component.SynthesizedComponent
	var allSettings map[string]interface{}

	init := func(enableLifecycleAction bool, enablePolicyRules bool) {
		By("Create a component definition")
		compDefFactory := testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			Create(&testCtx)

		// default spec has some lifecycle actions
		if !enableLifecycleAction {
			compDefFactory.Get().Spec.LifecycleActions = nil
		}

		if enablePolicyRules {
			compDefFactory.SetPolicyRules([]rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pod"},
					Verbs:     []string{"get", "list", "watch"},
				},
			})
		}
		compDefObj = compDefFactory.GetObject()

		By("Creating a cluster")
		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(1).
			GetObject()

		By("Creating a component")
		fullCompName := constant.GenerateClusterComponentName(cluster.Name, compName)
		compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefName).
			AddAnnotations(constant.KBAppClusterUIDKey, string(cluster.UID)).
			AddLabels(constant.AppInstanceLabelKey, cluster.Name).
			SetReplicas(1).
			GetObject()

		graphCli = model.NewGraphClient(k8sClient)

		var err error
		serviceAccountName = constant.GenerateDefaultServiceAccountName(cluster.Name, compName)
		synthesizedComp, err = component.BuildSynthesizedComponent(ctx, k8sClient, compDefObj, compObj, cluster)
		Expect(err).Should(Succeed())

		transCtx = &componentTransformContext{
			Context:             ctx,
			Client:              graphCli,
			EventRecorder:       clusterRecorder,
			Logger:              logger,
			Cluster:             cluster,
			CompDef:             compDefObj,
			Component:           compObj,
			ComponentOrig:       compObj.DeepCopy(),
			SynthesizeComponent: synthesizedComp,
		}

		dag = mockDAG(graphCli, cluster)
		transformer = &componentRBACTransformer{}

		saKey := types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      serviceAccountName,
		}
		Eventually(testapps.CheckObjExists(&testCtx, saKey,
			&corev1.ServiceAccount{}, false)).Should(Succeed())
	}

	BeforeEach(func() {
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

	Context("transformer rbac manager", func() {
		It("w/o any rolebindings", func() {
			init(false, false)
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())
			// nothing should be created
			dagExpected := mockDAG(graphCli, cluster)
			Expect(dag.Equals(dagExpected, model.DefaultLess)).Should(BeTrue())
		})

		It("w/ lifecycle actions", func() {
			init(true, false)
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())
			clusterPodRoleBinding := factory.BuildRoleBinding(synthesizedComp, serviceAccountName, &rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     constant.RBACRoleName,
			}, serviceAccountName)
			serviceAccount := factory.BuildServiceAccount(synthesizedComp, serviceAccountName)
			dagExpected := mockDAG(graphCli, cluster)
			graphCli.Create(dagExpected, serviceAccount)
			graphCli.Create(dagExpected, clusterPodRoleBinding)
			graphCli.DependOn(dagExpected, clusterPodRoleBinding, serviceAccount)
			itsList := graphCli.FindAll(dagExpected, &workloads.InstanceSet{})
			for i := range itsList {
				graphCli.DependOn(dagExpected, itsList[i], serviceAccount)
			}
			Expect(dag.Equals(dagExpected, model.DefaultLess)).Should(BeTrue())
		})

		FIt("w/ cmpd's PolicyRules", func() {
			init(false, true)
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())
			cmpdRole := factory.BuildComponentRole(synthesizedComp, compDefObj, serviceAccountName)
			cmpdRoleBinding := factory.BuildRoleBinding(synthesizedComp, fmt.Sprintf("%v-pod", serviceAccountName), &rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     constant.RBACRoleName,
			}, serviceAccountName)
			serviceAccount := factory.BuildServiceAccount(synthesizedComp, serviceAccountName)
			dagExpected := mockDAG(graphCli, cluster)
			graphCli.Create(dagExpected, serviceAccount)
			graphCli.Create(dagExpected, cmpdRoleBinding)
			graphCli.Create(dagExpected, cmpdRole)
			graphCli.DependOn(dagExpected, cmpdRoleBinding, serviceAccount, cmpdRole)
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
