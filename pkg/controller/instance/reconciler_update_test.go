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

package instance

import (
	"context"
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestInstanceConfigUpdateRouting(t *testing.T) {
	for _, tc := range []struct {
		name                                        string
		changeSource, addMount, restart, injectedCM bool
		reloadErr                                   error
	}{
		{name: "source switch", changeSource: true},
		{name: "added mount", addMount: true},
		{name: "same mount reload"},
		{name: "same mount reload before restart", restart: true},
		{name: "failed reload retains hash", reloadErr: errors.New("reload failed")},
		{name: "injected old-only ConfigMap", injectedCM: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, pod, tree, spy := newInstanceConfigUpdateFixture(t)
			inst.Spec.Configs[0].ConfigHash = ptr.To("desired-hash")
			inst.Spec.Configs[0].Restart = ptr.To(tc.restart)
			spy.reloadErr = tc.reloadErr
			if tc.changeSource {
				inst.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "desired-config"
				inst.Spec.Configs[0].Volume.ConfigMapName = "desired-config"
			}
			if tc.addMount {
				pod.Spec.Volumes = nil
				pod.Spec.Containers[0].VolumeMounts = nil
			}
			mountChanged := tc.changeSource || tc.addMount
			if tc.injectedCM {
				pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{Name: "injected", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "injected-config"}}}})
			}
			origSupportResize := intctrlutil.SupportResizeSubResource
			intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
			t.Cleanup(func() { intctrlutil.SupportResizeSubResource = origSupportResize })
			_, err := NewUpdateReconciler().Reconcile(tree)
			if !errors.Is(err, tc.reloadErr) {
				t.Fatalf("Reconcile() error = %v, want %v", err, tc.reloadErr)
			}
			var wantCalls []string
			if !mountChanged {
				wantCalls = []string{"main"}
			}
			if !reflect.DeepEqual(spy.configs, wantCalls) {
				t.Fatalf("reloads = %v, want %v", spy.configs, wantCalls)
			}
			pods := tree.List(&corev1.Pod{})
			if mountChanged || tc.restart {
				if len(pods) != 0 || spy.switchoverCalls != 1 {
					t.Fatalf("expected recreation, Pods=%d, switchover=%d", len(pods), spy.switchoverCalls)
				}
				if _, err := NewAlignmentReconciler().Reconcile(tree); err != nil {
					t.Fatalf("alignment Reconcile() error = %v", err)
				}
				pods = tree.List(&corev1.Pod{})
				if len(pods) != 1 {
					t.Fatalf("expected replacement Pod, got %d", len(pods))
				}
				if got := pods[0].(*corev1.Pod).Spec.Volumes[0].ConfigMap.Name; got != inst.Spec.Template.Spec.Volumes[0].ConfigMap.Name {
					t.Fatalf("replacement mounts %q, want desired ConfigMap", got)
				}
			} else if len(pods) != 1 || spy.switchoverCalls != 0 || pods[0].GetUID() != pod.UID {
				t.Fatalf("expected same Pod without switchover, Pods=%v, switchover=%d", pods, spy.switchoverCalls)
			}
			updated := pods[0].(*corev1.Pod)
			wantHash := "desired-hash"
			if tc.reloadErr != nil {
				wantHash = "old-hash"
			}
			configs, err := configsFromPod(updated)
			if err != nil || len(configs) != 1 || ptr.Deref(configs[0].ConfigHash, "") != wantHash {
				t.Fatalf("updated Pod configs = %#v, %v; want %q", configs, err, wantHash)
			}
			if tc.injectedCM && len(updated.Spec.Volumes) != 2 {
				t.Fatal("metadata update removed injected ConfigMap volume")
			}
		})
	}
}

