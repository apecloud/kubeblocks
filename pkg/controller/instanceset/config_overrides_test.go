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

package instanceset

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/klauspost/compress/zstd"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestDirectConfigOverridesCreationAndConvergence(t *testing.T) {
	for _, layout := range []string{"named", "compressed named", "compressed flat"} {
		t.Run(layout, func(t *testing.T) {
			its, tree, pods, member := newDirectConfigFixture(t, layout)
			specific := pods[member]
			if specific.Spec.Volumes[0].ConfigMap.Name != "main-a" || specific.Spec.Volumes[1].ConfigMap.Name != "aux-a" {
				t.Fatal("member bindings were not expanded on creation")
			}
			if pods["demo-0"].Spec.Volumes[0].ConfigMap.Name != "main" {
				t.Fatal("default member binding changed")
			}
			desired, err := desiredInstanceTemplates(its, tree)
			if err != nil {
				t.Fatal(err)
			}
			hashes, err := configsFromPod(specific)
			if err != nil {
				t.Fatal(err)
			}
			if len(hashes) != 2 || *hashes[0].ConfigHash != "aux-old" || *hashes[1].ConfigHash != "main-old" {
				t.Fatalf("wrong initial hashes: %#v", hashes)
			}
			if getPodRevision(specific) == getPodRevision(pods["demo-0"]) {
				t.Fatal("different mounted source was omitted from initial revision")
			}
			for name, pod := range pods {
				policy, _, _, err := getPodUpdatePolicy(its, pod, desired[name])
				if err != nil || policy != noOpsPolicy {
					t.Fatalf("fresh member %s needs update: %v %v", name, policy, err)
				}
			}
			mutateDirectTemplate(t, its, tree, func(tpl *workloads.InstanceTemplate) { tpl.Configs[0].ConfigHash = ptr.To("main-new") })
			oldRevision := getPodRevision(specific)
			its.Generation++
			runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
			if its.FindInstanceStatus(member).UpToDate {
				t.Fatal("member hash change did not invalidate convergence")
			}
			if !its.FindInstanceStatus("demo-0").UpToDate {
				t.Fatal("override invalidated the default member")
			}
			revisions, err := GetRevisions(its.Status.UpdateRevisions)
			if err != nil {
				t.Fatal(err)
			}
			if revisions[member] != oldRevision {
				t.Fatal("same-mount dynamic hash changed the Pod revision")
			}
			desired, err = desiredInstanceTemplates(its, tree)
			if err != nil {
				t.Fatal(err)
			}
			if err := configsToPod(desired[member].Configs, specific); err != nil {
				t.Fatal(err)
			}
			runDirectReconciler(t, NewStatusReconciler(), tree)
			if !its.FindInstanceStatus(member).UpToDate || !its.FindInstanceStatus("demo-0").UpToDate {
				t.Fatal("effective hashes did not converge per member")
			}
		})
	}
}

