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

package workloads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset2"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/workloads/instancereadiness"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbaproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("InstanceSet configured readiness", func() {
	for _, instanceAPI := range []bool{false, true} {
		Context(fmt.Sprintf("Instance API %t", instanceAPI), func() {
			newTree := func(replicas *int32, names ...string) (*workloads.InstanceSet, *kubebuilderx.ObjectTree) {
				its := &workloads.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 3},
					Spec: workloads.InstanceSetSpec{Replicas: replicas, Selector: &metav1.LabelSelector{},
						Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}}},
					Status: workloads.InstanceSetStatus{ObservedGeneration: 3},
				}
				tree := kubebuilderx.NewObjectTree()
				tree.SetRoot(its)
				for _, name := range names {
					pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: its.Namespace},
						Spec: *its.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue,
								LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute))}}}}
					if instanceAPI {
						inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: its.Namespace, UID: types.UID(name), Generation: 1},
							Spec: workloads.InstanceSpec{Template: *its.Spec.Template.DeepCopy()},
							Status: workloads.InstanceStatus2{ObservedGeneration: 1, CurrentState: workloads.InstanceCurrentStatePresent,
								Ready: true, Available: true, Conditions: []metav1.Condition{{Type: string(workloads.InstanceReady),
									Status: metav1.ConditionTrue, ObservedGeneration: 1}}}}
						pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: workloads.GroupVersion.String(), Kind: "Instance", Name: name, UID: inst.UID, Controller: ptr.To(true)}}
						Expect(tree.Add(inst)).Should(Succeed())
					}
					Expect(tree.Add(pod)).Should(Succeed())
				}
				return its, tree
			}
			reconcile := func(tree *kubebuilderx.ObjectTree) {
				r := instanceset.NewStatusReconciler()
				if instanceAPI {
					r = instanceset2.NewStatusReconciler()
				}
				result, err := r.Reconcile(tree)
				Expect(err).ShouldNot(HaveOccurred())
				if result == kubebuilderx.RetryAfter(time.Millisecond) {
					_, err = r.Reconcile(tree)
				}
				Expect(err).ShouldNot(HaveOccurred())
			}
			DescribeTable("uses the configured identities", func(replicas *int32, names []string, unhealthyExtra bool, stop bool, ready bool) {
				its, tree := newTree(replicas, names...)
				its.Spec.Stop = ptr.To(stop)
				if unhealthyExtra {
					for _, object := range tree.List(&corev1.Pod{}) {
						pod := object.(*corev1.Pod)
						if pod.Name == "demo-9" {
							pod.Status.Conditions[0].Status = corev1.ConditionFalse
						}
					}
				}
				reconcile(tree)
				for _, kind := range []workloads.ConditionType{workloads.InstanceReady, workloads.InstanceAvailable} {
					condition := meta.FindStatusCondition(its.Status.Conditions, string(kind))
					Expect(condition).ShouldNot(BeNil())
					Expect(condition.Status == metav1.ConditionTrue).Should(Equal(ready))
					Expect(condition.ObservedGeneration).Should(Equal(its.Generation))
				}
				Expect(its.Status.Replicas).Should(Equal(int32(len(names))))
			},
				Entry("omitted target requires one", nil, []string{}, false, false, false),
				Entry("omitted target with one healthy member", nil, []string{"demo-0"}, false, false, true),
				Entry("explicit zero", ptr.To[int32](0), []string{}, false, false, true),
				Entry("zero with healthy extra", ptr.To[int32](0), []string{"demo-9"}, false, false, true),
				Entry("zero with unhealthy extra", ptr.To[int32](0), []string{"demo-9"}, true, false, true),
				Entry("partial healthy set", ptr.To[int32](3), []string{"demo-0"}, false, false, false),
				Entry("wrong identity cannot replace target", ptr.To[int32](2), []string{"demo-0", "demo-9"}, false, false, false),
				Entry("healthy extra", ptr.To[int32](1), []string{"demo-0", "demo-9"}, false, false, true),
				Entry("unhealthy extra", ptr.To[int32](1), []string{"demo-0", "demo-9"}, true, false, true),
				Entry("Stop just submitted while healthy", ptr.To[int32](2), []string{"demo-0", "demo-1"}, false, true, true),
				Entry("Stop partially completed", ptr.To[int32](2), []string{"demo-0"}, false, true, false),
				Entry("Stop fully completed", ptr.To[int32](2), []string{}, false, true, false),
			)

			It("keeps each condition's history while evaluating the current availability policy", func() {
				its, tree := newTree(ptr.To[int32](1), "demo-0")
				its.Spec.MinReadySeconds = 3600
				readyTime := metav1.NewTime(time.Now().Add(-2 * time.Minute))
				availableTime := metav1.NewTime(time.Now().Add(-3 * time.Minute))
				its.Status.Conditions = []metav1.Condition{
					{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue, LastTransitionTime: readyTime},
					{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionFalse, LastTransitionTime: availableTime},
				}
				reconcile(tree)
				ready := func() *metav1.Condition {
					return meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady))
				}
				available := func() *metav1.Condition {
					return meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceAvailable))
				}
				Expect(ready().LastTransitionTime.Equal(&readyTime)).Should(BeTrue())
				Expect(available().LastTransitionTime.Equal(&availableTime)).Should(BeTrue())
				Expect(available().Status).Should(Equal(metav1.ConditionFalse))

				its.Generation++
				its.Spec.MinReadySeconds = 0
				for _, object := range tree.List(&workloads.Instance{}) {
					inst := object.(*workloads.Instance)
					inst.Spec.MinReadySeconds = 7200
					inst.Status.Available = false
				}
				reconcile(tree)
				Expect(ready().LastTransitionTime.Equal(&readyTime)).Should(BeTrue())
				Expect(available().Status).Should(Equal(metav1.ConditionTrue))
				Expect(available().LastTransitionTime.After(availableTime.Time)).Should(BeTrue())
				Expect(available().ObservedGeneration).Should(Equal(its.Generation))

				its.Generation++
				its.Spec.MinReadySeconds = 7200
				reconcile(tree)
				Expect(ready().LastTransitionTime.Equal(&readyTime)).Should(BeTrue())
				Expect(available().Status).Should(Equal(metav1.ConditionFalse))
				for _, object := range tree.List(&corev1.Pod{}) {
					object.(*corev1.Pod).Status.Conditions[0].LastTransitionTime = metav1.NewTime(time.Now().Add(-3 * time.Hour))
				}
				reconcile(tree)
				Expect(available().Status).Should(Equal(metav1.ConditionTrue))
				Expect(ready().LastTransitionTime.Equal(&readyTime)).Should(BeTrue())
			})

			It("reports extra member failures while readiness follows required identities", func() {
				its, tree := newTree(ptr.To[int32](1), "demo-0", "demo-9")
				for _, object := range tree.List(&corev1.Pod{}) {
					pod := object.(*corev1.Pod)
					if pod.Name == "demo-9" {
						pod.Status.Phase = corev1.PodFailed
						pod.Status.Conditions[0].Status = corev1.ConditionFalse
					}
				}
				for _, object := range tree.List(&workloads.Instance{}) {
					inst := object.(*workloads.Instance)
					if inst.Name == "demo-9" {
						inst.Status.Conditions = append(inst.Status.Conditions, metav1.Condition{Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue})
					}
				}
				reconcile(tree)
				Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
				Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
				Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceFailure))).Should(BeTrue())
				Expect(its.Status.Replicas).Should(Equal(int32(2)))
				Expect(its.Status.ReadyReplicas).Should(Equal(int32(1)))
				Expect(its.FindInstanceStatus("demo-9").Failed).Should(BeTrue())
				for _, object := range tree.List(&corev1.Pod{}) {
					if object.GetName() == "demo-9" {
						now := metav1.Now()
						object.SetDeletionTimestamp(&now)
					}
				}
				for _, object := range tree.List(&workloads.Instance{}) {
					if object.GetName() == "demo-9" {
						now := metav1.Now()
						object.SetDeletionTimestamp(&now)
						object.(*workloads.Instance).Status.CurrentState = workloads.InstanceCurrentStateTerminating
					}
				}
				reconcile(tree)
				Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
				Expect(its.Status.ReadyReplicas).Should(Equal(int32(1)))
				for _, object := range tree.List(&corev1.Pod{}) {
					if object.GetName() == "demo-0" {
						now := metav1.Now()
						object.SetDeletionTimestamp(&now)
					}
				}
				reconcile(tree)
				Expect(meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
				Expect(its.Status.ReadyReplicas).Should(BeZero())
				Expect(its.FindInstanceStatus("demo-0").Ready).Should(BeFalse())
			})

			It("keeps healthy history through a Stop/Start reversal before runtime removal", func() {
				var cli client.Client
				var read func() *workloads.InstanceSet
				var mutate func(func(*workloads.InstanceSet))
				var parent func()
				if instanceAPI {
					cli = newSetStopClient(GinkgoT(), setStopFixture())
					reconcileSetStop(GinkgoT(), cli, 5)
					bindSetStopClaims(GinkgoT(), cli)
					markSetStopPodsReady(GinkgoT(), cli)
					reconcileSetStop(GinkgoT(), cli, 5)
					read = func() *workloads.InstanceSet { return readSetStop(GinkgoT(), cli) }
					mutate = func(change func(*workloads.InstanceSet)) { mutateSetStop(GinkgoT(), cli, change) }
					parent = func() { _, err := reconcileSetStopParent(cli); Expect(err).ShouldNot(HaveOccurred()) }
				} else {
					cli = legacyStopRecoveryFixture(GinkgoT(), 2, 0)
					legacyStopRecoveryReady(GinkgoT(), cli)
					read = func() *workloads.InstanceSet { return legacyStopRecoverySet(GinkgoT(), cli) }
					mutate = func(change func(*workloads.InstanceSet)) { legacyStopRecoveryChange(GinkgoT(), cli, change) }
					parent = func() { legacyStopRecoveryReconcile(GinkgoT(), cli, 1) }
				}
				before := *meta.FindStatusCondition(read().Status.Conditions, string(workloads.InstanceReady))
				for _, stop := range []bool{true, false} {
					mutate(func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(stop) })
					parent()
					condition := meta.FindStatusCondition(read().Status.Conditions, string(workloads.InstanceReady))
					Expect(condition.Status).Should(Equal(metav1.ConditionTrue))
					Expect(condition.LastTransitionTime.Equal(&before.LastTransitionTime)).Should(BeTrue())
				}
				parent()
				Expect(read().Status.ReadyReplicas).Should(Equal(int32(2)))
			})

			It("becomes Available on its scheduled retry without another object event", func() {
				var cli client.Client
				var parent func() (ctrl.Result, error)
				if instanceAPI {
					its := setStopFixture()
					its.Spec.MinReadySeconds = 1
					cli = newSetStopClient(GinkgoT(), its)
					reconcileSetStop(GinkgoT(), cli, 5)
					bindSetStopClaims(GinkgoT(), cli)
					markSetStopPodsReady(GinkgoT(), cli)
					for _, pod := range listSetStopPods(GinkgoT(), cli) {
						pod.Status.Conditions[0].LastTransitionTime = metav1.Now()
						Expect(cli.Status().Update(context.Background(), &pod)).Should(Succeed())
					}
					reconcileSetStop(GinkgoT(), cli, 5)
					parent = func() (ctrl.Result, error) { return reconcileSetStopParent(cli) }
				} else {
					cli = legacyStopRecoveryFixture(GinkgoT(), 2, 1)
					legacyStopRecoveryReady(GinkgoT(), cli)
					parent = func() (ctrl.Result, error) {
						r := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
						return r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "legacy-stop"}})
					}
				}
				result, err := parent()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(result.RequeueAfter).Should(Equal(time.Second))
				time.Sleep(result.RequeueAfter + 100*time.Millisecond)
				_, err = parent()
				Expect(err).ShouldNot(HaveOccurred())
				var its *workloads.InstanceSet
				if instanceAPI {
					its = readSetStop(GinkgoT(), cli)
				} else {
					its = legacyStopRecoverySet(GinkgoT(), cli)
				}
				Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
			})

			DescribeTable("keeps availability wakeups for extra runtime while paused", func(target int32) {
				var cli client.Client
				var read func() *workloads.InstanceSet
				var mutate func(func(*workloads.InstanceSet))
				var parent func() (ctrl.Result, error)
				var pods []corev1.Pod
				if instanceAPI {
					cli = newSetStopClient(GinkgoT(), setStopFixture())
					reconcileSetStop(GinkgoT(), cli, 5)
					bindSetStopClaims(GinkgoT(), cli)
					markSetStopPodsReady(GinkgoT(), cli)
					reconcileSetStop(GinkgoT(), cli, 5)
					read = func() *workloads.InstanceSet { return readSetStop(GinkgoT(), cli) }
					mutate = func(change func(*workloads.InstanceSet)) { mutateSetStop(GinkgoT(), cli, change) }
					parent = func() (ctrl.Result, error) { return reconcileSetStopParent(cli) }
					pods = listSetStopPods(GinkgoT(), cli)
				} else {
					cli = legacyStopRecoveryFixture(GinkgoT(), 2, 0)
					legacyStopRecoveryReady(GinkgoT(), cli)
					read = func() *workloads.InstanceSet { return legacyStopRecoverySet(GinkgoT(), cli) }
					mutate = func(change func(*workloads.InstanceSet)) { legacyStopRecoveryChange(GinkgoT(), cli, change) }
					parent = func() (ctrl.Result, error) {
						r := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
						return r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(read())})
					}
					pods = legacyStopRecoveryPods(GinkgoT(), cli)
				}
				Expect(read().Status.AvailableReplicas).Should(Equal(int32(2)))
				mutate(func(its *workloads.InstanceSet) {
					its.Spec.Paused = true
					its.Spec.Replicas = ptr.To(target)
					its.Spec.MinReadySeconds = 3600
				})
				var extraName string
				for i := range pods {
					pod := &pods[i]
					pod.Status.Conditions[0].LastTransitionTime = metav1.NewTime(time.Now().Add(-2 * time.Hour))
					if strings.HasSuffix(pod.Name, "-1") {
						pod.Status.Conditions[0].LastTransitionTime = metav1.Now()
						extraName = pod.Name
					}
					Expect(cli.Status().Update(context.Background(), pod)).Should(Succeed())
				}
				Expect(extraName).ShouldNot(BeEmpty())
				var result ctrl.Result
				for i := 0; i < 5; i++ {
					var err error
					result, err = parent()
					Expect(err).ShouldNot(HaveOccurred())
				}
				its := read()
				Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
				Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
				Expect(its.Status.Replicas).Should(Equal(int32(2)))
				Expect(its.Status.ReadyReplicas).Should(Equal(int32(2)))
				Expect(its.Status.AvailableReplicas).Should(Equal(int32(1)))
				Expect(its.FindInstanceStatus(extraName).Available).Should(BeFalse())
				Expect(result.RequeueAfter).Should(Equal(time.Second))
			}, Entry("positive target with an extra", int32(1)), Entry("zero target with extras", int32(0)))
		})
	}
})