func TestInstanceConfigMountUpdateRestrictions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*workloads.Instance, *corev1.Pod)
		blocked bool
	}{
		{name: "OnDelete", mutate: func(inst *workloads.Instance, _ *corev1.Pod) {
			inst.Spec.InstanceUpdateStrategyType = ptr.To(kbappsv1.OnDeleteStrategyType)
		}},
		{name: "not ready", mutate: func(_ *workloads.Instance, pod *corev1.Pod) { pod.Status.Conditions[0].Status = corev1.ConditionFalse }},
		{name: "strict update", blocked: true, mutate: func(inst *workloads.Instance, _ *corev1.Pod) {
			inst.Spec.PodUpdatePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
		}},
		{name: "strict upgrade", blocked: true, mutate: func(inst *workloads.Instance, _ *corev1.Pod) {
			inst.Spec.PodUpgradePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
			inst.Spec.Template.Spec.Containers[0].Image = "mysql:new"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, pod, tree, spy := newInstanceConfigUpdateFixture(t)
			inst.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "desired-config"
			inst.Spec.Configs[0].Volume.ConfigMapName = "desired-config"
			inst.Spec.Configs[0].ConfigHash = ptr.To("desired-hash")
			tc.mutate(inst, pod)
			if _, err := NewUpdateReconciler().Reconcile(tree); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}
			if len(tree.List(&corev1.Pod{})) != 1 || spy.switchoverCalls != 0 || len(spy.configs) != 0 {
				t.Fatalf("restriction allowed update, Pods=%d, switchover=%d, reloads=%v", len(tree.List(&corev1.Pod{})), spy.switchoverCalls, spy.configs)
			}
			if meta.IsStatusConditionTrue(inst.Status.Conditions, string(workloads.InstanceUpdateRestricted)) != tc.blocked {
				t.Fatalf("update restriction condition = %v, want blocked=%v", inst.Status.Conditions, tc.blocked)
			}
		})
	}
}

func TestInstanceMixedConfigUpdateRouting(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		switchedRestart, mountedRestart bool
		updatePolicy, upgradePolicy     workloads.PodUpdatePolicyType
	}{
		{name: "reload unchanged mount", updatePolicy: kbappsv1.PreferInPlacePodUpdatePolicyType, upgradePolicy: kbappsv1.PreferInPlacePodUpdatePolicyType},
		{name: "switched config cannot select strict restart policy", switchedRestart: true, updatePolicy: kbappsv1.StrictInPlacePodUpdatePolicyType, upgradePolicy: kbappsv1.PreferInPlacePodUpdatePolicyType},
		{name: "mounted config restart precedes strict upgrade policy", mountedRestart: true, updatePolicy: kbappsv1.PreferInPlacePodUpdatePolicyType, upgradePolicy: kbappsv1.StrictInPlacePodUpdatePolicyType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, pod, tree, spy := newInstanceConfigUpdateFixture(t)
			volume := corev1.Volume{Name: "aux", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "aux-config"}}}}
			mount := corev1.VolumeMount{Name: "aux", MountPath: "/etc/aux"}
			inst.Spec.Template.Spec.Volumes = append(inst.Spec.Template.Spec.Volumes, volume)
			inst.Spec.Template.Spec.Containers[0].VolumeMounts = append(inst.Spec.Template.Spec.Containers[0].VolumeMounts, mount)
			pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
			pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, mount)
			inst.Spec.Configs = append(inst.Spec.Configs, workloads.ConfigTemplate{
				Name: "aux", ConfigHash: ptr.To("old-aux"), Reconfigure: &kbappsv1.Action{}, Parameters: map[string]string{"id": "aux"},
				Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "aux", ConfigMapName: "aux-config"},
			})
			if err := configsToPod(inst.Spec.Configs, pod); err != nil {
				t.Fatal(err)
			}
			inst.Spec.Configs[0].ConfigHash = ptr.To("desired-main")
			inst.Spec.Configs[0].Restart = ptr.To(tc.switchedRestart)
			inst.Spec.Configs[0].Volume.ConfigMapName = "desired-config"
			inst.Spec.Configs[1].ConfigHash = ptr.To("desired-aux")
			inst.Spec.Configs[1].Restart = ptr.To(tc.mountedRestart)
			inst.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "desired-config"
			inst.Spec.Template.Spec.Containers[0].Image = "mysql:new"
			inst.Spec.PodUpdatePolicy = tc.updatePolicy
			inst.Spec.PodUpgradePolicy = tc.upgradePolicy
			if _, err := NewUpdateReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(spy.configs, []string{"aux"}) {
				t.Fatalf("reloads = %v, want only unchanged mount aux", spy.configs)
			}
			if len(tree.List(&corev1.Pod{})) != 0 || spy.switchoverCalls != 1 {
				t.Fatalf("expected recreation after aux action, Pods=%d, switchover=%d", len(tree.List(&corev1.Pod{})), spy.switchoverCalls)
			}
			if meta.IsStatusConditionTrue(inst.Status.Conditions, string(workloads.InstanceUpdateRestricted)) {
				t.Fatal("mixed update was blocked by the wrong policy")
			}
			if _, err := NewAlignmentReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			pods := tree.List(&corev1.Pod{})
			if len(pods) != 1 {
				t.Fatalf("expected replacement Pod, got %d", len(pods))
			}
			replacement := pods[0].(*corev1.Pod)
			if replacement.Spec.Volumes[0].ConfigMap.Name != "desired-config" || replacement.Spec.Volumes[1].ConfigMap.Name != "aux-config" {
				t.Fatalf("replacement mounts = %#v", replacement.Spec.Volumes)
			}
			if got := replacement.Annotations[constant.CMInsConfigurationHashLabelKey]; got != `{"aux":"desired-aux","main":"desired-main"}` {
				t.Fatalf("replacement hashes = %s, want both desired hashes", got)
			}
		})
	}
}

