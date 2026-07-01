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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestReconfigureOptions(t *testing.T) {
	if opts := reconfigureOptions(workloads.ConfigTemplate{}); opts != nil {
		t.Fatalf("expected nil options for empty reconfigure args, got %v", opts)
	}

	args := [][]string{{"maxmemory", "1gb"}, {"timeout", "30"}}
	opts := reconfigureOptions(workloads.ConfigTemplate{ReconfigureArgs: args})
	if opts == nil {
		t.Fatalf("expected options for non-empty reconfigure args")
		return
	}
	if !reflect.DeepEqual(opts.Arguments, args) {
		t.Fatalf("expected arguments %v, got %v", args, opts.Arguments)
	}
}

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

func TestConfigsToUpdateTreatsNilAndEmptyConfigHashAsEqual(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "valkey-0").
		SetConfigs([]workloads.ConfigTemplate{{
			Name: "valkey-replication-config",
		}}).
		GetObject()
	pod := builder.NewPodBuilder("default", "valkey-0").GetObject()
	if err := configsToPod([]workloads.ConfigTemplate{{
		Name:       "valkey-replication-config",
		ConfigHash: ptr.To(""),
	}}, pod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}

	toUpdate, err := configsToUpdate(inst, pod)
	if err != nil {
		t.Fatalf("configsToUpdate() error = %v", err)
	}
	if len(toUpdate) != 0 {
		t.Fatalf("expected no config drift, got %#v", toUpdate)
	}
}

func TestSafeMetadataOnlyInPlaceUpdate(t *testing.T) {
	basePod := builder.NewPodBuilder("default", "valkey-0").
		AddAnnotations("kept", "value").
		AddLabels("app", "valkey").
		SetContainers([]corev1.Container{{Name: "valkey", Image: "valkey:9"}}).
		GetObject()
	if err := configsToPod([]workloads.ConfigTemplate{{
		Name:       "valkey-replication-config",
		ConfigHash: ptr.To("old-hash"),
	}}, basePod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}

	positiveCases := []struct {
		name   string
		mutate func(*corev1.Pod)
	}{{
		name: "config-hash annotation patch",
		mutate: func(pod *corev1.Pod) {
			if err := configsToPod([]workloads.ConfigTemplate{{
				Name:       "valkey-replication-config",
				ConfigHash: ptr.To("new-hash"),
			}}, pod); err != nil {
				t.Fatalf("configsToPod() error = %v", err)
			}
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
}

func TestConfigsToUpdateStillReportsRealConfigHashMismatch(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "valkey-0").
		SetConfigs([]workloads.ConfigTemplate{{
			Name:       "valkey-replication-config",
			ConfigHash: ptr.To("desired-hash"),
		}}).
		GetObject()
	pod := builder.NewPodBuilder("default", "valkey-0").GetObject()
	if err := configsToPod([]workloads.ConfigTemplate{{
		Name:       "valkey-replication-config",
		ConfigHash: ptr.To(""),
	}}, pod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}

	toUpdate, err := configsToUpdate(inst, pod)
	if err != nil {
		t.Fatalf("configsToUpdate() error = %v", err)
	}
	if len(toUpdate) != 1 {
		t.Fatalf("expected one config drift item, got %#v", toUpdate)
	}
	if toUpdate[0].Name != "valkey-replication-config" {
		t.Fatalf("unexpected config name %q", toUpdate[0].Name)
	}
}

func TestInstanceObjectHelpers(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetRoles([]workloads.ReplicaRole{
			{Name: "Leader", UpdatePriority: 10},
			{Name: "Follower", UpdatePriority: 1},
		}).
		GetObject()

	if got := podName(inst); got != "mysql-0" {
		t.Fatalf("podName() = %q, want mysql-0", got)
	}
	pod := podObj(inst)
	if pod.Namespace != "default" || pod.Name != "mysql-0" {
		t.Fatalf("unexpected pod object key: %s/%s", pod.Namespace, pod.Name)
	}

	roleMap := composeRoleMap(inst)
	if roleMap["leader"].UpdatePriority != 10 || roleMap["follower"].UpdatePriority != 1 {
		t.Fatalf("unexpected role map: %#v", roleMap)
	}

	pod.Labels = map[string]string{constant.RoleLabelKey: "Leader"}
	if got := getRoleName(pod); got != "leader" {
		t.Fatalf("getRoleName() = %q, want leader", got)
	}
	if !isRoleReady(pod, inst.Spec.Roles) {
		t.Fatal("pod with role label should be role-ready")
	}
	delete(pod.Labels, constant.RoleLabelKey)
	if isRoleReady(pod, inst.Spec.Roles) {
		t.Fatal("pod without role label should not be role-ready when roles are configured")
	}
	if !isRoleReady(pod, nil) {
		t.Fatal("pod should be role-ready when roles are not configured")
	}
}

func TestPodStateHelpers(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name        string
		pod         *corev1.Pod
		created     bool
		terminating bool
		pending     bool
	}{
		{
			name:    "empty pod",
			pod:     &corev1.Pod{},
			created: false,
		},
		{
			name: "pending pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{Phase: corev1.PodPending},
			},
			created: true,
			pending: true,
		},
		{
			name: "terminating pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			},
			created:     true,
			terminating: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCreated(tt.pod); got != tt.created {
				t.Fatalf("isCreated() = %v, want %v", got, tt.created)
			}
			if got := isTerminating(tt.pod); got != tt.terminating {
				t.Fatalf("isTerminating() = %v, want %v", got, tt.terminating)
			}
			if got := isPodPending(tt.pod); got != tt.pending {
				t.Fatalf("isPodPending() = %v, want %v", got, tt.pending)
			}
		})
	}
}

