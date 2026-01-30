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

package component

import (
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("object rbac transformer test.", func() {
	const (
		compDefName = "test-compdef"
		clusterName = "test-cluster"
		compName    = "test-comp"
	)

	var (
		serviceAccountName string
		transCtx           graph.TransformContext
		dag                *graph.DAG
		graphCli           model.GraphClient
		transformer        graph.Transformer
		compDefObj         *appsv1.ComponentDefinition
		clusterUID         = string(uuid.NewUUID())
		compObj            *appsv1.Component
		synthesizedComp    *component.SynthesizedComponent
	)

	AfterEach(func() {
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ServiceAccountSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RoleBindingSignature, true, inNS)
	})

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

		By("Creating a component")
		fullCompName := constant.GenerateClusterComponentName(clusterName, compName)
		compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefName).
			AddAnnotations(constant.KBAppClusterUIDKey, clusterUID).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			SetReplicas(1).
			GetObject()

		graphCli = model.NewGraphClient(k8sClient)

		var err error
		synthesizedComp, err = component.BuildSynthesizedComponent(ctx, k8sClient, compDefObj, compObj)
		Expect(err).Should(Succeed())

		transCtx = &componentTransformContext{
			Context:             ctx,
			Client:              graphCli,
			EventRecorder:       clusterRecorder,
			Logger:              logger,
			CompDef:             compDefObj,
			Component:           compObj,
			ComponentOrig:       compObj.DeepCopy(),
			SynthesizeComponent: synthesizedComp,
		}

		dag = mockDAG(graphCli, compObj)
		transformer = &componentRBACTransformer{}

		serviceAccountName = constant.GenerateDefaultServiceAccountNameNew(fullCompName)
		saKey := types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      serviceAccountName,
		}
		Eventually(testapps.CheckObjExists(&testCtx, saKey,
			&corev1.ServiceAccount{}, false)).Should(Succeed())
	}

	Context("transformer rbac manager", func() {
		It("tests labelAndAnnotationEqual", func() {
			// nil and not nil
			Expect(labelAndAnnotationEqual(
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{},
					},
				}, &corev1.ServiceAccount{}),
			).Should(BeTrue())
			// labels are equal
			Expect(labelAndAnnotationEqual(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": "test",
					},
				},
			}, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": "test",
					},
				},
			})).Should(BeTrue())
			// labels are not equal
			Expect(labelAndAnnotationEqual(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": "test",
					},
				},
			}, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": "test1",
					},
				},
			})).Should(BeFalse())
			// annotations are not equal
			Expect(labelAndAnnotationEqual(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"test": "test",
					},
				},
			}, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"test": "test1",
					},
				},
			})).Should(BeFalse())
		})

		It("w/o any rolebindings", func() {
			init(false, false)
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())
			// sa should be created
			serviceAccount := factory.BuildServiceAccount(synthesizedComp, serviceAccountName)

			dagExpected := mockDAG(graphCli, compObj)
			graphCli.Create(dagExpected, serviceAccount)

			Expect(dag.Equals(dagExpected, model.DefaultLess)).Should(BeTrue())
		})

		It("w/ lifecycle actions", func() {
			init(true, false)
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())
			clusterPodRoleBinding := factory.BuildRoleBinding(synthesizedComp, fmt.Sprintf("%v-pod", serviceAccountName), &rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     constant.RBACRoleName,
			}, serviceAccountName)
			serviceAccount := factory.BuildServiceAccount(synthesizedComp, serviceAccountName)
			dagExpected := mockDAG(graphCli, compObj)
			graphCli.Create(dagExpected, serviceAccount)
			graphCli.Create(dagExpected, clusterPodRoleBinding)
			graphCli.DependOn(dagExpected, clusterPodRoleBinding, serviceAccount)
			itsList := graphCli.FindAll(dagExpected, &workloads.InstanceSet{})
			for i := range itsList {
				graphCli.DependOn(dagExpected, itsList[i], serviceAccount)
			}
			Expect(dag.Equals(dagExpected, model.DefaultLess)).Should(BeTrue())
		})

		It("w/ cmpd's PolicyRules", func() {
			init(false, true)
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())
			cmpdRoleBinding := factory.BuildRoleBinding(synthesizedComp, serviceAccountName, &rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     constant.GenerateDefaultRoleName(compDefObj.Name),
			}, serviceAccountName)
			serviceAccount := factory.BuildServiceAccount(synthesizedComp, serviceAccountName)
			dagExpected := mockDAG(graphCli, compObj)
			graphCli.Create(dagExpected, serviceAccount)
			graphCli.Create(dagExpected, cmpdRoleBinding)
			graphCli.DependOn(dagExpected, cmpdRoleBinding, serviceAccount)
			itsList := graphCli.FindAll(dagExpected, &workloads.InstanceSet{})
			for i := range itsList {
				graphCli.DependOn(dagExpected, itsList[i], serviceAccount)
			}
			Expect(dag.Equals(dagExpected, model.DefaultLess)).Should(BeTrue())
			// DefaultLess doesn't compare objs' contents
			actualRoleBinding := graphCli.FindAll(dag, &rbacv1.RoleBinding{})
			Expect(actualRoleBinding).To(HaveLen(1))
			rb := actualRoleBinding[0].(*rbacv1.RoleBinding)
			Expect(reflect.DeepEqual(rb.Subjects, cmpdRoleBinding.Subjects)).To(BeTrue())
			Expect(reflect.DeepEqual(rb.RoleRef, cmpdRoleBinding.RoleRef)).To(BeTrue())
		})

		Context("rollback behavior", func() {
			It("tests needRollbackServiceAccount", func() {
				init(true, false)
				ctx := transCtx.(*componentTransformContext)

				By("create another cmpd")
				anotherTpl := testapps.NewComponentDefinitionFactory(compDefName).
					WithRandomName().
					SetDefaultSpec().
					Create(&testCtx).
					GetObject()
				hash, err := computeServiceAccountRuleHash(ctx)
				Expect(err).ShouldNot(HaveOccurred())

				// Case: No label, should return false
				needRollback, err := needRollbackServiceAccount(ctx)
				Expect(err).Should(BeNil())
				Expect(needRollback).Should(BeFalse())

				// Case: With same cmpd
				ctx.Component.Annotations[constant.ComponentLastServiceAccountRuleHashAnnotationKey] = hash
				ctx.Component.Annotations[constant.ComponentLastServiceAccountNameAnnotationKey] = constant.GenerateDefaultServiceAccountName(synthesizedComp.CompDefName)
				needRollback, err = needRollbackServiceAccount(ctx)
				Expect(err).Should(BeNil())
				Expect(needRollback).Should(BeTrue())

				// Case: Different cmpd, same spec
				another := anotherTpl.DeepCopy()
				ctx.SynthesizeComponent, err = component.BuildSynthesizedComponent(ctx, k8sClient, another, compObj)
				Expect(err).Should(Succeed())
				needRollback, err = needRollbackServiceAccount(ctx)
				Expect(err).Should(BeNil())
				Expect(needRollback).Should(BeTrue())

				// Case: Different cmpd, different policy rules
				another = anotherTpl.DeepCopy()
				another.Spec.PolicyRules = []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get"},
					},
				}
				ctx.SynthesizeComponent, err = component.BuildSynthesizedComponent(ctx, k8sClient, another, compObj)
				Expect(err).Should(Succeed())
				needRollback, err = needRollbackServiceAccount(ctx)
				Expect(err).Should(BeNil())
				Expect(needRollback).Should(BeFalse())

				// Case: Different cmpd, different lifecycle action
				another = anotherTpl.DeepCopy()
				another.Spec.PolicyRules = nil
				another.Spec.LifecycleActions = nil
				ctx.SynthesizeComponent, err = component.BuildSynthesizedComponent(ctx, k8sClient, another, compObj)
				Expect(err).Should(Succeed())
				needRollback, err = needRollbackServiceAccount(ctx)
				Expect(err).Should(BeNil())
				Expect(needRollback).Should(BeFalse())
			})

			mockDAGWithUpdate := func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
				d := graph.NewDAG()
				graphCli.Root(d, comp, comp, model.ActionUpdatePtr())
				its := &workloads.InstanceSet{}
				graphCli.Create(d, its)
				return d
			}

			less := func(v1, v2 graph.Vertex) bool {
				o1, ok1 := v1.(*model.ObjectVertex)
				o2, ok2 := v2.(*model.ObjectVertex)
				if !ok1 || !ok2 {
					return false
				}
				if o1.String() != o2.String() {
					return o1.String() < o2.String()
				}
				if !reflect.DeepEqual(o1.Obj.GetLabels(), o2.Obj.GetLabels()) {
					return true
				}
				return !reflect.DeepEqual(o1.Obj.GetAnnotations(), o2.Obj.GetAnnotations())
			}

			It("adds labels for an old component", func() {
				init(false, false)
				// mock a running workload
				ctx := transCtx.(*componentTransformContext)
				ctx.RunningWorkload = &workloads.InstanceSet{}
				expectedComp := compObj.DeepCopy()
				Expect(transformer.Transform(transCtx, dag)).Should(BeNil())
				// sa should be created
				oldSAName := constant.GenerateDefaultServiceAccountName(compDefObj.Name)
				serviceAccount := factory.BuildServiceAccount(synthesizedComp, oldSAName)
				newServiceAccount := factory.BuildServiceAccount(synthesizedComp, serviceAccountName)

				hash, err := computeServiceAccountRuleHash(ctx)
				Expect(err).ShouldNot(HaveOccurred())
				expectedComp.Annotations[constant.ComponentLastServiceAccountRuleHashAnnotationKey] = hash
				expectedComp.Annotations[constant.ComponentLastServiceAccountNameAnnotationKey] = constant.GenerateDefaultServiceAccountName(synthesizedComp.CompDefName)
				dagExpected := mockDAGWithUpdate(graphCli, expectedComp)
				graphCli.Create(dagExpected, serviceAccount)
				graphCli.Create(dagExpected, newServiceAccount)

				Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
			})
		})
	})
})

func mockDAG(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
	d := graph.NewDAG()
	graphCli.Root(d, comp, comp, model.ActionStatusPtr())
	its := &workloads.InstanceSet{}
	graphCli.Create(d, its)
	return d
}

func mockDAGWithUpdate(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
	d := graph.NewDAG()
	graphCli.Root(d, comp, comp, model.ActionUpdatePtr())
	its := &workloads.InstanceSet{}
	graphCli.Create(d, its)
	return d
}
