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
	"reflect"
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
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
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
		Configs:          []workloads.ConfigTemplate{{Name: "main", ConfigHash: ptr.To("default-hash"), Volume: &workloads.ConfigVolumeBinding{VolumeName: "config"}}},
		LifecycleActions: component.SynthesizedLifecycleActions{ComponentLifecycleActions: &appsv1.ComponentLifecycleActions{}},
	}
	return comp, synth, scheme
}

func managedInstanceConfig(comp *appsv1.Component, name string) *corev1.ConfigMap {
	labels := constant.GetCompLabels("db", "mysql")
	labels[instanceConfigMapLabelKey] = "true"
	labels[constant.KBAppConfigMapOwnerLabelKey] = string(comp.UID)
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: comp.Namespace, UID: types.UID(name + "-uid"), Labels: labels,
		OwnerReferences: []metav1.OwnerReference{{APIVersion: appsv1.GroupVersion.String(), Kind: "Component", Name: comp.Name, UID: comp.UID, Controller: ptr.To(true)}}}, Data: map[string]string{"config": "value"}}
}

func workloadConfigPlan(t *testing.T, cli client.Client, comp *appsv1.Component, synth *component.SynthesizedComponent, running *workloads.InstanceSet, maps ...*corev1.ConfigMap) (*componentPlan, error) {
	t.Helper()
	return workloadConfigPlanWithProviders(t, cli, comp, synth, running, nil, maps...)
}

func workloadConfigPlanWithProviders(t *testing.T, cli client.Client, comp *appsv1.Component, synth *component.SynthesizedComponent, running *workloads.InstanceSet, providers []string, maps ...*corev1.ConfigMap) (*componentPlan, error) {
	t.Helper()
	graphCli := model.NewGraphClient(cli)
	dag := graph.NewDAG()
	graphCli.Root(dag, comp, comp, model.ActionStatusPtr())
	transCtx := &componentTransformContext{Context: context.Background(), Client: graphCli, Component: comp, ComponentOrig: comp.DeepCopy(), CompDef: &appsv1.ComponentDefinition{}, SynthesizeComponent: synth, RunningWorkload: running}
	for _, cm := range maps {
		old := &corev1.ConfigMap{}
		if err := cli.Get(transCtx.Context, client.ObjectKeyFromObject(cm), old); apierrors.IsNotFound(err) {
			graphCli.Create(dag, cm)
		} else if err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(old.Data, cm.Data) || !reflect.DeepEqual(old.Annotations, cm.Annotations) {
			updated := old.DeepCopy()
			updated.Data = cm.Data
			updated.Annotations = cm.Annotations
			graphCli.Update(dag, old, updated)
		}
	}
	builder := &componentPlanBuilder{cli: cli, transCtx: transCtx}
	plan := &componentPlan{dag: dag, transCtx: transCtx, walkFunc: func(v graph.Vertex) error {
		if _, ok := v.(*model.ObjectVertex).Obj.(*appsv1.Component); ok {
			return nil
		}
		return builder.defaultWalkFunc(v)
	}}
	err := (&componentWorkloadTransformer{Client: cli, configProviders: providers}).Transform(transCtx, dag)
	return plan, err
}

