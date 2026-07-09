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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestGetPodUpdatePolicyInSpecForKBManagedToolsImage(t *testing.T) {
	oldToolsImage := viper.GetString(constant.KBToolsImage)
	defer viper.Set(constant.KBToolsImage, oldToolsImage)
	viper.Set(constant.KBToolsImage, "docker.io/apecloud/kubeblocks-tools:1.0.0")

	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inst-0",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init-kbagent", Image: "docker.io/apecloud/kubeblocks-tools:1.0.0", Command: []string{"cp"}},
			},
			Containers: []corev1.Container{
				{Name: "app", Image: "docker.io/apecloud/redis:7.2"},
				{Name: "kbagent", Image: "docker.io/apecloud/kubeblocks-tools:1.0.0", Command: []string{"/bin/kbagent"}},
			},
		},
	}
	newPod := oldPod.DeepCopy()
	newPod.Spec.InitContainers[0].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
	newPod.Spec.Containers[1].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"

	inst := builder.NewInstanceBuilder("default", "inst-0").
		SetPodUpdatePolicy(kbappsv1.ReCreatePodUpdatePolicyType).
		SetPodUpgradePolicy(kbappsv1.ReCreatePodUpdatePolicyType).
		GetObject()

	if policy := getPodUpdatePolicyInSpec(inst, oldPod, newPod); policy != kbappsv1.PreferInPlacePodUpdatePolicyType {
		t.Fatalf("expected PreferInPlace for KB-managed tools image change, got %s", policy)
	}
	strictInPlaceInst := builder.NewInstanceBuilder("default", "inst-0").
		SetPodUpdatePolicy(kbappsv1.ReCreatePodUpdatePolicyType).
		SetPodUpgradePolicy(kbappsv1.StrictInPlacePodUpdatePolicyType).
		GetObject()
	if policy := getPodUpdatePolicyInSpec(strictInPlaceInst, oldPod, newPod); policy != kbappsv1.StrictInPlacePodUpdatePolicyType {
		t.Fatalf("expected StrictInPlace for KB-managed tools image change with StrictInPlace policy, got %s", policy)
	}
	if !safeKBManagedImageOnlyInPlaceUpdate(oldPod, newPod) {
		t.Fatalf("expected KB-managed tools image-only change to skip switchover")
	}

	labelChangedPod := newPod.DeepCopy()
	labelChangedPod.Labels = map[string]string{"extra": "true"}
	if policy := getPodUpdatePolicyInSpec(inst, oldPod, labelChangedPod); policy != kbappsv1.ReCreatePodUpdatePolicyType {
		t.Fatalf("expected ReCreate for KB-managed tools image plus label change, got %s", policy)
	}
	if safeKBManagedImageOnlyInPlaceUpdate(oldPod, labelChangedPod) {
		t.Fatalf("expected KB-managed tools image plus label change not to skip switchover")
	}

	appChangedPod := oldPod.DeepCopy()
	appChangedPod.Spec.Containers[0].Image = "docker.io/apecloud/redis:7.4"
	if policy := getPodUpdatePolicyInSpec(inst, oldPod, appChangedPod); policy != kbappsv1.ReCreatePodUpdatePolicyType {
		t.Fatalf("expected ReCreate for app image change, got %s", policy)
	}
}