func TestDirectConfigOverridesUpdateRouting(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*workloads.InstanceTemplate)
		wantCalls []string
		deleted   bool
	}{
		{"source switch preserves other bound reload", func(tpl *workloads.InstanceTemplate) {
			tpl.Configs[0].Volume.ConfigMapName = "main-new"
			tpl.Configs[0].ConfigHash = ptr.To("new")
			tpl.Configs[1].ConfigHash = ptr.To("aux-new")
			tpl.Configs[1].Restart = ptr.To(true)
		}, []string{"aux"}, true},
		{"same mount reload then restart", func(tpl *workloads.InstanceTemplate) {
			tpl.Configs[0].ConfigHash = ptr.To("new")
			tpl.Configs[0].Restart = ptr.To(true)
		}, []string{"main"}, true},
		{"same mount reload only", func(tpl *workloads.InstanceTemplate) { tpl.Configs[0].ConfigHash = ptr.To("new") }, []string{"main"}, false},
		{"removed overrides return to inherited mounts", func(tpl *workloads.InstanceTemplate) { tpl.Configs = nil }, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			its, tree, pods, member := newDirectConfigFixture(t, "named")
			spy := installDirectConfigSpy(t)
			mutateDirectTemplate(t, its, tree, tc.mutate)
			its.Generation++
			runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
			runDirectReconciler(t, NewUpdateReconciler(), tree)
			if !reflect.DeepEqual(spy.configs, tc.wantCalls) {
				t.Fatalf("reload calls %v, want %v", spy.configs, tc.wantCalls)
			}
			obj, err := tree.Get(pods[member])
			if err != nil {
				t.Fatal(err)
			}
			if (obj == nil) != tc.deleted {
				t.Fatalf("deleted=%v, want %v", obj == nil, tc.deleted)
			}
			obj, err = tree.Get(pods["demo-0"])
			if err != nil || obj != pods["demo-0"] {
				t.Fatal("update changed unaffected default member")
			}
			if tc.deleted {
				if spy.switchoverCalls != 1 {
					t.Fatal("recreation bypassed switchover")
				}
				runDirectReconciler(t, NewReplicasAlignmentReconciler(), tree)
				desired, err := desiredInstanceTemplates(its, tree)
				if err != nil {
					t.Fatal(err)
				}
				obj, err = tree.Get(pods[member])
				if err != nil || obj == nil {
					t.Fatal("member not recreated")
				}
				recreated := obj.(*corev1.Pod)
				pending, err := configsToUpdate(desired[member].Configs, recreated)
				if err != nil || len(pending) != 0 {
					t.Fatal("recreated member did not start with effective hashes")
				}
				if recreated.Spec.Volumes[0].ConfigMap.Name != desired[member].Spec.Volumes[0].ConfigMap.Name {
					t.Fatal("recreated member has stale mount")
				}
			}
		})
	}
}

func TestDirectConfigOverridePolicyRestrictions(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "strict update", true: "strict upgrade"}[upgrade], func(t *testing.T) {
			its, tree, pods, member := newDirectConfigFixture(t, "named")
			spy := installDirectConfigSpy(t)
			its.Spec.PodUpdatePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
			if upgrade {
				its.Spec.PodUpdatePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
				its.Spec.PodUpgradePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
			}
			mutateDirectTemplate(t, its, tree, func(tpl *workloads.InstanceTemplate) {
				tpl.Configs[0].Volume.ConfigMapName = "new"
				if upgrade {
					tpl.Images = map[string]string{"db": "mysql:new"}
				}
			})
			runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
			runDirectReconciler(t, NewUpdateReconciler(), tree)
			obj, err := tree.Get(pods[member])
			if err != nil || obj == nil {
				t.Fatal("strict policy allowed recreation")
			}
			if !meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceUpdateRestricted)) {
				t.Fatal("strict policy block was not reported")
			}
			if len(spy.configs) != 0 || spy.switchoverCalls != 0 {
				t.Fatal("strict policy called lifecycle actions")
			}
		})
	}
}

func TestDirectConfigOverrideMemberPlan(t *testing.T) {
	its, tree, pods, member := newDirectConfigFixture(t, "named")
	spy := installDirectConfigSpy(t)
	its.Spec.Roles = []workloads.ReplicaRole{{Name: "follower", UpdatePriority: 1}, {Name: "leader", UpdatePriority: 2}}
	its.Spec.MemberUpdateStrategy = ptr.To(workloads.SerialUpdateStrategy)
	pods[member].Labels[RoleLabelKey] = "follower"
	pods["demo-0"].Labels[RoleLabelKey] = "leader"
	its.Spec.Configs[0].ConfigHash = ptr.To("global-new")
	runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
	runDirectReconciler(t, NewUpdateReconciler(), tree)
	if !reflect.DeepEqual(spy.configs, []string{"global-main"}) {
		t.Fatalf("member plan used global hashes for overridden member: %v", spy.configs)
	}
}