func TestInstanceConfigPublicationPrecedesWorkload(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "start update"}[update], func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1))}}
			synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{"canary": {{Name: "main", ConfigHash: ptr.To("special-hash"), Volume: &workloads.ConfigVolumeBinding{VolumeName: "config", ConfigMapName: "special"}}}}
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
			if update && (its.Spec.Template.Spec.Containers[0].Image != intctrlutil.ReplaceImageRegistry("db:new") || its.Annotations[constant.KubeBlocksGenerationKey] != "1") {
				t.Fatal("Start did not publish the config runtime while deferring the generation")
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
				running.Spec.Instances = []workloads.InstanceTemplate{{Name: "removed", Replicas: ptr.To(int32(0)), Configs: []workloads.ConfigTemplate{{Name: "main", Volume: &workloads.ConfigVolumeBinding{ConfigMapName: cm.Name}}}}}
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
			if err != nil && !intctrlutil.IsDelayedRequeueError(err) {
				t.Fatal(err)
			}
			if retained && !intctrlutil.IsDelayedRequeueError(err) {
				t.Fatalf("expected retry while old reference exists, got %v", err)
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
	authoritative.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "old-provider"}
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
	plan, err := workloadConfigPlanWithProviders(t, cli, comp, synth, nil, []string{"old-provider", "new-provider"})
	if !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("expected remote reference retry, got %v", err)
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
	plan, err = workloadConfigPlanWithProviders(t, cli, comp, synth, current, []string{"old-provider", "new-provider"})
	if err != nil && !intctrlutil.IsDelayedRequeueError(err) {
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
		plan, err = workloadConfigPlanWithProviders(t, cli, comp, synth, current, []string{"old-provider", "new-provider"})
		if err != nil && !intctrlutil.IsDelayedRequeueError(err) {
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

func TestInstanceConfigStartPublishesCoherentRuntime(t *testing.T) {
	for _, mode := range []struct {
		name              string
		enableInstanceAPI bool
	}{{"direct Pod", false}, {"Instance", true}} {
		t.Run(mode.name, func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			synth.EnableInstanceAPI = ptr.To(mode.enableInstanceAPI)
			fileVolume := func(volume, source string) corev1.Volume {
				return corev1.Volume{Name: volume, VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: source}}}}
			}
			config := func(name, volume, source, hash string) workloads.ConfigTemplate {
				return workloads.ConfigTemplate{Name: name, Volume: &workloads.ConfigVolumeBinding{VolumeName: volume, ConfigMapName: source}, ConfigHash: ptr.To(hash)}
			}
			configRef := func(name string) corev1.ObjectReference {
				return corev1.ObjectReference{Kind: "ConfigMap", Namespace: comp.Namespace, Name: name}
			}
			synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1))}}
			synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{
				"canary": {config("main", "config", "special-config", "old")},
			}
			synth.PodSpec.Volumes = append(synth.PodSpec.Volumes, fileVolume("script", "old-script"))
			synth.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "config", MountPath: "/config"}, {Name: "script", MountPath: "/extra"}}
			synth.InstanceAssistantObjects = []corev1.ObjectReference{configRef("default-config"), configRef("old-script")}
			running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
			if err != nil {
				t.Fatal(err)
			}
			running = running.DeepCopy()
			running.Spec.Stop = ptr.To(true)

			synth.Configs = append(synth.Configs, config("extra", "extra", "extra-config", "extra-base"))
			synth.InstanceConfigs["canary"] = []workloads.ConfigTemplate{
				config("main", "config", "special-config", "latest"),
				config("extra", "extra", "special-extra", "extra-latest"),
			}
			synth.PodSpec.Volumes[1] = fileVolume("script", "new-script")
			synth.PodSpec.Volumes = append(synth.PodSpec.Volumes, fileVolume("extra", "extra-config"))
			synth.PodSpec.Containers[0].Image = "db:new"
			synth.PodSpec.Containers[0].VolumeMounts[1].MountPath = "/new-script"
			synth.PodSpec.Containers[0].VolumeMounts = append(synth.PodSpec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "extra", MountPath: "/extra"})
			synth.InstanceAssistantObjects = []corev1.ObjectReference{configRef("default-config"), configRef("extra-config"), configRef("new-script")}

			ctx := context.Background()
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
			maps := []*corev1.ConfigMap{managedInstanceConfig(comp, "special-config"), managedInstanceConfig(comp, "special-extra")}
			plan, err := workloadConfigPlan(t, cli, comp, synth, running, maps...)
			if err != nil {
				t.Fatal(err)
			}
			if err := plan.Execute(); err != nil {
				t.Fatal(err)
			}

			starting := &workloads.InstanceSet{}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(running), starting); err != nil {
				t.Fatal(err)
			}
			if ptr.Deref(starting.Spec.Stop, false) || !reflect.DeepEqual(starting.Spec.Instances[0].Configs, synth.InstanceConfigs["canary"]) {
				t.Fatalf("Start did not publish both latest config bindings: %#v", starting.Spec.Instances[0].Configs)
			}
			templates, err := instancetemplate.BuildInstanceName2TemplateMap(starting, nil)
			if err != nil {
				t.Fatal(err)
			}
			pod := templates[starting.Name+"-canary-0"].Spec
			wantVolumes := []corev1.Volume{fileVolume("config", "special-config"), fileVolume("script", "new-script"), fileVolume("extra", "special-extra")}
			if !reflect.DeepEqual(pod.Volumes, wantVolumes) {
				t.Fatalf("Start did not publish coherent volumes: %#v", pod.Volumes)
			}
			container := pod.Containers[0]
			wantMounts := []corev1.VolumeMount{{Name: "config", MountPath: "/config"}, {Name: "script", MountPath: "/new-script"}, {Name: "extra", MountPath: "/extra"}}
			if container.Image != intctrlutil.ReplaceImageRegistry("db:new") || !reflect.DeepEqual(container.VolumeMounts, wantMounts) {
				t.Fatalf("Start mixed runtime versions: image=%s mounts=%#v", container.Image, container.VolumeMounts)
			}
			wantAssistants := []corev1.ObjectReference{configRef("default-config"), configRef("extra-config"), configRef("new-script")}
			if !reflect.DeepEqual(starting.Spec.InstanceAssistantObjects, wantAssistants) {
				t.Fatalf("Start did not publish coherent assistant objects: %#v", starting.Spec.InstanceAssistantObjects)
			}
		})
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
	running.Spec.Instances[0].Configs = []workloads.ConfigTemplate{{Name: "main", ConfigHash: ptr.To("old-special-hash"), Volume: &workloads.ConfigVolumeBinding{VolumeName: "config", ConfigMapName: "removed-special"}}}
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
	if !reflect.DeepEqual(current.Spec.InstanceAssistantObjects, synth.InstanceAssistantObjects) {
		t.Fatal("Start retained obsolete assistant objects")
	}
}