func TestSafeMetadataOnlyInPlaceUpdate(t *testing.T) {
	basePod := builder.NewPodBuilder("default", "valkey-0").
		AddAnnotations("kept", "value", constant.CMInsConfigurationHashLabelKey, "old-hash").
		AddLabels("app", "valkey").
		SetContainers([]corev1.Container{{Name: "valkey", Image: "valkey:9"}}).
		GetObject()

	positiveCases := []struct {
		name   string
		mutate func(*corev1.Pod)
	}{{
		name: "config-hash annotation patch",
		mutate: func(pod *corev1.Pod) {
			pod.Annotations[constant.CMInsConfigurationHashLabelKey] = "new-hash"
		},
	}, {
		name: "non-restart annotation added",
		mutate: func(pod *corev1.Pod) {
			pod.Annotations["custom"] = "value"
		},
	}, {
		name: "non-restart annotation value changed",
		mutate: func(pod *corev1.Pod) {
			pod.Annotations["kept"] = "changed"
		},
	}, {
		name: "label added",
		mutate: func(pod *corev1.Pod) {
			pod.Labels["extra"] = "value"
		},
	}, {
		name: "label value changed",
		mutate: func(pod *corev1.Pod) {
			pod.Labels["app"] = "valkey-renamed"
		},
	}, {
		name: "role label state synchronization",
		mutate: func(pod *corev1.Pod) {
			pod.Labels[constant.RoleLabelKey] = "primary"
		},
	}}
	for _, tc := range positiveCases {
		t.Run("skip switchover when "+tc.name, func(t *testing.T) {
			newPod := basePod.DeepCopy()
			tc.mutate(newPod)
			if !safeMetadataOnlyInPlaceUpdate(basePod, newPod) {
				t.Fatalf("expected %s to be a safe metadata-only update", tc.name)
			}
		})
	}

	negativeCases := []struct {
		name   string
		mutate func(*corev1.Pod)
	}{{
		name:   "no diff",
		mutate: func(pod *corev1.Pod) {},
	}, {
		name: "restart annotation added",
		mutate: func(pod *corev1.Pod) {
			pod.Annotations[constant.RestartAnnotationKey] = "2026-05-19T14:00:00Z"
		},
	}, {
		name: "restart annotation value changed",
		mutate: func(pod *corev1.Pod) {
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations[constant.RestartAnnotationKey] = "next"
		},
	}, {
		name: "upgrade-restart prefixed annotation added (config.kubeblocks.io/restart-mysql-config)",
		mutate: func(pod *corev1.Pod) {
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations[constant.UpgradeRestartAnnotationKey+"-mysql-config"] = "hash-1"
		},
	}, {
		name: "exact UpgradeRestartAnnotationKey annotation added (no suffix)",
		mutate: func(pod *corev1.Pod) {
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations[constant.UpgradeRestartAnnotationKey] = "trigger"
		},
	}, {
		name: "container image changed",
		mutate: func(pod *corev1.Pod) {
			pod.Spec.Containers[0].Image = "valkey:10"
		},
	}, {
		name: "container resources changed",
		mutate: func(pod *corev1.Pod) {
			pod.Spec.Containers[0].Resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			}
		},
	}, {
		name: "container env added",
		mutate: func(pod *corev1.Pod) {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{Name: "EXTRA", Value: "v"})
		},
	}}
	for _, tc := range negativeCases {
		t.Run("invoke switchover when "+tc.name, func(t *testing.T) {
			newPod := basePod.DeepCopy()
			tc.mutate(newPod)
			if safeMetadataOnlyInPlaceUpdate(basePod, newPod) {
				t.Fatalf("expected %s not to be a safe metadata-only update", tc.name)
			}
		})
	}

	t.Run("invoke switchover when an existing upgrade-restart prefixed annotation value changes", func(t *testing.T) {
		podWithUpgradeRestart := basePod.DeepCopy()
		podWithUpgradeRestart.Annotations[constant.UpgradeRestartAnnotationKey+"-mysql-config"] = "hash-1"
		mutated := podWithUpgradeRestart.DeepCopy()
		mutated.Annotations[constant.UpgradeRestartAnnotationKey+"-mysql-config"] = "hash-2"
		if safeMetadataOnlyInPlaceUpdate(podWithUpgradeRestart, mutated) {
			t.Fatalf("expected upgrade-restart prefixed annotation value change to invoke switchover")
		}
	})

	t.Run("invoke switchover when an existing upgrade-restart prefixed annotation is removed", func(t *testing.T) {
		podWithUpgradeRestart := basePod.DeepCopy()
		podWithUpgradeRestart.Annotations[constant.UpgradeRestartAnnotationKey+"-mysql-config"] = "hash-1"
		removed := podWithUpgradeRestart.DeepCopy()
		delete(removed.Annotations, constant.UpgradeRestartAnnotationKey+"-mysql-config")
		if safeMetadataOnlyInPlaceUpdate(podWithUpgradeRestart, removed) {
			t.Fatalf("expected upgrade-restart prefixed annotation removal to invoke switchover")
		}
	})

	t.Run("skip switchover when a non-restart config.kubeblocks.io annotation changes (prefix does not match)", func(t *testing.T) {
		base := basePod.DeepCopy()
		base.Annotations["config.kubeblocks.io/non-restart-key"] = "value-1"
		mutated := base.DeepCopy()
		mutated.Annotations["config.kubeblocks.io/non-restart-key"] = "value-2"
		if !safeMetadataOnlyInPlaceUpdate(base, mutated) {
			t.Fatalf("expected non-restart config.kubeblocks.io/* annotation change to be a safe metadata-only update")
		}
	})
}