var _ = Describe("InstanceSet Controller", func() {
	var (
		itsName = "test-instance-set"
		itsObj  *workloads.InstanceSet
		itsKey  client.ObjectKey
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ServiceSignature, true, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	createITSObj := func(name string, processors ...func(factory *testapps.MockInstanceSetFactory)) {
		By("create a ITS object")
		container := corev1.Container{
			Name:  "foo",
			Image: "bar",
		}
		f := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace, name, "test-cluster", "comp").
			WithRandomName().
			AddContainer(container).
			SetReplicas(1)
		for _, processor := range processors {
			if processor != nil {
				processor(f)
			}
		}
		itsObj = f.Create(&testCtx).GetObject()
		itsKey = client.ObjectKeyFromObject(itsObj)

		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, set *workloads.InstanceSet) {
			g.Expect(set.Status.ObservedGeneration).Should(BeEquivalentTo(1))
		}),
		).Should(Succeed())
	}

	mockPodReady := func(podNames ...string) []*corev1.Pod {
		By("mock pods ready")
		pods := make([]*corev1.Pod, 0)
		for _, podName := range podNames {
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      podName,
			}
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, podKey, func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = []corev1.PodCondition{
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
					},
				}
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{
						Name: pod.Spec.Containers[0].Name,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
						Image: pod.Spec.Containers[0].Image,
					},
				}
				pods = append(pods, pod)
			})()).Should(Succeed())
		}
		return pods
	}

	Context("reconciliation", func() {
		It("should reconcile well", func() {
			name := "test-instance-set"
			port := int32(12345)
			commonLabels := map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppNameLabelKey:        "ClusterDefName",
				constant.AppComponentLabelKey:   "CompDefName",
				constant.AppInstanceLabelKey:    "clusterName",
				constant.KBAppComponentLabelKey: "componentName",
			}
			pod := builder.NewPodBuilder(testCtx.DefaultNamespace, "foo").
				AddLabelsInMap(commonLabels).
				AddContainer(corev1.Container{
					Name:  "foo",
					Image: "bar",
					Ports: []corev1.ContainerPort{
						{
							Name:          "foo",
							Protocol:      corev1.ProtocolTCP,
							ContainerPort: port,
						},
					},
				}).GetObject()
			template := corev1.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			}
			its := builder.NewInstanceSetBuilder(testCtx.DefaultNamespace, name).
				SetSelectorMatchLabel(commonLabels).
				SetTemplate(template).
				GetObject()
			viper.Set(constant.KBToolsImage, "kb-tool-image")
			Expect(k8sClient.Create(ctx, its)).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(its),
				func(g Gomega, set *workloads.InstanceSet) {
					g.Expect(set.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				}),
			).Should(Succeed())
			Expect(k8sClient.Delete(ctx, its)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(its), &workloads.InstanceSet{}, false)).
				Should(Succeed())
		})

		It("rolling", func() {
			replicas := int32(3)
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetReplicas(replicas).
					SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
						Type: kbappsv1.RollingUpdateStrategyType,
						RollingUpdate: &workloads.RollingUpdate{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 1, // one instance at a time
							},
						},
					}).SetPodManagementPolicy(appsv1.ParallelPodManagement)
			})

			podsKey := []types.NamespacedName{
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-0", itsObj.Name),
				},
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-1", itsObj.Name),
				},
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-2", itsObj.Name),
				},
			}
			mockPodReady(podsKey[0].Name, podsKey[1].Name, podsKey[2].Name)

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			By("update its spec")
			beforeUpdate := time.Now()
			time.Sleep(1 * time.Second)
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
			})()).ShouldNot(HaveOccurred())

			for i := replicas; i > 0; i-- {
				By("wait new pod created")
				podKey := podsKey[i-1]
				Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
					g.Expect(pod.CreationTimestamp.After(beforeUpdate)).Should(BeTrue())
				})).Should(Succeed())

				// mock new pod ready
				mockPodReady(podKey.Name)

				By(fmt.Sprintf("check its status updated: %s", podKey.Name))
				Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
					g.Expect(its.Status.UpdatedReplicas).Should(Equal(replicas - i + 1))
				})).Should(Succeed())
			}

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())
		})
	})

	Context("PVC retention policy", func() {
		var (
			pvc = corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "data",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			}
		)

		It("provision", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc)
			})

			By("check pods created")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())

			By("check PVCs created")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
		})

		It("when deleted - delete", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("delete the ITS object")
			Expect(k8sClient.Delete(ctx, itsObj)).Should(Succeed())

			By("check its object NOT deleted")
			Consistently(testapps.CheckObjExists(&testCtx, itsKey, &workloads.InstanceSet{}, true)).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs deleted, but the pvc-protection finalizer prevent the pvc to be deleted physically")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
				g.Expect(pvc.Finalizers).To(HaveLen(1))
				g.Expect(pvc.Finalizers[0]).To(Equal("kubernetes.io/pvc-protection"))
			})).Should(Succeed())
		})

		It("when deleted - retain", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenDeleted: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("delete the ITS object")
			Expect(k8sClient.Delete(ctx, itsObj)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, itsKey, &workloads.InstanceSet{}, false)).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs retained and not deleted")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Consistently(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).Should(BeNil())
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				// verify owner references are cleared to prevent garbage collection
				ownerRefs := pvc.GetOwnerReferences()
				g.Expect(ownerRefs).Should(HaveLen(0), "Owner references should be cleared when retention policy is Retain")
			})).Should(Succeed())
		})

		It("when scaled - delete", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenScaled: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("scale-in")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(0))
			})()).ShouldNot(HaveOccurred())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs deleted, but the pvc-protection finalizer prevent the pvc to be deleted physically")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
				g.Expect(pvc.Finalizers).To(HaveLen(1))
				g.Expect(pvc.Finalizers[0]).To(Equal("kubernetes.io/pvc-protection"))
			})).Should(Succeed())
		})

		It("when scaled - retain", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("scale-in")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(0))
			})()).ShouldNot(HaveOccurred())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs retained and not deleted")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Consistently(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).Should(BeNil())
				// verify owner references still exist since InstanceSet is not deleted
				ownerRefs := pvc.GetOwnerReferences()
				g.Expect(ownerRefs).Should(HaveLen(1), "Owner references should still exist when scaling down and retention policy is Retain")
				g.Expect(ownerRefs[0].Kind).Should(Equal("InstanceSet"))
			})).Should(Succeed())
		})
	})

	Context("reconfigure", func() {
		var (
			supportResizeSubResource func() (bool, error)
		)

		BeforeEach(func() {
			supportResizeSubResource = intctrlutil.SupportResizeSubResource
			intctrlutil.SupportResizeSubResource = func() (bool, error) { return true, nil }
		})

		AfterEach(func() {
			intctrlutil.SupportResizeSubResource = supportResizeSubResource
		})

		It("instance status", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			By("check pod config annotation")
			configMap := map[string]string{
				"log":    "123456",
				"server": "123456",
			}
			configVal, err := json.Marshal(configMap)
			Expect(err).NotTo(HaveOccurred())
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Annotations).Should(HaveKeyWithValue(constant.CMInsConfigurationHashLabelKey, string(configVal)))
			})).Should(Succeed())

			By("check instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
		})

		It("reconfigure", func() {
			By("mock reconfigure action calls")
			var (
				reconfigure string
				parameters  map[string]string
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
					if req.Action == "reconfigure" || strings.HasPrefix(req.Action, "udf-reconfigure") {
						reconfigure = req.Action
						parameters = req.Parameters
					}
					return kbaproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))

			By("check the reconfigure action NOT called")
			Consistently(func(g Gomega) {
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to reconfigure")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[0].ReconfigureActionName = ""
				its.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(Equal("reconfigure"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
		})

		It("reconfigure - udf", func() {
			By("mock reconfigure action calls")
			var (
				reconfigure string
				parameters  map[string]string
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
					if req.Action == "reconfigure" || strings.HasPrefix(req.Action, "udf-reconfigure") {
						reconfigure = req.Action
						parameters = req.Parameters
					}
					return kbaproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))

			By("check the reconfigure action NOT called")
			Consistently(func(g Gomega) {
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to reconfigure")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[0].ReconfigureActionName = "reconfigure-server"
				its.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(ContainSubstring("reconfigure-server"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
		})

		It("restart", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			pods := mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))
			Expect(pods).Should(HaveLen(1))

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to restart")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Restart = ptr.To(true)
			})()).ShouldNot(HaveOccurred())

			By("check the instance restarted")
			podKey := client.ObjectKeyFromObject(pods[0])
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(pods[0].UID))
			})).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
		})

		It("reconfigure & restart", func() {
			By("mock reconfigure action calls")
			var (
				reconfigure          string
				parameters           map[string]string
				reconfigureTimestamp time.Time
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
					if req.Action == "reconfigure" || strings.HasPrefix(req.Action, "udf-reconfigure") {
						reconfigure = req.Action
						parameters = req.Parameters
						reconfigureTimestamp = time.Now()
						time.Sleep(1200 * time.Millisecond) // The precision of pod creationTimestamp is only in seconds.
					}
					return kbaproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			pods := mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))
			Expect(pods).Should(HaveLen(1))

			By("check the reconfigure action NOT called")
			Consistently(func(g Gomega) {
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to reconfigure and restart")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Restart = ptr.To(true)
				its.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[0].ReconfigureActionName = ""
				its.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(Equal("reconfigure"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			By("check the instance restarted")
			podKey := client.ObjectKeyFromObject(pods[0])
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(pods[0].UID))
				// Check that the new pod is created after reconfigure action called.
				g.Expect(pod.CreationTimestamp.Time.After(reconfigureTimestamp)).Should(BeTrue())
			})).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
		})

		It("reconfigure and restart", func() {
			By("mock reconfigure action calls")
			var (
				cnt                  int32
				reconfigure          string
				parameters           map[string]string
				reconfigureTimestamp time.Time
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
					if req.Action == "reconfigure" || strings.HasPrefix(req.Action, "udf-reconfigure") {
						cnt += 1
						reconfigure = req.Action
						parameters = req.Parameters
						reconfigureTimestamp = time.Now()
						time.Sleep(1200 * time.Millisecond) // The precision of pod creationTimestamp is only in seconds.
					}
					return kbaproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "client",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			pods := mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))
			Expect(pods).Should(HaveLen(1))

			By("check the reconfigure action NOT called")
			Consistently(func(g Gomega) {
				g.Expect(cnt).Should(Equal(int32(0)))
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "client",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to reconfigure and restart")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[0].ReconfigureActionName = ""
				its.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
				its.Spec.Configs[1].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[1].Restart = ptr.To(true)
				its.Spec.Configs[2].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[2].Restart = ptr.To(true)
				its.Spec.Configs[2].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[2].ReconfigureActionName = ""
				its.Spec.Configs[2].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(cnt).Should(Equal(int32(2)))
				g.Expect(reconfigure).Should(Equal("reconfigure"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			By("check the instance restarted")
			podKey := client.ObjectKeyFromObject(pods[0])
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(pods[0].UID))
				// Check that the new pod is created after reconfigure action called.
				g.Expect(pod.CreationTimestamp.Time.After(reconfigureTimestamp)).Should(BeTrue())
			})).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "client",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("abcdef"),
						},
					},
				}))
			})).Should(Succeed())
		})
	})

	Context("pod naming rule", func() {
		checkPodOrdinal := func(ordinals []int, checkFunc func(podKey types.NamespacedName)) {
			for _, ordinal := range ordinals {
				podKey := types.NamespacedName{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%v-%v", itsObj.Name, ordinal),
				}
				checkFunc(podKey)
			}
		}

		eventuallyExist := func(podKey types.NamespacedName) {
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())
		}

		eventuallyNotExist := func(podKey types.NamespacedName) {
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())
		}

		consistentlyExist := func(podKey types.NamespacedName) {
			Consistently(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())
		}

		consistentlyNotExist := func(podKey types.NamespacedName) {
			Consistently(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())
		}

		It("works with FlatInstanceOrdinal", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetFlatInstanceOrdinal(true)
				f.SetPodManagementPolicy(appsv1.ParallelPodManagement)
				f.SetReplicas(3)
			})

			checkPodOrdinal([]int{0, 1, 2}, eventuallyExist)
			mockPodReady(itsObj.Name+"-0", itsObj.Name+"-1", itsObj.Name+"-2")
			By("check its status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.AssignedOrdinals).Should(HaveKey(instancetemplate.DefaultTemplateName))
				g.Expect(its.Status.AssignedOrdinals[instancetemplate.DefaultTemplateName].Discrete).Should(HaveExactElements(int32(0), int32(1), int32(2)))
			})).Should(Succeed())

			// offline one instance
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To[int32](2)
				its.Spec.OfflineInstances = []string{itsObj.Name + "-1"}
			})()).Should(Succeed())
			checkPodOrdinal([]int{1}, eventuallyNotExist)
			By("check its status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.AssignedOrdinals).Should(HaveKey(instancetemplate.DefaultTemplateName))
				g.Expect(its.Status.AssignedOrdinals[instancetemplate.DefaultTemplateName].Discrete).Should(HaveExactElements(int32(0), int32(2)))
			})).Should(Succeed())

			// scale up
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To[int32](4)
			})()).Should(Succeed())
			checkPodOrdinal([]int{0, 2, 3, 4}, eventuallyExist)
			mockPodReady(itsObj.Name+"-3", itsObj.Name+"-4")
			By("check its status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.AssignedOrdinals).Should(HaveKey(instancetemplate.DefaultTemplateName))
				g.Expect(its.Status.AssignedOrdinals[instancetemplate.DefaultTemplateName].Discrete).Should(HaveExactElements(int32(0), int32(2), int32(3), int32(4)))
			})).Should(Succeed())

			// delete OfflineInstances will not affect running instances
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.OfflineInstances = []string{}
			})()).Should(Succeed())
			checkPodOrdinal([]int{0, 2, 3, 4}, consistentlyExist)
			checkPodOrdinal([]int{1}, consistentlyNotExist)
			By("check its status")
			Consistently(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.AssignedOrdinals).Should(HaveKey(instancetemplate.DefaultTemplateName))
				g.Expect(its.Status.AssignedOrdinals[instancetemplate.DefaultTemplateName].Discrete).Should(HaveExactElements(int32(0), int32(2), int32(3), int32(4)))
			})).Should(Succeed())
		})
	})

	Context("start & stop", func() {
		var (
			pvc = corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "data",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			}
		)

		BeforeEach(func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetFlatInstanceOrdinal(true).
					AddVolumeClaimTemplate(pvc)
			})

			By("check pods created")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())

			By("check PVCs created")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
		})

		It("stop", func() {
			By("stop the its")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
			})()).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs still exist")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Consistently(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
		})

		It("start", func() {
			By("stop the its first")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
			})()).Should(Succeed())

			By("stop the its")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
			})()).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("start it")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = nil
			})()).Should(Succeed())

			By("check pods created")
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())
		})

		It("stop & start - discrete ordinals", func() {
			By("scale up to 3 replicas")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
				its.Spec.Replicas = ptr.To(int32(3))
			})()).Should(Succeed())

			By("check pods created and mock them ready")
			for i := 0; i < 3; i++ {
				podKey := types.NamespacedName{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-%d", itsObj.Name, i),
				}
				Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())

				mockPodReady(podKey.Name)
			}

			By("offline instance 1")
			offlineOrdinal := 1
			offlinePodKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%d", itsObj.Name, offlineOrdinal),
			}
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(2))
				its.Spec.OfflineInstances = []string{offlinePodKey.Name}
			})()).Should(Succeed())

			By("check instance 1 offline")
			Eventually(testapps.CheckObjExists(&testCtx, offlinePodKey, &corev1.Pod{}, false)).Should(Succeed())

			By("cleanup offline instances & stop the its")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.OfflineInstances = nil
				its.Spec.Stop = ptr.To(true)
			})()).Should(Succeed())

			By("check pods deleted")
			for i := 0; i < 3; i++ {
				podKey := types.NamespacedName{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-%d", itsObj.Name, i),
				}
				Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())
			}

			By("start it")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = nil
			})()).Should(Succeed())

			By("check pods created")
			for i := 0; i < 3; i++ {
				podKey := types.NamespacedName{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-%d", itsObj.Name, i),
				}
				if i == offlineOrdinal {
					Consistently(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())
				} else {
					Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())
				}
			}
		})
	})

	Context("deferred update", func() {
		It("handles serviceaccount update", func() {
			createITSObj(itsName, func(factory *testapps.MockInstanceSetFactory) {
				factory.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				})
			})
			By("update proposed sa name")
			newSAName := "new-sa"
			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				if its.Annotations == nil {
					its.Annotations = map[string]string{}
				}
				its.Annotations[constant.ProposedServiceAccountNameAnnotationKey] = newSAName
			})).Should(Succeed())
			podsKey := []types.NamespacedName{
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-0", itsObj.Name),
				},
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-1", itsObj.Name),
				},
			}
			mockPodReady(podsKey[0].Name)
			Consistently(testapps.CheckObj(&testCtx, podsKey[0], func(g Gomega, pod *corev1.Pod) {
				// default sa name is empty
				g.Expect(pod.Spec.ServiceAccountName).Should(BeEmpty())
			})).Should(Succeed())
			By("check scale out does not affect current pod")
			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To[int32](2)
			})).Should(Succeed())
			Consistently(testapps.CheckObj(&testCtx, podsKey[0], func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.ServiceAccountName).Should(BeEmpty())
			})).Should(Succeed())
			By("check new pod uses new sa name")
			mockPodReady(podsKey[1].Name)
			Eventually(testapps.CheckObj(&testCtx, podsKey[1], func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.ServiceAccountName).Should(Equal(newSAName))
			})).Should(Succeed())

			beforeUpdate := time.Now()
			time.Sleep(time.Second)
			By("update its spec")
			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.Containers[0].Command = []string{"new-command"}
			})).Should(Succeed())

			replicas := 2
			for i := replicas; i > 0; i-- {
				By("wait new pod created")
				podKey := podsKey[i-1]
				Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
					g.Expect(pod.CreationTimestamp.After(beforeUpdate)).Should(BeTrue())
					g.Expect(pod.Spec.ServiceAccountName).Should(Equal(newSAName))
				})).Should(Succeed())

				mockPodReady(podKey.Name)
			}

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
				g.Expect(its.Annotations).Should(HaveKeyWithValue(constant.ServiceAccountInUseAnnotationKey, newSAName))
			})).Should(Succeed())
		})
	})

	Context("readiness", func() {
		It("reports a fresh Ready transition after stop and resume", func() {
			t := GinkgoT()
			cli := legacyStopRecoveryFixture(t, 2, 0)
			legacyStopRecoveryReady(t, cli)
			before := ageInstanceSetReadyCondition(cli, legacyStopRecoverySet(t, cli))

			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
			legacyStopRecoveryReconcile(t, cli, 6)
			expectInstanceSetNoRuntime(legacyStopRecoverySet(t, cli))

			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
			legacyStopRecoveryReconcile(t, cli, 1)
			expectInstanceSetNotReady(legacyStopRecoverySet(t, cli))
			legacyStopRecoveryReconcile(t, cli, 1)
			legacyStopRecoveryReady(t, cli)
			conditions := legacyStopRecoverySet(t, cli).Status.Conditions
			Expect(meta.IsStatusConditionTrue(conditions, string(workloads.InstanceReady))).Should(BeTrue())
			Expect(meta.FindStatusCondition(conditions, string(workloads.InstanceReady)).LastTransitionTime.After(before.Time)).Should(BeTrue())
		})

		It("reports Ready and Available through explicit zero-target scale transitions", func() {
			t := GinkgoT()
			cli := legacyStopRecoveryFixture(t, 0, 0)
			its := legacyStopRecoverySet(t, cli)
			Expect(its.Status.Replicas).Should(BeZero())
			Expect(its.Status.ReadyReplicas).Should(BeZero())
			Expect(its.Status.AvailableReplicas).Should(BeZero())
			Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
			Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
			before := ageInstanceSetReadyCondition(cli, its)

			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To[int32](1) })
			legacyStopRecoveryReconcile(t, cli, 3)
			its = legacyStopRecoverySet(t, cli)
			expectInstanceSetNotReady(its)
			Expect(meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
			Expect(its.Status.ObservedGeneration).Should(Equal(its.Generation))
			Expect(meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady)).ObservedGeneration).Should(Equal(its.Generation))
			Expect(meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceAvailable)).ObservedGeneration).Should(Equal(its.Generation))
			legacyStopRecoveryReady(t, cli)
			its = legacyStopRecoverySet(t, cli)
			Expect(its.Status.Replicas).Should(Equal(int32(1)))
			Expect(its.Status.ReadyReplicas).Should(Equal(int32(1)))
			Expect(its.Status.AvailableReplicas).Should(Equal(int32(1)))
			Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
			Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
			Expect(meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady)).LastTransitionTime.After(before.Time)).Should(BeTrue())

			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To[int32](0) })
			legacyStopRecoveryReconcile(t, cli, 6)
			its = legacyStopRecoverySet(t, cli)
			Expect(legacyStopRecoveryPods(t, cli)).Should(BeEmpty())
			Expect(its.Status.Replicas).Should(BeZero())
			Expect(its.Status.ReadyReplicas).Should(BeZero())
			Expect(its.Status.AvailableReplicas).Should(BeZero())
			Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
			Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
		})

		It("reports Ready before MinReadySeconds makes pods Available", func() {
			t := GinkgoT()
			cli := legacyStopRecoveryFixture(t, 2, 3600)
			legacyStopRecoveryReady(t, cli)
			conditions := legacyStopRecoverySet(t, cli).Status.Conditions
			Expect(meta.IsStatusConditionTrue(conditions, string(workloads.InstanceReady))).Should(BeTrue())
			Expect(meta.IsStatusConditionFalse(conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
		})

		It("publishes the missing desired pod before recreating it", func() {
			t := GinkgoT()
			cli := legacyStopRecoveryFixture(t, 2, 0)
			legacyStopRecoveryReady(t, cli)
			pod := &legacyStopRecoveryPods(t, cli)[0]
			pod.Finalizers = nil
			Expect(cli.Update(context.Background(), pod)).Should(Succeed())
			Expect(cli.Delete(context.Background(), pod)).Should(Succeed())
			legacyStopRecoveryReconcile(t, cli, 1)

			its := legacyStopRecoverySet(t, cli)
			Expect(meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
			Expect(meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
			Expect(meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady)).Message).Should(ContainSubstring(pod.Name))
		})

		It("preserves the Ready transition across unrelated generation changes", func() {
			t := GinkgoT()
			cli := legacyStopRecoveryFixture(t, 2, 0)
			legacyStopRecoveryReady(t, cli)
			before := ageInstanceSetReadyCondition(cli, legacyStopRecoverySet(t, cli))
			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
			})
			legacyStopRecoveryReconcile(t, cli, 2)

			conditions := legacyStopRecoverySet(t, cli).Status.Conditions
			Expect(meta.IsStatusConditionTrue(conditions, string(workloads.InstanceReady))).Should(BeTrue())
			Expect(meta.FindStatusCondition(conditions, string(workloads.InstanceReady)).LastTransitionTime.Equal(&before)).Should(BeTrue())
		})
	})
})