func TestInstanceConfigStartKeepsRenderInputsAndTopology(t *testing.T) {
	for _, enableInstanceAPI := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct Pod", true: "Instance"}[enableInstanceAPI], func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			resources := func(memory string) corev1.ResourceRequirements {
				return corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse(memory)}}
			}
			claim := func(size string) appsv1.PersistentVolumeClaimTemplate {
				return appsv1.PersistentVolumeClaimTemplate{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}}}}
			}
			comp.Spec.Resources, comp.Spec.ServiceVersion = resources("1Gi"), "1.0.0"
			comp.Spec.VolumeClaimTemplates = []appsv1.PersistentVolumeClaimTemplate{claim("10Gi")}
			synth.CompDefName, synth.ServiceVersion, synth.Generation = "mysql-v1", "1.0.0", "1"
			synth.EnableInstanceAPI = ptr.To(enableInstanceAPI)
			synth.PodSpec.Containers[0].Resources = comp.Spec.Resources
			synth.PodSpec.Containers[0].Image = "db:1"
			synth.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "config", MountPath: "/config"}}
			synth.PodSpec.Volumes = nil
			synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1)), ServiceVersion: "2.0.0", Resources: ptr.To(resources("2Gi")), VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{claim("20Gi")}}}
			synth.InstanceImages = map[string]map[string]string{"canary": {"db": "db:2"}}
			synth.FileTemplates = []component.SynthesizedFileTemplate{{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "main", Template: "config-source", Namespace: comp.Namespace, VolumeName: "config"}, Config: true}}
			source := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config-source", Namespace: comp.Namespace}, Data: map[string]string{"server.conf": "{{ .MEMORY }}/{{ .VERSION }}/{{ .DATA }}"}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(comp, source).Build()
			required := ptr.To(appsv1.VarRequired)
			definition := &appsv1.ComponentDefinition{Spec: appsv1.ComponentDefinitionSpec{Vars: []appsv1.EnvVar{
				{Name: "MEMORY", ValueFrom: &appsv1.VarSource{ResourceVarRef: &appsv1.ResourceVarSelector{ResourceVars: appsv1.ResourceVars{Memory: required}}}},
				{Name: "VERSION", ValueFrom: &appsv1.VarSource{ComponentVarRef: &appsv1.ComponentVarSelector{ComponentVars: appsv1.ComponentVars{ServiceVersion: required}}}},
				{Name: "DATA", ValueFrom: &appsv1.VarSource{ResourceVarRef: &appsv1.ResourceVarSelector{ResourceVars: appsv1.ResourceVars{Storage: &appsv1.NamedVar{Name: "data", Option: required}}}}},
			}}}
			render := func() []*corev1.ConfigMap {
				t.Helper()
				synth.Configs, synth.InstanceAssistantObjects = nil, nil
				graphCli := model.NewGraphClient(cli)
				dag := graph.NewDAG()
				graphCli.Root(dag, comp, comp, model.ActionStatusPtr())
				transCtx := &componentTransformContext{Context: context.Background(), Client: graphCli, Component: comp, ComponentOrig: comp.DeepCopy(), CompDef: definition, SynthesizeComponent: synth}
				if err := (&componentVarsTransformer{}).Transform(transCtx, dag); err != nil {
					t.Fatal(err)
				}
				if err := (&componentFileTemplateTransformer{}).Transform(transCtx, dag); err != nil {
					t.Fatal(err)
				}
				var rendered []*corev1.ConfigMap
				for _, vertex := range dag.Vertices() {
					if obj, ok := vertex.(*model.ObjectVertex).Obj.(*corev1.ConfigMap); ok && obj.Labels[kubeBlockFileTemplateLabelKey] == "true" {
						rendered = append(rendered, obj)
					}
				}
				return rendered
			}
			render()
			running, err := component.BuildInstanceSet(synth, definition)
			if err != nil {
				t.Fatal(err)
			}
			running.Spec.Stop = ptr.To(true)
			if err := cli.Create(context.Background(), running); err != nil {
				t.Fatal(err)
			}
			comp.Spec.Resources, comp.Spec.ServiceVersion = resources("2Gi"), "2.0.0"
			comp.Spec.VolumeClaimTemplates = []appsv1.PersistentVolumeClaimTemplate{claim("20Gi")}
			if err := cli.Update(context.Background(), comp); err != nil {
				t.Fatal(err)
			}
			synth.ServiceVersion, synth.Generation = "2.0.0", "2"
			synth.PodSpec.Containers[0].Resources, synth.PodSpec.Containers[0].Image = comp.Spec.Resources, "db:2"
			synth.Instances[0].Resources, synth.Instances[0].ServiceVersion = ptr.To(resources("4Gi")), "3.0.0"
			synth.Instances[0].VolumeClaimTemplates = []appsv1.PersistentVolumeClaimTemplate{claim("30Gi")}
			synth.InstanceImages["canary"]["db"] = "db:3"
			synth.Replicas, synth.FlatInstanceOrdinal = 3, true
			synth.Instances[0].Replicas = ptr.To(int32(3))
			synth.Instances[0].Ordinals = appsv1.Ordinals{Discrete: []int32{4, 5, 6}}
			synth.OfflineInstances = []string{"db-mysql-9"}
			rendered := render()
			plan, err := workloadConfigPlan(t, cli, comp, synth, running, rendered...)
			if err != nil {
				t.Fatal(err)
			}
			if err := plan.Execute(); err != nil {
				t.Fatal(err)
			}
			starting := &workloads.InstanceSet{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(running), starting); err != nil {
				t.Fatal(err)
			}
			if ptr.Deref(starting.Spec.Stop, false) || starting.Annotations[constant.KubeBlocksGenerationKey] != "1" || !reflect.DeepEqual(starting.Spec.Replicas, running.Spec.Replicas) || starting.Spec.FlatInstanceOrdinal != running.Spec.FlatInstanceOrdinal || !reflect.DeepEqual(starting.Spec.OfflineInstances, running.Spec.OfflineInstances) || !reflect.DeepEqual(starting.Spec.Instances[0].Replicas, running.Spec.Instances[0].Replicas) || !reflect.DeepEqual(starting.Spec.Instances[0].Ordinals, running.Spec.Instances[0].Ordinals) {
				t.Fatal("Start applied scaling topology or acknowledged it before the normal lifecycle")
			}
			templates, err := instancetemplate.BuildInstanceName2TemplateMap(starting, nil)
			if err != nil {
				t.Fatal(err)
			}
			template := templates[starting.Name+"-canary-0"]
			if len(templates) != 1 || template == nil || template.Spec.Containers[0].Resources.Requests.Memory().Cmp(resource.MustParse("4Gi")) != 0 || template.Spec.Containers[0].Image != "db:3" || template.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Cmp(resource.MustParse("30Gi")) != 0 {
				t.Fatalf("Start did not keep the rendered resource, version and storage inputs: %#v", template)
			}
			cm := &corev1.ConfigMap{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: comp.Namespace, Name: template.Configs[0].Volume.ConfigMapName}, cm); err != nil {
				t.Fatal(err)
			}
			if cm.Data["server.conf"] != "4294967296/3.0.0/32212254720" || ptr.Deref(template.Configs[0].ConfigHash, "") != cm.Annotations[constant.CMInsConfigurationHashLabelKey] {
				t.Fatalf("Start used files or hash from another runtime: %#v", cm)
			}
		})
	}
}

