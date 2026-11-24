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
	"slices"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	k8sappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	workloadctrl "github.com/apecloud/kubeblocks/controllers/workloads"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: comp.Namespace, UID: types.UID(name + "-uid"), Labels: labels,
		OwnerReferences: []metav1.OwnerReference{{APIVersion: appsv1.GroupVersion.String(), Kind: "Component", Name: comp.Name, UID: comp.UID, Controller: ptr.To(true)}}}, Data: map[string]string{"config": "value"}}
}

func workloadConfigPlan(t *testing.T, cli client.Client, comp *appsv1.Component, synth *component.SynthesizedComponent, running *workloads.InstanceSet, maps ...*corev1.ConfigMap) (*componentPlan, error) {
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
	err := (&componentWorkloadTransformer{Client: cli}).Transform(transCtx, dag)
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
				running = running.DeepCopy()
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
			if update && (its.Spec.Template.Spec.Containers[0].Image != running.Spec.Template.Spec.Containers[0].Image ||
				its.Annotations[constant.KubeBlocksGenerationKey] != "1") {
				t.Fatal("Start applied a pending shared image or acknowledged the deferred generation")
			}
		})
	}
}

func TestInstanceConfigStartPublishesOnlyConfigsWithExistingVolumes(t *testing.T) {
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
				config("main", "config", "next-special", "latest"),
				config("extra", "extra", "special-extra", "extra-latest"),
			}
			synth.PodSpec.Volumes[1] = fileVolume("script", "new-script")
			synth.PodSpec.Volumes = append(synth.PodSpec.Volumes, fileVolume("extra", "extra-config"))
			synth.PodSpec.Containers[0].Image = "db:new"
			synth.PodSpec.Containers[0].VolumeMounts[1].MountPath = "/new-script"
			synth.PodSpec.Containers[0].VolumeMounts = append(synth.PodSpec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "extra", MountPath: "/extra"})
			synth.InstanceAssistantObjects = []corev1.ObjectReference{configRef("default-config"), configRef("extra-config"), configRef("new-script"), configRef("next-special")}

			ctx := context.Background()
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
			maps := []*corev1.ConfigMap{managedInstanceConfig(comp, "next-special"), managedInstanceConfig(comp, "special-extra")}
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
			configs := starting.Spec.Instances[0].Configs
			if ptr.Deref(starting.Spec.Stop, false) || len(configs) != 1 || ptr.Deref(configs[0].ConfigHash, "") != "latest" {
				t.Fatalf("Start did not publish the config for the existing volume: %#v", configs)
			}
			templates, err := instancetemplate.BuildInstanceTemplateExtMap(starting, nil)
			if err != nil {
				t.Fatal(err)
			}
			pod := templates[starting.Name+"-canary-0"].Spec
			wantVolumes := []corev1.Volume{fileVolume("config", "next-special"), fileVolume("script", "old-script")}
			if !reflect.DeepEqual(pod.Volumes, wantVolumes) {
				t.Fatalf("Start did not publish coherent volumes: %#v", pod.Volumes)
			}
			container := pod.Containers[0]
			wantMounts := []corev1.VolumeMount{{Name: "config", MountPath: "/config"}, {Name: "script", MountPath: "/extra"}}
			if container.Image != running.Spec.Template.Spec.Containers[0].Image || !reflect.DeepEqual(container.VolumeMounts, wantMounts) {
				t.Fatalf("Start mixed runtime versions: image=%s mounts=%#v", container.Image, container.VolumeMounts)
			}
			wantAssistants := []corev1.ObjectReference{configRef("default-config"), configRef("old-script"), configRef("next-special")}
			if !reflect.DeepEqual(starting.Spec.InstanceAssistantObjects, wantAssistants) {
				t.Fatalf("Start did not publish coherent assistant objects: %#v", starting.Spec.InstanceAssistantObjects)
			}
		})
	}
}

func TestInstanceConfigStartRestoresSharedConfigForAffectedTemplate(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1))}}
	running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
	if err != nil {
		t.Fatal(err)
	}
	running = running.DeepCopy()
	running.Spec.Stop = ptr.To(true)
	running.Spec.Instances[0].Configs = []workloads.ConfigTemplate{{Name: "main", ConfigHash: ptr.To("old-special-hash"), Volume: &workloads.ConfigVolumeBinding{VolumeName: "config", ConfigMapName: "removed-special"}}}
	running.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "old-default"
	running.Spec.InstanceAssistantObjects = []corev1.ObjectReference{{Kind: "Secret", Name: "kept"}, {Kind: "ConfigMap", Name: "removed-special"}}
	synth.InstanceAssistantObjects = []corev1.ObjectReference{{Kind: "Secret", Name: "deferred"}, {Kind: "ConfigMap", Namespace: comp.Namespace, Name: "default-config"}}
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
	templates, err := instancetemplate.BuildInstanceTemplateExtMap(current, nil)
	if err != nil {
		t.Fatal(err)
	}
	canary := templates[current.Name+"-canary-0"]
	if canary.Spec.Volumes[0].ConfigMap.Name != "default-config" || ptr.Deref(canary.Configs[0].ConfigHash, "") != "new-default-hash" {
		t.Fatal("Start did not restore the latest shared configuration for the affected template")
	}
	if current.Spec.Template.Spec.Volumes[0].ConfigMap.Name != "old-default" || ptr.Deref(current.Spec.Configs[0].ConfigHash, "") != "default-hash" {
		t.Fatal("Start applied pending changes to the shared configuration source")
	}
	wantAssistants := append(slices.Clone(running.Spec.InstanceAssistantObjects), corev1.ObjectReference{Kind: "ConfigMap", Namespace: comp.Namespace, Name: "default-config"})
	if !reflect.DeepEqual(current.Spec.InstanceAssistantObjects, wantAssistants) {
		t.Fatalf("Start changed unrelated assistant objects: %#v", current.Spec.InstanceAssistantObjects)
	}
}