func instanceStatusWithoutRevisionAndHealth(status workloads.InstanceStatus) workloads.InstanceStatus {
	status.CurrentRevision = ""
	status.UpdateRevision = ""
	status.UpToDate = false
	status.Ready = false
	status.Available = false
	status.Failed = false
	return status
}

func TestInstanceSetFlatOrdinalTransfer(t *testing.T) {
	for _, swap := range []bool{false, true} {
		for _, failStatus := range []bool{false, true} {
			t.Run(fmt.Sprintf("swap=%t/statusFailure=%t", swap, failStatus), func(t *testing.T) {
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
					if err := add(scheme); err != nil {
						t.Fatal(err)
					}
				}
				model.AddScheme(workloads.AddToScheme)
				its := &workloads.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{Name: "flat-transfer", Namespace: "default", UID: "its-uid", Generation: 1},
					Spec: workloads.InstanceSetSpec{
						Replicas: ptr.To(int32(1)), FlatInstanceOrdinal: true,
						PodManagementPolicy:    appsv1.ParallelPodManagement,
						InstanceUpdateStrategy: &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType},
						Selector:               &metav1.LabelSelector{MatchLabels: map[string]string{"app": "flat-transfer"}},
						Template:               corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "flat-transfer"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
						Instances:              []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}}},
						VolumeClaimTemplates:   []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}},
					},
				}
				if swap {
					its.Spec.Replicas = ptr.To(int32(2))
					its.Spec.Instances = append(its.Spec.Instances, workloads.InstanceTemplate{Name: "b", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{1}}})
					its.Spec.InstanceUpdateStrategy = nil
				}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
				key := client.ObjectKeyFromObject(its)
				reconcile := func(c client.Client) error {
					_, err := (&InstanceSetReconciler{Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
					return err
				}
				run := func() {
					t.Helper()
					if err := reconcile(cli); err != nil {
						t.Fatal(err)
					}
				}
				for i := 0; i < 3; i++ {
					run()
				}
				pod := &corev1.Pod{}
				if err := cli.Get(context.Background(), client.ObjectKey{Namespace: key.Namespace, Name: its.Name + "-0"}, pod); err != nil {
					t.Fatal(err)
				}
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
				if err := cli.Status().Update(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				pod.Finalizers = []string{"test/hold"}
				if err := cli.Update(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				if err := cli.Delete(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				claim := &corev1.PersistentVolumeClaim{}
				if err := cli.Get(context.Background(), client.ObjectKey{Namespace: key.Namespace, Name: "data-" + pod.Name}, claim); err != nil {
					t.Fatal(err)
				}
				claim.UID = "claim-uid"
				claim.Spec.VolumeName = "bound-volume"
				if err := cli.Update(context.Background(), claim); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(context.Background(), key, its); err != nil {
					t.Fatal(err)
				}
				if swap {
					its.Spec.Instances[0].Ordinals.Discrete = []int32{1}
					its.Spec.Instances[1].Ordinals.Discrete = []int32{0}
				} else {
					its.Spec.Instances[0].Name = "b"
				}
				its.Generation++
				if err := cli.Update(context.Background(), its); err != nil {
					t.Fatal(err)
				}
				expectedTemplate := "b"
				if failStatus {
					injected := errors.New("injected status failure")
					faulty := interceptor.NewClient(cli, interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
						if _, ok := obj.(*workloads.InstanceSet); ok && sub == "status" {
							return injected
						}
						return c.SubResource(sub).Update(ctx, obj, opts...)
					}})
					if err := reconcile(faulty); !errors.Is(err, injected) {
						t.Fatalf("expected status failure, got %v", err)
					}
					persisted := &workloads.InstanceSet{}
					if err := cli.Get(context.Background(), key, persisted); err != nil {
						t.Fatal(err)
					}
					if persisted.Status.ObservedGeneration == its.Generation {
						t.Fatal("status failure was not persisted as an incomplete commit")
					}
					persistedClaim := &corev1.PersistentVolumeClaim{}
					if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), persistedClaim); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(persistedClaim, claim) {
						t.Fatal("status failure changed a terminating Pod's PVC")
					}
					if !swap {
						its.Spec.Instances[0].Name = "c"
						its.Generation++
						if err := cli.Update(context.Background(), its); err != nil {
							t.Fatal(err)
						}
						expectedTemplate = "c"
					}
				}
				for i := 0; i < 3; i++ {
					run()
				}
				current := &corev1.PersistentVolumeClaim{}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), current); err != nil {
					t.Fatalf("allocated claim was deleted: %v", err)
				}
				if current.UID != claim.UID || current.Spec.VolumeName != claim.Spec.VolumeName || !reflect.DeepEqual(current.OwnerReferences, claim.OwnerReferences) || !current.DeletionTimestamp.IsZero() {
					t.Fatal("allocated storage identity changed")
				}
				if err := cli.Get(context.Background(), key, its); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(its.Status.AssignedOrdinals[expectedTemplate].Discrete, []int32{0}) || len(its.Status.AssignedOrdinals) != len(its.Spec.Instances) {
					t.Fatalf("ordinal did not transfer: %#v", its.Status.AssignedOrdinals)
				}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), pod); err != nil {
					t.Fatal(err)
				}
				if pod.DeletionTimestamp.IsZero() {
					t.Fatal("fixture did not retain terminating Pod")
				}
				if !reflect.DeepEqual(current, claim) {
					t.Fatal("transfer changed storage while the old Pod was terminating")
				}
				pod.Finalizers = nil
				if err := cli.Update(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				if failStatus {
					injected := errors.New("injected replacement Pod create failure")
					faulty := interceptor.NewClient(cli, interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
						if replacement, ok := obj.(*corev1.Pod); ok && replacement.Name == pod.Name {
							return injected
						}
						return c.Create(ctx, obj, opts...)
					}})
					if err := reconcile(faulty); !errors.Is(err, injected) {
						t.Fatalf("expected replacement create failure, got %v", err)
					}
					if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), current); err != nil {
						t.Fatal(err)
					}
					if current.Labels[constant.KBAppInstanceTemplateLabelKey] != expectedTemplate || current.UID != claim.UID || current.Spec.VolumeName != claim.Spec.VolumeName || !reflect.DeepEqual(current.OwnerReferences, claim.OwnerReferences) {
						t.Fatal("fixture did not commit the retained PVC transfer before Pod create failure")
					}
					if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), &corev1.Pod{}); !apierrors.IsNotFound(err) {
						t.Fatalf("failed replacement unexpectedly exists: %v", err)
					}
					if !swap {
						if err := cli.Get(context.Background(), key, its); err != nil {
							t.Fatal(err)
						}
						its.Spec.Instances[0].Name = "d"
						its.Generation++
						if err := cli.Update(context.Background(), its); err != nil {
							t.Fatal(err)
						}
						expectedTemplate = "d"
					}
				}
				for i := 0; i < 3; i++ {
					run()
				}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), pod); err != nil {
					t.Fatalf("Pod did not recover: %v", err)
				}
				if !pod.DeletionTimestamp.IsZero() || pod.Labels[instancetemplate.TemplateNameLabelKey] != expectedTemplate {
					t.Fatal("replacement Pod did not use desired template")
				}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), current); err != nil {
					t.Fatal(err)
				}
				if current.UID != claim.UID || current.Spec.VolumeName != claim.Spec.VolumeName || !reflect.DeepEqual(current.OwnerReferences, claim.OwnerReferences) || current.Labels[constant.KBAppInstanceTemplateLabelKey] != expectedTemplate {
					t.Fatal("replacement lost retained storage or its latest template assignment")
				}
				if err := cli.Get(context.Background(), key, its); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(its.Status.AssignedOrdinals[expectedTemplate].Discrete, []int32{0}) {
					t.Fatal("recovery did not publish the latest ordinal assignment")
				}
			})
		}
	}
}

