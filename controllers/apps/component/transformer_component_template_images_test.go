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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestFirstExplicitParentTemplateSelection(t *testing.T) {
	for _, mode := range []string{"new-inherited-first-selection", "established-same-pair", "released-unmarked-bootstrap"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			oldRegistry := viper.Get(constant.CfgRegistries)
			t.Cleanup(func() {
				viper.Set(constant.CfgRegistries, oldRegistry)
				if err := intctrlutil.LoadRegistryConfig(); err != nil {
					t.Fatal(err)
				}
			})
			registry := func(value string) {
				t.Helper()
				viper.Set(constant.CfgRegistries, map[string]any{"defaultRegistry": value})
				if err := intctrlutil.LoadRegistryConfig(); err != nil {
					t.Fatal(err)
				}
			}
			registry("")
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, workloads.AddToScheme, corev1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			mkdef := func(name, team, version string) *appsv1.ComponentDefinition {
				def := testapps.NewComponentDefinitionFactory(name).SetServiceVersion(version).SetRuntime(&corev1.Container{Name: "db", Image: "docker.io/" + team + "/db:1"}).GetObject()
				def.Spec.Runtime.InitContainers = []corev1.Container{{Name: "setup", Image: "docker.io/" + team + "/setup:1"}}
				def.Spec.LifecycleActions = nil
				def.Status.Phase = appsv1.AvailablePhase
				return def
			}
			parent := mkdef("review-parent", "parent", "9.0.0")
			cb := fake.NewClientBuilder().WithScheme(scheme).WithObjects(parent)
			cli := cb.Build()
			cluster := testapps.NewClusterFactory("default", "review-image-chain", "").GetObject()
			cluster.UID = "review-cluster"
			comp, err := component.BuildComponent(cluster, &appsv1.ClusterComponentSpec{Name: "db", ComponentDef: parent.Name, ServiceVersion: "9.0.0", Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary"}}}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if mode != "new-inherited-first-selection" {
				comp.Spec.Instances[0].CompDef = parent.Name
				comp.Spec.Instances[0].ServiceVersion = parent.Spec.ServiceVersion
			}
			graphCli := model.NewGraphClient(cli)
			var running *workloads.InstanceSet
			reconcile := func(stage string) *workloads.InstanceSet {
				t.Helper()
				trans := &componentTransformContext{Context: ctx, Client: graphCli, Component: comp, ComponentOrig: comp.DeepCopy(), Logger: logr.Discard()}
				if err := (&componentLoadResourcesTransformer{}).Transform(trans, graph.NewDAG()); err != nil {
					t.Fatalf("%s load: %v", stage, err)
				}
				dag := graph.NewDAG()
				graphCli.Root(dag, comp, comp, model.ActionStatusPtr())
				if err := (&componentWorkloadTransformer{Client: cli}).Transform(trans, dag); err != nil {
					t.Fatalf("%s workload: %v", stage, err)
				}
				objs := graphCli.FindAll(dag, &workloads.InstanceSet{})
				if len(objs) == 0 {
					return running
				}
				result := objs[0].(*workloads.InstanceSet)
				if running == nil {
					err = cli.Create(ctx, result)
				} else {
					err = cli.Update(ctx, result)
				}
				if err != nil {
					t.Fatalf("%s persist: %v", stage, err)
				}
				running = result
				return result
			}
			check := func(stage, db, setup string) {
				t.Helper()
				its := reconcile(stage)
				if its == nil {
					t.Fatalf("%s no workload", stage)
				}
				tree := kubebuilderx.NewObjectTree()
				tree.Logger = logr.Discard()
				tree.Context = ctx
				tree.SetRoot(its)
				if _, err := instanceset.NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
					t.Fatalf("%s revision: %v", stage, err)
				}
				if _, err := instanceset.NewReplicasAlignmentReconciler().Reconcile(tree); err != nil {
					t.Fatalf("%s alignment: %v", stage, err)
				}
				pods := tree.List(&corev1.Pod{})
				if len(pods) != 1 {
					t.Fatalf("%s pods: %d", stage, len(pods))
				}
				pod := pods[0].(*corev1.Pod)
				if got := pod.Spec.Containers[0].Image; got != db {
					t.Errorf("%s replacement db: got %q want %q", stage, got, db)
				}
				if got := pod.Spec.InitContainers[0].Image; got != setup {
					t.Errorf("%s replacement setup: got %q want %q", stage, got, setup)
				}
			}
			check("initial persisted workload", "docker.io/parent/db:1", "docker.io/parent/setup:1")
			t.Logf("initial inherited=%v marker=%q overrides=%v parent=%v", mode == "new-inherited-first-selection", running.Annotations[instanceTemplateImageSelectionsAnnotation], running.Spec.Instances[0].Images, running.Spec.Template.Spec.Containers[0].Image)
			if mode == "new-inherited-first-selection" {
				if len(running.Spec.Instances[0].Images) > 0 {
					t.Fatal("expected initial inheritance without overrides")
				}
			}
			if mode == "released-unmarked-bootstrap" {
				delete(running.Annotations, instanceTemplateImageSelectionsAnnotation)
				running.Spec.Instances[0].Images = nil
				if err := cli.Update(ctx, running); err != nil {
					t.Fatal(err)
				}
			}
			registry("mirror.invalid")
			check("registry reload before selection", "docker.io/parent/db:1", "docker.io/parent/setup:1")
			if mode == "new-inherited-first-selection" {
				comp.Spec.Instances[0].CompDef = parent.Name
				comp.Spec.Instances[0].ServiceVersion = parent.Spec.ServiceVersion
				check("first explicit parent pair resolves current registry", "mirror.invalid/parent/db:1", "mirror.invalid/parent/setup:1")
				t.Logf("after first selection marker=%q overrides=%v", running.Annotations[instanceTemplateImageSelectionsAnnotation], running.Spec.Instances[0].Images)
			} else {
				check("existing selection retains old exact references", "docker.io/parent/db:1", "docker.io/parent/setup:1")
			}
		})
	}
}

