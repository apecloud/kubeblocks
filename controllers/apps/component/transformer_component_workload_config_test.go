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
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func instanceConfigFixture(t *testing.T) (*appsv1.Component, *component.SynthesizedComponent, *runtime.Scheme) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "db-mysql", Namespace: "default", UID: types.UID("current-component"), Labels: constant.GetCompLabels("db", "mysql")}}
	synth := &component.SynthesizedComponent{
		Namespace: comp.Namespace, ClusterName: "db", Name: "mysql", FullCompName: comp.Name, Generation: "2", Replicas: 1,
		PodSpec:          &corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:old"}}, Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "default-config"}}}}}},
		Configs:          []workloads.ConfigTemplate{{Name: "main", ConfigHash: ptr.To("default-hash"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "default-config"}}},
		LifecycleActions: component.SynthesizedLifecycleActions{ComponentLifecycleActions: &appsv1.ComponentLifecycleActions{}},
	}
	return comp, synth, scheme
}

func managedInstanceConfig(comp *appsv1.Component, name string) *corev1.ConfigMap {
	labels := constant.GetCompLabels("db", "mysql")
	labels[instanceConfigMapLabelKey] = "true"
	labels[instanceConfigMapOwnerLabelKey] = string(comp.UID)
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: comp.Namespace, UID: types.UID(name + "-uid"), Labels: labels,
		OwnerReferences: []metav1.OwnerReference{{APIVersion: appsv1.GroupVersion.String(), Kind: "Component", Name: comp.Name, UID: comp.UID, Controller: ptr.To(true)}}}, Data: map[string]string{"config": "value"}}
}

func workloadConfigPlan(t *testing.T, cli client.Client, comp *appsv1.Component, synth *component.SynthesizedComponent, running *workloads.InstanceSet, maps ...*corev1.ConfigMap) (*componentPlan, error) {
	t.Helper()
	graphCli := model.NewGraphClient(cli)
	dag := graph.NewDAG()
	graphCli.Root(dag, comp, comp, model.ActionStatusPtr())
	transCtx := &componentTransformContext{Context: context.Background(), Client: graphCli, Component: comp, ComponentOrig: comp.DeepCopy(), CompDef: &appsv1.ComponentDefinition{}, SynthesizeComponent: synth, RunningWorkload: running, InstanceConfigMaps: maps}
	for _, cm := range maps {
		old := &corev1.ConfigMap{}
		if err := cli.Get(transCtx.Context, client.ObjectKeyFromObject(cm), old); apierrors.IsNotFound(err) {
			graphCli.Create(dag, cm)
		} else if err != nil {
			t.Fatal(err)
		} else {
			updated := old.DeepCopy()
			updated.Data = cm.Data
			graphCli.Update(dag, old, updated)
		}
	}
	builder := &componentPlanBuilder{cli: cli, transCtx: transCtx}
	plan := &componentPlan{dag: dag, transCtx: transCtx, cleanupClient: cli, walkFunc: func(v graph.Vertex) error {
		if _, ok := v.(*model.ObjectVertex).Obj.(*appsv1.Component); ok {
			return nil
		}
		return builder.defaultWalkFunc(v)
	}}
	err := (&componentWorkloadTransformer{Client: cli}).Transform(transCtx, dag)
	return plan, err
}

