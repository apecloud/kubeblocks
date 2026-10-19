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
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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

var _ = Describe("instance template service versions in component transforms", func() {
	It("loads images, creates InstanceSet overrides, and updates existing templates", func() {
		scheme := runtime.NewScheme()
		Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
		Expect(workloads.AddToScheme(scheme)).Should(Succeed())
		Expect(corev1.AddToScheme(scheme)).Should(Succeed())
		def := testapps.NewComponentDefinitionFactory("template-images").SetServiceVersion("1.0.0").
			SetRuntime(&corev1.Container{Name: "db", Image: "db:1"}).GetObject()
		def.Spec.Runtime.InitContainers = []corev1.Container{{Name: "setup", Image: "setup:1"}}
		def.Status.Phase = appsv1.AvailablePhase
		templateDef := def.DeepCopy()
		templateDef.Name = "template-images-v2"
		ver := &appsv1.ComponentVersion{ObjectMeta: metav1.ObjectMeta{Name: "template-images", Labels: map[string]string{def.Name: def.Name, templateDef.Name: templateDef.Name}},
			Spec: appsv1.ComponentVersionSpec{Releases: []appsv1.ComponentVersionRelease{
				{Name: "r2", ServiceVersion: "2.0.0", Images: map[string]string{"db": "db:2", "setup": "setup:2"}},
				{Name: "r3", ServiceVersion: "3.0.0", Images: map[string]string{"db": "db:3", "setup": "setup:3"}},
			}}, Status: appsv1.ComponentVersionStatus{Phase: appsv1.AvailablePhase}}
		cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(def, templateDef, ver).WithObjects(def, templateDef, ver).Build()
		cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, "template-images", "").GetObject()
		cluster.UID = "cluster-uid"
		comp, err := component.BuildComponent(cluster, &appsv1.ClusterComponentSpec{Name: "db", ComponentDef: def.Name,
			ServiceVersion: "1.0.0", Replicas: 2, Instances: []appsv1.InstanceTemplate{
				{Name: "explicit", CompDef: templateDef.Name, ServiceVersion: "2.0.0"}, {Name: "inherited"},
			}}, nil, nil)
		Expect(err).Should(Succeed())
		graphCli := model.NewGraphClient(cli)
		transCtx := &componentTransformContext{Context: ctx, Client: graphCli, Component: comp, ComponentOrig: comp.DeepCopy(), Logger: logger}
		load := &componentLoadResourcesTransformer{}
		Expect(load.Transform(transCtx, graph.NewDAG())).Should(Succeed())
		Expect(transCtx.SynthesizeComponent.InstanceImages["explicit"]).Should(Equal(map[string]string{"db": "docker.io/library/db:2", "setup": "docker.io/library/setup:2"}))
		Expect(transCtx.SynthesizeComponent.InstanceImages).ShouldNot(HaveKey("inherited"))
		transform := &componentWorkloadTransformer{Client: cli}
		dag := graph.NewDAG()
		graphCli.Root(dag, comp, comp, model.ActionStatusPtr())
		Expect(transform.Transform(transCtx, dag)).Should(Succeed())
		objects := graphCli.FindAll(dag, &workloads.InstanceSet{})
		Expect(objects).Should(HaveLen(1))
		running := objects[0].(*workloads.InstanceSet)
		Expect(running.Spec.Instances[0].Images).Should(Equal(map[string]string{"db": "docker.io/library/db:2", "setup": "docker.io/library/setup:2"}))
		Expect(running.Spec.Instances[1].Images).Should(BeNil())
		Expect(running.Spec.Template.Spec.Containers[0].Image).Should(Equal(intctrlutil.ReplaceImageRegistry("db:1")))
		Expect(cli.Create(ctx, running)).Should(Succeed())

		By("updating an existing InstanceSet when the template version changes")
		comp.Spec.Instances[0].ServiceVersion = "3.0.0"
		Expect(load.Transform(transCtx, graph.NewDAG())).Should(Succeed())
		dag = graph.NewDAG()
		graphCli.Root(dag, comp, comp, model.ActionStatusPtr())
		Expect(transform.Transform(transCtx, dag)).Should(Succeed())
		updated := graphCli.FindAll(dag, &workloads.InstanceSet{})
		Expect(updated).Should(HaveLen(1))
		Expect(graphCli.IsAction(dag, updated[0], model.ActionUpdatePtr())).Should(BeTrue())
		Expect(updated[0].(*workloads.InstanceSet).Spec.Instances[0].Images).Should(Equal(map[string]string{"db": "docker.io/library/db:3", "setup": "docker.io/library/setup:3"}))

		By("requeueing unavailable template definitions and retrying after they recover")
		templateDef.Status.Phase = appsv1.UnavailablePhase
		Expect(cli.Status().Update(ctx, templateDef)).Should(Succeed())
		err = load.Transform(transCtx, graph.NewDAG())
		Expect(intctrlutil.IsRequeueError(err)).Should(BeTrue())
		templateDef.Status.Phase = appsv1.AvailablePhase
		Expect(cli.Status().Update(ctx, templateDef)).Should(Succeed())
		Expect(load.Transform(transCtx, graph.NewDAG())).Should(Succeed())
		By("requeueing unavailable component versions and retrying after they recover")
		ver.Status.Phase = appsv1.UnavailablePhase
		Expect(cli.Status().Update(ctx, ver)).Should(Succeed())
		err = load.Transform(transCtx, graph.NewDAG())
		Expect(intctrlutil.IsRequeueError(err)).Should(BeTrue())
		ver.Status.Phase = appsv1.AvailablePhase
		Expect(cli.Status().Update(ctx, ver)).Should(Succeed())
		Expect(load.Transform(transCtx, graph.NewDAG())).Should(Succeed())
		By("requeueing invalid or missing template images")
		comp.Spec.Instances[0].ServiceVersion = "invalid"
		err = load.Transform(transCtx, graph.NewDAG())
		Expect(intctrlutil.IsRequeueError(err)).Should(BeTrue())
		comp.Spec.Instances[0].ServiceVersion = "4.0.0"
		err = load.Transform(transCtx, graph.NewDAG())
		Expect(intctrlutil.IsRequeueError(err)).Should(BeTrue())
		comp.Spec.Instances[0].ServiceVersion = "2.0.0"
		Expect(load.Transform(transCtx, graph.NewDAG())).Should(Succeed())
	})
})