func buildLargeTemplateWorkload(t *testing.T, profile string) *workloads.InstanceSet {
	t.Helper()
	const replicas = int32(3000)
	scheme := runtime.NewScheme()
	for _, f := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme} {
		if err := f(scheme); err != nil {
			t.Fatal(err)
		}
	}
	def := &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "selected-postgres-17-v1"}, Spec: appsv1.ComponentDefinitionSpec{ServiceVersion: "17.0.0", Runtime: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "docker.io/postgres:17"}}, InitContainers: []corev1.Container{{Name: "prepare", Image: "docker.io/prepare:1"}}}}, Status: appsv1.ComponentDefinitionStatus{Phase: appsv1.AvailablePhase}}
	if profile == "long-common" {
		def.Name = strings.Repeat("d", 63)
		def.Spec.ServiceVersion = "17.0.0-" + strings.Repeat("a", 25)
	}
	if profile == "long-common" || profile == "diverse-long-containers" {
		def.Spec.Runtime.Containers[0].Name = strings.Repeat("d", 63)
		def.Spec.Runtime.InitContainers[0].Name = strings.Repeat("p", 63)
	}
	objects := []client.Object{def}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: types.UID("cluster-uid")}, Spec: appsv1.ClusterSpec{TerminationPolicy: appsv1.Delete}}
	spec := &appsv1.ClusterComponentSpec{Name: "db", ComponentDef: def.Name, ServiceVersion: def.Spec.ServiceVersion, Replicas: replicas}
	for i := int32(0); i < replicas; i++ {
		selectedName := def.Name
		if strings.HasPrefix(profile, "diverse") {
			selected := def.DeepCopy()
			selected.Name = strings.Repeat("d", 58) + fmt.Sprintf("%05d", i)
			selectedName = selected.Name
			objects = append(objects, selected)
		}
		spec.Instances = append(spec.Instances, appsv1.InstanceTemplate{Name: fmt.Sprintf("replica-%04d", i), Replicas: ptr.To(int32(1)), CompDef: selectedName, ServiceVersion: def.Spec.ServiceVersion, Env: []corev1.EnvVar{{Name: "REPLICA_ID", Value: fmt.Sprint(i)}}})
	}
	comp, err := component.BuildComponent(cluster, spec, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	comp.UID = types.UID("component-uid")
	if err := validateCompReplicas(comp, def); err != nil {
		t.Fatal(err)
	}
	objects = append(objects, comp)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	gc := model.NewGraphClient(cli)
	ctx := &componentTransformContext{Context: context.Background(), Client: gc, Component: comp, ComponentOrig: comp.DeepCopy(), Logger: logr.Discard()}
	dag := graph.NewDAG()
	gc.Root(dag, comp, comp, model.ActionStatusPtr())
	if err := (&componentLoadResourcesTransformer{}).Transform(ctx, dag); err != nil {
		t.Fatal(err)
	}
	if err := (&componentWorkloadTransformer{Client: cli}).Transform(ctx, dag); err != nil {
		t.Fatal(err)
	}
	objs := gc.FindAll(dag, &workloads.InstanceSet{})
	if len(objs) != 1 {
		t.Fatalf("workloads=%d", len(objs))
	}
	its := objs[0].(*workloads.InstanceSet)
	selections, marked, err := readInstanceTemplateImageSelections(its)
	if err != nil || !marked || len(selections) != int(replicas) {
		t.Fatalf("selection history lost records: count=%d marked=%v err=%v", len(selections), marked, err)
	}
	for _, tpl := range spec.Instances {
		selection := selections[tpl.Name]
		if selection.CompDef != tpl.CompDef || selection.ServiceVersion != tpl.ServiceVersion || len(selection.RuntimeDefaults) != 2 {
			t.Fatalf("selection history changed for %s: %#v", tpl.Name, selection)
		}
	}
	return its
}