func TestInstanceConfigPublicationPrecedesWorkload(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "start update"}[update], func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1))}}
			synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{"canary": {{Name: "main", ConfigHash: ptr.To("special-hash"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "special"}}}}
			cm := managedInstanceConfig(comp, "special")
			var running *workloads.InstanceSet
			var objects []client.Object
			if update {
				var err error
				running, err = component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
				if err != nil {
					t.Fatal(err)
				}
				running.Spec.Stop = ptr.To(true)
				running.Spec.Instances[0].Configs = nil
				running.Annotations[constant.KubeBlocksGenerationKey] = "1"
				objects = append(objects, running)
				synth.PodSpec.Containers[0].Image = "db:new"
			}
			failMap := true
			workloadWrites := 0
			base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			cli := interceptor.NewClient(base, interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if _, ok := obj.(*corev1.ConfigMap); ok && failMap {
						return errors.New("config persistence failed")
					}
					if _, ok := obj.(*workloads.InstanceSet); ok {
						workloadWrites++
						if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
							t.Fatal("workload published before config")
						}
					}
					return c.Create(ctx, obj, opts...)
				},
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if _, ok := obj.(*workloads.InstanceSet); ok {
						workloadWrites++
						if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
							t.Fatal("workload published before config")
						}
					}
					return c.Update(ctx, obj, opts...)
				},
			})
			plan, err := workloadConfigPlan(t, cli, comp, synth, running, cm)
			if err != nil {
				t.Fatal(err)
			}
			if err := plan.Execute(); err == nil {
				t.Fatal("expected CM failure")
			}
			if workloadWrites != 0 {
				t.Fatal("workload written despite prerequisite failure")
			}
			failMap = false
			if err := plan.Execute(); err != nil {
				t.Fatal(err)
			}
			its := &workloads.InstanceSet{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: comp.Namespace, Name: comp.Name}, its); err != nil {
				t.Fatal(err)
			}
			if ptr.Deref(its.Spec.Stop, false) {
				t.Fatal("Start barrier not released")
			}
			if its.Spec.Instances[0].Configs[0].Volume.ConfigMapName != "special" || *its.Spec.Instances[0].Configs[0].ConfigHash != "special-hash" {
				t.Fatal("first running workload lacks desired config")
			}
			if update && (its.Spec.Template.Spec.Containers[0].Image != "db:old" || its.Annotations[constant.KubeBlocksGenerationKey] != "1") {
				t.Fatal("Start published deferred unrelated payload")
			}
		})
	}
}

func TestInstanceConfigCollectionWaitsForReferences(t *testing.T) {
	for _, reference := range []string{"published workload", "instance", "terminating pod", "none", "previous component"} {
		t.Run(reference, func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			cm := managedInstanceConfig(comp, "retired")
			if reference == "previous component" {
				cm.OwnerReferences[0].UID = "previous-component"
			}
			running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
			if err != nil {
				t.Fatal(err)
			}
			objects := []client.Object{cm, running}
			spec := corev1.PodSpec{Volumes: []corev1.Volume{{Name: "old", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name}}}}}}
			if reference == "published workload" {
				running.Spec.Instances = []workloads.InstanceTemplate{{Name: "removed", Replicas: ptr.To(int32(0)), Configs: []workloads.ConfigTemplate{{Name: "main", Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "old", ConfigMapName: cm.Name}}}}}
			}
			if reference == "instance" {
				inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: comp.Name + "-0", Namespace: comp.Namespace, Labels: constant.GetCompLabels("db", "mysql")}}
				inst.Spec.Template.Spec = spec
				objects = append(objects, inst)
			}
			if reference == "terminating pod" {
				objects = append(objects, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: comp.Name + "-0", Namespace: comp.Namespace, Labels: constant.GetCompLabels("db", "mysql"), DeletionTimestamp: ptr.To(metav1.Now()), Finalizers: []string{"hold"}}, Spec: spec})
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			plan, err := workloadConfigPlan(t, cli, comp, synth, running)
			retained := reference != "none" && reference != "previous component"
			if err != nil {
				t.Fatal(err)
			}
			if retained && !intctrlutil.IsDelayedRequeueError(plan.transCtx.instanceConfigMapRequeue) {
				t.Fatalf("expected retry while old reference exists, got %v", plan.transCtx.instanceConfigMapRequeue)
			}
			if err := plan.Execute(); err != nil {
				t.Fatal(err)
			}
			err = cli.Get(context.Background(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})
			shouldExist := reference != "none"
			if shouldExist && err != nil {
				t.Fatalf("referenced or predecessor CM deleted: %v", err)
			}
			if !shouldExist && !apierrors.IsNotFound(err) {
				t.Fatalf("unreferenced CM not collected: %v", err)
			}
		})
	}
}

