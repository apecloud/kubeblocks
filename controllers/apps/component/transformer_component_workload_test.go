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

package component

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Component Workload Operations Test", func() {
	Context("copyAndMergeITS restart annotations", func() {
		const older = "2026-09-01T00:00:00Z"
		const newer = "2026-09-02T00:00:00Z"

		DescribeTable("merge restart timestamps", func(runningRestart, desiredRestart, expectedRestart string) {
			running := &workloads.InstanceSet{Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"custom": "old", "retained": "value"},
				}},
			}}
			desired := running.DeepCopy()
			desired.Spec.Template.Annotations = map[string]string{"custom": "new"}
			if runningRestart != "" {
				running.Spec.Template.Annotations[constant.RestartAnnotationKey] = runningRestart
			}
			if desiredRestart != "" {
				desired.Spec.Template.Annotations[constant.RestartAnnotationKey] = desiredRestart
			}
			before := running.DeepCopy()
			merged, err := copyAndMergeITS(running, desired)
			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Annotations).Should(Equal(map[string]string{
				"custom": "new", "retained": "value", constant.RestartAnnotationKey: expectedRestart,
			}))
			Expect(running).Should(Equal(before))
			mergedAgain, err := copyAndMergeITS(merged, desired)
			Expect(err).Should(Succeed())
			Expect(mergedAgain).Should(BeNil())
		},
			Entry("preserve config restart against older ops", newer, older, newer),
			Entry("accept newer ops restart", older, newer, newer),
			Entry("equal timestamp", newer, newer, newer),
			Entry("compare timezone offsets", "2026-09-02T01:00:00+08:00", "2026-09-01T18:00:00Z", "2026-09-01T18:00:00Z"),
			Entry("preserve equal instant representation", newer, "2026-09-02T08:00:00+08:00", newer),
			Entry("compare fractional seconds", "2026-09-02T00:00:00.5Z", newer, "2026-09-02T00:00:00.5Z"),
			Entry("first restart", "", newer, newer),
			Entry("no incoming restart", newer, "", newer),
			Entry("replace invalid running value", "invalid", newer, newer),
			Entry("preserve opaque incoming value behavior", newer, "force-restart", "force-restart"),
		)
	})

	const (
		clusterName    = "test-cluster"
		compName       = "test-comp"
		kubeblocksName = "kubeblocks"
	)

	var (
		reader         *appsutil.MockReader
		dag            *graph.DAG
		comp           *appsv1.Component
		synthesizeComp *component.SynthesizedComponent
	)

	roles := []appsv1.ReplicaRole{
		{Name: "leader", UpdatePriority: 3},
		{Name: "follower", UpdatePriority: 2},
	}

	newDAG := func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, comp, comp, model.ActionStatusPtr())
		return d
	}

	BeforeEach(func() {
		reader = &appsutil.MockReader{}
		comp = &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
			},
			Spec: appsv1.ComponentSpec{},
		}

		synthesizeComp = &component.SynthesizedComponent{
			Namespace:   testCtx.DefaultNamespace,
			ClusterName: clusterName,
			Name:        compName,
			Roles:       roles,
			LifecycleActions: &appsv1.ComponentLifecycleActions{
				MemberJoin: &appsv1.Action{
					Exec: &appsv1.ExecAction{
						Image: "test-image",
					},
				},
				MemberLeave: &appsv1.Action{
					Exec: &appsv1.ExecAction{
						Image: "test-image",
					},
				},
				Switchover: &appsv1.Action{
					Exec: &appsv1.ExecAction{
						Image: "test-image",
					},
				},
			},
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)
	})

	Context("Member Leave Operations", func() {
		var (
			ops  *componentWorkloadOps
			pod0 *corev1.Pod
			pod1 *corev1.Pod
			pods []*corev1.Pod
		)

		BeforeEach(func() {
			pod0 = testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod-0").
				AddContainer(corev1.Container{
					Image: "test-image",
					Name:  "test-container",
				}).
				AddLabels(
					constant.AppManagedByLabelKey, kubeblocksName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, compName,
				).
				GetObject()

			pod1 = testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod-1").
				AddContainer(corev1.Container{
					Image: "test-image",
					Name:  "test-container",
				}).
				AddLabels(
					constant.AppManagedByLabelKey, kubeblocksName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, compName,
				).
				GetObject()

			pods = []*corev1.Pod{pod0, pod1}

			container := corev1.Container{
				Name:            "mock-container-name",
				Image:           testapps.ApeCloudMySQLImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			}

			mockITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"test-its", clusterName, compName).
				AddFinalizers([]string{constant.DBClusterFinalizerName}).
				AddContainer(container).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(compName).
				AddAppManagedByLabel().
				SetReplicas(2).
				SetRoles(roles).
				GetObject()

			ops = &componentWorkloadOps{
				transCtx: &componentTransformContext{
					Context:       ctx,
					Logger:        logger,
					EventRecorder: clusterRecorder,
				},
				cli:            k8sClient,
				component:      comp,
				synthesizeComp: synthesizeComp,
				runningITS:     mockITS,
				protoITS:       mockITS.DeepCopy(),
				dag:            dag,
			}
		})

		It("should handle switchover for when scale in", func() {
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					GinkgoWriter.Printf("ActionRequest: %#v\n", req)
					switch req.Action {
					case "switchover":
						Expect(req.Parameters["KB_SWITCHOVER_CURRENT_NAME"]).Should(Equal(pod1.Name))
					case "memberLeave":
						Expect(req.Parameters["KB_LEAVE_MEMBER_POD_NAME"]).Should(Equal(pod1.Name))
					}
					rsp := kbagentproto.ActionResponse{Message: "mock success"}
					return rsp, nil
				})
			})

			By("setting up leader pod")
			pod1.Labels[constant.RoleLabelKey] = "follower"
			pod1.Labels[constant.RoleLabelKey] = "leader"

			By("executing leave member for leader")
			Expect(ops.leaveMemberForPod(pod1, pods)).Should(Succeed())
		})
	})

	Context("start and stop operations", func() {
		It("removes the stop snapshot when starting a zero-replica workload", func() {
			const snapshot = `{"":0}`
			runningITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"test-its", clusterName, compName).
				SetReplicas(0).
				AddAnnotations(stopReplicasSnapshotKey, snapshot).
				GetObject()
			protoITS := runningITS.DeepCopy()

			transformer := &componentWorkloadTransformer{}
			Expect(transformer.startWorkload(synthesizeComp, runningITS, protoITS)).Should(Succeed())

			By("not mutating the informer/cache object")
			Expect(runningITS.Annotations).Should(HaveKeyWithValue(stopReplicasSnapshotKey, snapshot))
			Expect(protoITS.Annotations).ShouldNot(HaveKey(stopReplicasSnapshotKey))

			By("producing an update even though the restored replica count remains zero")
			updatedITS, err := copyAndMergeITS(runningITS, protoITS)
			Expect(err).Should(Succeed())
			Expect(updatedITS).ShouldNot(BeNil())
			Expect(updatedITS.Annotations).ShouldNot(HaveKey(stopReplicasSnapshotKey))
			Expect(updatedITS.Spec.Replicas).ShouldNot(BeNil())
			Expect(*updatedITS.Spec.Replicas).Should(BeEquivalentTo(0))
		})
	})
})