func newDirectConfigFixture(t *testing.T, layout string) (*workloads.InstanceSet, *kubebuilderx.ObjectTree, map[string]*corev1.Pod, string) {
	t.Helper()
	originalResizeSupport := intctrlutil.SupportResizeSubResource
	intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
	t.Cleanup(func() { intctrlutil.SupportResizeSubResource = originalResizeSupport })
	its := legacyInstanceStatusSet(2)
	its.Spec.FlatInstanceOrdinal = layout == "compressed flat"
	its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
	its.Spec.PodUpdatePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
	its.Spec.PodUpgradePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
	action := &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"reload"}}}
	its.Spec.LifecycleActions = &workloads.LifecycleActions{Switchover: action}
	its.Spec.Configs = []workloads.ConfigTemplate{
		{Name: "main", ConfigHash: ptr.To("global"), Reconfigure: action, Parameters: map[string]string{"id": "global-main"}},
		{Name: "aux", ConfigHash: ptr.To("global-aux"), Reconfigure: action, Parameters: map[string]string{"id": "global-aux"}},
	}
	its.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "main"}}}},
		{Name: "aux", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "aux"}}}},
	}
	tpl := workloads.InstanceTemplate{Name: "a", Replicas: ptr.To[int32](1), Configs: []workloads.ConfigTemplate{
		{Name: "main", ConfigHash: ptr.To("main-old"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "main-a"}, Reconfigure: action, Parameters: map[string]string{"id": "main"}},
		{Name: "aux", ConfigHash: ptr.To("aux-old"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "aux", ConfigMapName: "aux-a"}, Reconfigure: action, Parameters: map[string]string{"id": "aux"}},
	}}
	member := "demo-a-0"
	if its.Spec.FlatInstanceOrdinal {
		tpl.Ordinals = workloads.Ordinals{Discrete: []int32{7}}
		its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{0}}
		member = "demo-7"
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if layout == "named" {
		its.Spec.Instances = []workloads.InstanceTemplate{tpl}
	} else {
		its.Spec.Instances = nil
		setDirectCompressedTemplate(t, its, tree, tpl)
	}
	runDirectReconciler(t, NewValidationReconciler(), tree)
	runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
	runDirectReconciler(t, NewReplicasAlignmentReconciler(), tree)
	pods := map[string]*corev1.Pod{}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pod := obj.(*corev1.Pod)
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		pods[pod.Name] = pod
	}
	if len(pods) != 2 || pods[member] == nil || pods["demo-0"] == nil {
		t.Fatalf("unexpected members: %v", pods)
	}
	runDirectReconciler(t, NewStatusReconciler(), tree)
	return its, tree, pods, member
}

func runDirectReconciler(t *testing.T, r kubebuilderx.Reconciler, tree *kubebuilderx.ObjectTree) {
	t.Helper()
	if _, err := r.Reconcile(tree); err != nil {
		t.Fatal(err)
	}
}

func setDirectCompressedTemplate(t *testing.T, its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree, tpl workloads.InstanceTemplate) {
	t.Helper()
	data, err := json.Marshal([]workloads.InstanceTemplate{tpl})
	if err != nil {
		t.Fatal(err)
	}
	writer, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "templates", Namespace: its.Namespace}, BinaryData: map[string][]byte{"instances": writer.EncodeAll(data, nil)}}
	its.Annotations = map[string]string{templateRefAnnotationKey: `{"demo":"templates"}`}
	if err := tree.Add(cm); err != nil {
		t.Fatal(err)
	}
}