func TestInstanceSetStopDuringFlatOrdinalTransfer(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "stop-transfer", Namespace: "default", UID: "its-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
		Replicas: ptr.To(int32(2)), FlatInstanceOrdinal: true, PodManagementPolicy: appsv1.ParallelPodManagement,
		InstanceUpdateStrategy: &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType},
		Selector:               &metav1.LabelSelector{MatchLabels: map[string]string{"app": "stop-transfer"}},
		Template:               corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "stop-transfer"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
		Instances:              []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(2)), Ordinals: workloads.Ordinals{Discrete: []int32{0, 1}}}},
		VolumeClaimTemplates:   []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
	key := client.ObjectKeyFromObject(its)
	reconcile := func() error {
		_, err := (&InstanceSetReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
		return err
	}
	for i := 0; i < 4; i++ {
		if err := reconcile(); err != nil {
			t.Fatal(err)
		}
	}
	claimsBefore := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claimsBefore); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	if len(its.Status.InstanceStatus) != 2 {
		t.Fatalf("fixture did not publish both historical identities: %#v", its.Status.InstanceStatus)
	}
	for _, status := range its.Status.InstanceStatus {
		if status.TemplateName == nil || *status.TemplateName != "a" {
			t.Fatalf("fixture did not publish the historical template: %#v", status)
		}
	}
	its.Spec.Instances[0].Name = "b"
	its.Spec.Stop = ptr.To(true)
	its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	publishLegacyInstanceSetReadiness(t, cli, its.Name)
	for i := 0; i < 3; i++ {
		if err := reconcile(); err != nil {
			pod := &corev1.Pod{}
			podErr := cli.Get(context.Background(), client.ObjectKey{Namespace: key.Namespace, Name: "stop-transfer-0"}, pod)
			t.Fatalf("Stop round %d failed; remaining Pod get=%v deletion=%v: %v", i, podErr, pod.DeletionTimestamp, err)
		}
	}
	pods := &corev1.PodList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 0 {
		t.Fatalf("Stop did not drain all Pods: %#v", pods.Items)
	}
	claimsAfter := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claimsAfter); err != nil {
		t.Fatal(err)
	}
	if len(claimsBefore.Items) != 2 || !reflect.DeepEqual(claimsBefore.Items, claimsAfter.Items) {
		t.Fatal("Stop changed retained storage")
	}
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	if len(its.Status.InstanceStatus) != 2 {
		t.Fatalf("incomplete stopped identities: %#v", its.Status)
	}
	for _, status := range its.Status.InstanceStatus {
		if status.TemplateName == nil || *status.TemplateName != "a" || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
			t.Fatalf("Stop lost historical template identity: %#v", status)
		}
	}
	its.Spec.Stop = ptr.To(false)
	its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := reconcile(); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 2 {
		t.Fatal("resume did not recreate both transferred identities")
	}
	for _, pod := range pods.Items {
		if pod.Labels[instancetemplate.TemplateNameLabelKey] != "b" {
			t.Fatal("resume used obsolete template")
		}
	}
}