func TestExpandInstanceVolumes(t *testing.T) {
	for _, componentSize := range []string{"1Gi", "2Gi"} {
		t.Run(componentSize, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "test-db", Namespace: "default"}}
			dag := graph.NewDAG()
			model.NewGraphClient(cli).Root(dag, comp, comp, nil)
			vct := corev1.PersistentVolumeClaimTemplate{ObjectMeta: metav1.ObjectMeta{Name: "data"}}
			vct.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(componentSize)}
			logs := *vct.DeepCopy()
			logs.Name = "logs"
			logs.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("3Gi")
			untouched := *vct.DeepCopy()
			untouched.Name = "untouched"
			untouched.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1Gi")
			override := appsv1.ClusterComponentVolumeClaimTemplate{Name: "data"}
			override.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("6Gi")}
			extra := *override.DeepCopy()
			extra.Name = "extra"
			extra.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("4Gi")
			unchanged := *override.DeepCopy()
			unchanged.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("5Gi")
			ops := &componentWorkloadOps{
				transCtx: &componentTransformContext{Context: context.Background(), EventRecorder: record.NewFakeRecorder(10)},
				cli:      cli, component: comp, dag: dag,
				runningITS: &workloads.InstanceSet{ObjectMeta: comp.ObjectMeta, Spec: workloads.InstanceSetSpec{
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}, {ObjectMeta: metav1.ObjectMeta{Name: "logs"}}, {ObjectMeta: metav1.ObjectMeta{Name: "untouched"}}},
					Instances:            []workloads.InstanceTemplate{{Name: "custom", VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "extra"}}}}},
				}},
				synthesizeComp: &component.SynthesizedComponent{Namespace: "default", VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{vct, logs, untouched}, Instances: []appsv1.InstanceTemplate{
					{Name: "custom", VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{override, extra}},
					{Name: "unchanged", VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{unchanged}},
				}},
				runningItsPodNames: []string{"test-db-0", "test-db-inherit-3", "test-db-custom-0", "test-db-unchanged-0"},
			}
			initial := map[string]string{"data-test-db-0": "1Gi", "data-test-db-inherit-3": "1Gi", "data-test-db-custom-0": "5Gi", "extra-test-db-custom-0": "3Gi", "data-test-db-unchanged-0": "5Gi"}
			for _, pod := range ops.runningItsPodNames {
				initial["logs-"+pod] = "1Gi"
				initial["untouched-"+pod] = "1Gi"
			}
			for name, size := range initial {
				pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
				pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}
				pvc.Status.Capacity = pvc.Spec.Resources.Requests.DeepCopy()
				if err := cli.Create(context.Background(), pvc); err != nil {
					t.Fatal(err)
				}
			}
			if err := ops.expandVolume(); err != nil {
				t.Fatal(err)
			}
			expected := map[string]string{"data-test-db-custom-0": "6Gi", "extra-test-db-custom-0": "4Gi"}
			if componentSize != "1Gi" {
				expected["data-test-db-0"] = componentSize
				expected["data-test-db-inherit-3"] = componentSize
			}
			for _, pod := range ops.runningItsPodNames {
				expected["logs-"+pod] = "3Gi"
			}
			for _, vertex := range dag.Vertices() {
				if pvc, ok := vertex.(*model.ObjectVertex).Obj.(*corev1.PersistentVolumeClaim); ok {
					want, ok := expected[pvc.Name]
					if !ok || pvc.Spec.Resources.Requests.Storage().Cmp(resource.MustParse(want)) != 0 {
						t.Fatalf("unexpected PVC update: %+v", pvc)
					}
					delete(expected, pvc.Name)
				}
			}
			if len(expected) != 0 {
				t.Fatalf("missing PVC updates: %v", expected)
			}
		})
	}
}