func TestInstanceConfigStartKeepsRetiringTemplateOnBaseRuntime(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1)), Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")}}}}
	synth.InstanceImages = map[string]map[string]string{"canary": {"db": "db:old-canary"}}
	synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{"canary": {{Name: "main", ConfigHash: ptr.To("old"), Volume: &workloads.ConfigVolumeBinding{VolumeName: "config", ConfigMapName: "old-special"}}}}
	running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
	if err != nil {
		t.Fatal(err)
	}
	running.Spec.Stop = ptr.To(true)
	synth.Instances[0].Replicas = ptr.To(int32(0))
	synth.InstanceConfigs = nil
	synth.Configs[0].ConfigHash = ptr.To("latest-base")
	synth.PodSpec.Containers[0].Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
	plan, err := workloadConfigPlan(t, cli, comp, synth, running)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Execute(); err != nil {
		t.Fatal(err)
	}
	starting := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(running), starting); err != nil {
		t.Fatal(err)
	}
	templates, err := instancetemplate.BuildInstanceName2TemplateMap(starting, nil)
	if err != nil {
		t.Fatal(err)
	}
	template := templates[starting.Name+"-canary-0"]
	if len(templates) != 1 || template == nil || ptr.Deref(starting.Spec.Instances[0].Replicas, 0) != 1 || template.Spec.Containers[0].Resources.Requests.Memory().Cmp(resource.MustParse("2Gi")) != 0 || template.Spec.Containers[0].Image != starting.Spec.Template.Spec.Containers[0].Image || ptr.Deref(template.Configs[0].ConfigHash, "") != "latest-base" {
		t.Fatalf("Start mixed a retiring template's overrides with the base config or scaled it early: %#v", template)
	}
}

