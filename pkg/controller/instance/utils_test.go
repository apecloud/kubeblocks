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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

func TestPodName(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "my-inst-0"},
	}
	assert.Equal(t, "my-inst-0", podName(inst))
}

func TestPodObj(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "my-inst-0", Namespace: "ns1"},
	}
	pod := podObj(inst)
	assert.Equal(t, "my-inst-0", pod.Name)
	assert.Equal(t, "ns1", pod.Namespace)
}

func TestGetRoleName(t *testing.T) {
	t.Run("no labels", func(t *testing.T) {
		pod := &corev1.Pod{}
		assert.Equal(t, "", getRoleName(pod))
	})

	t.Run("with role label", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{constant.RoleLabelKey: "Leader"},
			},
		}
		assert.Equal(t, "leader", getRoleName(pod))
	})
}

func TestComposeRoleMap(t *testing.T) {
	inst := &workloads.Instance{
		Spec: workloads.InstanceSpec{
			Roles: []workloads.ReplicaRole{
				{Name: "Leader", UpdatePriority: 3},
				{Name: "Follower", UpdatePriority: 2},
			},
		},
	}
	m := composeRoleMap(inst)
	assert.Equal(t, "Leader", m["leader"].Name)
	assert.Equal(t, "Follower", m["follower"].Name)
	assert.Len(t, m, 2)
}

func TestMergeMap(t *testing.T) {
	t.Run("merge into existing", func(t *testing.T) {
		dst := map[string]string{"a": "1", "b": "2"}
		src := map[string]string{"b": "3", "c": "4"}
		mergeMap(&src, &dst)
		assert.Equal(t, "1", dst["a"])
		assert.Equal(t, "3", dst["b"])
		assert.Equal(t, "4", dst["c"])
	})

	t.Run("merge into nil", func(t *testing.T) {
		var dst map[string]string
		src := map[string]string{"a": "1"}
		mergeMap(&src, &dst)
		assert.Equal(t, "1", dst["a"])
	})

	t.Run("empty src is noop", func(t *testing.T) {
		dst := map[string]string{"a": "1"}
		src := map[string]string{}
		mergeMap(&src, &dst)
		assert.Equal(t, map[string]string{"a": "1"}, dst)
	})
}

func TestGetMatchLabels(t *testing.T) {
	labels := getMatchLabels("my-inst")
	assert.Equal(t, constant.AppName, labels[constant.AppManagedByLabelKey])
	assert.Equal(t, "my-inst", labels[constant.KBAppInstanceNameLabelKey])
}

func TestIsRoleReady(t *testing.T) {
	t.Run("no roles always ready", func(t *testing.T) {
		pod := &corev1.Pod{}
		assert.True(t, isRoleReady(pod, nil))
	})

	t.Run("has roles, no label", func(t *testing.T) {
		pod := &corev1.Pod{}
		roles := []workloads.ReplicaRole{{Name: "Leader"}}
		assert.False(t, isRoleReady(pod, roles))
	})

	t.Run("has roles, has label", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{constant.RoleLabelKey: "leader"},
			},
		}
		roles := []workloads.ReplicaRole{{Name: "Leader"}}
		assert.True(t, isRoleReady(pod, roles))
	})
}

func TestIsCreated(t *testing.T) {
	assert.False(t, isCreated(&corev1.Pod{}))
	assert.True(t, isCreated(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}))
}

func TestIsTerminating(t *testing.T) {
	assert.False(t, isTerminating(&corev1.Pod{}))
	now := metav1.Now()
	assert.True(t, isTerminating(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}}))
}

func TestIsPodPending(t *testing.T) {
	assert.False(t, isPodPending(&corev1.Pod{}))
	assert.True(t, isPodPending(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}))
	assert.False(t, isPodPending(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}))
}

func TestImageSplit(t *testing.T) {
	tests := []struct {
		image      string
		wantName   string
		wantTag    string
		wantDigest string
	}{
		{"nginx", "nginx", "", ""},
		{"nginx:1.19", "nginx", "1.19", ""},
		{"nginx@sha256:abc123", "nginx", "", "sha256:abc123"},
		{"nginx:1.19@sha256:abc123", "nginx", "1.19", "sha256:abc123"},
		{"registry.example.com/nginx:1.19", "registry.example.com/nginx", "1.19", ""},
		{"registry.example.com:5000/nginx:1.19", "registry.example.com:5000/nginx", "1.19", ""},
		{"registry.example.com/org/nginx", "registry.example.com/org/nginx", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			name, tag, digest := imageSplit(tt.image)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantTag, tag)
			assert.Equal(t, tt.wantDigest, digest)
		})
	}
}