type instanceConfigUpdateSpy struct {
	lifecycle.Lifecycle
	configs         []string
	switchoverCalls int
	reloadErr       error
}

func (s *instanceConfigUpdateSpy) Reconfigure(_ context.Context, _ client.Reader, _ *lifecycle.Options, parameters map[string]string) error {
	s.configs = append(s.configs, parameters["id"])
	return s.reloadErr
}

func (s *instanceConfigUpdateSpy) Switchover(_ context.Context, _ client.Reader, _ *lifecycle.Options, _ string) error {
	s.switchoverCalls++
	return nil
}

func newInstanceConfigUpdateFixture(t *testing.T) (*workloads.Instance, *corev1.Pod, *kubebuilderx.ObjectTree, *instanceConfigUpdateSpy) {
	t.Helper()
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetPodTemplate(corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.4", VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/etc/mysql"}}}},
			Volumes:    []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "old-config"}}}}},
		}}).
		SetConfigs([]workloads.ConfigTemplate{{Name: "main", ConfigHash: ptr.To("old-hash"), Reconfigure: &kbappsv1.Action{}, Parameters: map[string]string{"id": "main"}, Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "old-config"}}}).
		SetPodUpdatePolicy(kbappsv1.PreferInPlacePodUpdatePolicyType).
		SetPodUpgradePolicy(kbappsv1.PreferInPlacePodUpdatePolicyType).
		GetObject()
	inst.Spec.LifecycleActions = &workloads.LifecycleActions{Switchover: &kbappsv1.Action{}}
	pod, err := buildInstancePod(inst, "revision")
	if err != nil {
		t.Fatal(err)
	}
	inst.Status.UpdateRevision = "revision"
	pod.UID = "old-pod"
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if err := tree.Add(pod); err != nil {
		t.Fatal(err)
	}
	spy := &instanceConfigUpdateSpy{}
	original := newLifecycleAction
	newLifecycleAction = func(_ *workloads.Instance, _ []*corev1.Pod, _ *corev1.Pod) (lifecycle.Lifecycle, error) {
		return spy, nil
	}
	t.Cleanup(func() { newLifecycleAction = original })
	return inst, pod, tree, spy
}