func TestInstanceConfigCollectionRetainsRemotePod(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	comp.Spec.EnableInstanceAPI = ptr.To(true)
	comp.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "new-provider"}
	authoritative := managedInstanceConfig(comp, "retired")
	mirrored := authoritative.DeepCopy()
	mirrored.OwnerReferences = nil
	mirrored.UID = "remote-copy"
	mirrored.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "old-provider,new-provider"}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: comp.Name + "-0", Namespace: comp.Namespace, Labels: constant.GetCompLabels("db", "mysql")}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: mirrored.Name}}}}}}}
	controlPod := pod.DeepCopy()
	controlPod.Name += "-terminating"
	controlPod.DeletionTimestamp = ptr.To(metav1.Now())
	controlPod.Finalizers = []string{"hold"}
	control := fake.NewClientBuilder().WithScheme(scheme).WithObjects(authoritative, controlPod).Build()
	oldProvider := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mirrored, pod).Build()
	newProvider := fake.NewClientBuilder().WithScheme(scheme).Build()
	cli := multicluster.NewClient(control, map[string]client.Client{"old-provider": oldProvider, "new-provider": newProvider})
	plan, err := workloadConfigPlan(t, cli, comp, synth, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !intctrlutil.IsDelayedRequeueError(plan.transCtx.instanceConfigMapRequeue) {
		t.Fatalf("expected remote reference retry, got %v", plan.transCtx.instanceConfigMapRequeue)
	}
	if err := plan.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := oldProvider.Delete(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	current := &workloads.InstanceSet{}
	if err := control.Get(context.Background(), client.ObjectKeyFromObject(comp), current); err != nil {
		t.Fatal(err)
	}
	plan, err = workloadConfigPlan(t, cli, comp, synth, current)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := oldProvider.Get(context.Background(), client.ObjectKeyFromObject(mirrored), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("control-plane terminating Pod lost its reference: %v", err)
	}
	if err := control.Get(context.Background(), client.ObjectKeyFromObject(controlPod), controlPod); err != nil {
		t.Fatal(err)
	}
	controlPod.Finalizers = nil
	if err := control.Update(context.Background(), controlPod); err != nil {
		t.Fatal(err)
	}
	if err := control.Get(context.Background(), client.ObjectKeyFromObject(comp), current); err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		plan, err = workloadConfigPlan(t, cli, comp, synth, current)
		if err != nil {
			t.Fatal(err)
		}
		if err := plan.Execute(); err != nil {
			t.Fatal(err)
		}
		if pass == 0 {
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(authoritative), &corev1.ConfigMap{}); err != nil {
				t.Fatalf("authority removed before mirror absence was observed: %v", err)
			}
		}
		if err := control.Get(context.Background(), client.ObjectKeyFromObject(comp), current); err != nil {
			t.Fatal(err)
		}
	}
	for _, provider := range []client.Client{control, oldProvider} {
		if err := provider.Get(context.Background(), client.ObjectKeyFromObject(authoritative), &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
			t.Fatalf("retired provider CM remains: %v", err)
		}
	}
}

func TestInstanceConfigStartClearsRemovedOverride(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1))}}
	running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
	if err != nil {
		t.Fatal(err)
	}
	running.Spec.Stop = ptr.To(true)
	running.Spec.Instances[0].Configs = []workloads.ConfigTemplate{{Name: "main", ConfigHash: ptr.To("old-special-hash"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "removed-special"}}}
	running.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "old-default"
	running.Spec.InstanceAssistantObjects = []corev1.ObjectReference{{Kind: "Secret", Name: "kept"}, {Kind: "ConfigMap", Name: "removed-special"}}
	synth.InstanceAssistantObjects = []corev1.ObjectReference{{Kind: "Secret", Name: "deferred"}, {Kind: "ConfigMap", Name: "default-config"}}
	synth.Configs[0].ConfigHash = ptr.To("new-default-hash")
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
	plan, err := workloadConfigPlan(t, cli, comp, synth, running)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Execute(); err != nil {
		t.Fatal(err)
	}
	current := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(running), current); err != nil {
		t.Fatal(err)
	}
	if len(current.Spec.Instances[0].Configs) != 0 || current.Spec.Template.Spec.Volumes[0].ConfigMap.Name != "default-config" || *current.Spec.Configs[0].ConfigHash != "new-default-hash" {
		t.Fatal("Start retained removed override or stale default config")
	}
	if len(current.Spec.InstanceAssistantObjects) != 2 || current.Spec.InstanceAssistantObjects[0].Name != "kept" || current.Spec.InstanceAssistantObjects[1].Name != "default-config" {
		t.Fatal("Start changed unrelated assistant objects or retained old config assistant")
	}
}