func TestIsImageMatched(t *testing.T) {
	t.Run("matching images", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "main", Image: "nginx:1.19"},
				},
			},
		}
		assert.True(t, isImageMatched(pod))
	})

	t.Run("mismatching tag", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "main", Image: "nginx:1.18"},
				},
			},
		}
		assert.False(t, isImageMatched(pod))
	})

	t.Run("no status for container", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
			},
		}
		assert.True(t, isImageMatched(pod))
	})

	t.Run("short name matches full registry path", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "main", Image: "docker.io/library/nginx:1.19"},
				},
			},
		}
		assert.True(t, isImageMatched(pod))
	})

	t.Run("digest mismatch", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main", Image: "nginx@sha256:aaa"}},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "main", Image: "nginx@sha256:bbb"},
				},
			},
		}
		assert.False(t, isImageMatched(pod))
	})
}

func TestConfigsToPod(t *testing.T) {
	t.Run("empty configs", func(t *testing.T) {
		pod := &corev1.Pod{}
		assert.NoError(t, configsToPod(nil, pod))
		assert.Nil(t, pod.Annotations)
	})

	t.Run("with configs", func(t *testing.T) {
		pod := &corev1.Pod{}
		configs := []workloads.ConfigTemplate{
			{Name: "cfg1", ConfigHash: ptr.To("hash1")},
		}
		require.NoError(t, configsToPod(configs, pod))
		assert.Contains(t, pod.Annotations[constant.CMInsConfigurationHashLabelKey], "cfg1")
		assert.Contains(t, pod.Annotations[constant.CMInsConfigurationHashLabelKey], "hash1")
	})
}

func TestConfigsFromPod(t *testing.T) {
	t.Run("no annotation", func(t *testing.T) {
		pod := &corev1.Pod{}
		configs, err := configsFromPod(pod)
		assert.NoError(t, err)
		assert.Nil(t, configs)
	})

	t.Run("with annotation", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constant.CMInsConfigurationHashLabelKey: `{"cfg1":"hash1","cfg2":"hash2"}`,
				},
			},
		}
		configs, err := configsFromPod(pod)
		require.NoError(t, err)
		assert.Len(t, configs, 2)
		// sorted by name
		assert.Equal(t, "cfg1", configs[0].Name)
		assert.Equal(t, "cfg2", configs[1].Name)
	})

	t.Run("invalid json", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constant.CMInsConfigurationHashLabelKey: `{invalid}`,
				},
			},
		}
		_, err := configsFromPod(pod)
		assert.Error(t, err)
	})
}

func TestHasConfigRestart(t *testing.T) {
	t.Run("no config drift", func(t *testing.T) {
		inst := builder.NewInstanceBuilder("default", "inst-0").
			SetConfigs([]workloads.ConfigTemplate{{
				Name:       "cfg1",
				ConfigHash: ptr.To("hash1"),
			}}).
			GetObject()
		pod := builder.NewPodBuilder("default", "inst-0").GetObject()
		require.NoError(t, configsToPod([]workloads.ConfigTemplate{{
			Name:       "cfg1",
			ConfigHash: ptr.To("hash1"),
		}}, pod))

		restart, names, err := hasConfigRestart(inst, pod)
		require.NoError(t, err)
		assert.False(t, restart)
		assert.Empty(t, names)
	})

	t.Run("config drift with restart", func(t *testing.T) {
		inst := builder.NewInstanceBuilder("default", "inst-0").
			SetConfigs([]workloads.ConfigTemplate{{
				Name:       "cfg1",
				ConfigHash: ptr.To("new-hash"),
				Restart:    ptr.To(true),
			}}).
			GetObject()
		pod := builder.NewPodBuilder("default", "inst-0").GetObject()
		require.NoError(t, configsToPod([]workloads.ConfigTemplate{{
			Name:       "cfg1",
			ConfigHash: ptr.To("old-hash"),
		}}, pod))

		restart, names, err := hasConfigRestart(inst, pod)
		require.NoError(t, err)
		assert.True(t, restart)
		assert.Contains(t, names, "cfg1")
	})
}

func TestCopyAndMerge(t *testing.T) {
	t.Run("service merge", func(t *testing.T) {
		oldSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "svc1", Namespace: "ns",
				Labels:      map[string]string{"old": "label"},
				Annotations: map[string]string{"old": "ann"},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
			},
		}
		newSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "svc1", Namespace: "ns",
				Labels:      map[string]string{"new": "label"},
				Annotations: map[string]string{"new": "ann"},
			},
			Spec: corev1.ServiceSpec{
				Ports:    []corev1.ServicePort{{Name: "http", Port: 8080}},
				Selector: map[string]string{"app": "test"},
			},
		}
		result := copyAndMerge(oldSvc, newSvc).(*corev1.Service)
		assert.Equal(t, "label", result.Labels["new"])
		assert.Equal(t, "label", result.Labels["old"])
		assert.Equal(t, int32(8080), result.Spec.Ports[0].Port)
		assert.Equal(t, "test", result.Spec.Selector["app"])
	})

	t.Run("configmap merge", func(t *testing.T) {
		oldCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "cm1"},
			Data:       map[string]string{"old": "data"},
		}
		newCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "cm1"},
			Data:       map[string]string{"new": "data"},
		}
		result := copyAndMerge(oldCm, newCm).(*corev1.ConfigMap)
		assert.Equal(t, "data", result.Data["new"])
		assert.NotContains(t, result.Data, "old")
	})

	t.Run("pvc no expansion needed", func(t *testing.T) {
		oldPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
				},
			},
		}
		newPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")},
				},
			},
		}
		result := copyAndMerge(oldPVC, newPVC).(*corev1.PersistentVolumeClaim)
		// no shrink — keeps old 10Gi
		assert.True(t, result.Spec.Resources.Requests.Storage().Equal(resource.MustParse("10Gi")))
	})

	t.Run("pvc with expansion", func(t *testing.T) {
		oldPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")},
				},
			},
		}
		newPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
				},
			},
		}
		result := copyAndMerge(oldPVC, newPVC).(*corev1.PersistentVolumeClaim)
		assert.True(t, result.Spec.Resources.Requests.Storage().Equal(resource.MustParse("10Gi")))
	})

	t.Run("type mismatch returns nil", func(t *testing.T) {
		result := copyAndMerge(&corev1.Pod{}, &corev1.Service{})
		assert.Nil(t, result)
	})

	t.Run("unknown type returns new", func(t *testing.T) {
		oldSA := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "old"}}
		newSA := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "new"}}
		result := copyAndMerge(oldSA, newSA)
		assert.Equal(t, "new", result.GetName())
	})
}