func TestInstanceAPIStopRetainsTransferredIdentity(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	model.AddScheme(workloads.AddToScheme)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-stop-transfer", Namespace: "default", UID: "its-uid", Generation: 1},
		Spec: workloads.InstanceSetSpec{
			EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(int32(1)), FlatInstanceOrdinal: true,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector:            &metav1.LabelSelector{MatchLabels: map[string]string{"app": "instance-stop-transfer"}},
			Template:            corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "instance-stop-transfer"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
			Instances:           []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}).WithObjects(its).Build()
	key := client.ObjectKeyFromObject(its)
	run := func() {
		t.Helper()
		_, err := (&InstanceSetReconciler2{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		run()
	}
	inst := &workloads.Instance{}
	instKey := client.ObjectKey{Namespace: key.Namespace, Name: its.Name + "-0"}
	if err := cli.Get(context.Background(), instKey, inst); err != nil {
		t.Fatal(err)
	}
	inst.UID = "retained-instance-uid"
	if err := cli.Update(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	inst.Status.ObservedGeneration = inst.Generation
	inst.Status.CurrentState = workloads.InstanceCurrentStatePresent
	inst.Status.Ready, inst.Status.Available = true, true
	inst.Status.Conditions = []metav1.Condition{
		{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
		{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
	}
	if err := cli.Status().Update(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	run()
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	its.Spec.Instances[0].Name = "b"
	its.Spec.Stop = ptr.To(true)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		run()
	}
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	status := its.FindInstanceStatus(instKey.Name)
	if status == nil || status.TemplateName == nil || *status.TemplateName != "a" || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStatePresent || status.UpToDate {
		t.Fatalf("Stop lost retained parent status: %#v", status)
	}
	if err := cli.Get(context.Background(), instKey, inst); err != nil {
		t.Fatal(err)
	}
	if inst.UID != "retained-instance-uid" || !inst.DeletionTimestamp.IsZero() || ptr.Deref(inst.Spec.ScaledDown, false) {
		t.Fatal("Stop replaced or retired allocated Instance")
	}
	its.Spec.Stop = ptr.To(false)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		run()
	}
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	status = its.FindInstanceStatus(instKey.Name)
	if status == nil || status.TemplateName == nil || *status.TemplateName != "b" || status.DesiredState != workloads.InstanceDesiredStateActive || status.CurrentState != workloads.InstanceCurrentStatePresent {
		t.Fatalf("resume lost desired parent status: %#v", status)
	}
	if err := cli.Get(context.Background(), instKey, inst); err != nil {
		t.Fatal(err)
	}
	if inst.UID != "retained-instance-uid" || inst.Spec.InstanceTemplateName != "b" {
		t.Fatal("resume replaced identity or missed desired template")
	}
}

func TestInstanceSetTransferredIdentityCleanup(t *testing.T) {
	for _, transfer := range []string{"none", "named", "default"} {
		for _, offline := range []bool{false, true} {
			for _, retain := range []bool{false, true} {
				t.Run(fmt.Sprintf("transfer=%s/offline=%t/retain=%t", transfer, offline, retain), func(t *testing.T) {
					scheme := runtime.NewScheme()
					for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
						if err := add(scheme); err != nil {
							t.Fatal(err)
						}
					}
					its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "cleanup-transfer", Namespace: "default", UID: "its-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
						Replicas: ptr.To(int32(2)), FlatInstanceOrdinal: true, PodManagementPolicy: appsv1.ParallelPodManagement,
						InstanceUpdateStrategy: &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType},
						Selector:               &metav1.LabelSelector{MatchLabels: map[string]string{"app": "cleanup-transfer"}},
						Template:               corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "cleanup-transfer"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
						Instances:              []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(2)), Ordinals: workloads.Ordinals{Discrete: []int32{0, 1}}}},
						VolumeClaimTemplates:   []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}},
					}}
					if retain {
						its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType}
					}
					cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
					key := client.ObjectKeyFromObject(its)
					run := func() {
						t.Helper()
						_, err := (&InstanceSetReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
						if err != nil {
							t.Fatal(err)
						}
					}
					for i := 0; i < 3; i++ {
						run()
					}
					pod := &corev1.Pod{}
					podKey := client.ObjectKey{Namespace: key.Namespace, Name: its.Name + "-0"}
					if err := cli.Get(context.Background(), podKey, pod); err != nil {
						t.Fatal(err)
					}
					pod.Finalizers = []string{"test/hold"}
					if err := cli.Update(context.Background(), pod); err != nil {
						t.Fatal(err)
					}
					claims := &corev1.PersistentVolumeClaimList{}
					if err := cli.List(context.Background(), claims); err != nil {
						t.Fatal(err)
					}
					if len(claims.Items) != 2 {
						t.Fatalf("initial PVC count: %d", len(claims.Items))
					}
					originalClaims := make(map[string]*corev1.PersistentVolumeClaim, len(claims.Items))
					for i := range claims.Items {
						claim := &claims.Items[i]
						claim.UID = types.UID(claim.Name)
						claim.Spec.VolumeName = "bound-" + claim.Name
						if err := cli.Update(context.Background(), claim); err != nil {
							t.Fatal(err)
						}
						originalClaims[claim.Name] = claim.DeepCopy()
					}
					if err := cli.Get(context.Background(), key, its); err != nil {
						t.Fatal(err)
					}
					expectedTemplate := "a"
					switch transfer {
					case "named":
						its.Spec.Instances[0].Name = "b"
						expectedTemplate = "b"
					case "default":
						its.Spec.Instances = nil
						its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{0, 1}}
						expectedTemplate = ""
					}
					its.Generation++
					if err := cli.Update(context.Background(), its); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 3; i++ {
						run()
					}
					if err := cli.Get(context.Background(), key, its); err != nil {
						t.Fatal(err)
					}
					if status := its.FindInstanceStatus(podKey.Name); status == nil || status.TemplateName == nil || *status.TemplateName != expectedTemplate || status.DesiredState != workloads.InstanceDesiredStateActive {
						t.Fatalf("transfer did not publish target identity: %#v", status)
					}
					if err := cli.Get(context.Background(), podKey, pod); err != nil {
						t.Fatal(err)
					}
					if template, ok := instancetemplate.TemplateNameFromLabels(pod.Labels); !ok || template != "a" || !pod.DeletionTimestamp.IsZero() {
						t.Fatal("OnDelete fixture did not preserve the old Pod")
					}
					its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
					wantDesired := workloads.InstanceDesiredStateReleased
					wantClaims := 0
					if offline {
						its.Spec.Replicas = ptr.To(int32(1))
						its.Spec.OfflineInstances = []string{podKey.Name}
						if len(its.Spec.Instances) != 0 {
							its.Spec.Instances[0].Replicas = ptr.To(int32(1))
						}
						wantDesired = workloads.InstanceDesiredStateOffline
						wantClaims = 1
					} else {
						its.Spec.Replicas = ptr.To(int32(0))
						its.Spec.Instances = nil
						its.Spec.Ordinals = workloads.Ordinals{}
					}
					if retain {
						wantClaims = 2
					}
					its.Generation++
					if err := cli.Update(context.Background(), its); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 4; i++ {
						run()
					}
					if err := cli.Get(context.Background(), podKey, pod); err != nil {
						t.Fatal(err)
					}
					if pod.DeletionTimestamp.IsZero() {
						t.Fatal("cleanup never reached the remaining Pod")
					}
					if err := cli.Get(context.Background(), key, its); err != nil {
						t.Fatal(err)
					}
					if status := its.FindInstanceStatus(podKey.Name); status == nil || status.TemplateName == nil || *status.TemplateName != expectedTemplate || status.DesiredState != wantDesired || status.CurrentState != workloads.InstanceCurrentStateTerminating || status.UpToDate {
						t.Fatalf("incorrect retained cleanup status: %#v", status)
					}
					if err := cli.List(context.Background(), claims); err != nil {
						t.Fatal(err)
					}
					if len(claims.Items) != wantClaims {
						t.Fatalf("PVC cleanup policy: got %d claims, want %d", len(claims.Items), wantClaims)
					}
					for _, claim := range claims.Items {
						original := originalClaims[claim.Name]
						if original == nil || claim.UID != original.UID || claim.Spec.VolumeName != original.Spec.VolumeName || !reflect.DeepEqual(claim.OwnerReferences, original.OwnerReferences) {
							t.Fatalf("retained PVC identity changed: %#v", claim)
						}
					}
					pod.Finalizers = nil
					if err := cli.Update(context.Background(), pod); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 2; i++ {
						run()
					}
					if err := cli.Get(context.Background(), key, its); err != nil {
						t.Fatal(err)
					}
					status := its.FindInstanceStatus(podKey.Name)
					if offline {
						if status == nil || status.TemplateName == nil || *status.TemplateName != expectedTemplate || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
							t.Fatalf("offline identity was lost after deletion: %#v", status)
						}
					} else if status != nil {
						t.Fatalf("released identity survived deletion: %#v", status)
					}
				})
			}
		}
	}
}

func TestInstanceAPITransferredIdentityCleanup(t *testing.T) {
	for _, offline := range []bool{false, true} {
		t.Run(fmt.Sprintf("offline=%t", offline), func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			model.AddScheme(workloads.AddToScheme)
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "instance-cleanup", Namespace: "default", UID: "its-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
				EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(int32(1)), FlatInstanceOrdinal: true, PodManagementPolicy: appsv1.ParallelPodManagement,
				Selector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "instance-cleanup"}},
				Template:  corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "instance-cleanup"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
				Instances: []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}}},
			}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}).WithObjects(its).Build()
			key := client.ObjectKeyFromObject(its)
			run := func() {
				t.Helper()
				_, err := (&InstanceSetReconciler2{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
				if err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 3; i++ {
				run()
			}
			inst := &workloads.Instance{}
			instKey := client.ObjectKey{Namespace: key.Namespace, Name: its.Name + "-0"}
			if err := cli.Get(context.Background(), instKey, inst); err != nil {
				t.Fatal(err)
			}
			inst.UID = "retained-instance"
			inst.Finalizers = []string{"test/hold"}
			if err := cli.Update(context.Background(), inst); err != nil {
				t.Fatal(err)
			}
			inst.Status.CurrentState = workloads.InstanceCurrentStatePresent
			if err := cli.Status().Update(context.Background(), inst); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(context.Background(), key, its); err != nil {
				t.Fatal(err)
			}
			its.Spec.Instances[0].Name = "b"
			its.Generation++
			if err := cli.Update(context.Background(), its); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				run()
			}
			if err := cli.Get(context.Background(), key, its); err != nil {
				t.Fatal(err)
			}
			if status := its.FindInstanceStatus(instKey.Name); status == nil || status.TemplateName == nil || *status.TemplateName != "b" || status.DesiredState != workloads.InstanceDesiredStateActive {
				t.Fatalf("transfer did not publish desired identity: %#v", status)
			}
			if err := cli.Get(context.Background(), instKey, inst); err != nil {
				t.Fatal(err)
			}
			if inst.Spec.InstanceTemplateName != "a" {
				t.Fatal("fixture did not retain the unready Instance's old spec")
			}
			its.Spec.Replicas = ptr.To(int32(0))
			its.Spec.Instances = nil
			its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
			wantDesired := workloads.InstanceDesiredStateReleased
			if offline {
				its.Spec.OfflineInstances = []string{instKey.Name}
				wantDesired = workloads.InstanceDesiredStateOffline
			}
			its.Generation++
			if err := cli.Update(context.Background(), its); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				run()
			}
			if err := cli.Get(context.Background(), instKey, inst); err != nil {
				t.Fatal(err)
			}
			if inst.UID != "retained-instance" || !ptr.Deref(inst.Spec.ScaledDown, false) || inst.DeletionTimestamp.IsZero() {
				t.Fatal("cleanup did not retire the retained Instance")
			}
			if err := cli.Get(context.Background(), key, its); err != nil {
				t.Fatal(err)
			}
			if status := its.FindInstanceStatus(instKey.Name); status == nil || status.TemplateName == nil || *status.TemplateName != "b" || status.DesiredState != wantDesired || status.UpToDate {
				t.Fatalf("cleanup lost the published identity: %#v", status)
			}
			inst.Finalizers = nil
			if err := cli.Update(context.Background(), inst); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				run()
			}
			if err := cli.Get(context.Background(), key, its); err != nil {
				t.Fatal(err)
			}
			status := its.FindInstanceStatus(instKey.Name)
			if offline {
				if status == nil || status.TemplateName == nil || *status.TemplateName != "b" || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
					t.Fatalf("offline identity was lost after deletion: %#v", status)
				}
			} else if status != nil {
				t.Fatalf("released identity survived deletion: %#v", status)
			}
		})
	}
}

func ageInstanceSetReadyCondition(cli client.Client, its *workloads.InstanceSet) metav1.Time {
	GinkgoHelper()
	Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
	ready := meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady))
	ready.LastTransitionTime = metav1.NewTime(time.Now().Truncate(time.Second).Add(-2 * time.Minute))
	Expect(cli.Status().Update(context.Background(), its)).Should(Succeed())
	return ready.LastTransitionTime
}

func expectInstanceSetNoRuntime(its *workloads.InstanceSet) {
	GinkgoHelper()
	Expect(its.Status.Replicas).Should(BeZero())
	Expect(its.Status.ReadyReplicas).Should(BeZero())
	Expect(its.Status.AvailableReplicas).Should(BeZero())
	expectInstanceSetNotReady(its)
	Expect(meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
}

func expectInstanceSetNotReady(its *workloads.InstanceSet) {
	GinkgoHelper()
	Expect(meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
}

func publishLegacyInstanceSetReadiness(t GinkgoTInterface, cli client.Client, name string) {
	t.Helper()
	podsBefore, claimsBefore := &corev1.PodList{}, &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), podsBefore); err != nil {
		t.Fatal(err)
	}
	if err := cli.List(context.Background(), claimsBefore); err != nil {
		t.Fatal(err)
	}
	key := client.ObjectKey{Namespace: "default", Name: name}
	reconciler := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatal(err)
	}
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []workloads.ConditionType{workloads.InstanceReady, workloads.InstanceAvailable} {
		condition := meta.FindStatusCondition(its.Status.Conditions, string(kind))
		if condition == nil || condition.ObservedGeneration != its.Generation {
			t.Fatalf("%s was not published for generation %d: %#v", kind, its.Generation, condition)
		}
	}
	podsAfter, claimsAfter := &corev1.PodList{}, &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), podsAfter); err != nil {
		t.Fatal(err)
	}
	if err := cli.List(context.Background(), claimsAfter); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(podsBefore.Items, podsAfter.Items) || !reflect.DeepEqual(claimsBefore.Items, claimsAfter.Items) {
		t.Fatal("readiness publication changed Pods or PVCs")
	}
}

func legacyStopRecoveryFixture(t GinkgoTInterface, replicas int32, minReadySeconds int32) client.WithWatch {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-stop", Namespace: "default", UID: "its-uid", Generation: 1},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(replicas), MinReadySeconds: minReadySeconds, PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "legacy-stop"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "legacy-stop"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			}}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
	legacyStopRecoveryReconcile(t, cli, 4)
	if len(legacyStopRecoveryPods(t, cli)) != int(replicas) {
		t.Fatal("fixture did not provision running identities")
	}
	return cli
}

func legacyStopRecoveryReconcile(t GinkgoTInterface, cli client.Client, count int) ctrl.Result {
	t.Helper()
	var result ctrl.Result
	for i := 0; i < count; i++ {
		reconciler := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
		var err error
		result, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func legacyStopRecoverySet(t GinkgoTInterface, cli client.Client) *workloads.InstanceSet {
	t.Helper()
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "legacy-stop"}, its); err != nil {
		t.Fatal(err)
	}
	return its
}

func legacyStopRecoveryChange(t GinkgoTInterface, cli client.Client, change func(*workloads.InstanceSet)) {
	t.Helper()
	its := legacyStopRecoverySet(t, cli)
	change(its)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
}

func legacyStopRecoveryPods(t GinkgoTInterface, cli client.Client) []corev1.Pod {
	t.Helper()
	pods := &corev1.PodList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	return pods.Items
}

func legacyStopRecoveryClaims(t GinkgoTInterface, cli client.Client) []corev1.PersistentVolumeClaim {
	t.Helper()
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	return claims.Items
}