func TestIsImageMatched(t *testing.T) {
	if !isImageMatched(&corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{Name: "sidecar", Image: "sidecar:1.0"}},
		},
	}) {
		t.Fatal("missing matching status should be ignored")
	}

	if !isImageMatched(&corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "mysql", Image: "docker.io/library/mysql:8.0"}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "mysql",
				Image: "mysql:8.0",
			}},
		},
	}) {
		t.Fatal("equivalent image references should match")
	}

	if isImageMatched(&corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "mysql",
				Image: "mysql:8.4",
			}},
		},
	}) {
		t.Fatal("different image references should not match")
	}
}

func TestBuildInstancePodAndPVCs(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("12345678-1234-1234-1234-1234567890ab")).
		AddAnnotations("instance-annotation", "true").
		SetPodTemplate(corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      map[string]string{"template-label": "true"},
				Annotations: map[string]string{"template-annotation": "true"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
				Volumes:    []corev1.Volume{{Name: "config"}},
			},
		}).
		SetInstanceSetName("mysql").
		SetInstanceTemplateName("az-a").
		SetConfigs([]workloads.ConfigTemplate{{Name: "mysql-conf", ConfigHash: ptr.To("hash")}}).
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
	if pod.Annotations["template-annotation"] != "true" ||
		pod.Annotations[constant.CMInsConfigurationHashLabelKey] == "" {
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
	if pvc.Labels[constant.KBAppPodNameLabelKey] != "mysql-0" ||
		pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] != "data" ||
		pvc.Labels[constant.KBAppInstanceTemplateLabelKey] != "az-a" ||
		pvc.Labels["pvc-label"] != "true" {
		t.Fatalf("unexpected pvc labels: %#v", pvc.Labels)
	}
	if pvc.Annotations["pvc-annotation"] != "true" {
		t.Fatalf("unexpected pvc annotations: %#v", pvc.Annotations)
	}
	if len(pvc.OwnerReferences) != 1 || pvc.OwnerReferences[0].Name != "mysql-0" {
		t.Fatalf("unexpected pvc owner references: %#v", pvc.OwnerReferences)
	}
}