func mutateDirectTemplate(t *testing.T, its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree, mutate func(*workloads.InstanceTemplate)) {
	t.Helper()
	if len(its.Spec.Instances) > 0 {
		mutate(&its.Spec.Instances[0])
		return
	}
	cmObj, err := tree.Get(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "templates", Namespace: its.Namespace}})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := reader.DecodeAll(cmObj.(*corev1.ConfigMap).BinaryData["instances"], nil)
	if err != nil {
		t.Fatal(err)
	}
	var templates []workloads.InstanceTemplate
	if err := json.Unmarshal(data, &templates); err != nil {
		t.Fatal(err)
	}
	mutate(&templates[0])
	setDirectCompressedTemplate(t, its, tree, templates[0])
}

type directConfigSpy struct {
	lifecycleCallSpy
	configs []string
}

func (s *directConfigSpy) Reconfigure(_ context.Context, _ client.Reader, _ *lifecycle.Options, params map[string]string) error {
	s.configs = append(s.configs, params["id"])
	return nil
}
func installDirectConfigSpy(t *testing.T) *directConfigSpy {
	t.Helper()
	spy := &directConfigSpy{}
	original := newLifecycleAction
	newLifecycleAction = func(_ *workloads.InstanceSet, _ *kubebuilderx.ObjectTree, _ *corev1.Pod) (lifecycle.Lifecycle, error) {
		return spy, nil
	}
	t.Cleanup(func() { newLifecycleAction = original })
	return spy
}

func TestDirectConfigOverrideRetainedAndInjectedPods(t *testing.T) {
	its, tree, pods, member := newDirectConfigFixture(t, "named")
	desired, err := desiredInstanceTemplates(its, tree)
	if err != nil {
		t.Fatal(err)
	}
	pods[member].Spec.Volumes = append(pods[member].Spec.Volumes, corev1.Volume{Name: "admission-config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "injected"}}}})
	policy, _, _, err := getPodUpdatePolicy(its, pods[member], desired[member])
	if err != nil || policy != noOpsPolicy {
		t.Fatalf("admission ConfigMap volume triggered update: %v, %v", policy, err)
	}
	updated, err := isPodUpdated(its, pods[member], nil)
	if err != nil || !updated {
		t.Fatalf("retained member without desired template lost previous handling: %v, %v", updated, err)
	}
	its.Spec.Replicas = ptr.To[int32](0)
	its.Status.UpdateRevisions = nil
	policy, _, reason, err := getPodUpdatePolicy(its, pods[member], nil)
	if err != nil || policy != recreatePolicy || reason != "0 replica" {
		t.Fatalf("zero-replica handling changed: %v, %s, %v", policy, reason, err)
	}
}

func TestDirectConfigOverridesValidateInactiveTemplate(t *testing.T) {
	its, tree, _, _ := newDirectConfigFixture(t, "named")
	its.Spec.Instances[0].Replicas = ptr.To[int32](0)
	its.Spec.Instances[0].Configs[0].Volume.VolumeName = "missing"
	if _, err := NewValidationReconciler().Reconcile(tree); err == nil {
		t.Fatal("inactive template's invalid binding was accepted")
	}
}

func TestDirectConfigOverridesRejectPVCVolumeCollision(t *testing.T) {
	for _, inactive := range []bool{false, true} {
		t.Run(map[bool]string{false: "active inherited claim", true: "inactive template claim"}[inactive], func(t *testing.T) {
			its, tree, _, _ := newDirectConfigFixture(t, "named")
			claim := corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
				},
			}
			if inactive {
				its.Spec.Instances[0].Replicas = ptr.To[int32](0)
				its.Spec.Instances[0].VolumeClaimTemplates = []corev1.PersistentVolumeClaim{claim}
			} else {
				its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{claim}
			}
			if _, err := NewValidationReconciler().Reconcile(tree); err == nil {
				t.Fatal("binding was accepted despite PVC replacing the ConfigMap volume")
			}
			// The existing same-name volume/PVC behavior remains valid without a binding.
			its.Spec.Instances[0].Configs = nil
			runDirectReconciler(t, NewValidationReconciler(), tree)
		})
	}
}