func legacyStopRecoveryReady(t GinkgoTInterface, cli client.Client) {
	t.Helper()
	for _, claim := range legacyStopRecoveryClaims(t, cli) {
		claim.Status.Capacity = claim.Spec.Resources.Requests.DeepCopy()
		if err := cli.Status().Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	for _, pod := range legacyStopRecoveryPods(t, cli) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.PodIP = "127.0.0.1"
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	legacyStopRecoveryReconcile(t, cli, 2)
}

func legacyStopRecoveryHold(t *testing.T, cli client.Client) {
	t.Helper()
	pod := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "legacy-stop-0"}, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = append(pod.Finalizers, "test/hold")
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyInstanceSetStopRecoveryMinReadySeconds(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 2, 3600)
	legacyStopRecoveryReady(t, cli)
	status := legacyStopRecoverySet(t, cli).Status
	if status.ReadyReplicas != 2 || status.AvailableReplicas != 0 {
		t.Fatalf("fixture did not exercise MinReadySeconds: %#v", status)
	}
	if result := legacyStopRecoveryReconcile(t, cli, 1); result.RequeueAfter != time.Second {
		t.Fatalf("fixture did not reach status cutoff: %#v", result)
	}
	claims := legacyStopRecoveryClaims(t, cli)
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(true)
		its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	})
	legacyStopRecoveryReconcile(t, cli, 6)
	if len(legacyStopRecoveryPods(t, cli)) != 0 || legacyStopRecoverySet(t, cli).Status.Replicas != 0 {
		t.Fatal("MinReadySeconds blocked ordered Stop after the first Pod disappeared")
	}
	if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
		t.Fatal("Stop changed storage")
	}
}

func TestLegacyInstanceSetStopRecoveryFlatTemplateChange(t *testing.T) {
	for _, retargetFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("retargetFirst=%t", retargetFirst), func(t *testing.T) {
			cli := legacyStopRecoveryFixture(t, 2, 0)
			legacyStopRecoveryReady(t, cli)
			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.FlatInstanceOrdinal = true })
			legacyStopRecoveryReconcile(t, cli, 3)
			changeTemplate := func(its *workloads.InstanceSet) {
				its.Spec.Instances = []workloads.InstanceTemplate{{Name: "named", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}}}
			}
			if retargetFirst {
				legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
					changeTemplate(its)
					its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType}
				})
				publishLegacyInstanceSetReadiness(t, cli, "legacy-stop")
				legacyStopRecoveryReconcile(t, cli, 2)
			}
			claims := legacyStopRecoveryClaims(t, cli)
			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
				its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
				changeTemplate(its)
			})
			legacyStopRecoveryReconcile(t, cli, 6)
			its := legacyStopRecoverySet(t, cli)
			if len(legacyStopRecoveryPods(t, cli)) != 0 || its.Status.Replicas != 0 || its.Status.ObservedGeneration != its.Generation {
				t.Fatal("Stop with a template change did not finish draining Pods")
			}
			historicalTemplate := ""
			if retargetFirst {
				historicalTemplate = "named"
			}
			status := its.FindInstanceStatus("legacy-stop-0")
			if status == nil || status.TemplateName == nil || *status.TemplateName != historicalTemplate || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
				t.Fatalf("Stop did not retain the retained template identity after Pod deletion: %#v", status)
			}
			if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
				t.Fatal("Stop with a template change modified retained PVCs")
			}
			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
			legacyStopRecoveryReconcile(t, cli, 2)
			legacyStopRecoveryReady(t, cli)
			legacyStopRecoveryReconcile(t, cli, 3)
			status = legacyStopRecoverySet(t, cli).FindInstanceStatus("legacy-stop-0")
			if len(legacyStopRecoveryPods(t, cli)) != 2 || status == nil || status.TemplateName == nil || *status.TemplateName != "named" || status.DesiredState != workloads.InstanceDesiredStateActive {
				t.Fatal("resume did not apply the desired template assignment")
			}
			for _, pod := range legacyStopRecoveryPods(t, cli) {
				if pod.Name == "legacy-stop-0" && pod.Labels[instancetemplate.TemplateNameLabelKey] != "named" {
					t.Fatal("resumed Pod did not use the desired template")
				}
			}
		})
	}
}

func TestLegacyInstanceSetStopRecoveryMalformedObservedConfig(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 2, 0)
	legacyStopRecoveryReady(t, cli)
	for _, status := range legacyStopRecoverySet(t, cli).Status.InstanceStatus {
		if !status.UpToDate || status.CurrentState != workloads.InstanceCurrentStatePresent {
			t.Fatal("fixture did not publish applied runtime before Stop")
		}
	}
	claims := legacyStopRecoveryClaims(t, cli)
	for _, pod := range legacyStopRecoveryPods(t, cli) {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[constant.CMInsConfigurationHashLabelKey] = "invalid-json"
		if err := cli.Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(true)
		its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	})
	legacyStopRecoveryReconcile(t, cli, 6)
	if len(legacyStopRecoveryPods(t, cli)) != 0 || legacyStopRecoverySet(t, cli).Status.Replicas != 0 {
		t.Fatal("malformed observed config blocked Stop")
	}
	if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
		t.Fatal("Stop with malformed observed config changed storage")
	}
}

func TestLegacyInstanceSetStopRecoveryTerminatingSummary(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 1, 0)
	legacyStopRecoveryReady(t, cli)
	legacyStopRecoveryHold(t, cli)
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	publishLegacyInstanceSetReadiness(t, cli, "legacy-stop")
	legacyStopRecoveryReconcile(t, cli, 3)
	assertTerminating := func() {
		t.Helper()
		its := legacyStopRecoverySet(t, cli)
		if its.Status.Replicas != 1 || its.Status.ReadyReplicas != 0 || its.Status.AvailableReplicas != 0 || its.Status.CurrentReplicas != 0 || its.Status.UpdatedReplicas != 0 {
			t.Fatalf("terminating runtime counted as usable: %#v", its.Status)
		}
		instance := its.FindInstanceStatus("legacy-stop-0")
		if instance == nil || instance.CurrentState != workloads.InstanceCurrentStateTerminating || instance.Ready || instance.Available || instance.UpToDate {
			t.Fatalf("detailed runtime contradicts terminating summary: %#v", instance)
		}
	}
	assertTerminating()
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
	publishLegacyInstanceSetReadiness(t, cli, "legacy-stop")
	legacyStopRecoveryReconcile(t, cli, 1)
	assertTerminating()
}

func TestLegacyInstanceSetStopRecoveryWaitsForTermination(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 2, 0)
	legacyStopRecoveryReady(t, cli)
	legacyStopRecoveryHold(t, cli)
	heldClaim := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "data-legacy-stop-0"}, heldClaim); err != nil {
		t.Fatal(err)
	}
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	failure := errors.New("injected Stop status failure before early resume")
	fired := false
	faulty := interceptor.NewClient(cli, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, delegate client.Client, name string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if _, ok := obj.(*workloads.InstanceSet); ok && name == "status" && !fired {
				fired = true
				return failure
			}
			return delegate.SubResource(name).Update(ctx, obj, opts...)
		},
	})
	controller := &InstanceSetReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	podsBefore := legacyStopRecoveryPods(t, cli)
	claimsBefore := legacyStopRecoveryClaims(t, cli)
	if _, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}}); !errors.Is(err, failure) || !fired {
		t.Fatalf("expected injected Stop status failure, got %v", err)
	}
	if !reflect.DeepEqual(podsBefore, legacyStopRecoveryPods(t, cli)) || !reflect.DeepEqual(claimsBefore, legacyStopRecoveryClaims(t, cli)) {
		t.Fatal("failed readiness publication changed Pods or PVCs")
	}
	publishLegacyInstanceSetReadiness(t, cli, "legacy-stop")
	fired = false
	if _, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}}); !errors.Is(err, failure) || !fired {
		t.Fatalf("expected injected Stop status failure after deletion, got %v", err)
	}
	its := legacyStopRecoverySet(t, cli)
	status := its.FindInstanceStatus("legacy-stop-0")
	if its.Status.ObservedGeneration == its.Generation || status == nil || !status.UpToDate || status.CurrentState != workloads.InstanceCurrentStatePresent {
		t.Fatal("fixture did not retain stale applied status after persisted Stop deletion")
	}
	if pods := legacyStopRecoveryPods(t, cli); len(pods) != 1 || pods[0].DeletionTimestamp.IsZero() {
		t.Fatal("Stop status failure did not follow actual Pod deletions")
	}
	pod := legacyStopRecoveryPods(t, cli)[0]
	pod.Annotations = map[string]string{constant.CMInsConfigurationHashLabelKey: "invalid-json"}
	if err := cli.Update(context.Background(), &pod); err != nil {
		t.Fatal(err)
	}
	mock := kbacli.NewMockClient(gomock.NewController(t))
	kbacli.SetMockClient(mock, nil)
	t.Cleanup(kbacli.UnsetMockClient)
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(false)
		its.Spec.Template.Spec.Containers[0].Image = "db:v2"
		its.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{Name: "db", ContainerPort: 5432, Protocol: corev1.ProtocolTCP}}
		its.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{"desired": "latest"}
		its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.RollingUpdateStrategyType, RollingUpdate: &workloads.RollingUpdate{MaxUnavailable: ptr.To(intstr.FromInt(2))}}
		its.Spec.Configs = []workloads.ConfigTemplate{{Name: "server", ConfigHash: ptr.To("new"), Reconfigure: &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"true"}}}}}
	})
	publishLegacyInstanceSetReadiness(t, cli, "legacy-stop")
	before := legacyStopRecoveryPods(t, cli)[0]
	if result := legacyStopRecoveryReconcile(t, cli, 1); result.RequeueAfter != time.Second {
		t.Fatalf("early resume did not wait for termination: %#v", result)
	}
	status = legacyStopRecoverySet(t, cli).FindInstanceStatus(before.Name)
	if status == nil || status.UpToDate {
		t.Fatal("resume target publication trusted stale applied status for a terminating Pod")
	}
	pods := legacyStopRecoveryPods(t, cli)
	if len(pods) != 2 {
		t.Fatal("termination wait blocked independent Parallel creation")
	}
	for _, pod := range pods {
		if pod.Name == before.Name {
			if !reflect.DeepEqual(before, pod) {
				t.Fatal("resume modified the terminating Pod")
			}
		} else {
			if pod.Spec.Containers[0].Image != "db:v2" || !pod.DeletionTimestamp.IsZero() {
				t.Fatal("independent Pod did not receive the latest resume payload")
			}
			pod.Status.Phase = corev1.PodPending
			if err := cli.Status().Update(context.Background(), &pod); err != nil {
				t.Fatal(err)
			}
		}
	}
	if result := legacyStopRecoveryReconcile(t, cli, 1); result.RequeueAfter != time.Second {
		t.Fatalf("termination wait did not reenter: %#v", result)
	}
	currentClaim := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(heldClaim), currentClaim); err != nil || !reflect.DeepEqual(heldClaim, currentClaim) {
		t.Fatalf("termination wait changed held Pod storage: %v", err)
	}
	for _, claim := range legacyStopRecoveryClaims(t, cli) {
		if claim.Name != heldClaim.Name && claim.Annotations["desired"] != "latest" {
			t.Fatal("independent Pod PVC did not commit during termination wait")
		}
	}
	services := &corev1.ServiceList{}
	if err := cli.List(context.Background(), services); err != nil || len(services.Items) != 1 || len(services.Items[0].Spec.Ports) != 1 || services.Items[0].Spec.Ports[0].Port != 5432 {
		t.Fatalf("termination wait did not commit assistant changes: %v", err)
	}
	its = legacyStopRecoverySet(t, cli)
	if its.Status.ObservedGeneration != its.Generation || its.Status.Replicas != 2 || its.Status.UpdatedReplicas != 1 {
		t.Fatalf("termination wait did not commit current generation and runtime counts: generation=%d status=%#v", its.Generation, its.Status)
	}
}

func TestLegacyInstanceSetStopRecoveryFreezesPVCOnEarlyResume(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 1, 0)
	legacyStopRecoveryHold(t, cli)
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	legacyStopRecoveryReconcile(t, cli, 2)
	claim := legacyStopRecoveryClaims(t, cli)[0]
	claim.UID = "claim-uid"
	claim.Spec.VolumeName = "retained-volume"
	if err := cli.Update(context.Background(), &claim); err != nil {
		t.Fatal(err)
	}
	before := legacyStopRecoveryClaims(t, cli)[0]
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(false)
		its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType}
		its.Spec.Template.Spec.Containers[0].Image = "db:v2"
		its.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{"desired": "latest"}
		its.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	})
	legacyStopRecoveryReconcile(t, cli, 3)
	if after := legacyStopRecoveryClaims(t, cli)[0]; !reflect.DeepEqual(before, after) {
		t.Fatalf("early resume mutated PVC before terminating Pod disappeared: before=%#v after=%#v", before, after)
	}
	pod := legacyStopRecoveryPods(t, cli)[0]
	if pod.DeletionTimestamp.IsZero() || pod.Spec.Containers[0].Image != "db:v1" {
		t.Fatal("early resume replaced or changed terminating runtime")
	}
	pod.Finalizers = nil
	if err := cli.Update(context.Background(), &pod); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected legacy resume Pod create failure")
	fired, created := false, 0
	faulty := interceptor.NewClient(cli, interceptor.Funcs{
		Create: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.Pod); ok {
				if !fired {
					fired = true
					return failure
				}
				err := delegate.Create(ctx, obj, opts...)
				if err == nil {
					created++
				}
				return err
			}
			return delegate.Create(ctx, obj, opts...)
		},
	})
	controller := &InstanceSetReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	_, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}})
	if !errors.Is(err, failure) || !fired {
		t.Fatalf("expected injected resume failure, got %v", err)
	}
	committed := legacyStopRecoveryClaims(t, cli)[0]
	if len(legacyStopRecoveryPods(t, cli)) != 0 || created != 0 || committed.UID != before.UID || !reflect.DeepEqual(committed.OwnerReferences, before.OwnerReferences) || committed.Spec.VolumeName != before.Spec.VolumeName || committed.Annotations["desired"] != "latest" || committed.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatal("failure did not follow an actual update of the retained PVC")
	}
	legacyStopRecoveryReconcile(t, faulty, 3)
	pods := legacyStopRecoveryPods(t, cli)
	if created != 1 || len(pods) != 1 || pods[0].Spec.Containers[0].Image != "db:v2" || !pods[0].DeletionTimestamp.IsZero() {
		t.Fatal("resume did not create latest runtime after held Pod disappeared")
	}
	current := legacyStopRecoveryClaims(t, cli)[0]
	if !reflect.DeepEqual(committed, current) {
		t.Fatal("resume did not apply latest target to retained claim")
	}
}