func TestInstanceConfigStartDefersRuntimeChangesWhenFilesAreUnchanged(t *testing.T) {
	for _, mode := range []struct {
		name              string
		enableInstanceAPI bool
	}{{"direct Pod", false}, {"Instance", true}} {
		t.Run(mode.name, func(t *testing.T) {
			for _, scenario := range []string{"shared config", "specialized config", "shared files with template action parameters"} {
				t.Run(scenario, func(t *testing.T) {
					comp, synth, scheme := instanceConfigFixture(t)
					synth.EnableInstanceAPI = ptr.To(mode.enableInstanceAPI)
					synth.PodSpec.Containers[0].Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}
					config := *synth.Configs[0].DeepCopy()
					if scenario != "shared config" {
						synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1))}}
					}
					if scenario == "specialized config" {
						config.ConfigHash = ptr.To("specialized-hash")
						config.Volume = &workloads.ConfigVolumeBinding{ConfigMapName: "specialized-config"}
						synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{"canary": {config}}
					}
					running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
					if err != nil {
						t.Fatal(err)
					}
					running = running.DeepCopy()
					running.Spec.Stop = ptr.To(true)
					previousReload := map[string]string{"KB_CONFIG_FILES_UPDATED": "/etc/db/server.conf:previous-checksum"}
					switch scenario {
					case "shared config":
						running.Spec.Configs[0].Parameters = previousReload
					case "specialized config":
						running.Spec.Instances[0].Configs[0].Parameters = previousReload
					case "shared files with template action parameters":
						config.Volume = nil
						config.Parameters = map[string]string{"ACTION_PARAM": "new-value"}
						synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{"canary": {config}}
					}
					synth.PodSpec.Containers[0].Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")}

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
					if ptr.Deref(starting.Spec.Stop, false) {
						t.Fatal("Start did not clear Stop")
					}
					if cpu := starting.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String(); cpu != "1" {
						t.Errorf("Start applied deferred CPU %s for unchanged config files", cpu)
					}
					if !reflect.DeepEqual(starting.Spec.Configs, running.Spec.Configs) ||
						!reflect.DeepEqual(starting.Spec.Instances, running.Spec.Instances) {
						t.Error("Start published deferred config action parameters")
					}
				})
			}
		})
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
			templates, err := instancetemplate.BuildInstanceTemplateExtMap(starting, nil)
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

func TestInstanceConfigStartDefersRetiringTemplate(t *testing.T) {
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
	templates, err := instancetemplate.BuildInstanceTemplateExtMap(starting, nil)
	if err != nil {
		t.Fatal(err)
	}
	template := templates[starting.Name+"-canary-0"]
	if len(templates) != 1 || template == nil || ptr.Deref(starting.Spec.Instances[0].Replicas, 0) != 1 ||
		template.Spec.Containers[0].Resources.Requests.Memory().Cmp(resource.MustParse("8Gi")) != 0 ||
		template.Spec.Containers[0].Image != "db:old-canary" ||
		template.Spec.Volumes[0].ConfigMap.Name != "old-special" || ptr.Deref(template.Configs[0].ConfigHash, "") != "old" {
		t.Fatalf("Start changed the retiring instance before scale-in: %#v", template)
	}
}

func TestInstanceConfigStartDefersSharedMountAndBindingChanges(t *testing.T) {
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
			if ptr.Deref(current.Spec.Stop, false) || !reflect.DeepEqual(current.Spec.Configs, running.Spec.Configs) {
				t.Fatalf("Start changed the shared config binding: Stop=%v, Configs=%v", ptr.Deref(current.Spec.Stop, false), current.Spec.Configs)
			}
			container := current.Spec.Template.Spec.Containers[0]
			if container.VolumeMounts[0].MountPath != "/old" || container.Resources.Limits.Memory().Value() != 0 {
				t.Fatalf("unexpected runtime on Start: %#v", container)
			}
		})
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