func TestInstanceConfigStartMountsAndLegacyBinding(t *testing.T) {
	for _, moveMount := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy binding", true: "mount changed"}[moveMount], func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			synth.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "config", MountPath: "/old"}}
			running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
			if err != nil {
				t.Fatal(err)
			}
			running = running.DeepCopy()
			running.Spec.Stop = ptr.To(true)
			if moveMount {
				synth.PodSpec.Containers[0].VolumeMounts[0].MountPath = "/new"
			} else {
				running.Spec.Configs[0].Volume = nil
				running.Spec.Template.Spec.Containers = append(running.Spec.Template.Spec.Containers,
					corev1.Container{Name: legacyConfigManagerContainerName, VolumeMounts: running.Spec.Template.Spec.Containers[0].VolumeMounts})
			}
			synth.PodSpec.Containers[0].Resources.Limits = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")}
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
			if ptr.Deref(current.Spec.Stop, false) || current.Spec.Configs[0].Volume == nil {
				t.Fatalf("Start did not publish the config binding: Stop=%v, Configs=%v", ptr.Deref(current.Spec.Stop, false), current.Spec.Configs)
			}
			container := current.Spec.Template.Spec.Containers[0]
			wantMount := "/old"
			var wantMemory int64
			if moveMount {
				wantMount = "/new"
				wantMemory = 8 << 30
			}
			if container.VolumeMounts[0].MountPath != wantMount || container.Resources.Limits.Memory().Value() != wantMemory {
				t.Fatalf("unexpected runtime on Start: %#v", container)
			}
		})
	}
}