func TestLegacyInstanceSetStopRecoveryPartialCommit(t *testing.T) {
	for _, failStatus := range []bool{false, true} {
		t.Run(fmt.Sprintf("failStatus=%t", failStatus), func(t *testing.T) {
			cli := legacyStopRecoveryFixture(t, 3, 0)
			legacyStopRecoveryReady(t, cli)
			claims := legacyStopRecoveryClaims(t, cli)
			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
			failure := errors.New("injected legacy Stop commit failure")
			fired, attempts, deleted := false, 0, 0
			faulty := interceptor.NewClient(cli, interceptor.Funcs{
				Delete: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if _, ok := obj.(*corev1.Pod); ok {
						attempts++
						if !failStatus && !fired && attempts == 2 {
							fired = true
							return failure
						}
						err := delegate.Delete(ctx, obj, opts...)
						if err == nil {
							deleted++
						}
						return err
					}
					return delegate.Delete(ctx, obj, opts...)
				},
				SubResourceUpdate: func(ctx context.Context, delegate client.Client, name string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					if _, ok := obj.(*workloads.InstanceSet); ok && name == "status" && failStatus && !fired {
						fired = true
						return failure
					}
					return delegate.SubResource(name).Update(ctx, obj, opts...)
				},
			})
			controller := &InstanceSetReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
			if failStatus {
				podsBefore := legacyStopRecoveryPods(t, cli)
				_, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}})
				if !errors.Is(err, failure) || !fired {
					t.Fatalf("expected injected readiness publication failure, got %v", err)
				}
				if attempts != 0 || deleted != 0 || !reflect.DeepEqual(podsBefore, legacyStopRecoveryPods(t, cli)) || !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
					t.Fatal("failed readiness publication changed Pods or PVCs")
				}
			}
			publishLegacyInstanceSetReadiness(t, cli, "legacy-stop")
			fired = false
			_, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}})
			if !errors.Is(err, failure) || !fired {
				t.Fatalf("expected injected Stop failure, got %v", err)
			}
			wantDeleted := 1
			if failStatus {
				wantDeleted = 3
			}
			if deleted != wantDeleted || len(legacyStopRecoveryPods(t, cli)) != 3-wantDeleted {
				t.Fatalf("failure did not follow persisted Pod removal: deleted=%d", deleted)
			}
			if legacyStopRecoverySet(t, cli).Status.ObservedGeneration == legacyStopRecoverySet(t, cli).Generation {
				t.Fatal("failed commit unexpectedly persisted current root status")
			}
			if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
				t.Fatal("partial Stop changed storage")
			}
			legacyStopRecoveryReconcile(t, faulty, 6)
			its := legacyStopRecoverySet(t, cli)
			if deleted != 3 || len(legacyStopRecoveryPods(t, cli)) != 0 || its.Status.Replicas != 0 || its.Status.ObservedGeneration != its.Generation {
				t.Fatal("fresh Stop retries did not converge from persisted resources")
			}
			if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
				t.Fatal("Stop retry changed storage")
			}
		})
	}
}

func pvcLifecycleFixture(t *testing.T, replicas int32) client.WithWatch {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	model.AddScheme(workloads.AddToScheme)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-lifecycle", Namespace: "default", UID: "its-uid", Generation: 1},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(replicas), PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "pvc-lifecycle"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "pvc-lifecycle"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			}}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
	pvcLifecycleReconcile(t, cli, 4)
	if len(pvcLifecyclePods(t, cli)) != int(replicas) {
		t.Fatal("fixture did not provision running identities")
	}
	for _, claim := range pvcLifecycleClaims(t, cli) {
		claim.UID = types.UID(claim.Name + "-uid")
		claim.Spec.VolumeName = claim.Name + "-volume"
		if err := cli.Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	return cli
}

func pvcLifecycleReconcile(t *testing.T, cli client.Client, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		reconciler := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
		_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func pvcLifecycleSet(t *testing.T, cli client.Client) *workloads.InstanceSet {
	t.Helper()
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}, its); err != nil {
		t.Fatal(err)
	}
	return its
}

func pvcLifecycleChange(t *testing.T, cli client.Client, change func(*workloads.InstanceSet)) {
	t.Helper()
	its := pvcLifecycleSet(t, cli)
	change(its)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
}

func pvcLifecyclePods(t *testing.T, cli client.Client) []corev1.Pod {
	t.Helper()
	pods := &corev1.PodList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	return pods.Items
}

func pvcLifecycleClaims(t *testing.T, cli client.Client) []corev1.PersistentVolumeClaim {
	t.Helper()
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	return claims.Items
}

func pvcLifecycleReady(t *testing.T, cli client.Client) {
	t.Helper()
	for _, claim := range pvcLifecycleClaims(t, cli) {
		claim.Status.Capacity = claim.Spec.Resources.Requests.DeepCopy()
		if err := cli.Status().Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	for _, pod := range pvcLifecyclePods(t, cli) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.PodIP = "127.0.0.1"
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	pvcLifecycleReconcile(t, cli, 2)
}

func TestInstanceSetRetainedClaimsAfterRecreation(t *testing.T) {
	cli := pvcLifecycleFixture(t, 2)
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{
			WhenDeleted: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
			WhenScaled:  kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
		}
	})
	original := pvcLifecycleSet(t, cli)
	if err := cli.Delete(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleReconcile(t, cli, 6)
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(original), &workloads.InstanceSet{}); !apierrors.IsNotFound(err) {
		t.Fatalf("original InstanceSet was not deleted: %v", err)
	}
	claims := pvcLifecycleClaims(t, cli)
	if len(claims) != 2 {
		t.Fatal("root deletion did not retain all claims")
	}
	for _, claim := range claims {
		if len(claim.OwnerReferences) != 0 {
			t.Fatal("retained claim was not detached from the deleted root")
		}
	}
	replacement := original.DeepCopy()
	replacement.ResourceVersion = ""
	replacement.UID = "replacement-root"
	replacement.Generation = 1
	replacement.Finalizers = nil
	replacement.Status = workloads.InstanceSetStatus{}
	replacement.Spec.Replicas = ptr.To(int32(0))
	if err := cli.Create(context.Background(), replacement); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleReconcile(t, cli, 4)
	if !reflect.DeepEqual(claims, pvcLifecycleClaims(t, cli)) {
		t.Fatal("same-name recreation changed unadopted retained claims")
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(1)) })
	pvcLifecycleReconcile(t, cli, 4)
	for _, claim := range pvcLifecycleClaims(t, cli) {
		switch claim.Name {
		case "data-pvc-lifecycle-0":
			if !metav1.IsControlledBy(&claim, replacement) || !reflect.DeepEqual(claim.Spec, claims[0].Spec) {
				t.Fatal("active claim was not adopted with its storage preserved")
			}
		default:
			if !reflect.DeepEqual(claim, claims[1]) {
				t.Fatal("adoption changed the unused retained claim")
			}
		}
	}
	pods := pvcLifecyclePods(t, cli)
	if len(pods) != 1 {
		t.Fatal("replacement did not create its active Pod")
	}
	if err := cli.Delete(context.Background(), &pods[0]); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(0)) })
	pvcLifecycleReconcile(t, cli, 4)
	if remaining := pvcLifecycleClaims(t, cli); !reflect.DeepEqual(remaining, claims[1:]) {
		t.Fatal("retirement must remove only the adopted claim and preserve unowned storage")
	}
}

func TestInstanceSetPVCAllocationAndRetirement(t *testing.T) {
	cases := []struct {
		policy   kbappsv1.PersistentVolumeClaimRetentionPolicyType
		replicas int32
	}{
		{kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, 0},
		{kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, 1},
	}
	for _, tc := range cases {
		policy, replicas := tc.policy, tc.replicas
		t.Run(fmt.Sprintf("%s/replicas=%d", policy, replicas), func(t *testing.T) {
			cli := pvcLifecycleFixture(t, 2)
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
				its.Spec.Replicas = ptr.To(replicas)
				its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy}
			})
			pvcLifecycleReconcile(t, cli, 6)
			claims := pvcLifecycleClaims(t, cli)
			if len(pvcLifecyclePods(t, cli)) != 0 || len(claims) != 2 {
				t.Fatal("simultaneous Stop and scale-down did not retain both claims while draining")
			}
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
			pvcLifecycleReconcile(t, cli, 3)
			if len(pvcLifecyclePods(t, cli)) != int(replicas) {
				t.Fatal("resume did not restore exactly the desired identities")
			}
			if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
				if !reflect.DeepEqual(claims, pvcLifecycleClaims(t, cli)) {
					t.Fatal("WhenScaled=Retain changed stopped claims")
				}
			} else {
				for _, claim := range claims {
					current := &corev1.PersistentVolumeClaim{}
					err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current)
					if !apierrors.IsNotFound(err) {
						t.Fatalf("WhenScaled=Delete retained absent retired identity claim: %v", err)
					}
				}
			}
		})
	}
}

func TestInstanceSetPVCRetirementAfterPartialCommit(t *testing.T) {
	cli := pvcLifecycleFixture(t, 4)
	for _, pod := range pvcLifecyclePods(t, cli) {
		if err := cli.Delete(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	allocated := []corev1.PersistentVolumeClaim{}
	for _, name := range []string{"data-pvc-lifecycle-0", "data-pvc-lifecycle-1"} {
		claim := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, claim); err != nil {
			t.Fatal(err)
		}
		allocated = append(allocated, *claim)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Replicas = ptr.To(int32(2))
	})
	publishLegacyInstanceSetReadiness(t, cli, "pvc-lifecycle")
	failure := errors.New("injected retired PVC delete failure")
	fired, attempts, deleted := false, 0, 0
	faulty := interceptor.NewClient(cli, interceptor.Funcs{
		Delete: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, ok := obj.(*corev1.PersistentVolumeClaim); ok {
				attempts++
				if !fired && attempts == 2 {
					fired = true
					return failure
				}
				err := delegate.Delete(ctx, obj, opts...)
				if err == nil {
					deleted++
				}
				return err
			}
			return delegate.Delete(ctx, obj, opts...)
		},
	})
	controller := &InstanceSetReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	_, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}})
	if !errors.Is(err, failure) || !fired || attempts != 2 || deleted != 1 || len(pvcLifecycleClaims(t, cli)) != 3 || len(pvcLifecyclePods(t, cli)) != 0 {
		t.Fatalf("expected one persisted retired-claim deletion before failure, got %v", err)
	}
	for _, claim := range allocated {
		current := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current); err != nil || !reflect.DeepEqual(claim, *current) {
			t.Fatal("partial retirement changed desired storage")
		}
	}
	pvcLifecycleReconcile(t, faulty, 5)
	if deleted != 2 || len(pvcLifecycleClaims(t, cli)) != 2 || len(pvcLifecyclePods(t, cli)) != 2 {
		t.Fatal("fresh retry did not complete retirement and resume")
	}
	for _, claim := range allocated {
		current := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current); err != nil || !reflect.DeepEqual(claim, *current) {
			t.Fatal("retirement retry changed desired storage")
		}
	}
}