func TestStartScopesConfigChangesToAffectedInstanceTemplate(t *testing.T) {
	model.AddScheme(workloads.AddToScheme)
	for _, viaInstance := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct Pod", true: "Instance"}[viaInstance], func(t *testing.T) {
			for _, specialized := range []bool{false, true} {
				t.Run(map[bool]string{false: "shared config update", true: "specialized config update"}[specialized], func(t *testing.T) {
					ctx := context.Background()
					comp, synth, scheme := instanceConfigFixture(t)
					if err := clientgoscheme.AddToScheme(scheme); err != nil {
						t.Fatal(err)
					}
					synth.EnableInstanceAPI = ptr.To(viaInstance)
					synth.PodManagementPolicy = ptr.To(k8sappsv1.ParallelPodManagement)
					synth.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "config", MountPath: "/etc/db"}}
					synth.PodSpec.Containers[0].Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}
					if specialized {
						synth.Replicas = 3
						synth.Instances = []appsv1.InstanceTemplate{
							{Name: "canary", Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")}}},
							{Name: "other", Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3")}}},
						}
					}
					running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
					if err != nil {
						t.Fatal(err)
					}
					running = running.DeepCopy()
					running.UID = "running-its"
					running.Spec.Stop = ptr.To(true)
					synth.PodSpec.Containers[0].Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")}
					shared := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "default-config", Namespace: comp.Namespace}, Data: map[string]string{"server.conf": "shared"}}
					previousShared := shared.DeepCopy()
					maps := []*corev1.ConfigMap{shared}
					if specialized {
						synth.Instances[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse("12Gi")
						synth.Instances[0].Env = []corev1.EnvVar{{Name: "PENDING_ENV", Value: "deferred"}}
						synth.Instances[1].Resources.Requests[corev1.ResourceCPU] = resource.MustParse("4")
						synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{"canary": {{
							Name: "main", ConfigHash: ptr.To("specialized-hash"), Volume: &workloads.ConfigVolumeBinding{ConfigMapName: "specialized-config"},
						}}}
						cm := managedInstanceConfig(comp, "specialized-config")
						cm.Data["server.conf"] = "cpu=2 memory=12Gi"
						maps = append(maps, cm)
					} else {
						synth.Configs[0].ConfigHash = ptr.To("new-shared-hash")
						shared.Data["server.conf"] = "updated shared config"
					}
					cli := fake.NewClientBuilder().WithScheme(scheme).
						WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}).
						WithObjects(running, previousShared).Build()
					plan, err := workloadConfigPlan(t, cli, comp, synth, running, maps...)
					if err != nil {
						t.Fatal(err)
					}
					if err := plan.Execute(); err != nil {
						t.Fatal(err)
					}
					starting := &workloads.InstanceSet{}
					key := client.ObjectKeyFromObject(running)
					if err := cli.Get(ctx, key, starting); err != nil {
						t.Fatal(err)
					}
					if ptr.Deref(starting.Spec.Stop, false) || !ptr.Equal(starting.Spec.Configs[0].ConfigHash, running.Spec.Configs[0].ConfigHash) {
						t.Errorf("Start did not preserve shared config state: stop=%v, config hash=%q", starting.Spec.Stop, ptr.Deref(starting.Spec.Configs[0].ConfigHash, ""))
					}
					recorder := record.NewFakeRecorder(100)
					for range 6 {
						if viaInstance {
							if _, err := (&workloadctrl.InstanceSetReconciler2{Client: cli, Scheme: scheme, Recorder: recorder}).Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
								t.Fatal(err)
							}
							children := &workloads.InstanceList{}
							if err := cli.List(ctx, children); err != nil {
								t.Fatal(err)
							}
							for _, child := range children.Items {
								if _, err := (&workloadctrl.InstanceReconciler{Client: cli, Scheme: scheme, Recorder: recorder}).Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&child)}); err != nil {
									t.Fatal(err)
								}
							}
						} else if _, err := (&workloadctrl.InstanceSetReconciler{Client: cli, Scheme: scheme, Recorder: recorder}).Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
							t.Fatal(err)
						}
					}
					checkPod := func(name, cpu, memory, configMap, configContents string) {
						t.Helper()
						pod := &corev1.Pod{}
						if err := cli.Get(ctx, client.ObjectKey{Namespace: comp.Namespace, Name: running.Name + name}, pod); err != nil {
							t.Fatal(err)
						}
						container := pod.Spec.Containers[0]
						if container.Resources.Requests.Cpu().String() != cpu ||
							(memory != "" && container.Resources.Requests.Memory().String() != memory) ||
							len(container.Env) != 0 || container.VolumeMounts[0].MountPath != "/etc/db" {
							t.Errorf("unexpected config runtime for %s: %+v", pod.Name, container)
						}
						for _, volume := range pod.Spec.Volumes {
							if volume.Name != "config" {
								continue
							}
							if volume.ConfigMap == nil || volume.ConfigMap.Name != configMap {
								t.Fatalf("%s mounts the wrong configuration: %+v", pod.Name, volume)
							}
							cm := &corev1.ConfigMap{}
							if err := cli.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: configMap}, cm); err != nil || cm.Data["server.conf"] != configContents {
								t.Fatalf("%s did not mount the expected config contents: %v, %v", pod.Name, cm.Data, err)
							}
							return
						}
						t.Fatalf("%s has no config volume", pod.Name)
					}
					checkPod("-0", "1", "", "default-config", shared.Data["server.conf"])
					if specialized {
						checkPod("-canary-0", "2", "12Gi", "specialized-config", "cpu=2 memory=12Gi")
						checkPod("-other-0", "3", "", "default-config", "shared")
					}

					if err := cli.Get(ctx, key, starting); err != nil {
						t.Fatal(err)
					}
					plan, err = workloadConfigPlan(t, cli, comp, synth, starting, maps...)
					if err != nil {
						t.Fatal(err)
					}
					if err := plan.Execute(); err != nil {
						t.Fatal(err)
					}
					updated := &workloads.InstanceSet{}
					if err := cli.Get(ctx, key, updated); err != nil {
						t.Fatal(err)
					}
					if updated.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String() != "2" ||
						!ptr.Equal(updated.Spec.Configs[0].ConfigHash, synth.Configs[0].ConfigHash) {
						t.Error("normal reconciliation did not apply the deferred shared changes")
					}
					if specialized && (len(updated.Spec.Instances[0].Env) != 1 ||
						updated.Spec.Instances[1].Resources.Requests.Cpu().String() != "4") {
						t.Error("normal reconciliation did not apply the deferred template changes")
					}
				})
			}
		})
	}
}

func TestStartingManagedConfigurationWithLegacyConfigManager(t *testing.T) {
	for _, enableInstanceAPI := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct Pod", true: "Instance"}[enableInstanceAPI], func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			synth.EnableInstanceAPI = ptr.To(enableInstanceAPI)
			mount := corev1.VolumeMount{Name: "config", MountPath: "/opt/db"}
			synth.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{mount}
			running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
			if err != nil {
				t.Fatal(err)
			}
			running.Spec.Stop = ptr.To(true)
			running.Spec.Configs = []workloads.ConfigTemplate{{Name: "old-config"}}
			running.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "old-config"
			running.Spec.Template.Spec.Volumes = append(running.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: "legacy-scripts", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "legacy-scripts"},
				}},
			})
			running.Spec.Template.Spec.Containers = append(running.Spec.Template.Spec.Containers, corev1.Container{
				Name: "config-manager", Image: "kb:old", VolumeMounts: []corev1.VolumeMount{
					mount, {Name: "legacy-scripts", MountPath: "/opt/scripts"},
				},
			})

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
			plan, err := workloadConfigPlan(t, cli, comp, synth, running)
			if err != nil {
				t.Fatal(err)
			}
			if err = plan.Execute(); err != nil {
				t.Fatal(err)
			}
			current := &workloads.InstanceSet{}
			if err = cli.Get(context.Background(), client.ObjectKeyFromObject(running), current); err != nil {
				t.Fatal(err)
			}
			if ptr.Deref(current.Spec.Stop, false) {
				t.Fatal("workload is still stopped")
			}
			if _, err = instancetemplate.BuildInstanceTemplateExtMap(current, nil); err != nil {
				t.Fatalf("Start published invalid managed config bindings: %v", err)
			}
			_, sidecar := intctrlutil.GetContainerByName(current.Spec.Template.Spec.Containers, "config-manager")
			if sidecar == nil {
				t.Fatal("Start removed the legacy sidecar")
			}
			volumes := current.Spec.Template.Spec.Volumes
			if len(volumes) != 2 || volumes[1].Name != "legacy-scripts" || volumes[1].ConfigMap == nil || volumes[1].ConfigMap.Name != "legacy-scripts" {
				t.Fatalf("Start changed the legacy-only volume: %#v", volumes)
			}
		})
	}
}