func TestTemplateImageSelectionAnnotationBudget(t *testing.T) {
	for _, profile := range []string{"shared", "long-common", "diverse", "diverse-long-containers"} {
		t.Run(profile, func(t *testing.T) {
			its := buildLargeTemplateWorkload(t, profile)
			data, err := json.Marshal(its)
			if err != nil {
				t.Fatal(err)
			}
			var budget int
			for k, v := range its.Annotations {
				budget += len(k) + len(v)
			}
			t.Logf("replicas=%d templates=%d workload JSON bytes=%d annotation bytes=%d", *its.Spec.Replicas, len(its.Spec.Instances), len(data), budget)
			if err := apivalidation.ValidateAnnotationsSize(its.Annotations); err != nil {
				t.Fatalf("generated workload cannot pass API metadata validation: %v", err)
			}
		})
	}
}

func TestTemplateImageSelectionAnnotationAPI(t *testing.T) {
	env := &envtest.Environment{CRDDirectoryPaths: []string{"../../../config/crd/bases"}, ErrorIfCRDPathMissing: true}
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Error(err)
		}
	})
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cli, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "size-probe"}}
	if err := cli.Create(context.Background(), ns); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"shared", "long-common", "diverse", "diverse-long-containers"} {
		t.Run(profile, func(t *testing.T) {
			its := buildLargeTemplateWorkload(t, profile)
			its.Namespace = ns.Name
			its.Name = profile
			want, _, err := readInstanceTemplateImageSelections(its)
			if err != nil {
				t.Fatal(err)
			}
			err = cli.Create(context.Background(), its)
			if err != nil {
				t.Fatalf("API rejected generated workload: %v", err)
			}
			stored := &workloads.InstanceSet{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(its), stored); err != nil {
				t.Fatal(err)
			}
			got, marked, err := readInstanceTemplateImageSelections(stored)
			if err != nil || !marked || !reflect.DeepEqual(got, want) {
				t.Fatalf("API persistence changed selection history: records=%d want=%d marked=%v err=%v", len(got), len(want), marked, err)
			}
		})
	}
}