func TestInstanceSetPVCMixedRetirement(t *testing.T) {
	for _, tc := range []struct {
		management appsv1.PodManagementPolicyType
		policy     kbappsv1.PersistentVolumeClaimRetentionPolicyType
	}{
		{appsv1.OrderedReadyPodManagement, kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType},
		{appsv1.ParallelPodManagement, kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType},
		{appsv1.OrderedReadyPodManagement, kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType},
	} {
		t.Run(string(tc.management)+"/"+string(tc.policy), func(t *testing.T) {
			management, policy := tc.management, tc.policy
			cli := pvcLifecycleFixture(t, 4)
			pvcLifecycleReady(t, cli)
			orphan := &corev1.Pod{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle-3"}, orphan); err != nil {
				t.Fatal(err)
			}
			if err := cli.Delete(context.Background(), orphan); err != nil {
				t.Fatal(err)
			}
			claims := pvcLifecycleClaims(t, cli)
			unknown := claims[0].DeepCopy()
			unknown.Name = "unassigned-claim"
			unknown.ResourceVersion = ""
			unknown.UID = "unassigned-claim-uid"
			unknown.Labels[constant.KBAppPodNameLabelKey] = ""
			if err := cli.Create(context.Background(), unknown); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(unknown), unknown); err != nil {
				t.Fatal(err)
			}
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(1))
				its.Spec.PodManagementPolicy = management
				its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
				its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy}
			})
			publishLegacyInstanceSetReadiness(t, cli, "pvc-lifecycle")
			pvcLifecycleReconcile(t, cli, 1)
			pods := pvcLifecyclePods(t, cli)
			if len(pods) != 2 {
				t.Fatalf("expected exactly one admitted Pod retirement, got %d", len(pods))
			}
			for _, pod := range pods {
				if pod.Name == "pvc-lifecycle-2" {
					t.Fatal("highest live ordinal was not retired")
				}
			}
			actual := map[string]corev1.PersistentVolumeClaim{}
			for _, claim := range pvcLifecycleClaims(t, cli) {
				actual[claim.Name] = claim
			}
			expectedClaims := 3
			if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
				expectedClaims = 5
			}
			if len(actual) != expectedClaims {
				t.Fatalf("expected %d claims after admitted retirement, got %d", expectedClaims, len(actual))
			}
			for _, claim := range claims {
				current, found := actual[claim.Name]
				retired := policy != kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType && (claim.Name == "data-pvc-lifecycle-2" || claim.Name == "data-pvc-lifecycle-3")
				if retired {
					if found {
						t.Fatalf("retired claim survived: %s", claim.Name)
					}
				} else if !found || !reflect.DeepEqual(claim, current) {
					t.Fatalf("non-admitted or retained claim changed: %s", claim.Name)
				}
			}
			if current, found := actual[unknown.Name]; !found || !reflect.DeepEqual(*unknown, current) {
				t.Fatal("blank identity claim changed")
			}
		})
	}
}

func TestInstanceSetPVCAbsentOwnership(t *testing.T) {
	cli := pvcLifecycleFixture(t, 1)
	owned := pvcLifecycleClaims(t, cli)[0]
	claims := map[string]corev1.PersistentVolumeClaim{}
	for _, ownership := range []string{"detached", "previous-root", "noncontroller"} {
		claim := owned.DeepCopy()
		claim.Name = "foreign-" + ownership
		claim.ResourceVersion = ""
		switch ownership {
		case "detached":
			claim.OwnerReferences = nil
		case "previous-root":
			claim.OwnerReferences[0].UID = "previous-root"
		case "noncontroller":
			claim.OwnerReferences[0].Controller = ptr.To(false)
		}
		if err := cli.Create(context.Background(), claim); err != nil {
			t.Fatal(err)
		}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), claim); err != nil {
			t.Fatal(err)
		}
		claims[claim.Name] = *claim
	}
	pods := pvcLifecyclePods(t, cli)
	if err := cli.Delete(context.Background(), &pods[0]); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(0)) })
	pvcLifecycleReconcile(t, cli, 4)
	actual := pvcLifecycleClaims(t, cli)
	if len(actual) != len(claims) {
		t.Fatalf("absent cleanup must remove only the current-root controlled claim, got %d claims", len(actual))
	}
	for _, claim := range actual {
		if !reflect.DeepEqual(claim, claims[claim.Name]) {
			t.Fatalf("absent cleanup changed foreign sibling claim %s", claim.Name)
		}
	}
}

func TestInstanceSetPVCOfflineRetirementPolicy(t *testing.T) {
	for _, policy := range []kbappsv1.PersistentVolumeClaimRetentionPolicyType{kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType} {
		for _, absent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/absent=%t", policy, absent), func(t *testing.T) {
				cli := pvcLifecycleFixture(t, 1)
				original := pvcLifecycleClaims(t, cli)[0]
				if absent {
					pods := pvcLifecyclePods(t, cli)
					if err := cli.Delete(context.Background(), &pods[0]); err != nil {
						t.Fatal(err)
					}
				}
				pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
					its.Spec.OfflineInstances = []string{"pvc-lifecycle-0"}
					its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy}
				})
				publishLegacyInstanceSetReadiness(t, cli, "pvc-lifecycle")
				pvcLifecycleReconcile(t, cli, 1)
				pvcLifecycleReady(t, cli)
				pvcLifecycleReconcile(t, cli, 4)
				pods := pvcLifecyclePods(t, cli)
				if len(pods) != 1 || pods[0].Name != "pvc-lifecycle-1" {
					t.Fatalf("specified replica was not retired: %#v", pods)
				}
				current := &corev1.PersistentVolumeClaim{}
				err := cli.Get(context.Background(), client.ObjectKeyFromObject(&original), current)
				if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
					if err != nil || current.UID != original.UID || !reflect.DeepEqual(current.Spec, original.Spec) {
						t.Fatalf("retention policy changed original storage: %v", err)
					}
				} else if !apierrors.IsNotFound(err) {
					t.Fatalf("Offline bypassed Delete retention policy: %v", err)
				}
				expected := 1
				if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
					expected++
				}
				if len(pvcLifecycleClaims(t, cli)) != expected {
					t.Fatal("retirement left unexpected claims")
				}
			})
		}
	}
}

var _ = Describe("InstanceSet readiness publication", func() {
	newFixture := func(instanceAPI bool) (client.WithWatch, client.ObjectKey, func(client.Client) (ctrl.Result, error)) {
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).Should(Succeed())
		Expect(workloads.AddToScheme(scheme)).Should(Succeed())
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "readiness-publication", Namespace: "default", UID: "its-uid", Generation: 1},
			Spec: workloads.InstanceSetSpec{
				EnableInstanceAPI: ptr.To(instanceAPI), Replicas: ptr.To[int32](1), PodManagementPolicy: appsv1.ParallelPodManagement,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "readiness-publication"}},
				Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "readiness-publication"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
			},
		}
		cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}).WithObjects(its).Build()
		key := client.ObjectKeyFromObject(its)
		run := func(c client.Client) (ctrl.Result, error) {
			if instanceAPI {
				return (&InstanceSetReconciler2{Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
			}
			return (&InstanceSetReconciler{Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
		}
		for i := 0; i < 10; i++ {
			_, err := run(cli)
			Expect(err).ShouldNot(HaveOccurred())
			if instanceAPI {
				instances := &workloads.InstanceList{}
				Expect(cli.List(context.Background(), instances)).Should(Succeed())
				for j := range instances.Items {
					inst := &instances.Items[j]
					if inst.UID == "" {
						inst.UID = types.UID(inst.Name)
						inst.Generation = 1
						Expect(cli.Update(context.Background(), inst)).Should(Succeed())
					}
					_, err := (&InstanceReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)})
					Expect(err).ShouldNot(HaveOccurred())
				}
			}
			pods := &corev1.PodList{}
			Expect(cli.List(context.Background(), pods)).Should(Succeed())
			for j := range pods.Items {
				pod := &pods.Items[j]
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true}}
				Expect(cli.Status().Update(context.Background(), pod)).Should(Succeed())
			}
		}
		Expect(cli.Get(context.Background(), key, its)).Should(Succeed())
		Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
		Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceAvailable))).Should(BeTrue())
		return cli, key, run
	}

	DescribeTable("blocks child repair until a changed readiness observation persists", func(instanceAPI, conflict bool) {
		cli, key, run := newFixture(instanceAPI)
		its := &workloads.InstanceSet{}
		Expect(cli.Get(context.Background(), key, its)).Should(Succeed())
		its.Spec.Replicas = ptr.To[int32](2)
		its.Generation++
		Expect(cli.Update(context.Background(), its)).Should(Succeed())
		failure := errors.New("readiness publication failed")
		if conflict {
			failure = apierrors.NewConflict(workloads.Resource("instancesets"), its.Name, failure)
		}
		writes := 0
		faulty := interceptor.NewClient(cli, interceptor.Funcs{
			Create: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				writes++
				return delegate.Create(ctx, obj, opts...)
			},
			SubResourceUpdate: func(_ context.Context, _ client.Client, sub string, obj client.Object, _ ...client.SubResourceUpdateOption) error {
				Expect(sub).Should(Equal("status"))
				Expect(obj).Should(BeAssignableToTypeOf(&workloads.InstanceSet{}))
				return failure
			},
		})
		result, err := run(faulty)
		if conflict {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(result.Requeue).Should(BeTrue())
		} else {
			Expect(errors.Is(err, failure)).Should(BeTrue())
		}
		Expect(writes).Should(BeZero())
		Expect(cli.Get(context.Background(), key, its)).Should(Succeed())
		Expect(meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
		Expect(its.Status.ObservedGeneration).Should(BeNumerically("<", its.Generation))

		_, err = run(cli)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cli.Get(context.Background(), key, its)).Should(Succeed())
		condition := meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady))
		Expect(condition.Status).Should(Equal(metav1.ConditionFalse))
		Expect(condition.ObservedGeneration).Should(Equal(its.Generation))
		Expect(its.Status.ObservedGeneration).Should(BeNumerically("<", its.Generation))
		_, err = run(cli)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cli.Get(context.Background(), key, its)).Should(Succeed())
		Expect(its.Status.ObservedGeneration).Should(Equal(its.Generation))
		if instanceAPI {
			instances := &workloads.InstanceList{}
			Expect(cli.List(context.Background(), instances)).Should(Succeed())
			Expect(instances.Items).Should(HaveLen(2))
		} else {
			pods := &corev1.PodList{}
			Expect(cli.List(context.Background(), pods)).Should(Succeed())
			Expect(pods.Items).Should(HaveLen(2))
		}
	},
		Entry("legacy status error", false, false),
		Entry("legacy optimistic conflict", false, true),
		Entry("Instance API status error", true, false),
		Entry("Instance API optimistic conflict", true, true),
	)

	DescribeTable("keeps the published deficit when child creation fails", func(instanceAPI bool) {
		cli, key, run := newFixture(instanceAPI)
		its := &workloads.InstanceSet{}
		Expect(cli.Get(context.Background(), key, its)).Should(Succeed())
		its.Spec.Replicas = ptr.To[int32](2)
		its.Generation++
		Expect(cli.Update(context.Background(), its)).Should(Succeed())
		_, err := run(cli)
		Expect(err).ShouldNot(HaveOccurred())
		failure := errors.New("child create failed")
		fired := false
		faulty := interceptor.NewClient(cli, interceptor.Funcs{Create: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			switch obj.(type) {
			case *corev1.Pod, *workloads.Instance:
				fired = true
				return failure
			}
			return delegate.Create(ctx, obj, opts...)
		}})
		_, err = run(faulty)
		Expect(errors.Is(err, failure)).Should(BeTrue())
		Expect(fired).Should(BeTrue())
		Expect(cli.Get(context.Background(), key, its)).Should(Succeed())
		Expect(meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
		Expect(meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceReady)).ObservedGeneration).Should(Equal(its.Generation))
	}, Entry("legacy", false), Entry("Instance API", true))

	DescribeTable("allows repair when only the deficit message changes", func(instanceAPI bool) {
		cli, key, run := newFixture(instanceAPI)
		its := &workloads.InstanceSet{}
		Expect(cli.Get(context.Background(), key, its)).Should(Succeed())
		its.Spec.Replicas = ptr.To[int32](2)
		its.Generation++
		Expect(cli.Update(context.Background(), its)).Should(Succeed())
		for i := 0; i < 2; i++ {
			_, err := run(cli)
			Expect(err).ShouldNot(HaveOccurred())
		}
		var child client.Object = &corev1.Pod{}
		if instanceAPI {
			child = &workloads.Instance{}
		}
		Expect(cli.Get(context.Background(), client.ObjectKey{Namespace: key.Namespace, Name: its.Name + "-0"}, child)).Should(Succeed())
		child.SetFinalizers(nil)
		Expect(cli.Update(context.Background(), child)).Should(Succeed())
		Expect(cli.Delete(context.Background(), child)).Should(Succeed())
		created := false
		observed := interceptor.NewClient(cli, interceptor.Funcs{Create: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if obj.GetName() == child.GetName() {
				created = true
			}
			return delegate.Create(ctx, obj, opts...)
		}})
		_, err := run(observed)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(created).Should(BeTrue())
		Expect(cli.Get(context.Background(), key, its)).Should(Succeed())
		Expect(meta.IsStatusConditionFalse(its.Status.Conditions, string(workloads.InstanceReady))).Should(BeTrue())
	}, Entry("legacy", false), Entry("Instance API", true))

	It("does not require a publication barrier for stable Unknown observations", func() {
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Generation: 2}}
		for _, kind := range []string{string(workloads.InstanceReady), string(workloads.InstanceAvailable)} {
			meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{Type: kind, Status: metav1.ConditionUnknown, ObservedGeneration: 2, Reason: "PendingAssignment", Message: "previous allocation"})
		}
		previous := append([]metav1.Condition(nil), its.Status.Conditions...)
		for _, kind := range []string{string(workloads.InstanceReady), string(workloads.InstanceAvailable)} {
			meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{Type: kind, Status: metav1.ConditionUnknown, ObservedGeneration: 2, Reason: "AssignmentPending", Message: "current allocation"})
		}
		Expect(instancereadiness.PublicationChanged(previous, its.Status.Conditions)).Should(BeFalse())
		for _, condition := range previous {
			Expect(meta.FindStatusCondition(its.Status.Conditions, condition.Type).LastTransitionTime.Equal(&condition.LastTransitionTime)).Should(BeTrue())
		}
	})
})