func TestCopyAndMerge_Pod(t *testing.T) {
	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pod1",
			Labels:      map[string]string{"old": "label"},
			Annotations: map[string]string{"old": "ann"},
		},
		Spec: corev1.PodSpec{
			Containers:     []corev1.Container{{Name: "main", Image: "nginx:old"}},
			InitContainers: []corev1.Container{{Name: "init", Image: "busybox:old"}},
		},
	}
	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pod1",
			Labels:      map[string]string{"new": "label"},
			Annotations: map[string]string{"new": "ann"},
		},
		Spec: corev1.PodSpec{
			Containers:     []corev1.Container{{Name: "main", Image: "nginx:new"}},
			InitContainers: []corev1.Container{{Name: "init", Image: "busybox:new"}},
		},
	}
	result := copyAndMerge(oldPod, newPod).(*corev1.Pod)
	// mergeInPlaceFields updates images and merges labels/annotations
	assert.Equal(t, "nginx:new", result.Spec.Containers[0].Image)
	assert.Equal(t, "busybox:new", result.Spec.InitContainers[0].Image)
}

func TestCopyAndMerge_PVC_AccessModesChange(t *testing.T) {
	oldPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			},
		},
	}
	newPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc1"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			},
		},
	}
	result := copyAndMerge(oldPVC, newPVC).(*corev1.PersistentVolumeClaim)
	assert.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, result.Spec.AccessModes)
}

func TestBuildInstancePodWithConfigs(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inst-0",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "nginx:1.19"},
					},
				},
			},
			Configs: []workloads.ConfigTemplate{
				{Name: "cfg1", ConfigHash: ptr.To("hash1")},
			},
		},
	}
	pod, err := buildInstancePod(inst, "")
	require.NoError(t, err)
	assert.Contains(t, pod.Annotations[constant.CMInsConfigurationHashLabelKey], "cfg1")
	assert.Contains(t, pod.Annotations[constant.CMInsConfigurationHashLabelKey], "hash1")
}

func TestBuildInstancePodWithVolumeClaimTemplates(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inst-0",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: workloads.InstanceSpec{
			InstanceSetName: "my-its",
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "nginx:1.19"},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
						},
					},
				},
			},
		},
	}
	pod, err := buildInstancePod(inst, "")
	require.NoError(t, err)
	// Should have volumes for the VCTs
	found := false
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "data" {
			found = true
			assert.NotNil(t, vol.PersistentVolumeClaim)
		}
	}
	assert.True(t, found, "expected volume 'data' to be present")
}

func TestBuildInstancePod(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inst-0",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": "test"},
					Annotations: map[string]string{"ann": "val"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "nginx:1.19"},
					},
				},
			},
		},
	}
	pod, err := buildInstancePod(inst, "")
	require.NoError(t, err)
	assert.Equal(t, "inst-0", pod.Name)
	assert.Equal(t, "default", pod.Namespace)
	assert.Equal(t, "test", pod.Labels["app"])
	assert.Equal(t, "val", pod.Annotations["ann"])
	assert.Equal(t, "inst-0", pod.Labels[constant.KBAppPodNameLabelKey])
	assert.NotEmpty(t, pod.Labels[constant.KBAppInstanceNameLabelKey])
}

func TestBuildInstancePVCs(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inst-0",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: workloads.InstanceSpec{
			InstanceSetName: "my-its",
			VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
						},
					},
				},
			},
		},
	}
	pvcs, err := buildInstancePVCs(inst)
	require.NoError(t, err)
	assert.Len(t, pvcs, 1)
	assert.Contains(t, pvcs[0].Name, "inst-0")
	assert.Equal(t, "inst-0", pvcs[0].Labels[constant.KBAppPodNameLabelKey])
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