func TestBuildInstancePodAndPVCs(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("12345678-1234-1234-1234-1234567890ab")).
		AddAnnotations("instance-annotation", "true").
		AddLabels("instance-only-label", "true").
		SetPodTemplate(corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"template-label":                   "true",
					constant.AppInstanceLabelKey:       "cluster",
					constant.KBAppComponentLabelKey:    "mysql",
					constant.KBAppInstanceNameLabelKey: "template-instance",
				},
				Annotations: map[string]string{"template-annotation": "true"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
				Volumes:    []corev1.Volume{{Name: "config"}},
			},
		}).
		SetInstanceSetName("mysql").
		SetInstanceTemplateName("az-a").
		AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "data",
				Labels:      map[string]string{"pvc-label": "true"},
				Annotations: map[string]string{"pvc-annotation": "true"},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}).
		GetObject()

	pod, err := buildInstancePod(inst, "revision")
	if err != nil {
		t.Fatalf("buildInstancePod() error = %v", err)
	}
	if pod.Name != "mysql-0" || pod.Namespace != "default" {
		t.Fatalf("unexpected pod key: %s/%s", pod.Namespace, pod.Name)
	}
	if pod.Labels[constant.KBAppInstanceNameLabelKey] != "mysql-0" ||
		pod.Labels[constant.KBAppPodNameLabelKey] != "mysql-0" ||
		pod.Labels[constant.KBAppInstanceTemplateLabelKey] != "az-a" ||
		pod.Labels["template-label"] != "true" {
		t.Fatalf("unexpected pod labels: %#v", pod.Labels)
	}
	if pod.Annotations["template-annotation"] != "true" {
		t.Fatalf("unexpected pod annotations: %#v", pod.Annotations)
	}
	if len(pod.Spec.Volumes) != 2 {
		t.Fatalf("pod volumes = %d, want 2: %#v", len(pod.Spec.Volumes), pod.Spec.Volumes)
	}
	if len(pod.OwnerReferences) != 1 || pod.OwnerReferences[0].Name != "mysql-0" {
		t.Fatalf("unexpected pod owner references: %#v", pod.OwnerReferences)
	}

	pvcs, err := buildInstancePVCs(inst)
	if err != nil {
		t.Fatalf("buildInstancePVCs() error = %v", err)
	}
	if len(pvcs) != 1 {
		t.Fatalf("pvcs = %d, want 1", len(pvcs))
	}
	pvc := pvcs[0]
	if pvc.Labels[constant.KBAppInstanceNameLabelKey] != "mysql-0" ||
		pvc.Labels[constant.KBAppPodNameLabelKey] != "mysql-0" ||
		pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] != "data" ||
		pvc.Labels[constant.KBAppInstanceTemplateLabelKey] != "az-a" ||
		pvc.Labels[constant.AppInstanceLabelKey] != "cluster" ||
		pvc.Labels[constant.KBAppComponentLabelKey] != "mysql" ||
		pvc.Labels["template-label"] != "true" ||
		pvc.Labels["pvc-label"] != "true" {
		t.Fatalf("unexpected pvc labels: %#v", pvc.Labels)
	}
	if _, ok := pvc.Labels["instance-only-label"]; ok {
		t.Fatalf("unexpected instance-only label on pvc: %#v", pvc.Labels)
	}
	if pvc.Annotations["pvc-annotation"] != "true" {
		t.Fatalf("unexpected pvc annotations: %#v", pvc.Annotations)
	}
	if len(pvc.OwnerReferences) != 1 || pvc.OwnerReferences[0].Name != "mysql-0" {
		t.Fatalf("unexpected pvc owner references: %#v", pvc.OwnerReferences)
	}
}