func TestInstanceConfigDeletionRejectsSameNameReplacement(t *testing.T) {
	comp, _, scheme := instanceConfigFixture(t)
	stale := managedInstanceConfig(comp, "retired")
	replacement := stale.DeepCopy()
	replacement.UID = "replacement-uid"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(replacement).Build()
	err := deleteInstanceConfigMaps(context.Background(), cli, []instanceConfigMapDeletion{{configMap: stale}})
	if err == nil {
		t.Fatal("stale UID deletion succeeded")
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(replacement), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("same-name replacement deleted: %v", err)
	}
}

func TestInstanceConfigIncompleteObservationDoesNotBlockPublication(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	comp.Spec.EnableInstanceAPI = ptr.To(true)
	comp.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "unavailable"}
	cm := managedInstanceConfig(comp, "retired")
	running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
	if err != nil {
		t.Fatal(err)
	}
	synth.PodSpec.Containers[0].Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, running).Build()
	cli := interceptor.NewClient(base, interceptor.Funcs{List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
		if _, ok := list.(*corev1.ConfigMapList); ok {
			for _, option := range opts {
				if _, data := option.(*multicluster.ClientOption); data {
					return errors.New("provider unavailable")
				}
			}
		}
		return c.List(ctx, list, opts...)
	}})
	plan, err := workloadConfigPlan(t, cli, comp, synth, running)
	if err != nil {
		t.Fatal(err)
	}
	if !intctrlutil.IsDelayedRequeueError(plan.transCtx.instanceConfigMapRequeue) {
		t.Fatalf("expected retry: %v", plan.transCtx.instanceConfigMapRequeue)
	}
	if err := plan.Execute(); err != nil {
		t.Fatal(err)
	}
	current := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(running), current); err != nil {
		t.Fatal(err)
	}
	if current.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String() != "1" {
		t.Fatal("unavailable GC observation blocked normal publication")
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("incomplete observation collected CM: %v", err)
	}
}

func TestInstanceConfigStopFreezesPublishedConfig(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
	if err != nil {
		t.Fatal(err)
	}
	running = running.DeepCopy()
	synth.Stop = ptr.To(true)
	synth.Configs[0].ConfigHash = ptr.To("deferred-hash")
	synth.Configs[0].Volume.ConfigMapName = "deferred-map"
	synth.PodSpec.Volumes[0].ConfigMap.Name = "deferred-map"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
	plan, err := workloadConfigPlan(t, cli, comp, synth, running)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Execute(); err != nil {
		t.Fatal(err)
	}
	current := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(running), current); err != nil {
		t.Fatal(err)
	}
	if !ptr.Deref(current.Spec.Stop, false) || current.Spec.Template.Spec.Volumes[0].ConfigMap.Name != "default-config" || *current.Spec.Configs[0].ConfigHash != "default-hash" {
		t.Fatal("Stop did not preserve published config")
	}
}