func TestInstanceConfigDeletionRejectsSameNameReplacement(t *testing.T) {
	comp, _, scheme := instanceConfigFixture(t)
	stale := managedInstanceConfig(comp, "retired")
	replacement := stale.DeepCopy()
	replacement.UID = "replacement-uid"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(replacement).Build()
	err := removeInstanceConfigMap(context.Background(), cli, stale, "")
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
	cm.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "unavailable"}
	running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
	if err != nil {
		t.Fatal(err)
	}
	synth.PodSpec.Containers[0].Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, running).Build()
	cli := interceptor.NewClient(base, interceptor.Funcs{Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if placement, _ := multicluster.FromContext(ctx); placement == "unavailable" && key.Name == cm.Name {
			return errors.New("provider unavailable")
		}
		return c.Get(ctx, key, obj, opts...)
	}})
	plan, err := workloadConfigPlanWithProviders(t, cli, comp, synth, running, []string{"unavailable"})
	if !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("expected retry: %v", err)
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
		if !intctrlutil.IsDelayedRequeueError(err) {
			t.Fatalf("expected collection retry: %v", err)
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
	foreign.Labels[constant.KBAppConfigMapOwnerLabelKey] = "previous-component"
	foreign.Data["config"] = "foreign"
	provider := fake.NewClientBuilder().WithScheme(scheme).WithObjects(foreign).Build()
	cli := multicluster.NewClient(fake.NewClientBuilder().WithScheme(scheme).Build(), map[string]client.Client{"a": provider})
	if err := syncInstanceConfigMap(context.Background(), cli, comp, desired, "a"); err == nil {
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
	if err := syncInstanceConfigMap(context.Background(), cli, comp, desired, "a"); !apierrors.IsConflict(err) {
		t.Fatalf("concurrent update was not guarded: %v", err)
	}
	current := &corev1.ConfigMap{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(mirror), current); err != nil || current.Data["config"] != "concurrent" {
		t.Fatalf("concurrent value was overwritten: %#v, %v", current, err)
	}
	if err := syncInstanceConfigMap(context.Background(), cli, comp, desired, "a"); err != nil {
		t.Fatal(err)
	}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(mirror), current); err != nil || current.Data["config"] != "value" {
		t.Fatalf("retry did not publish desired data: %#v, %v", current, err)
	}
}

func TestInstanceConfigMirrorPublicationPrecedesWorkload(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "update"}[update], func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			synth.EnableInstanceAPI = ptr.To(true)
			comp.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "a,b"}
			synth.Annotations = comp.Annotations
			synth.AnnotationsInjectedToWorkload = comp.Annotations
			synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1))}}
			cm := managedInstanceConfig(comp, "special")
			cm.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "a,b"}
			shared := managedInstanceConfig(comp, "default-config")
			delete(shared.Labels, instanceConfigMapLabelKey)
			shared.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "a,b"}
			synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{"canary": {{Name: "main", ConfigHash: ptr.To("special-hash"), Volume: &workloads.ConfigVolumeBinding{ConfigMapName: cm.Name}}}}
			var running *workloads.InstanceSet
			var objects []client.Object
			if update {
				var err error
				running, err = component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
				if err != nil {
					t.Fatal(err)
				}
				objects = append(objects, running)
				running.Spec.Stop = ptr.To(true)
				synth.InstanceConfigs["canary"][0].ConfigHash = ptr.To("new-special-hash")
				synth.PodSpec.Containers[0].Image = "db:new"
			}
			legacy := shared.DeepCopy()
			legacy.UID = "provider-a-legacy"
			legacy.OwnerReferences = nil
			delete(legacy.Labels, constant.KBAppConfigMapOwnerLabelKey)
			legacy.Annotations = map[string]string{"workloads.kubeblocks.io/assistant-object": "true"}
			legacy.Data["config"] = "stale assistant snapshot"
			providerA := fake.NewClientBuilder().WithScheme(scheme).WithObjects(legacy).Build()
			providerBBase := fake.NewClientBuilder().WithScheme(scheme).Build()
			failMirror := true
			mirrorUnavailable := errors.New("mirror unavailable")
			providerB := interceptor.NewClient(providerBBase, interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if failMirror {
					return mirrorUnavailable
				}
				return c.Create(ctx, obj, opts...)
			}})
			writes := 0
			checkMirrors := func() {
				for _, provider := range []client.Client{providerA, providerB} {
					for _, configMap := range []*corev1.ConfigMap{shared, cm} {
						mirror := &corev1.ConfigMap{}
						if err := provider.Get(context.Background(), client.ObjectKeyFromObject(configMap), mirror); err != nil || mirror.Data["config"] != "value" {
							t.Fatalf("workload published before mirror: %#v, %v", mirror, err)
						}
						if len(mirror.OwnerReferences) != 0 || len(mirror.Finalizers) != 0 ||
							mirror.Labels[constant.KBAppConfigMapOwnerLabelKey] != string(comp.UID) || mirror.Annotations["workloads.kubeblocks.io/assistant-object"] != "" {
							t.Fatalf("mirror has invalid ownership: %#v", mirror.ObjectMeta)
						}
						if provider == providerA && configMap == shared && mirror.UID != legacy.UID {
							t.Fatalf("legacy mirror was replaced instead of transferred in place: %#v", mirror.ObjectMeta)
						}
					}
				}
				writes++
			}
			controlBase := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			control := interceptor.NewClient(controlBase, interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if _, ok := obj.(*workloads.InstanceSet); ok {
						checkMirrors()
					}
					return c.Create(ctx, obj, opts...)
				},
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if _, ok := obj.(*workloads.InstanceSet); ok {
						checkMirrors()
					}
					return c.Update(ctx, obj, opts...)
				},
			})
			cli := multicluster.NewClient(control, map[string]client.Client{"a": providerA, "b": providerB})
			providers := []string{"a", "b"}
			plan, err := workloadConfigPlanWithProviders(t, cli, comp, synth, running, providers, shared, cm)
			if !errors.Is(err, mirrorUnavailable) {
				t.Fatalf("expected provider failure after publishing the authority: %v", err)
			}
			if err := plan.Execute(); err != nil || writes != 0 {
				t.Fatalf("mirror failure allowed workload publication: writes=%d, error=%v", writes, err)
			}
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(shared), &corev1.ConfigMap{}); err != nil {
				t.Fatalf("shared authority was not persisted before mirroring: %v", err)
			}

			failMirror = false
			plan, err = workloadConfigPlanWithProviders(t, cli, comp, synth, running, providers, shared, cm)
			if err != nil {
				t.Fatal(err)
			}
			if err := plan.Execute(); err != nil {
				t.Fatal(err)
			}
			if writes != 1 {
				t.Fatalf("workload writes = %d, want one", writes)
			}
			if update {
				return
			}

			// Reconcile a ConfigMap-only change against the published InstanceSet.
			running = &workloads.InstanceSet{}
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(comp), running); err != nil {
				t.Fatal(err)
			}
			cm.Data["config"] = "updated"
			plan, err = workloadConfigPlanWithProviders(t, cli, comp, synth, running, providers, shared, cm)
			if err != nil {
				t.Fatal(err)
			}
			if err := plan.Execute(); err != nil {
				t.Fatal(err)
			}
			for _, provider := range []client.Client{providerA, providerB} {
				mirror := &corev1.ConfigMap{}
				if err := provider.Get(context.Background(), client.ObjectKeyFromObject(cm), mirror); err != nil || mirror.Data["config"] != "updated" {
					t.Fatalf("mirror did not receive updated content: %v, %v", mirror.Data, err)
				}
			}
			if writes != 1 {
				t.Fatalf("ConfigMap-only change updated the workload: writes=%d", writes)
			}
		})
	}
}