func TestConfigsFromPod(t *testing.T) {
	pod := builder.NewPodBuilder("default", "mysql-0").GetObject()
	if got, err := configsFromPod(pod); err != nil || got != nil {
		t.Fatalf("empty configsFromPod() = %#v, %v; want nil, nil", got, err)
	}

	if err := configsToPod([]workloads.ConfigTemplate{
		{Name: "z-config", ConfigHash: ptr.To("z")},
		{Name: "a-config", ConfigHash: ptr.To("a")},
	}, pod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}
	got, err := configsFromPod(pod)
	if err != nil {
		t.Fatalf("configsFromPod() error = %v", err)
	}
	want := []workloads.ConfigTemplate{
		{Name: "a-config", ConfigHash: ptr.To("a")},
		{Name: "z-config", ConfigHash: ptr.To("z")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configsFromPod() = %#v, want %#v", got, want)
	}

	pod.Annotations[constant.CMInsConfigurationHashLabelKey] = "not-json"
	if _, err := configsFromPod(pod); err == nil {
		t.Fatal("expected invalid config hash annotation error")
	}
}

func TestHasConfigRestart(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetConfigs([]workloads.ConfigTemplate{
			{Name: "restart-conf", ConfigHash: ptr.To("new"), Restart: ptr.To(true)},
			{Name: "reload-conf", ConfigHash: ptr.To("new"), Restart: ptr.To(false)},
		}).
		GetObject()
	pod := builder.NewPodBuilder("default", "mysql-0").GetObject()
	if err := configsToPod([]workloads.ConfigTemplate{
		{Name: "restart-conf", ConfigHash: ptr.To("old")},
		{Name: "reload-conf", ConfigHash: ptr.To("old")},
	}, pod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}

	needRestart, names, err := hasConfigRestart(inst, pod)
	if err != nil {
		t.Fatalf("hasConfigRestart() error = %v", err)
	}
	if !needRestart || !reflect.DeepEqual(names, []string{"restart-conf"}) {
		t.Fatalf("hasConfigRestart() = %v, %v; want true, [restart-conf]", needRestart, names)
	}

	pod.Annotations[constant.CMInsConfigurationHashLabelKey] = "not-json"
	if _, _, err := hasConfigRestart(inst, pod); err == nil {
		t.Fatal("expected invalid config annotation error")
	}
}

func TestCopyAndMergeObjects(t *testing.T) {
	if got := copyAndMerge(&corev1.Service{}, &corev1.ConfigMap{}); got != nil {
		t.Fatalf("type mismatch should return nil, got %T", got)
	}

	oldCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "conf",
			Finalizers: []string{"old"},
			OwnerReferences: []metav1.OwnerReference{{
				UID:  types.UID("old"),
				Name: "old",
			}},
		},
		Data:       map[string]string{"old": "value"},
		BinaryData: map[string][]byte{"old": []byte("value")},
	}
	newCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "conf",
			Finalizers: []string{"new"},
			OwnerReferences: []metav1.OwnerReference{{
				UID:  types.UID("new"),
				Name: "new",
			}},
		},
		Data:       map[string]string{"new": "value"},
		BinaryData: map[string][]byte{"new": []byte("value")},
	}
	mergedCM := copyAndMerge(oldCM, newCM).(*corev1.ConfigMap)
	if !reflect.DeepEqual(mergedCM.Finalizers, []string{"old", "new"}) ||
		len(mergedCM.OwnerReferences) != 2 ||
		!reflect.DeepEqual(mergedCM.Data, newCM.Data) ||
		!reflect.DeepEqual(mergedCM.BinaryData, newCM.BinaryData) {
		t.Fatalf("unexpected merged configmap: %#v", mergedCM)
	}

	oldPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "data",
			Labels: map[string]string{"old": "kept"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
		},
	}
	newPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "data",
			Labels: map[string]string{"new": "added"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")},
			},
		},
	}
	mergedPVC := copyAndMerge(oldPVC, newPVC).(*corev1.PersistentVolumeClaim)
	if !reflect.DeepEqual(mergedPVC.Labels, map[string]string{"old": "kept", "new": "added"}) ||
		mergedPVC.Spec.Resources.Requests.Storage().String() != "2Gi" {
		t.Fatalf("unexpected merged pvc: %#v", mergedPVC)
	}

	newPod := builder.NewPodBuilder("default", "mysql-0").
		AddAnnotations("new", "added").
		AddLabels("new", "added").
		SetContainers([]corev1.Container{{
			Name:  "mysql",
			Image: "mysql:8.4",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		}}).
		GetObject()
	oldPod := builder.NewPodBuilder("default", "mysql-0").
		SetContainers([]corev1.Container{{Name: "mysql", Image: "mysql:8.0"}}).
		GetObject()
	mergedPod := copyAndMerge(oldPod, newPod).(*corev1.Pod)
	if mergedPod.Labels["new"] != "added" ||
		mergedPod.Annotations["new"] != "added" ||
		mergedPod.Spec.Containers[0].Image != "mysql:8.4" ||
		mergedPod.Spec.Containers[0].Resources.Requests.Cpu().String() != "1" {
		t.Fatalf("unexpected merged pod: %#v", mergedPod)
	}
}