var _ = Describe("Component Workload Operations Test", func() {
	const (
		clusterName    = "test-cluster"
		compName       = "test-comp"
		kubeblocksName = "kubeblocks"
	)

	var (
		reader         *appsutil.MockReader
		graphCli       model.GraphClient
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
			LifecycleActions: component.SynthesizedLifecycleActions{
				ComponentLifecycleActions: &appsv1.ComponentLifecycleActions{
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
			},
		}

		graphCli = model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)
	})

	Context("Start generation", func() {
		It("publishes the desired generation only after the deferred workload update", func() {
			comp.Generation = 2
			comp.Status.Phase = appsv1.StartingComponentPhase
			compDef := &appsv1.ComponentDefinition{}
			synthesizeComp.CompDefName = "db"
			synthesizeComp.ServiceVersion = "1.0.0"
			synthesizeComp.Generation = "1"
			synthesizeComp.Replicas = 1
			synthesizeComp.PodSpec = &corev1.PodSpec{
				Containers: []corev1.Container{{Name: "db", Image: "db:1"}},
			}
			synthesizeComp.LifecycleActions.ComponentLifecycleActions = &appsv1.ComponentLifecycleActions{}
			synthesizeComp.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1)), ServiceVersion: "1.0.0"}}
			synthesizeComp.InstanceImages = map[string]map[string]string{"canary": {"db": "db:1"}}
			stopped, err := component.BuildInstanceSet(synthesizeComp, compDef)
			Expect(err).ShouldNot(HaveOccurred())
			stopped.Spec.Stop = ptr.To(true)
			cli := fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).WithObjects(stopped).Build()

			synthesizeComp.Generation = "2"
			synthesizeComp.Instances[0].ServiceVersion = "2.0.0"
			synthesizeComp.InstanceImages["canary"]["db"] = "db:2"
			publish := func() *workloads.InstanceSet {
				current := &workloads.InstanceSet{}
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(stopped), current)).Should(Succeed())
				graphCli := model.NewGraphClient(cli)
				dag := newDAG(graphCli, comp)
				transCtx := &componentTransformContext{
					Context:             ctx,
					Client:              graphCli,
					Logger:              logger,
					EventRecorder:       clusterRecorder,
					Component:           comp,
					ComponentOrig:       comp.DeepCopy(),
					CompDef:             compDef,
					SynthesizeComponent: synthesizeComp,
					RunningWorkload:     current,
				}
				Expect((&componentWorkloadTransformer{Client: cli}).Transform(transCtx, dag)).Should(Succeed())
				Expect(graphCli.IsAction(dag, current, model.ActionUpdatePtr())).Should(BeTrue())
				updated := graphCli.FindMatchedVertex(dag, current).(*model.ObjectVertex).Obj.(*workloads.InstanceSet)
				Expect(cli.Update(ctx, updated)).Should(Succeed())
				persisted := &workloads.InstanceSet{}
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(current), persisted)).Should(Succeed())
				return persisted
			}

			started := publish()
			Expect(started.Spec.Stop).Should(BeNil())
			Expect(started.Spec.Instances[0].Images["db"]).Should(Equal("db:1"))
			Expect(started.Annotations[constant.KubeBlocksGenerationKey]).Should(Equal("1"))

			updated := publish()
			Expect(comp.Status.Phase).Should(Equal(appsv1.StartingComponentPhase))
			Expect(updated.Spec.Stop).Should(BeNil())
			Expect(updated.Spec.Instances[0].Images["db"]).Should(Equal("db:2"))
			Expect(updated.Annotations[constant.KubeBlocksGenerationKey]).Should(Equal("2"))
		})
	})

	Context("Data Replication Operations", func() {
		It("blocks scale-out when data actions have no source pod", func() {
			synthesizeComp.Replicas = 1
			synthesizeComp.LifecycleActions.DataDump = testapps.NewLifecycleAction("data-dump")
			synthesizeComp.LifecycleActions.DataLoad = testapps.NewLifecycleAction("data-load")

			runningITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"test-its", clusterName, compName).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(compName).
				AddAppManagedByLabel().
				SetReplicas(0).
				GetObject()
			protoITS := runningITS.DeepCopy()
			protoITS.Spec.Replicas = ptr.To(int32(1))

			transCtx := &componentTransformContext{
				Context:             ctx,
				Client:              graphCli,
				Logger:              logger,
				EventRecorder:       clusterRecorder,
				Component:           comp,
				SynthesizeComponent: synthesizeComp,
			}
			ops, err := newComponentWorkloadOps(transCtx, k8sClient, synthesizeComp,
				comp, runningITS, protoITS, dag)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(ops.scaleOut()).Should(MatchError("no available pod to dump data"))
		})
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
					Client:        graphCli,
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

		It("should emit events when member leave fails", func() {
			testapps.MockKBAgentClient(func(mockRecorder *kbacli.MockClientMockRecorder) {
				mockRecorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					if req.Action == "memberLeave" {
						return kbagentproto.ActionResponse{
							Error:   kbagentproto.Error2Type(kbagentproto.ErrTimedOut),
							Message: "member leave timed out",
						}, nil
					}
					return kbagentproto.ActionResponse{}, nil
				}).Times(2)
			})

			eventRecorder := &capturingEventRecorder{}
			ops.transCtx.Component = comp
			ops.transCtx.EventRecorder = eventRecorder

			Expect(ops.leaveMemberForPod(pod1, pods)).ShouldNot(Succeed())
			Expect(eventRecorder.events).Should(HaveLen(1))
			for _, event := range eventRecorder.events {
				Expect(event.reason).Should(Equal(memberLeaveFailedEventReason))
				Expect(event.message).Should(ContainSubstring("pod " + pod1.Name))
			}
		})

		It("should emit events when member join fails", func() {
			testapps.MockKBAgentClient(func(mockRecorder *kbacli.MockClientMockRecorder) {
				mockRecorder.Action(gomock.Any(), gomock.Any()).Return(kbagentproto.ActionResponse{
					Error:   kbagentproto.Error2Type(kbagentproto.ErrFailed),
					Message: "member join failed",
				}, nil).Times(1)
			})

			eventRecorder := &capturingEventRecorder{}
			ops.transCtx.Component = comp
			ops.transCtx.EventRecorder = eventRecorder

			Expect(ops.joinMemberForPod(pod1, pods)).ShouldNot(Succeed())
			Expect(eventRecorder.events).Should(HaveLen(1))
			for _, event := range eventRecorder.events {
				Expect(event.reason).Should(Equal(memberJoinFailedEventReason))
				Expect(event.message).Should(ContainSubstring("pod " + pod1.Name))
			}
		})

		It("should emit a member action failure only when its fingerprint changes", func() {
			eventRecorder := &capturingEventRecorder{}
			ops.transCtx.Component = comp
			ops.transCtx.EventRecorder = eventRecorder
			actionErr := errors.New("member join failed")

			ops.reportMemberActionFailure(pod1, memberJoinFailureFingerprintAnnotationKey,
				memberJoinFailedEventReason, "memberJoin", actionErr)
			Expect(eventRecorder.events).Should(HaveLen(1))

			vertex := graphCli.FindMatchedVertex(dag, pod1)
			Expect(vertex).ShouldNot(BeNil())
			patchedPod := vertex.(*model.ObjectVertex).Obj.(*corev1.Pod)
			Expect(patchedPod.Annotations[memberJoinFailureFingerprintAnnotationKey]).ShouldNot(BeEmpty())

			ops.dag = newDAG(graphCli, comp)
			ops.reportMemberActionFailure(patchedPod, memberJoinFailureFingerprintAnnotationKey,
				memberJoinFailedEventReason, "memberJoin", actionErr)
			Expect(eventRecorder.events).Should(HaveLen(1))

			ops.reportMemberActionFailure(patchedPod, memberJoinFailureFingerprintAnnotationKey,
				memberJoinFailedEventReason, "memberJoin", errors.New("member join timed out"))
			Expect(eventRecorder.events).Should(HaveLen(2))
			changedVertex := graphCli.FindMatchedVertex(ops.dag, patchedPod)
			changedPod := changedVertex.(*model.ObjectVertex).Obj.(*corev1.Pod)

			ops.dag = newDAG(graphCli, comp)
			ops.clearMemberActionFailureFingerprint(changedPod, memberJoinFailureFingerprintAnnotationKey)
			clearedVertex := graphCli.FindMatchedVertex(ops.dag, changedPod)
			clearedPod := clearedVertex.(*model.ObjectVertex).Obj.(*corev1.Pod)
			Expect(clearedPod.Annotations).ShouldNot(HaveKey(memberJoinFailureFingerprintAnnotationKey))
		})

		It("should eliminate upgrade-only diff by preserving legacy config-manager", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers,
				corev1.Container{
					Name:  "config-manager",
					Image: "cm-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
				corev1.Container{
					Name:  "metrics",
					Image: "metrics-image",
				},
			)
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers, corev1.Container{
				Name:  "install-config-manager-tool",
				Image: "tools-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})

			newITS := oldITS.DeepCopy()
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
				{
					Name:  "metrics",
					Image: "metrics-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = nil
			newITS.Spec.Template.Spec.Volumes = nil

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)

			Expect(err).Should(Succeed())
			Expect(merged).Should(BeNil())
		})

		It("should not reintroduce legacy config-manager for workloads that never had it", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-no-legacy", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
				}).
				GetObject()

			newITS := oldITS.DeepCopy()
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "new-image",
				},
			}

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)

			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(merged.Spec.Template.Spec.Containers[0].Name).Should(Equal("main"))
			Expect(merged.Spec.Template.Spec.Containers[0].Image).Should(Equal("new-image"))
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).Should(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).Should(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(BeEmpty())
		})

		It("preserves adopted role-label recovery fields when the gate is disabled", func() {
			oldGate := viper.GetBool(constant.FeatureGateRoleLabelRecovery)
			defer viper.Set(constant.FeatureGateRoleLabelRecovery, oldGate)
			viper.Set(constant.FeatureGateRoleLabelRecovery, false)

			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-role-recovery", clusterName, compName).
				AddContainer(corev1.Container{
					Name: "kbagent",
					VolumeMounts: []corev1.VolumeMount{{
						Name: "kubeblocks-role-label", MountPath: "/etc/kubeblocks/pod-metadata", ReadOnly: true,
					}},
					Env: []corev1.EnvVar{{Name: "KB_AGENT_PROBE", Value: `[{"instance":"","action":"roleProbe","reportPeriodSeconds":15,"reportOnFileChange":["/etc/kubeblocks/pod-metadata"]}]`}},
				}).
				GetObject()
			oldITS.Spec.Template.Spec.Volumes = []corev1.Volume{{
				Name:         "kubeblocks-role-label",
				VolumeSource: corev1.VolumeSource{DownwardAPI: &corev1.DownwardAPIVolumeSource{Items: []corev1.DownwardAPIVolumeFile{{Path: "role", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels['kubeblocks.io/role']"}}}}},
			}}
			newITS := oldITS.DeepCopy()
			newITS.Spec.Template.Spec.Volumes = nil
			newITS.Spec.Template.Spec.Containers[0].VolumeMounts = nil
			newITS.Spec.Template.Spec.Containers[0].Env[0].Value = `[{"instance":"","action":"roleProbe"}]`

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)

			Expect(err).Should(Succeed())
			Expect(merged).Should(BeNil())
		})

		It("should preserve only the legacy config-manager resources that still exist on the live template", func() {
			// Some clusters may carry partially migrated legacy resources. The compatibility logic should
			// keep only what still exists on the live template instead of synthesizing a full legacy bundle.
			buildBaseITS := func(name string) *workloads.InstanceSet {
				its := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace, name, clusterName, compName).
					AddContainer(corev1.Container{
						Name:  "main",
						Image: "test-image",
					}).
					GetObject()
				its.Spec.Template.Spec.Containers = append(its.Spec.Template.Spec.Containers, corev1.Container{
					Name:  "config-manager",
					Image: "cm-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				})
				return its
			}

			tests := []struct {
				name            string
				oldITS          *workloads.InstanceSet
				wantInit        bool
				wantVolume      bool
				wantMainMount   bool
				wantConfigMount bool
			}{
				{
					name: "missing legacy init container",
					oldITS: func() *workloads.InstanceSet {
						its := buildBaseITS("old-its-no-legacy-init")
						its.Spec.Template.Spec.Volumes = append(its.Spec.Template.Spec.Volumes, corev1.Volume{
							Name: "kb-tools",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						})
						its.Spec.Template.Spec.Containers[0].VolumeMounts = append(its.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
							Name:      "kb-tools",
							MountPath: "/opt/kb-tools",
						})
						return its
					}(),
					wantInit:        false,
					wantVolume:      true,
					wantMainMount:   true,
					wantConfigMount: true,
				},
				{
					name: "missing legacy volume",
					oldITS: func() *workloads.InstanceSet {
						its := buildBaseITS("old-its-no-legacy-volume")
						its.Spec.Template.Spec.InitContainers = append(its.Spec.Template.Spec.InitContainers, corev1.Container{
							Name:  "install-config-manager-tool",
							Image: "tools-image",
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "kb-tools",
								MountPath: "/opt/kb-tools",
							}},
						})
						its.Spec.Template.Spec.Containers[0].VolumeMounts = append(its.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
							Name:      "kb-tools",
							MountPath: "/opt/kb-tools",
						})
						return its
					}(),
					wantInit:        true,
					wantVolume:      false,
					wantMainMount:   true,
					wantConfigMount: true,
				},
				{
					name: "missing business mount for legacy volume",
					oldITS: func() *workloads.InstanceSet {
						its := buildBaseITS("old-its-no-business-mount")
						its.Spec.Template.Spec.InitContainers = append(its.Spec.Template.Spec.InitContainers, corev1.Container{
							Name:  "install-config-manager-tool",
							Image: "tools-image",
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "kb-tools",
								MountPath: "/opt/kb-tools",
							}},
						})
						its.Spec.Template.Spec.Volumes = append(its.Spec.Template.Spec.Volumes, corev1.Volume{
							Name: "kb-tools",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						})
						return its
					}(),
					wantInit:        true,
					wantVolume:      true,
					wantMainMount:   false,
					wantConfigMount: true,
				},
			}

			for _, tt := range tests {
				By(tt.name)
				newITS := tt.oldITS.DeepCopy()
				newITS.Spec.Template.Spec.Containers = []corev1.Container{
					{
						Name:  "main",
						Image: "new-image",
					},
				}
				newITS.Spec.Template.Spec.InitContainers = nil
				newITS.Spec.Template.Spec.Volumes = nil

				merged, err := copyAndMergeITS(tt.oldITS, newITS, legacyConfigManagerPolicyKeep)

				Expect(err).Should(Succeed())
				Expect(merged).ShouldNot(BeNil())

				_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
				Expect(cfg).ShouldNot(BeNil())
				Expect(hasVolumeMount(cfg.VolumeMounts, "kb-tools")).Should(Equal(tt.wantConfigMount))

				_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
				if tt.wantInit {
					Expect(init).ShouldNot(BeNil())
				} else {
					Expect(init).Should(BeNil())
				}

				if tt.wantVolume {
					Expect(merged.Spec.Template.Spec.Volumes).ShouldNot(BeEmpty())
					Expect(hasVolumeByName(func() map[string]struct{} {
						names := map[string]struct{}{}
						for _, volume := range merged.Spec.Template.Spec.Volumes {
							names[volume.Name] = struct{}{}
						}
						return names
					}(), "kb-tools")).Should(BeTrue())
				} else {
					Expect(merged.Spec.Template.Spec.Volumes).Should(BeEmpty())
				}

				_, main := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "main")
				Expect(main).ShouldNot(BeNil())
				Expect(hasVolumeMount(main.VolumeMounts, "kb-tools")).Should(Equal(tt.wantMainMount))
			}
		})

		It("should keep legacy config-manager ordering during in-place business changes", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-order", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers,
				corev1.Container{
					Name:  "config-manager",
					Image: "cm-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
				corev1.Container{
					Name:  "exporter",
					Image: "exporter-image",
				},
			)
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)
			oldITS.Spec.Template.Spec.Volumes = append(oldITS.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: "kb-tools",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			})

			newITS := oldITS.DeepCopy()
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "main-business-upgrade",
				},
				{
					Name:  "exporter",
					Image: "exporter-business-upgrade",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{
				{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-new"},
				},
			}
			newITS.Spec.Template.Spec.Volumes = nil

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)

			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(len(merged.Spec.Template.Spec.Volumes)).ShouldNot(BeZero())
			for _, c := range merged.Spec.Template.Spec.Containers {
				if c.Name == "main" {
					Expect(len(c.VolumeMounts)).Should(Equal(1))
					Expect(c.VolumeMounts[0].Name).Should(Equal("kb-tools"))
				}
			}
			Expect(merged.Spec.Template.Spec.Containers[0].Name).Should(Equal("main"))
			Expect(merged.Spec.Template.Spec.Containers[1].Name).Should(Equal("config-manager"))
			Expect(merged.Spec.Template.Spec.Containers[2].Name).Should(Equal("exporter"))
			Expect(merged.Spec.Template.Spec.InitContainers[0].Name).Should(Equal("business-init"))
			Expect(merged.Spec.Template.Spec.InitContainers[0].Command).Should(Equal([]string{"prepare-new"}))
			Expect(merged.Spec.Template.Spec.InitContainers[1].Name).Should(Equal("install-config-manager-tool"))
		})

		It("should drop legacy config-manager when business changes already recreate pods", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-recreate", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers, corev1.Container{
				Name:  "install-config-manager-tool",
				Image: "tools-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append([]corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-old"},
			}}, oldITS.Spec.Template.Spec.InitContainers...)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyCleanup)

			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).Should(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).Should(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(BeEmpty())
		})

		It("should clean legacy config-manager when component annotation allows cleanup on recreate rollout", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-recreate-false-annotation", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			comp := &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.LegacyConfigManagerRequiredAnnotationKey: "false",
					},
				},
			}

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{{
				Name:  "main",
				Image: "test-image",
			}}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerRequired(comp))

			Expect(err).Should(Succeed())
			Expect(legacyConfigManagerRequired(comp)).Should(Equal(legacyConfigManagerPolicyCleanup))
			Expect(merged).ShouldNot(BeNil())

			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).Should(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).Should(BeNil())
			Expect(hasVolumeByName(func() map[string]struct{} {
				names := map[string]struct{}{}
				for _, volume := range merged.Spec.Template.Spec.Volumes {
					names[volume.Name] = struct{}{}
				}
				return names
			}(), "kb-tools")).Should(BeFalse())
			_, main := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "main")
			Expect(main).ShouldNot(BeNil())
			Expect(hasVolumeMount(main.VolumeMounts, "kb-tools")).Should(BeFalse())
		})

		It("should keep legacy config-manager when compatibility annotation is still required", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-annotation-required", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)

			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(HaveLen(1))
			Expect(merged.Spec.Template.Spec.Volumes[0].Name).Should(Equal("kb-tools"))
		})

		It("should keep legacy config-manager when business changes stay in-place", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-inplace", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers, corev1.Container{
				Name:  "install-config-manager-tool",
				Image: "tools-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append([]corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-old"},
			}}, oldITS.Spec.Template.Spec.InitContainers...)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)

			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.InitContainers[0].Name).Should(Equal("business-init"))
			Expect(merged.Spec.Template.Spec.InitContainers[0].Command).Should(Equal([]string{"prepare-new"}))
			Expect(merged.Spec.Template.Spec.InitContainers[1].Name).Should(Equal("install-config-manager-tool"))
		})

		It("should keep legacy config-manager when annotation is absent and rollout stays in-place", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-inplace-no-annotation", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)

			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(HaveLen(1))
			Expect(merged.Spec.Template.Spec.Volumes[0].Name).Should(Equal("kb-tools"))
		})

		It("should keep legacy config-manager when annotation is absent and rollout recreates pods", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-recreate-no-annotation", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{{
				Name:  "main",
				Image: "test-image",
			}}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)

			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(HaveLen(1))
		})

		It("should keep legacy config-manager when annotation is false but rollout stays in-place", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-inplace-false-annotation", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{{
				Name:  "main",
				Image: "test-image",
			}}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged, err := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyCleanup)

			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(HaveLen(1))
		})

		It("should resolve legacy config-manager policy conservatively when annotation is missing", func() {
			Expect(legacyConfigManagerRequired(nil)).Should(Equal(legacyConfigManagerPolicyKeep))
			Expect(legacyConfigManagerRequired(&appsv1.Component{})).Should(Equal(legacyConfigManagerPolicyKeep))
			Expect(legacyConfigManagerRequired(&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.LegacyConfigManagerRequiredAnnotationKey: "true",
					},
				},
			})).Should(Equal(legacyConfigManagerPolicyKeep))
			Expect(legacyConfigManagerRequired(&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.LegacyConfigManagerRequiredAnnotationKey: "false",
					},
				},
			})).Should(Equal(legacyConfigManagerPolicyCleanup))
			Expect(legacyConfigManagerRequired(&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.LegacyConfigManagerRequiredAnnotationKey: "",
					},
				},
			})).Should(Equal(legacyConfigManagerPolicyKeep))
		})
	})

	Context("workload helper edge cases", func() {
		It("detects pod resource and metadata-only template changes", func() {
			emptySpec := corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main"}},
			}
			Expect(hasPodResourceChanges(emptySpec, emptySpec)).Should(BeFalse())

			resourceSpec := corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "main",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				}},
			}
			Expect(hasPodResourceChanges(emptySpec, resourceSpec)).Should(BeTrue())

			oldTemplate := corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "old"},
				},
				Spec: emptySpec,
			}
			newTemplate := *oldTemplate.DeepCopy()
			Expect(hasPodUpdateTemplateChanges(oldTemplate, newTemplate)).Should(BeFalse())

			newTemplate.Labels = map[string]string{"app": "new"}
			Expect(hasPodUpdateTemplateChanges(oldTemplate, newTemplate)).Should(BeTrue())

			newTemplate = *oldTemplate.DeepCopy()
			newTemplate.Annotations = map[string]string{"rollout": "yes"}
			Expect(hasPodUpdateTemplateChanges(oldTemplate, newTemplate)).Should(BeTrue())

			newTemplate = *oldTemplate.DeepCopy()
			deadline := int64(30)
			newTemplate.Spec.ActiveDeadlineSeconds = &deadline
			Expect(hasPodUpdateTemplateChanges(oldTemplate, newTemplate)).Should(BeTrue())

			newTemplate = *oldTemplate.DeepCopy()
			newTemplate.Spec.Tolerations = []corev1.Toleration{{Key: "dedicated", Value: "db"}}
			Expect(hasPodUpdateTemplateChanges(oldTemplate, newTemplate)).Should(BeTrue())
		})

		It("preserves legacy volume and mount order without synthesizing missing business entries", func() {
			oldVolumes := []corev1.Volume{
				{Name: "data"},
				{Name: "kb-tools"},
				{Name: "logs"},
			}
			newVolumes := []corev1.Volume{
				{Name: "logs"},
				{Name: "data"},
				{Name: "cache"},
			}
			mergedVolumes := preserveLegacyVolumeOrder(oldVolumes, newVolumes, map[string]struct{}{"kb-tools": {}})
			Expect([]string{mergedVolumes[0].Name, mergedVolumes[1].Name, mergedVolumes[2].Name, mergedVolumes[3].Name}).
				Should(Equal([]string{"data", "kb-tools", "logs", "cache"}))
			Expect(preserveLegacyVolumeOrder(oldVolumes, newVolumes, nil)).Should(Equal(newVolumes))
			Expect(hasVolumeByNameMap(map[string]corev1.Volume{"data": {Name: "data"}}, "data")).Should(BeTrue())
			Expect(hasVolumeByNameMap(map[string]corev1.Volume{"data": {Name: "data"}}, "logs")).Should(BeFalse())

			oldMounts := []corev1.VolumeMount{
				{Name: "data", MountPath: "/data"},
				{Name: "kb-tools", MountPath: "/opt/kb-tools"},
				{Name: "logs", MountPath: "/logs"},
			}
			newMounts := []corev1.VolumeMount{
				{Name: "logs", MountPath: "/logs"},
				{Name: "cache", MountPath: "/cache"},
				{Name: "data", MountPath: "/data"},
			}
			mergedMounts := preserveLegacyVolumeMountOrder(oldMounts, newMounts, "kb-tools")
			Expect([]string{mergedMounts[0].Name, mergedMounts[1].Name, mergedMounts[2].Name, mergedMounts[3].Name}).
				Should(Equal([]string{"data", "kb-tools", "logs", "cache"}))
			Expect(volumeMountKey(corev1.VolumeMount{Name: "data", MountPath: "/data", SubPath: "main"})).
				Should(Equal("data|/data|main"))
			Expect(preserveLegacyVolumeMountOrder(oldMounts, append(newMounts, corev1.VolumeMount{Name: "kb-tools"}), "kb-tools")).
				Should(HaveLen(4))
			Expect(preserveLegacyVolumeMountOrder(newMounts, newMounts, "kb-tools")).Should(Equal(newMounts))
		})

		It("collects and restores container resource and volume mount helpers", func() {
			containers := []corev1.Container{
				{
					Name: "main",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{{Name: "data"}},
				},
				{
					Name:         "sidecar",
					VolumeMounts: []corev1.VolumeMount{{Name: "logs"}},
				},
			}

			resources := getContainerResources(containers)
			Expect(resources).Should(HaveKey("main"))
			memory := resources["main"].Requests[corev1.ResourceMemory]
			Expect(memory.String()).Should(Equal("128Mi"))

			names := collectVolumeNamesFromContainers(&containers[0], nil, &containers[1])
			Expect(names).Should(HaveKey("data"))
			Expect(names).Should(HaveKey("logs"))
			Expect(hasVolumeMount(containers[0].VolumeMounts, "data")).Should(BeTrue())
			Expect(hasVolumeMount(containers[0].VolumeMounts, "logs")).Should(BeFalse())
		})

		It("decides legacy config-manager cleanup from concrete pod template change classes", func() {
			oldToolsImage := viper.GetString(constant.KBToolsImage)
			defer viper.Set(constant.KBToolsImage, oldToolsImage)
			viper.Set(constant.KBToolsImage, "docker.io/apecloud/kubeblocks-tools:1.0.0")

			baseTemplate := corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "mysql"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "mysql:1"}},
				},
			}
			newInstanceSet := func() *workloads.InstanceSet {
				return &workloads.InstanceSet{
					Spec: workloads.InstanceSetSpec{
						Template:         *baseTemplate.DeepCopy(),
						PodUpgradePolicy: appsv1.PreferInPlacePodUpdatePolicyType,
						PodUpdatePolicy:  appsv1.PreferInPlacePodUpdatePolicyType,
					},
				}
			}

			oldITS := newInstanceSet()
			Expect(shouldCleanupLegacyConfigManager(oldITS, newInstanceSet())).Should(BeFalse())

			upgradeITS := newInstanceSet()
			upgradeITS.Spec.Template.Spec.Containers[0].Image = "mysql:2"
			Expect(shouldCleanupLegacyConfigManager(oldITS, upgradeITS)).Should(BeFalse())
			upgradeITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			Expect(shouldCleanupLegacyConfigManager(oldITS, upgradeITS)).Should(BeTrue())

			kbManagedITS := newInstanceSet()
			kbManagedITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			kbManagedITS.Spec.Template.Spec.InitContainers = []corev1.Container{
				{Name: "init-kbagent", Image: "docker.io/apecloud/kubeblocks-tools:1.0.0", Command: []string{"cp"}},
			}
			kbManagedITS.Spec.Template.Spec.Containers = append(kbManagedITS.Spec.Template.Spec.Containers,
				corev1.Container{Name: "kbagent", Image: "docker.io/apecloud/kubeblocks-tools:1.0.0", Command: []string{"/bin/kbagent"}})
			kbManagedUpgradeITS := kbManagedITS.DeepCopy()
			kbManagedUpgradeITS.Spec.Template.Spec.InitContainers[0].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
			kbManagedUpgradeITS.Spec.Template.Spec.Containers[1].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
			Expect(shouldCleanupLegacyConfigManager(kbManagedITS, kbManagedUpgradeITS)).Should(BeFalse())

			resourceITS := newInstanceSet()
			resourceITS.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("2"),
				},
			}
			Expect(shouldCleanupLegacyConfigManager(oldITS, resourceITS)).Should(BeTrue())

			updateITS := newInstanceSet()
			updateITS.Spec.Template.Labels = map[string]string{"app": "mysql", "rollout": "yes"}
			Expect(shouldCleanupLegacyConfigManager(oldITS, updateITS)).Should(BeFalse())
			updateITS.Spec.PodUpdatePolicy = appsv1.ReCreatePodUpdatePolicyType
			Expect(shouldCleanupLegacyConfigManager(oldITS, updateITS)).Should(BeTrue())
		})

		It("copies multi-cluster placement annotation to generated InstanceSet only when present", func() {
			transformer := &componentWorkloadTransformer{}
			its := &workloads.InstanceSet{}

			transformer.buildInstanceSetPlacementAnnotation(&appsv1.Component{}, its)
			Expect(its.Annotations).Should(BeNil())

			transformer.buildInstanceSetPlacementAnnotation(&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: ""},
				},
			}, its)
			Expect(its.Annotations).Should(BeNil())

			transformer.buildInstanceSetPlacementAnnotation(&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "ctx-a,ctx-b"},
				},
			}, its)
			Expect(its.Annotations).Should(HaveKeyWithValue(constant.KBAppMultiClusterPlacementKey, "ctx-a,ctx-b"))
		})
	})
})