func TestInstanceConfigPublicationRequiresConfiguredProvider(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	placement := "worker"
	comp.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: placement}
	synth.EnableInstanceAPI = ptr.To(true)
	shared := managedInstanceConfig(comp, "default-config")
	delete(shared.Labels, instanceConfigMapLabelKey)
	shared.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: placement}
	ctx := context.Background()
	control := fake.NewClientBuilder().WithScheme(scheme).WithObjects(shared).Build()
	original := &corev1.ConfigMap{}
	if err := control.Get(ctx, client.ObjectKeyFromObject(shared), original); err != nil {
		t.Fatal(err)
	}

	plan, err := workloadConfigPlanWithProviders(t, control, comp, synth, nil, nil)
	if err == nil {
		t.Fatal("missing provider allowed publication")
	}
	if err := plan.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := control.Get(ctx, client.ObjectKeyFromObject(comp), &workloads.InstanceSet{}); !apierrors.IsNotFound(err) {
		t.Fatalf("published workload with no configured provider: %v", err)
	}

	cli := multicluster.NewClient(control, map[string]client.Client{placement: control})
	plan, err = workloadConfigPlanWithProviders(t, cli, comp, synth, nil, []string{placement})
	if err != nil {
		t.Fatalf("rejected configured alias of the control cluster: %v", err)
	}
	if err := plan.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := control.Get(ctx, client.ObjectKeyFromObject(comp), &workloads.InstanceSet{}); err != nil {
		t.Fatalf("workload was not published after the provider returned: %v", err)
	}
	current := &corev1.ConfigMap{}
	if err := control.Get(ctx, client.ObjectKeyFromObject(shared), current); err != nil || current.ResourceVersion != original.ResourceVersion {
		t.Fatalf("control alias rewrote the authority: %#v, %v", current.ObjectMeta, err)
	}
}