func TestInstanceConfigCollectionResumesAfterFinalizerRemoval(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	cm := managedInstanceConfig(comp, "retired")
	cm.Finalizers = []string{constant.DBComponentFinalizerName}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	var running *workloads.InstanceSet
	for attempt := 0; attempt < 2; attempt++ {
		plan, err := workloadConfigPlan(t, cli, comp, synth, running)
		if err != nil {
			t.Fatal(err)
		}
		if !intctrlutil.IsDelayedRequeueError(plan.transCtx.instanceConfigMapRequeue) {
			t.Fatalf("expected collection retry: %v", plan.transCtx.instanceConfigMapRequeue)
		}
		if err := plan.Execute(); err != nil {
			t.Fatal(err)
		}
		running = &workloads.InstanceSet{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(comp), running); err != nil {
			t.Fatal(err)
		}
		current := &corev1.ConfigMap{}
		err = cli.Get(context.Background(), client.ObjectKeyFromObject(cm), current)
		if attempt == 0 && (err != nil || len(current.Finalizers) != 0) {
			t.Fatalf("finalizer removal did not persist: %v", err)
		}
		if attempt == 1 && !apierrors.IsNotFound(err) {
			t.Fatalf("resumed cleanup did not delete CM: %v", err)
		}
	}
}

func TestInstanceConfigMirrorRejectsForeignOwnership(t *testing.T) {
	comp, _, scheme := instanceConfigFixture(t)
	desired := managedInstanceConfig(comp, "special")
	foreign := desired.DeepCopy()
	foreign.OwnerReferences = nil
	foreign.Labels[instanceConfigMapOwnerLabelKey] = "previous-component"
	foreign.Data["config"] = "foreign"
	provider := fake.NewClientBuilder().WithScheme(scheme).WithObjects(foreign).Build()
	cli := multicluster.NewClient(fake.NewClientBuilder().WithScheme(scheme).Build(), map[string]client.Client{"a": provider})
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "a"}},
		Spec: workloads.InstanceSetSpec{EnableInstanceAPI: ptr.To(true), Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: desired.Name}}}}},
		}}}}
	if err := syncInstanceConfigMaps(context.Background(), cli, comp, its, []*corev1.ConfigMap{desired}); err == nil {
		t.Fatal("mirror adopted a previous Component's ConfigMap")
	}
	current := &corev1.ConfigMap{}
	if err := provider.Get(context.Background(), client.ObjectKeyFromObject(foreign), current); err != nil || current.Data["config"] != "foreign" {
		t.Fatalf("foreign ConfigMap was changed: %#v, %v", current, err)
	}
}

func TestInstanceConfigMirrorRetriesConcurrentUpdate(t *testing.T) {
	comp, _, scheme := instanceConfigFixture(t)
	desired := managedInstanceConfig(comp, "special")
	mirror := desired.DeepCopy()
	mirror.OwnerReferences = nil
	mirror.Data["config"] = "old"
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mirror).Build()
	conflict := true
	provider := interceptor.NewClient(base, interceptor.Funcs{Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
		if conflict {
			conflict = false
			concurrent := &corev1.ConfigMap{}
			if err := c.Get(ctx, client.ObjectKeyFromObject(obj), concurrent); err != nil {
				return err
			}
			concurrent.Data["config"] = "concurrent"
			if err := c.Update(ctx, concurrent); err != nil {
				return err
			}
		}
		return c.Update(ctx, obj, opts...)
	}})
	cli := multicluster.NewClient(fake.NewClientBuilder().WithScheme(scheme).Build(), map[string]client.Client{"a": provider})
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "a"}},
		Spec: workloads.InstanceSetSpec{EnableInstanceAPI: ptr.To(true), Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: desired.Name}}}}},
		}}}}
	if err := syncInstanceConfigMaps(context.Background(), cli, comp, its, []*corev1.ConfigMap{desired}); !apierrors.IsConflict(err) {
		t.Fatalf("concurrent update was not guarded: %v", err)
	}
	current := &corev1.ConfigMap{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(mirror), current); err != nil || current.Data["config"] != "concurrent" {
		t.Fatalf("concurrent value was overwritten: %#v, %v", current, err)
	}
	if err := syncInstanceConfigMaps(context.Background(), cli, comp, its, []*corev1.ConfigMap{desired}); err != nil {
		t.Fatal(err)
	}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(mirror), current); err != nil || current.Data["config"] != "value" {
		t.Fatalf("retry did not publish desired data: %#v, %v", current, err)
	}
}