func TestStartDefersTemplatePublicationGeneration(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	def := testapps.NewComponentDefinitionFactory("start-template").SetServiceVersion("1.0.0").SetRuntime(&corev1.Container{Name: "db", Image: "docker.io/library/db:1"}).GetObject()
	def.Spec.LifecycleActions = nil
	def.Status.Phase = appsv1.AvailablePhase
	next := testapps.NewComponentDefinitionFactory("start-template-next").SetServiceVersion("2.0.0").SetRuntime(&corev1.Container{Name: "db", Image: "docker.io/library/db:2"}).GetObject()
	next.Spec.LifecycleActions = nil
	next.Status.Phase = appsv1.AvailablePhase
	cluster := testapps.NewClusterFactory("default", "start-upgrade", "").GetObject()
	cluster.UID = "cluster"
	comp, err := component.BuildComponent(cluster, &appsv1.ClusterComponentSpec{Name: "db", ComponentDef: def.Name, ServiceVersion: def.Spec.ServiceVersion, Replicas: 1, Instances: []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1)), CompDef: def.Name, ServiceVersion: def.Spec.ServiceVersion}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	comp.Generation = 1
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(def, next).Build()
	gc := model.NewGraphClient(cli)
	var running *workloads.InstanceSet
	var trans *componentTransformContext
	reconcile := func() {
		t.Helper()
		trans = &componentTransformContext{Context: ctx, Client: gc, Component: comp, ComponentOrig: comp.DeepCopy(), Logger: logr.Discard(), EventRecorder: record.NewFakeRecorder(100)}
		if err := (&componentLoadResourcesTransformer{}).Transform(trans, graph.NewDAG()); err != nil {
			t.Fatal(err)
		}
		dag := graph.NewDAG()
		gc.Root(dag, comp, comp, model.ActionStatusPtr())
		if err := (&componentWorkloadTransformer{Client: cli}).Transform(trans, dag); err != nil {
			t.Fatal(err)
		}
		objs := gc.FindAll(dag, &workloads.InstanceSet{})
		if len(objs) == 0 {
			return
		}
		result := objs[0].(*workloads.InstanceSet)
		if running == nil {
			err = cli.Create(ctx, result)
		} else {
			err = cli.Update(ctx, result)
		}
		if err != nil {
			t.Fatal(err)
		}
		running = result
	}
	reconcile()
	stopped := running.DeepCopy()
	if err := (&componentWorkloadTransformer{}).stopWorkload(trans.SynthesizeComponent, running, stopped); err != nil {
		t.Fatal(err)
	}
	running = stopped
	if err := cli.Update(ctx, running); err != nil {
		t.Fatal(err)
	}
	comp.Generation = 2
	comp.Status.Phase = appsv1.StartingComponentPhase
	comp.Spec.Instances[0].CompDef = next.Name
	comp.Spec.Instances[0].ServiceVersion = next.Spec.ServiceVersion
	reconcile()
	if isWorkloadStopped(running) {
		t.Fatal("Start did not clear stop")
	}
	if got := running.Spec.Instances[0].Images["db"]; got != "docker.io/library/db:1" {
		t.Fatalf("Start published unexpected image: %s", got)
	}
	if got := running.Annotations[constant.KubeBlocksGenerationKey]; got != "1" {
		t.Errorf("Start acknowledged unpublished template target: generation=%s", got)
	}
	updateStatus := func(wantGeneration int64, wantPhase appsv1.ComponentPhase) {
		t.Helper()
		tree := kubebuilderx.NewObjectTree()
		tree.Context = ctx
		tree.Logger = logr.Discard()
		tree.SetRoot(running)
		for _, r := range []kubebuilderx.Reconciler{instanceset.NewRevisionUpdateReconciler(), instanceset.NewReplicasAlignmentReconciler()} {
			if _, err := r.Reconcile(tree); err != nil {
				t.Fatal(err)
			}
		}
		pods := tree.List(&corev1.Pod{})
		if len(pods) != 1 {
			t.Fatalf("pods=%d", len(pods))
		}
		pod := pods[0].(*corev1.Pod)
		pod.CreationTimestamp = metav1.Now()
		pod.UID = "ready-pod"
		pod.Status = corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: pod.CreationTimestamp}}, ContainerStatuses: []corev1.ContainerStatus{{Name: "db", Image: pod.Spec.Containers[0].Image, ImageID: "containerd://sha256:1", Ready: true}}}
		if err := tree.Update(pod); err != nil {
			t.Fatal(err)
		}
		if _, err := instanceset.NewStatusReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		trans.RunningWorkload = running
		dag := graph.NewDAG()
		gc.Root(dag, trans.ComponentOrig, comp, model.ActionStatusPtr())
		if err := (&componentStatusTransformer{Client: cli}).Transform(trans, dag); err != nil {
			t.Fatal(err)
		}
		if comp.Status.ObservedGeneration != wantGeneration || comp.Status.Phase != wantPhase {
			t.Errorf("Component acknowledged generation=%d phase=%s, want generation=%d phase=%s", comp.Status.ObservedGeneration, comp.Status.Phase, wantGeneration, wantPhase)
		}
	}
	updateStatus(1, appsv1.StartingComponentPhase)
	reconcile()
	if got := running.Spec.Instances[0].Images["db"]; got != "docker.io/library/db:2" {
		t.Fatalf("normal reconcile did not publish target: %s", got)
	}
	if got := running.Annotations[constant.KubeBlocksGenerationKey]; got != "2" {
		t.Fatalf("normal reconcile generation=%s", got)
	}
	updateStatus(2, appsv1.RunningComponentPhase)
}