func TestInstanceConfigPublishesWhenEachRenderChanges(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	comp.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "worker"}
	synth.EnableInstanceAPI = ptr.To(true)
	source := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: comp.Namespace, Name: "source"},
		Data:       map[string]string{"db.conf": "{{ randAlphaNum 64 }}"},
	}
	synth.FileTemplates = []component.SynthesizedFileTemplate{{
		Config: true,
		ComponentFileTemplate: appsv1.ComponentFileTemplate{
			Name: "main", Template: source.Name, Namespace: source.Namespace, VolumeName: "config",
		},
	}}
	synth.PodSpec.Volumes, synth.Configs = nil, nil
	ctx := context.Background()
	controlBase := fake.NewClientBuilder().WithScheme(scheme).WithObjects(source).Build()
	rejected := errors.New("control CM write failed")
	failAuthority := true
	control := interceptor.NewClient(controlBase, interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
		if _, ok := obj.(*corev1.ConfigMap); ok && failAuthority {
			return rejected
		}
		return c.Create(ctx, obj, opts...)
	}})
	worker := fake.NewClientBuilder().WithScheme(scheme).Build()
	unavailable := errors.New("provider rejected the mirror")
	failMirror := true
	mirrorAttempts := 0
	provider := interceptor.NewClient(worker, interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
		mirrorAttempts++
		if failMirror {
			return unavailable
		}
		return c.Create(ctx, obj, opts...)
	}})
	cli := multicluster.NewClient(control, map[string]client.Client{"worker": provider})
	key := client.ObjectKey{Namespace: comp.Namespace, Name: fileTemplateObjectName(synth, "main")}
	reconcile := func() error {
		desired := *synth
		desired.PodSpec = synth.PodSpec.DeepCopy()
		transCtx := &componentTransformContext{
			Context: ctx, Client: model.NewGraphClient(cli), Component: comp, ComponentOrig: comp.DeepCopy(),
			SynthesizeComponent: &desired, CompDef: &appsv1.ComponentDefinition{},
		}
		dag := graph.NewDAG()
		transCtx.Client.(model.GraphClient).Root(dag, comp, comp, model.ActionStatusPtr())
		providers := []string{"worker"}
		if err := (&componentFileTemplateTransformer{configProviders: providers}).Transform(transCtx, dag); err != nil {
			t.Fatal(err)
		}
		err := (&componentWorkloadTransformer{Client: cli, configProviders: providers}).Transform(transCtx, dag)
		builder := &componentPlanBuilder{cli: cli, transCtx: transCtx}
		walkErr := dag.WalkReverseTopoOrder(func(v graph.Vertex) error {
			if _, ok := v.(*model.ObjectVertex).Obj.(*appsv1.Component); ok {
				return nil
			}
			return builder.defaultWalkFunc(v)
		}, nil)
		if walkErr != nil {
			t.Fatal(walkErr)
		}
		return err
	}
	if err := reconcile(); !errors.Is(err, rejected) || mirrorAttempts != 0 {
		t.Fatalf("provider publication started before the control write succeeded: attempts=%d, error=%v", mirrorAttempts, err)
	}
	failAuthority = false
	if err := reconcile(); !errors.Is(err, unavailable) {
		t.Fatalf("publish did not reach the provider after persisting the control CM: %v", err)
	}
	before := &corev1.ConfigMap{}
	if err := control.Get(ctx, key, before); err != nil {
		t.Fatal(err)
	}
	its := &workloads.InstanceSet{}
	if err := control.Get(ctx, client.ObjectKeyFromObject(comp), its); !apierrors.IsNotFound(err) {
		t.Fatalf("published the workload during provider failure: %v", err)
	}
	failMirror = false
	if err := reconcile(); err != nil {
		t.Fatalf("publish did not converge after the provider recovered: %v", err)
	}
	authority, mirror := &corev1.ConfigMap{}, &corev1.ConfigMap{}
	for _, read := range []struct {
		cli client.Client
		cm  *corev1.ConfigMap
	}{{control, authority}, {worker, mirror}} {
		if err := read.cli.Get(ctx, key, read.cm); err != nil {
			t.Fatal(err)
		}
	}
	if err := control.Get(ctx, client.ObjectKeyFromObject(comp), its); err != nil {
		t.Fatal(err)
	}
	if authority.Data["db.conf"] == before.Data["db.conf"] || authority.Data["db.conf"] != mirror.Data["db.conf"] ||
		ptr.Deref(its.Spec.Configs[0].ConfigHash, "") != authority.Annotations[constant.CMInsConfigurationHashLabelKey] {
		t.Fatal("the new render did not reach the control CM, provider copy and workload together")
	}
}