func TestInstanceTemplateRegistryPreservedOnReconcile(t *testing.T) {
	for _, withVersion := range []bool{false, true} {
		name := "without-cv"
		if withVersion {
			name = "equal-default-cv"
		}
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			oldRegistry := viper.Get(constant.CfgRegistries)
			t.Cleanup(func() {
				viper.Set(constant.CfgRegistries, oldRegistry)
				g.Expect(intctrlutil.LoadRegistryConfig()).To(Succeed())
			})
			viper.Set(constant.CfgRegistries, map[string]any{})
			g.Expect(intctrlutil.LoadRegistryConfig()).To(Succeed())
			scheme := runtime.NewScheme()
			g.Expect(appsv1.AddToScheme(scheme)).To(Succeed())
			g.Expect(workloads.AddToScheme(scheme)).To(Succeed())
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
			def := testapps.NewComponentDefinitionFactory("registry-pin").SetServiceVersion("1.0.0").SetRuntime(&corev1.Container{Name: "db", Image: "docker.io/apecloud/db:1"}).GetObject()
			def.Spec.Runtime.InitContainers = []corev1.Container{{Name: "setup", Image: "docker.io/apecloud/setup:1"}}
			def.Spec.LifecycleActions = nil
			def.Status.Phase = appsv1.AvailablePhase
			cb := fake.NewClientBuilder().WithScheme(scheme).WithObjects(def)
			if withVersion {
				ver := &appsv1.ComponentVersion{ObjectMeta: metav1.ObjectMeta{Name: "registry-pin", Labels: map[string]string{def.Name: def.Name}}, Spec: appsv1.ComponentVersionSpec{Releases: []appsv1.ComponentVersionRelease{{Name: "r1", ServiceVersion: "1.0.0", Images: map[string]string{"db": "docker.io/apecloud/db:1", "setup": "docker.io/apecloud/setup:1"}}}}, Status: appsv1.ComponentVersionStatus{Phase: appsv1.AvailablePhase}}
				cb.WithObjects(ver)
			}
			cli := cb.Build()
			cluster := testapps.NewClusterFactory("default", "registry-pin", "").GetObject()
			cluster.UID = "cluster-uid"
			comp, err := component.BuildComponent(cluster, &appsv1.ClusterComponentSpec{Name: "db", ComponentDef: def.Name, ServiceVersion: "1.0.0", Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "explicit", CompDef: def.Name, ServiceVersion: "1.0.0"}}}, nil, nil)
			g.Expect(err).To(Succeed())
			graphCli := model.NewGraphClient(cli)
			transform := &componentWorkloadTransformer{Client: cli}
			reconcile := func() *workloads.InstanceSet {
				transCtx := &componentTransformContext{Context: context.Background(), Client: graphCli, Component: comp, ComponentOrig: comp.DeepCopy(), Logger: logger}
				g.Expect((&componentLoadResourcesTransformer{}).Transform(transCtx, graph.NewDAG())).To(Succeed())
				dag := graph.NewDAG()
				graphCli.Root(dag, comp, comp, model.ActionStatusPtr())
				g.Expect(transform.Transform(transCtx, dag)).To(Succeed())
				objs := graphCli.FindAll(dag, &workloads.InstanceSet{})
				if len(objs) == 0 {
					return nil
				}
				return objs[0].(*workloads.InstanceSet)
			}
			running := reconcile()
			g.Expect(running).NotTo(BeNil())
			g.Expect(cli.Create(context.Background(), running)).To(Succeed())
			tree := kubebuilderx.NewObjectTree()
			tree.Logger = logger
			tree.Context = context.Background()
			tree.SetRoot(running)
			_, err = instanceset.NewRevisionUpdateReconciler().Reconcile(tree)
			g.Expect(err).To(Succeed())
			_, err = instanceset.NewReplicasAlignmentReconciler().Reconcile(tree)
			g.Expect(err).To(Succeed())
			pods := tree.List(&corev1.Pod{})
			g.Expect(pods).To(HaveLen(1))
			livePod := pods[0].(*corev1.Pod)
			initialUpdated, err := instanceset.IsPodUpdated(running, livePod)
			g.Expect(err).To(Succeed())
			g.Expect(initialUpdated).To(BeTrue())
			viper.Set(constant.CfgRegistries, map[string]any{"defaultRegistry": "mirror.invalid"})
			g.Expect(intctrlutil.LoadRegistryConfig()).To(Succeed())
			comp.Annotations["unrelated"] = "changed"
			changed := reconcile()
			if changed == nil {
				changed = running
			}
			tree.SetRoot(changed)
			_, err = instanceset.NewRevisionUpdateReconciler().Reconcile(tree)
			g.Expect(err).To(Succeed())
			unchanged, err := instanceset.IsPodUpdated(changed, livePod)
			g.Expect(err).To(Succeed())
			if !unchanged {
				t.Errorf("registry-only reconciliation now classifies the existing Pod as requiring an update")
			}
			g.Expect(tree.Delete(livePod)).To(Succeed())
			_, err = instanceset.NewReplicasAlignmentReconciler().Reconcile(tree)
			g.Expect(err).To(Succeed())
			replacements := tree.List(&corev1.Pod{})
			g.Expect(replacements).To(HaveLen(1))
			replacement := replacements[0].(*corev1.Pod)
			if replacement.Spec.Containers[0].Image != "docker.io/apecloud/db:1" {
				t.Errorf("natural replacement changes pinned runtime registry: %s", replacement.Spec.Containers[0].Image)
			}
			if replacement.Spec.InitContainers[0].Image != "docker.io/apecloud/setup:1" {
				t.Errorf("natural replacement changes pinned init registry: %s", replacement.Spec.InitContainers[0].Image)
			}
			templates := instanceset.BuildInstanceTemplateExts(&instanceset.InstanceSetExt{Its: changed, InstanceTemplates: instanceset.BuildInstanceTemplates(*changed.Spec.Replicas, changed.Spec.Instances, nil)})
			if got := templates[0].Spec.Containers[0].Image; got != "docker.io/apecloud/db:1" {
				t.Errorf("registry-only reconcile changed effective selected runtime: %s; parent image=%s; overrides=%v", got, changed.Spec.Template.Spec.Containers[0].Image, changed.Spec.Instances[0].Images)
			}
			if got := templates[0].Spec.InitContainers[0].Image; got != "docker.io/apecloud/setup:1" {
				t.Errorf("registry-only reconcile changed effective selected init: %s", got)
			}
		})
	}
}