func TestTemplateImageSelectionReorderAndRemoval(t *testing.T) {
	synth := &component.SynthesizedComponent{Instances: []appsv1.InstanceTemplate{{Name: "a", CompDef: "selected", ServiceVersion: "1.0.0"}, {Name: "b", CompDef: "selected", ServiceVersion: "2.0.0"}, {Name: "inherited"}}, InstanceImageDefaults: map[string][]string{"a": {"db"}, "b": {"db"}}}
	old := &workloads.InstanceSet{Spec: workloads.InstanceSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "parent"}}}}, Instances: []workloads.InstanceTemplate{{Name: "a", Images: map[string]string{"db": "exact-a"}}, {Name: "b", Images: map[string]string{"db": "exact-b"}}, {Name: "inherited"}}}}
	if err := reconcileInstanceTemplateImages(synth, nil, old); err != nil {
		t.Fatal(err)
	}
	desired := old.DeepCopy()
	desired.Spec.Instances = []workloads.InstanceTemplate{{Name: "b", Images: map[string]string{"db": "fresh-b"}}, {Name: "inherited"}, {Name: "a", Images: map[string]string{"db": "fresh-a"}}}
	if err := reconcileInstanceTemplateImages(synth, old, desired); err != nil {
		t.Fatal(err)
	}
	if desired.Spec.Instances[0].Images["db"] != "exact-b" || desired.Spec.Instances[2].Images["db"] != "exact-a" {
		t.Fatalf("reorder crossed image ownership: %#v", desired.Spec.Instances)
	}
	selections, marked, err := readInstanceTemplateImageSelections(desired)
	if err != nil || !marked || selections["a"].ServiceVersion != "1.0.0" || selections["b"].ServiceVersion != "2.0.0" {
		t.Fatalf("reordered selections: %#v, marked=%v, err=%v", selections, marked, err)
	}
	old = desired
	synth.Instances = []appsv1.InstanceTemplate{{Name: "a", CompDef: "selected", ServiceVersion: "1.0.0"}}
	desired = old.DeepCopy()
	desired.Spec.Instances = desired.Spec.Instances[2:]
	if err := reconcileInstanceTemplateImages(synth, old, desired); err != nil {
		t.Fatal(err)
	}
	selections, _, err = readInstanceTemplateImageSelections(desired)
	if err != nil || len(selections) != 1 || selections["a"].ServiceVersion != "1.0.0" {
		t.Fatalf("removed selection persisted: %#v, err=%v", selections, err)
	}
}

func TestReadTemplateImageSelectionBoundary(t *testing.T) {
	for _, value := range []string{"null", "", `{"canary":{"compDef":"selected","serviceVersion":"1.0.0"}}`, `{"s":[{"c":"selected","v":"1.0.0"}],"t":[]}`, `{"s":[{"c":"selected","v":"1.0.0"}],"t":[2]}`, `{"t":[-1]}`, `{"s":[{"v":"1.0.0"}],"t":[1]}`, `{} {}`} {
		t.Run(value, func(t *testing.T) {
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{instanceTemplateImageSelectionsAnnotation: encodeTemplateImageSelectionTestState(t, value)}}, Spec: workloads.InstanceSetSpec{Instances: []workloads.InstanceTemplate{{Name: "canary"}}}}
			if _, marked, err := readInstanceTemplateImageSelections(its); !marked || err == nil {
				t.Fatalf("malformed owned state accepted: marked=%v err=%v", marked, err)
			}
		})
	}
}

func encodeTemplateImageSelectionTestState(t *testing.T, value string) string {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	if _, err := compressed.Write([]byte(value)); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return instanceTemplateImageSelectionsVersion + base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func TestReadTemplateImageSelectionEncoding(t *testing.T) {
	valid := encodeTemplateImageSelectionTestState(t, `{}`)
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(valid, instanceTemplateImageSelectionsVersion))
	if err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Clone(raw)
	corrupt[len(corrupt)-8] ^= 1
	for name, value := range map[string]string{
		"unversioned prior map":     `{"canary":{"compDef":"selected","serviceVersion":"1.0.0"}}`,
		"unversioned compact state": `{"s":[],"t":[]}`,
		"unknown version":           "v2:" + strings.TrimPrefix(valid, instanceTemplateImageSelectionsVersion),
		"invalid base64":            instanceTemplateImageSelectionsVersion + "$$",
		"invalid gzip":              instanceTemplateImageSelectionsVersion + base64.StdEncoding.EncodeToString([]byte("not gzip")),
		"checksum failure":          instanceTemplateImageSelectionsVersion + base64.StdEncoding.EncodeToString(corrupt),
		"truncated gzip":            instanceTemplateImageSelectionsVersion + base64.StdEncoding.EncodeToString(raw[:len(raw)-1]),
		"gzip trailing data":        instanceTemplateImageSelectionsVersion + base64.StdEncoding.EncodeToString(append(bytes.Clone(raw), 'x')),
	} {
		t.Run(name, func(t *testing.T) {
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{instanceTemplateImageSelectionsAnnotation: value}}}
			if _, marked, err := readInstanceTemplateImageSelections(its); !marked || err == nil {
				t.Fatalf("malformed encoding accepted: marked=%v err=%v", marked, err)
			}
		})
	}
}
