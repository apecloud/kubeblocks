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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

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
			synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{"canary": {{Name: "main", ConfigHash: ptr.To("special-hash"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: cm.Name}}}}
			var running *workloads.InstanceSet
			var objects []client.Object
			if update {
				var err error
				running, err = component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
				if err != nil {
					t.Fatal(err)
				}
				objects = append(objects, running)
				synth.PodSpec.Containers[0].Image = "db:new"
			}
			providerA := fake.NewClientBuilder().WithScheme(scheme).Build()
			providerBBase := fake.NewClientBuilder().WithScheme(scheme).Build()
			failMirror := true
			providerB := interceptor.NewClient(providerBBase, interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if failMirror {
					return errors.New("mirror unavailable")
				}
				return c.Create(ctx, obj, opts...)
			}})
			writes := 0
			checkMirrors := func() {
				for _, provider := range []client.Client{providerA, providerB} {
					mirror := &corev1.ConfigMap{}
					if err := provider.Get(context.Background(), client.ObjectKeyFromObject(cm), mirror); err != nil || mirror.Data["config"] != "value" {
						t.Fatalf("workload published before mirror: %#v, %v", mirror, err)
					}
					if len(mirror.OwnerReferences) != 0 || len(mirror.Finalizers) != 0 || mirror.Labels[instanceConfigMapOwnerLabelKey] != string(comp.UID) {
						t.Fatalf("mirror has invalid ownership: %#v", mirror.ObjectMeta)
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
			plan, err := workloadConfigPlan(t, cli, comp, synth, running, cm)
			if err != nil {
				t.Fatal(err)
			}
			if err := plan.Execute(); err == nil || writes != 0 {
				t.Fatalf("mirror failure allowed workload publication: writes=%d, error=%v", writes, err)
			}
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
				t.Fatalf("mirror ran before authoritative CM dependency: %v", err)
			}
			failMirror = false
			if err := plan.Execute(); err != nil {
				t.Fatal(err)
			}
			if writes != 1 {
				t.Fatalf("workload writes = %d, want one", writes)
			}
		})
	}
}

func TestInstanceConfigMirrorsUpdateWithStableWorkload(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	synth.EnableInstanceAPI = ptr.To(true)
	comp.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "a"}
	synth.Annotations = comp.Annotations
	synth.AnnotationsInjectedToWorkload = comp.Annotations
	synth.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1))}}
	cm := managedInstanceConfig(comp, "special")
	synth.InstanceConfigs = map[string][]workloads.ConfigTemplate{"canary": {{Name: "main", ConfigHash: ptr.To("fixed-user-version"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: cm.Name}}}}
	running, err := component.BuildInstanceSet(synth, &appsv1.ComponentDefinition{})
	if err != nil {
		t.Fatal(err)
	}
	controlBase := fake.NewClientBuilder().WithScheme(scheme).WithObjects(running).Build()
	workloadWrites := 0
	control := interceptor.NewClient(controlBase, interceptor.Funcs{Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
		if _, ok := obj.(*workloads.InstanceSet); ok {
			workloadWrites++
		}
		return c.Update(ctx, obj, opts...)
	}})
	provider := fake.NewClientBuilder().WithScheme(scheme).Build()
	cli := multicluster.NewClient(control, map[string]client.Client{"a": provider})
	for _, value := range []string{"first", "second"} {
		cm.Data["config"] = value
		graphCli := model.NewGraphClient(cli)
		transCtx := &componentTransformContext{Context: context.Background(), Component: comp,
			ProtoWorkload: running, InstanceConfigMaps: []*corev1.ConfigMap{cm}}
		dag := graph.NewDAG()
		graphCli.Root(dag, comp, comp, model.ActionStatusPtr())
		previous := &corev1.ConfigMap{}
		if err := control.Get(context.Background(), client.ObjectKeyFromObject(cm), previous); err != nil {
			graphCli.Create(dag, cm)
		} else {
			desired := previous.DeepCopy()
			desired.Data = cm.Data
			graphCli.Update(dag, previous, desired)
		}
		builder := &componentPlanBuilder{cli: cli, transCtx: transCtx}
		plan := &componentPlan{dag: dag, transCtx: transCtx, cleanupClient: cli, walkFunc: func(v graph.Vertex) error {
			if _, ok := v.(*model.ObjectVertex).Obj.(*appsv1.Component); ok {
				return nil
			}
			return builder.defaultWalkFunc(v)
		}}
		if err := plan.Execute(); err != nil {
			t.Fatal(err)
		}
		mirror := &corev1.ConfigMap{}
		if err := provider.Get(context.Background(), client.ObjectKeyFromObject(cm), mirror); err != nil || mirror.Data["config"] != value {
			t.Fatalf("stable-workload mirror = %#v, %v, want %s", mirror, err, value)
		}
	}
	if workloadWrites != 0 {
		t.Fatalf("CM-only change updated workload %d times", workloadWrites)
	}
}

var _ = Describe("component plan builder test", func() {
	const (
		compDefName = "test-compdef"
		compName    = "test-comp"
	)

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete components (and all dependent sub-resources), and component definitions & versions
		testapps.ClearComponentResourcesWithRemoveFinalizerOption(&testCtx)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("test init", func() {
		It("should init successfully", func() {
			compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, compName, compDefName).WithRandomName().GetObject()
			Expect(testCtx.Cli.Create(testCtx.Ctx, compObj)).Should(Succeed())
			compKey := client.ObjectKeyFromObject(compObj)
			Eventually(testapps.CheckObjExists(&testCtx, compKey, &appsv1.Component{}, true)).Should(Succeed())

			req := ctrl.Request{
				NamespacedName: compKey,
			}
			reqCtx := intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Req: req,
				Log: log.FromContext(ctx).WithValues("component", req.NamespacedName),
			}
			planBuilder := newComponentPlanBuilder(reqCtx, testCtx.Cli)
			Expect(planBuilder.Init()).Should(Succeed())
		})
	})
})
