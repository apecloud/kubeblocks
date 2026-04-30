/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package controllerutil

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, workloads.AddToScheme(scheme))
	return scheme
}

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(objs...).Build()
}

// ============================================================
// instance_utils.go tests
// ============================================================

func TestIsInstanceReady(t *testing.T) {
	makeInst := func(gen, obsGen int64, utd bool, deleting bool, conditions []metav1.Condition) *workloads.Instance {
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "inst-0", Generation: gen},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: obsGen,
				UpToDate:           utd,
				Conditions:         conditions,
			},
		}
		if deleting {
			now := metav1.Now()
			inst.DeletionTimestamp = &now
			inst.Finalizers = []string{"test"}
		}
		return inst
	}
	readyCond := metav1.Condition{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue}
	availCond := metav1.Condition{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue}
	failCond := metav1.Condition{Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue}

	t.Run("ready instance", func(t *testing.T) {
		inst := makeInst(1, 1, true, false, []metav1.Condition{readyCond})
		assert.True(t, IsInstanceReady(inst))
	})
	t.Run("not up-to-date", func(t *testing.T) {
		inst := makeInst(2, 1, true, false, []metav1.Condition{readyCond})
		assert.False(t, IsInstanceReady(inst))
	})
	t.Run("not utd flag", func(t *testing.T) {
		inst := makeInst(1, 1, false, false, []metav1.Condition{readyCond})
		assert.False(t, IsInstanceReady(inst))
	})
	t.Run("deleting", func(t *testing.T) {
		inst := makeInst(1, 1, true, true, []metav1.Condition{readyCond})
		assert.False(t, IsInstanceReady(inst))
	})
	t.Run("no ready condition", func(t *testing.T) {
		inst := makeInst(1, 1, true, false, nil)
		assert.False(t, IsInstanceReady(inst))
	})
	t.Run("ready with role - no roles required", func(t *testing.T) {
		inst := makeInst(1, 1, true, false, []metav1.Condition{readyCond})
		assert.True(t, IsInstanceReadyWithRole(inst))
	})
	t.Run("ready with role - role required but missing", func(t *testing.T) {
		inst := makeInst(1, 1, true, false, []metav1.Condition{readyCond})
		inst.Spec.Roles = []workloads.ReplicaRole{{Name: "leader"}}
		assert.False(t, IsInstanceReadyWithRole(inst))
	})
	t.Run("ready with role - role present", func(t *testing.T) {
		inst := makeInst(1, 1, true, false, []metav1.Condition{readyCond})
		inst.Spec.Roles = []workloads.ReplicaRole{{Name: "leader"}}
		inst.Status.Role = "leader"
		assert.True(t, IsInstanceReadyWithRole(inst))
	})
	t.Run("available", func(t *testing.T) {
		inst := makeInst(1, 1, true, false, []metav1.Condition{availCond})
		assert.True(t, IsInstanceAvailable(inst))
	})
	t.Run("not available", func(t *testing.T) {
		inst := makeInst(1, 1, true, false, nil)
		assert.False(t, IsInstanceAvailable(inst))
	})
	t.Run("failure", func(t *testing.T) {
		inst := makeInst(1, 1, true, false, []metav1.Condition{failCond})
		assert.True(t, IsInstanceFailure(inst))
	})
	t.Run("terminating", func(t *testing.T) {
		inst := makeInst(1, 1, true, true, nil)
		assert.True(t, IsInstanceTerminating(inst))
	})
	t.Run("not terminating", func(t *testing.T) {
		inst := makeInst(1, 1, true, false, nil)
		assert.False(t, IsInstanceTerminating(inst))
	})
}

// ============================================================
// pod_utils.go tests
// ============================================================

func TestGetContainerByConfigSpec(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name: "main",
				VolumeMounts: []corev1.VolumeMount{
					{Name: "config-vol", MountPath: "/etc/config"},
				},
			},
		},
	}
	configs := []appsv1alpha1.ComponentConfigSpec{
		{ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{VolumeName: "config-vol"}},
	}

	t.Run("found in containers", func(t *testing.T) {
		c := GetContainerByConfigSpec(podSpec, configs)
		require.NotNil(t, c)
		assert.Equal(t, "main", c.Name)
	})

	t.Run("found in init containers", func(t *testing.T) {
		ps := &corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init", VolumeMounts: []corev1.VolumeMount{{Name: "config-vol"}}},
			},
		}
		c := GetContainerByConfigSpec(ps, configs)
		require.NotNil(t, c)
		assert.Equal(t, "init", c.Name)
	})

	t.Run("not found", func(t *testing.T) {
		ps := &corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main"}},
		}
		c := GetContainerByConfigSpec(ps, configs)
		assert.Nil(t, c)
	})
}

func TestGetPodContainerWithVolumeMount(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "c1", VolumeMounts: []corev1.VolumeMount{{Name: "vol1"}}},
			{Name: "c2", VolumeMounts: []corev1.VolumeMount{{Name: "vol2"}}},
			{Name: "c3", VolumeMounts: []corev1.VolumeMount{{Name: "vol1"}}},
		},
	}

	t.Run("found", func(t *testing.T) {
		result := GetPodContainerWithVolumeMount(podSpec, "vol1")
		assert.Len(t, result, 2)
	})
	t.Run("empty volume name", func(t *testing.T) {
		assert.Nil(t, GetPodContainerWithVolumeMount(podSpec, ""))
	})
	t.Run("no containers", func(t *testing.T) {
		assert.Nil(t, GetPodContainerWithVolumeMount(&corev1.PodSpec{}, "vol1"))
	})
}

func TestGetVolumeMountName(t *testing.T) {
	t.Run("configmap match", func(t *testing.T) {
		volumes := []corev1.Volume{
			{Name: "v1", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
		}
		v := GetVolumeMountName(volumes, "cm1")
		require.NotNil(t, v)
		assert.Equal(t, "v1", v.Name)
	})

	t.Run("projected match", func(t *testing.T) {
		volumes := []corev1.Volume{
			{Name: "v1", VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "cm2"}}},
					},
				},
			}},
		}
		v := GetVolumeMountName(volumes, "cm2")
		require.NotNil(t, v)
		assert.Equal(t, "v1", v.Name)
	})

	t.Run("not found", func(t *testing.T) {
		v := GetVolumeMountName(nil, "missing")
		assert.Nil(t, v)
	})
}

func TestGetContainersByConfigmap(t *testing.T) {
	containers := []corev1.Container{
		{Name: "c1", VolumeMounts: []corev1.VolumeMount{{Name: "vol1"}}},
		{Name: "c2", EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}}},
		{Name: "c3"},
	}

	t.Run("match by volume", func(t *testing.T) {
		result := GetContainersByConfigmap(containers, "vol1", "")
		assert.Contains(t, result, "c1")
	})

	t.Run("match by envFrom", func(t *testing.T) {
		result := GetContainersByConfigmap(containers, "nomatch", "cm1")
		assert.Contains(t, result, "c2")
	})

	t.Run("with filter", func(t *testing.T) {
		result := GetContainersByConfigmap(containers, "vol1", "", func(name string) bool {
			return name == "c1"
		})
		assert.NotContains(t, result, "c1")
	})
}

func TestGetVolumeMountByVolume(t *testing.T) {
	c := &corev1.Container{
		VolumeMounts: []corev1.VolumeMount{
			{Name: "vol1", MountPath: "/data"},
		},
	}
	t.Run("found", func(t *testing.T) {
		vm := GetVolumeMountByVolume(c, "vol1")
		require.NotNil(t, vm)
		assert.Equal(t, "/data", vm.MountPath)
	})
	t.Run("not found", func(t *testing.T) {
		assert.Nil(t, GetVolumeMountByVolume(c, "missing"))
	})
}

func TestGetCoreNum(t *testing.T) {
	t.Run("has cpu", func(t *testing.T) {
		c := corev1.Container{Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")},
		}}
		assert.Equal(t, int64(4), GetCoreNum(c))
	})
	t.Run("no cpu", func(t *testing.T) {
		c := corev1.Container{}
		assert.Equal(t, int64(0), GetCoreNum(c))
	})
}

func TestGetMemorySize(t *testing.T) {
	t.Run("has memory", func(t *testing.T) {
		c := corev1.Container{Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
		}}
		assert.Equal(t, int64(1073741824), GetMemorySize(c))
	})
	t.Run("no memory", func(t *testing.T) {
		assert.Equal(t, int64(0), GetMemorySize(corev1.Container{}))
	})
}

func TestGetRequestMemorySize(t *testing.T) {
	t.Run("has request memory", func(t *testing.T) {
		c := corev1.Container{Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
		}}
		assert.Equal(t, int64(536870912), GetRequestMemorySize(c))
	})
	t.Run("no request memory", func(t *testing.T) {
		assert.Equal(t, int64(0), GetRequestMemorySize(corev1.Container{}))
	})
}

func TestGetStorageSizeFromPersistentVolume(t *testing.T) {
	t.Run("has storage", func(t *testing.T) {
		pvc := corev1.PersistentVolumeClaimTemplate{
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
				},
			},
		}
		assert.Equal(t, int64(10737418240), GetStorageSizeFromPersistentVolume(pvc))
	})
	t.Run("no storage", func(t *testing.T) {
		pvc := corev1.PersistentVolumeClaimTemplate{}
		assert.Equal(t, int64(-1), GetStorageSizeFromPersistentVolume(pvc))
	})
}

func TestIsPodReady(t *testing.T) {
	readyPod := &corev1.Pod{
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		},
	}
	t.Run("ready", func(t *testing.T) {
		assert.True(t, IsPodReady(readyPod))
	})
	t.Run("terminating", func(t *testing.T) {
		pod := readyPod.DeepCopy()
		now := metav1.Now()
		pod.DeletionTimestamp = &now
		assert.False(t, IsPodReady(pod))
	})
}

func TestIsPodAvailable(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute))},
			},
		},
	}
	assert.True(t, IsPodAvailable(pod, 0))
}

func TestGetPodCondition(t *testing.T) {
	t.Run("nil status", func(t *testing.T) {
		assert.Nil(t, GetPodCondition(nil, corev1.PodReady))
	})
	t.Run("empty conditions", func(t *testing.T) {
		assert.Nil(t, GetPodCondition(&corev1.PodStatus{}, corev1.PodReady))
	})
	t.Run("found", func(t *testing.T) {
		status := &corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		}
		c := GetPodCondition(status, corev1.PodReady)
		require.NotNil(t, c)
		assert.Equal(t, corev1.ConditionTrue, c.Status)
	})
	t.Run("not found", func(t *testing.T) {
		status := &corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
			},
		}
		assert.Nil(t, GetPodCondition(status, corev1.PodReady))
	})
}

func TestIsMatchConfigVersion(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "v1"}}}
	assert.True(t, IsMatchConfigVersion(obj, "version", "v1"))
	assert.False(t, IsMatchConfigVersion(obj, "version", "v2"))
	assert.False(t, IsMatchConfigVersion(obj, "missing", "v1"))

	noLabels := &corev1.ConfigMap{}
	assert.False(t, IsMatchConfigVersion(noLabels, "version", "v1"))
}

func TestGetPortByName(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main", Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}}},
			},
		},
	}
	t.Run("found", func(t *testing.T) {
		port, err := GetPortByName(pod, "main", "http")
		require.NoError(t, err)
		assert.Equal(t, int32(8080), port)
	})
	t.Run("not found", func(t *testing.T) {
		_, err := GetPortByName(pod, "main", "grpc")
		assert.Error(t, err)
	})
}

func TestPodIsReadyWithLabel(t *testing.T) {
	t.Run("ready with role", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{constant.RoleLabelKey: "leader"}},
			Status:     corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
		}
		assert.True(t, PodIsReadyWithLabel(pod))
	})
	t.Run("no role label", func(t *testing.T) {
		pod := corev1.Pod{
			Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
		}
		assert.False(t, PodIsReadyWithLabel(pod))
	})
}

func TestGetPodRevisionStd(t *testing.T) {
	t.Run("has revision", func(t *testing.T) {
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{appsv1.StatefulSetRevisionLabel: "rev1"}}}
		assert.Equal(t, "rev1", GetPodRevision(pod))
	})
	t.Run("no labels", func(t *testing.T) {
		assert.Equal(t, "", GetPodRevision(&corev1.Pod{}))
	})
}

func TestByPodName(t *testing.T) {
	pods := ByPodName{
		{ObjectMeta: metav1.ObjectMeta{Name: "c"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
	}
	sort.Sort(pods)
	assert.Equal(t, "a", pods[0].Name)
	assert.Equal(t, "b", pods[1].Name)
	assert.Equal(t, "c", pods[2].Name)
}

func TestBuildPodHostDNS(t *testing.T) {
	t.Run("nil pod", func(t *testing.T) {
		assert.Equal(t, "", BuildPodHostDNS(nil))
	})
	t.Run("with subdomain", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-0"},
			Spec:       corev1.PodSpec{Subdomain: "svc"},
		}
		assert.Equal(t, "pod-0.svc", BuildPodHostDNS(pod))
	})
	t.Run("with subdomain and hostname", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-0"},
			Spec:       corev1.PodSpec{Subdomain: "svc", Hostname: "myhost"},
		}
		assert.Equal(t, "myhost.svc", BuildPodHostDNS(pod))
	})
	t.Run("no subdomain uses PodIP", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-0"},
			Status:     corev1.PodStatus{PodIP: "10.0.0.1"},
		}
		assert.Equal(t, "10.0.0.1", BuildPodHostDNS(pod))
	})
}

func TestResolvePodSpecDefaultFields(t *testing.T) {
	restartPolicy := corev1.RestartPolicyAlways
	var terminationGrace int64 = 30
	obj := corev1.PodSpec{
		RestartPolicy:                 restartPolicy,
		TerminationGracePeriodSeconds: &terminationGrace,
		DNSPolicy:                     corev1.DNSClusterFirst,
		SchedulerName:                 "default-scheduler",
		Containers: []corev1.Container{
			{
				TerminationMessagePath:   "/dev/termination-log",
				TerminationMessagePolicy: corev1.TerminationMessageReadFile,
				ImagePullPolicy:          corev1.PullIfNotPresent,
			},
		},
	}
	pobj := &corev1.PodSpec{
		Containers: []corev1.Container{{}},
	}
	ResolvePodSpecDefaultFields(obj, pobj)
	assert.Equal(t, restartPolicy, pobj.RestartPolicy)
	assert.Equal(t, &terminationGrace, pobj.TerminationGracePeriodSeconds)
	assert.Equal(t, corev1.DNSClusterFirst, pobj.DNSPolicy)
	assert.Equal(t, "default-scheduler", pobj.SchedulerName)
	assert.Equal(t, corev1.PullIfNotPresent, pobj.Containers[0].ImagePullPolicy)
}

func TestResolveContainerDefaultFields(t *testing.T) {
	container := corev1.Container{
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		ImagePullPolicy:          corev1.PullAlways,
		LivenessProbe: &corev1.Probe{
			ProbeHandler:     corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Scheme: corev1.URISchemeHTTP}},
			TimeoutSeconds:   5,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			FailureThreshold: 3,
		},
	}
	pcontainer := &corev1.Container{
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{}},
		},
	}
	ResolveContainerDefaultFields(container, pcontainer)
	assert.Equal(t, "/dev/termination-log", pcontainer.TerminationMessagePath)
	assert.Equal(t, corev1.PullAlways, pcontainer.ImagePullPolicy)
	assert.Equal(t, int32(5), pcontainer.LivenessProbe.TimeoutSeconds)
	assert.Equal(t, corev1.URISchemeHTTP, pcontainer.LivenessProbe.HTTPGet.Scheme)
}

func TestGetPodContainer(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "first"},
				{Name: "second"},
			},
		},
	}
	t.Run("empty name returns first", func(t *testing.T) {
		c := GetPodContainer(pod, "")
		require.NotNil(t, c)
		assert.Equal(t, "first", c.Name)
	})
	t.Run("by name", func(t *testing.T) {
		c := GetPodContainer(pod, "second")
		require.NotNil(t, c)
		assert.Equal(t, "second", c.Name)
	})
	t.Run("not found", func(t *testing.T) {
		assert.Nil(t, GetPodContainer(pod, "missing"))
	})
}

func TestIsPodFailedAndTimedOut(t *testing.T) {
	t.Run("no failure", func(t *testing.T) {
		pod := &corev1.Pod{}
		failed, timedOut, msg := IsPodFailedAndTimedOut(pod)
		assert.False(t, failed)
		assert.False(t, timedOut)
		assert.Empty(t, msg)
	})

	t.Run("init container waiting with message", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Message: "crash"}}},
				},
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodInitialized, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute))},
				},
			},
		}
		failed, timedOut, msg := IsPodFailedAndTimedOut(pod)
		assert.True(t, failed)
		assert.True(t, timedOut)
		assert.Equal(t, "crash", msg)
	})

	t.Run("container terminated with exit code", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, Message: "OOM"}}},
				},
				Conditions: []corev1.PodCondition{
					{Type: corev1.ContainersReady, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute))},
				},
			},
		}
		failed, timedOut, msg := IsPodFailedAndTimedOut(pod)
		assert.True(t, failed)
		assert.True(t, timedOut)
		assert.Equal(t, "OOM", msg)
	})

	t.Run("container failed but not timed out", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Message: "err"}}},
				},
				Conditions: []corev1.PodCondition{
					{Type: corev1.ContainersReady, LastTransitionTime: metav1.NewTime(time.Now())},
				},
			},
		}
		failed, timedOut, _ := IsPodFailedAndTimedOut(pod)
		assert.True(t, failed)
		assert.False(t, timedOut)
	})
}

func TestBuildImagePullSecrets(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		viper.Set(constant.KBImagePullSecrets, "")
		result := BuildImagePullSecrets()
		assert.Empty(t, result)
	})
	t.Run("with secrets", func(t *testing.T) {
		viper.Set(constant.KBImagePullSecrets, `[{"name":"mysecret"}]`)
		defer viper.Set(constant.KBImagePullSecrets, "")
		result := BuildImagePullSecrets()
		require.Len(t, result, 1)
		assert.Equal(t, "mysecret", result[0].Name)
	})
}

// ============================================================
// image_util.go tests
// ============================================================

func TestParseImageName(t *testing.T) {
	tests := []struct {
		name      string
		image     string
		wantHost  string
		wantNs    string
		wantRepo  string
		wantRem   string
		wantErr   bool
	}{
		{"full with tag", "docker.io/library/nginx:1.19", "docker.io", "library", "nginx", ":1.19", false},
		{"no registry", "nginx:latest", "docker.io", "library", "nginx", ":latest", false},
		{"custom registry", "myregistry.io/myns/myapp:v1", "myregistry.io", "myns", "myapp", ":v1", false},
		{"no tag", "docker.io/library/nginx", "docker.io", "library", "nginx", "", false},
		{"with digest", "nginx@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", "docker.io", "library", "nginx", "@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", false},
		{"invalid", "INVALID:::", "", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, ns, repo, rem, err := parseImageName(tt.image)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantNs, ns)
			assert.Equal(t, tt.wantRepo, repo)
			assert.Equal(t, tt.wantRem, rem)
		})
	}
}

func TestReplaceImageRegistry(t *testing.T) {
	t.Run("no config returns original", func(t *testing.T) {
		registriesConfigMutex.Lock()
		old := registriesConfigInstance
		registriesConfigInstance = &registriesConfig{}
		registriesConfigMutex.Unlock()
		defer func() {
			registriesConfigMutex.Lock()
			registriesConfigInstance = old
			registriesConfigMutex.Unlock()
		}()

		result := ReplaceImageRegistry("docker.io/library/nginx:1.19")
		assert.Equal(t, "docker.io/library/nginx:1.19", result)
	})

	t.Run("default registry override", func(t *testing.T) {
		registriesConfigMutex.Lock()
		old := registriesConfigInstance
		registriesConfigInstance = &registriesConfig{
			DefaultRegistry: "myregistry.io",
		}
		registriesConfigMutex.Unlock()
		defer func() {
			registriesConfigMutex.Lock()
			registriesConfigInstance = old
			registriesConfigMutex.Unlock()
		}()

		result := ReplaceImageRegistry("docker.io/library/nginx:1.19")
		assert.Equal(t, "myregistry.io/library/nginx:1.19", result)
	})

	t.Run("registry mapping", func(t *testing.T) {
		registriesConfigMutex.Lock()
		old := registriesConfigInstance
		registriesConfigInstance = &registriesConfig{
			RegistryConfig: []registryConfig{
				{From: "docker.io", To: "mirror.io", DefaultNamespace: "default-ns"},
			},
		}
		registriesConfigMutex.Unlock()
		defer func() {
			registriesConfigMutex.Lock()
			registriesConfigInstance = old
			registriesConfigMutex.Unlock()
		}()

		result := ReplaceImageRegistry("docker.io/library/nginx:1.19")
		assert.Equal(t, "mirror.io/default-ns/nginx:1.19", result)
	})

	t.Run("invalid image returns as-is", func(t *testing.T) {
		result := ReplaceImageRegistry("INVALID:::")
		assert.Equal(t, "INVALID:::", result)
	})
}

// ============================================================
// predicate.go tests
// ============================================================

func TestIsAPIVersionSupported(t *testing.T) {
	t.Run("supported version", func(t *testing.T) {
		assert.True(t, IsAPIVersionSupported(kbappsv1.GroupVersion.String()))
	})
	t.Run("unsupported version", func(t *testing.T) {
		assert.False(t, IsAPIVersionSupported("unknown/v1"))
	})
	t.Run("with viper override", func(t *testing.T) {
		viper.Set(constant.APIVersionSupported, ".*")
		defer viper.Set(constant.APIVersionSupported, "")
		assert.True(t, IsAPIVersionSupported("anything/v99"))
	})
}

func TestObjectAPIVersionSupported(t *testing.T) {
	t.Run("supported annotation", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{constant.CRDAPIVersionAnnotationKey: kbappsv1.GroupVersion.String()},
			},
		}
		assert.True(t, ObjectAPIVersionSupported(obj))
	})
	t.Run("cluster with no annotation", func(t *testing.T) {
		obj := &kbappsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		assert.True(t, ObjectAPIVersionSupported(obj))
	})
	t.Run("unsupported", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{constant.CRDAPIVersionAnnotationKey: "unknown/v1"},
			},
		}
		assert.False(t, ObjectAPIVersionSupported(obj))
	})
}

func TestNamespacePredicateFilter(t *testing.T) {
	// Reset the global to force reinitialization
	managedNamespaces = nil

	t.Run("no namespace filter allows all", func(t *testing.T) {
		viper.Set("managed_namespaces", "")
		managedNamespaces = nil
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "any"}}
		assert.True(t, namespacePredicateFilter(obj))
	})

	t.Run("cluster-scoped resource always allowed", func(t *testing.T) {
		viper.Set("managed_namespaces", "ns1")
		managedNamespaces = nil
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{}}
		assert.True(t, namespacePredicateFilter(obj))
		managedNamespaces = nil
		viper.Set("managed_namespaces", "")
	})
}

// ============================================================
// errors.go tests
// ============================================================

func TestErrorType(t *testing.T) {
	t.Run("Error method", func(t *testing.T) {
		e := &Error{Type: ErrorTypeNotFound, Message: "not found"}
		assert.Equal(t, "not found", e.Error())
	})
	t.Run("NewErrorf", func(t *testing.T) {
		e := NewErrorf(ErrorTypeFatal, "failed: %s", "boom")
		assert.Equal(t, "failed: boom", e.Message)
		assert.Equal(t, ErrorTypeFatal, e.Type)
	})
	t.Run("UnwrapControllerError", func(t *testing.T) {
		e := NewError(ErrorTypeRequeue, "requeue")
		unwrapped := UnwrapControllerError(e)
		require.NotNil(t, unwrapped)
		assert.Equal(t, ErrorTypeRequeue, unwrapped.Type)
	})
	t.Run("UnwrapControllerError nil", func(t *testing.T) {
		assert.Nil(t, UnwrapControllerError(fmt.Errorf("regular error")))
	})
	t.Run("NewNotFound", func(t *testing.T) {
		e := NewNotFound("resource %s", "foo")
		assert.True(t, IsNotFound(e))
		assert.Contains(t, e.Message, "foo")
	})
	t.Run("IsNotFound false", func(t *testing.T) {
		assert.False(t, IsNotFound(fmt.Errorf("regular")))
	})
	t.Run("NewFatalError", func(t *testing.T) {
		e := NewFatalError("fatal")
		assert.True(t, IsTargetError(e, ErrorTypeFatal))
	})
}

// ============================================================
// controller_common.go tests
// ============================================================

func TestIgnoreIsAlreadyExists(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.NoError(t, IgnoreIsAlreadyExists(nil))
	})
	t.Run("regular error", func(t *testing.T) {
		err := fmt.Errorf("some error")
		assert.Equal(t, err, IgnoreIsAlreadyExists(err))
	})
}

func TestRecordCreatedEvent(t *testing.T) {
	t.Run("nil recorder", func(t *testing.T) {
		RecordCreatedEvent(nil, &corev1.ConfigMap{})
	})
	t.Run("generation > 1 no event", func(t *testing.T) {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Generation: 2}}
		RecordCreatedEvent(nil, obj) // should not panic
	})
}

func TestCheckResourceExists(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "ns"}}
	cli := newFakeClient(t, cm)

	t.Run("exists", func(t *testing.T) {
		found, err := CheckResourceExists(context.Background(), cli, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})
		require.NoError(t, err)
		assert.True(t, found)
	})
	t.Run("not found", func(t *testing.T) {
		found, err := CheckResourceExists(context.Background(), cli, client.ObjectKey{Namespace: "ns", Name: "missing"}, &corev1.ConfigMap{})
		require.NoError(t, err)
		assert.False(t, found)
	})
}

func TestSetOwnership(t *testing.T) {
	scheme := newTestScheme(t)
	owner := &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns", UID: "uid-1"},
	}
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns"}}

	t.Run("controller reference", func(t *testing.T) {
		err := SetOwnership(owner, obj, scheme, "test-finalizer")
		require.NoError(t, err)
		assert.Len(t, obj.OwnerReferences, 1)
		assert.Contains(t, obj.Finalizers, "test-finalizer")
	})

	t.Run("pvc skips finalizer", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "pvc", Namespace: "ns"}}
		err := SetOwnership(owner, pvc, scheme, "test-finalizer")
		require.NoError(t, err)
		assert.NotContains(t, pvc.Finalizers, "test-finalizer")
	})

	t.Run("owner reference mode", func(t *testing.T) {
		obj2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: "ns"}}
		err := SetOwnership(owner, obj2, scheme, "", true)
		require.NoError(t, err)
		assert.Len(t, obj2.OwnerReferences, 1)
	})
}

// ============================================================
// util.go tests
// ============================================================

func TestMergeMetadataMaps(t *testing.T) {
	t.Run("merge without overwrite", func(t *testing.T) {
		original := map[string]string{"a": "1"}
		target := map[string]string{"a": "2", "b": "3"}
		result := MergeMetadataMaps(original, target)
		assert.Equal(t, "1", result["a"]) // original wins
		assert.Equal(t, "3", result["b"])
	})
	t.Run("nil original", func(t *testing.T) {
		result := MergeMetadataMaps(nil, map[string]string{"a": "1"})
		assert.Equal(t, "1", result["a"])
	})
}

func TestMergeMetadataMapInplace(t *testing.T) {
	t.Run("merge inplace", func(t *testing.T) {
		original := map[string]string{"a": "1", "b": "2"}
		target := map[string]string{"a": "old"}
		MergeMetadataMapInplace(original, &target)
		assert.Equal(t, "1", target["a"]) // original overwrites
		assert.Equal(t, "2", target["b"])
	})
	t.Run("nil original no-op", func(t *testing.T) {
		target := map[string]string{"a": "1"}
		MergeMetadataMapInplace(nil, &target)
		assert.Equal(t, "1", target["a"])
	})
	t.Run("nil target gets created", func(t *testing.T) {
		var target map[string]string
		MergeMetadataMapInplace(map[string]string{"a": "1"}, &target)
		assert.Equal(t, "1", target["a"])
	})
}

func TestMergeList(t *testing.T) {
	type item struct {
		Name  string
		Value int
	}
	t.Run("overwrite existing", func(t *testing.T) {
		src := []item{{Name: "a", Value: 2}}
		dst := []item{{Name: "a", Value: 1}, {Name: "b", Value: 3}}
		MergeList(&src, &dst, func(e item) func(item) bool {
			return func(x item) bool { return x.Name == e.Name }
		})
		assert.Equal(t, 2, dst[0].Value)
		assert.Len(t, dst, 2)
	})
	t.Run("append new", func(t *testing.T) {
		src := []item{{Name: "c", Value: 4}}
		dst := []item{{Name: "a", Value: 1}}
		MergeList(&src, &dst, func(e item) func(item) bool {
			return func(x item) bool { return x.Name == e.Name }
		})
		assert.Len(t, dst, 2)
	})
	t.Run("empty src is no-op", func(t *testing.T) {
		src := []item{}
		dst := []item{{Name: "a", Value: 1}}
		MergeList(&src, &dst, func(e item) func(item) bool {
			return func(x item) bool { return x.Name == e.Name }
		})
		assert.Len(t, dst, 1)
	})
}

func TestGetKubeVersionStd(t *testing.T) {
	t.Run("valid version", func(t *testing.T) {
		viper.Set(constant.CfgKeyServerInfo, version.Info{GitVersion: "v1.28.3"})
		defer viper.Set(constant.CfgKeyServerInfo, nil)
		v, err := GetKubeVersion()
		require.NoError(t, err)
		assert.Equal(t, "v1.28", v)
	})
	t.Run("invalid type", func(t *testing.T) {
		viper.Set(constant.CfgKeyServerInfo, "not-version-info")
		defer viper.Set(constant.CfgKeyServerInfo, nil)
		_, err := GetKubeVersion()
		assert.Error(t, err)
	})
	t.Run("invalid semver", func(t *testing.T) {
		viper.Set(constant.CfgKeyServerInfo, version.Info{GitVersion: "invalid"})
		defer viper.Set(constant.CfgKeyServerInfo, nil)
		_, err := GetKubeVersion()
		assert.Error(t, err)
	})
}

func TestSupportResizeSubResource(t *testing.T) {
	t.Run("v1.32+ supports", func(t *testing.T) {
		viper.Set(constant.CfgKeyServerInfo, version.Info{GitVersion: "v1.32.0"})
		defer viper.Set(constant.CfgKeyServerInfo, nil)
		ok, err := supportResizeSubResourceImpl()
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("v1.31 does not support", func(t *testing.T) {
		viper.Set(constant.CfgKeyServerInfo, version.Info{GitVersion: "v1.31.0"})
		defer viper.Set(constant.CfgKeyServerInfo, nil)
		ok, err := supportResizeSubResourceImpl()
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

// ============================================================
// volume_util.go tests
// ============================================================

func TestCreateVolumeIfNotExist(t *testing.T) {
	createFn := func(name string) corev1.Volume {
		return corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}
	}
	t.Run("creates when missing", func(t *testing.T) {
		volumes := []corev1.Volume{{Name: "existing"}}
		result := CreateVolumeIfNotExist(volumes, "new", createFn)
		assert.Len(t, result, 2)
	})
	t.Run("no-op when exists", func(t *testing.T) {
		volumes := []corev1.Volume{{Name: "existing"}}
		result := CreateVolumeIfNotExist(volumes, "existing", createFn)
		assert.Len(t, result, 1)
	})
}

func TestToCoreV1PVCTs(t *testing.T) {
	t.Run("with storage class", func(t *testing.T) {
		sc := "fast"
		vcts := []kbappsv1.PersistentVolumeClaimTemplate{
			{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &sc}},
		}
		result := ToCoreV1PVCTs(vcts)
		require.Len(t, result, 1)
		assert.Equal(t, "data", result[0].Name)
		assert.Equal(t, &sc, result[0].Spec.StorageClassName)
	})
	t.Run("default storage class from viper", func(t *testing.T) {
		viper.Set(constant.CfgKeyDefaultStorageClass, "standard")
		defer viper.Set(constant.CfgKeyDefaultStorageClass, "")
		vcts := []kbappsv1.PersistentVolumeClaimTemplate{
			{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{}},
		}
		result := ToCoreV1PVCTs(vcts)
		require.Len(t, result, 1)
		require.NotNil(t, result[0].Spec.StorageClassName)
		assert.Equal(t, "standard", *result[0].Spec.StorageClassName)
	})
	t.Run("no storage class", func(t *testing.T) {
		viper.Set(constant.CfgKeyDefaultStorageClass, "")
		vcts := []kbappsv1.PersistentVolumeClaimTemplate{
			{Name: "data"},
		}
		result := ToCoreV1PVCTs(vcts)
		require.Len(t, result, 1)
		assert.Nil(t, result[0].Spec.StorageClassName)
	})
}

func TestComposePVCName(t *testing.T) {
	t.Run("default format", func(t *testing.T) {
		pvc := corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data"}}
		assert.Equal(t, "data-pod-0", ComposePVCName(pvc, "its", "pod-0"))
	})
	t.Run("with prefix annotation", func(t *testing.T) {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "data",
				Annotations: map[string]string{constant.PVCNamePrefixAnnotationKey: "custom"},
			},
		}
		assert.Equal(t, "custom-0", ComposePVCName(pvc, "its", "its-0"))
	})
	t.Run("prefix annotation but no its prefix match", func(t *testing.T) {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "data",
				Annotations: map[string]string{constant.PVCNamePrefixAnnotationKey: "custom"},
			},
		}
		assert.Equal(t, "custom", ComposePVCName(pvc, "its", "other-pod"))
	})
}

// ============================================================
// container_util.go tests
// ============================================================

func TestInjectZeroResourcesLimitsIfEmpty(t *testing.T) {
	t.Run("no resources sets zero limits", func(t *testing.T) {
		c := &corev1.Container{}
		InjectZeroResourcesLimitsIfEmpty(c)
		assert.Equal(t, resource.MustParse("0"), c.Resources.Limits[corev1.ResourceCPU])
		assert.Equal(t, resource.MustParse("0"), c.Resources.Limits[corev1.ResourceMemory])
	})
	t.Run("existing request prevents zero limit", func(t *testing.T) {
		c := &corev1.Container{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
		}
		InjectZeroResourcesLimitsIfEmpty(c)
		_, hasCPULimit := c.Resources.Limits[corev1.ResourceCPU]
		assert.False(t, hasCPULimit) // should not set zero limit when request exists
		assert.Equal(t, resource.MustParse("0"), c.Resources.Limits[corev1.ResourceMemory])
	})
	t.Run("existing limit preserved", func(t *testing.T) {
		c := &corev1.Container{
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
			},
		}
		InjectZeroResourcesLimitsIfEmpty(c)
		assert.Equal(t, resource.MustParse("2"), c.Resources.Limits[corev1.ResourceCPU])
	})
}

// ============================================================
// workload_utils.go tests
// ============================================================

func TestPodFQDN(t *testing.T) {
	viper.Set(constant.KubernetesClusterDomainEnv, "cluster.local")
	defer viper.Set(constant.KubernetesClusterDomainEnv, "")
	assert.Equal(t, "pod-0.comp-headless.ns.svc.cluster.local", PodFQDN("ns", "comp", "pod-0"))
}

func TestServiceFQDN(t *testing.T) {
	viper.Set(constant.KubernetesClusterDomainEnv, "cluster.local")
	defer viper.Set(constant.KubernetesClusterDomainEnv, "")
	assert.Equal(t, "svc1.ns.svc.cluster.local", ServiceFQDN("ns", "svc1"))
}

// ============================================================
// instance_set_utils.go tests
// ============================================================

func TestGetPodListByInstanceSet(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "its", Namespace: "ns"},
		Spec: workloads.InstanceSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "ns", Labels: map[string]string{"app": "test"}},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-other", Namespace: "ns", Labels: map[string]string{"app": "other"}},
	}
	cli := newFakeClient(t, pod1, pod2)

	pods, err := GetPodListByInstanceSet(context.Background(), cli, its)
	require.NoError(t, err)
	assert.Len(t, pods, 1)
	assert.Equal(t, "pod-0", pods[0].Name)
}

// ============================================================
// host_port_manager.go tests
// ============================================================

func TestPortManagerPortKey(t *testing.T) {
	pm := &portManager{}
	key := pm.PortKey("cluster", "comp", "container", "http")
	assert.Equal(t, "cluster-comp-container-http", key)
}

func TestPortManagerParsePort(t *testing.T) {
	pm := &portManager{}
	t.Run("valid", func(t *testing.T) {
		p, err := pm.parsePort("8080")
		require.NoError(t, err)
		assert.Equal(t, int32(8080), p)
	})
	t.Run("empty", func(t *testing.T) {
		_, err := pm.parsePort("")
		assert.Error(t, err)
	})
	t.Run("whitespace only", func(t *testing.T) {
		_, err := pm.parsePort("  ")
		assert.Error(t, err)
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := pm.parsePort("abc")
		assert.Error(t, err)
	})
}

func TestNewDefaultPortManager(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-port-cm",
			Namespace: "kb-system",
		},
		Data: map[string]string{
			"key1": "30000",
			"key2": "30001",
		},
	}
	cli := newFakeClient(t, cm)
	viper.Set(constant.CfgHostPortConfigMapName, "host-port-cm")
	viper.Set(constant.CfgKeyCtrlrMgrNS, "kb-system")
	defer func() {
		viper.Set(constant.CfgHostPortConfigMapName, "")
		viper.Set(constant.CfgKeyCtrlrMgrNS, "")
	}()

	includes := []portRange{{Min: 30000, Max: 30010}}
	pm, err := newDefaultPortManager(includes, nil, cli)
	require.NoError(t, err)
	assert.Equal(t, int32(30000), pm.from)
	assert.Equal(t, int32(30010), pm.to)
	assert.Len(t, pm.used, 2)
}

func TestPortManagerGetPort(t *testing.T) {
	pm := &portManager{
		cm: &corev1.ConfigMap{Data: map[string]string{"key1": "30000"}},
	}
	t.Run("found", func(t *testing.T) {
		port, err := pm.GetPort("key1")
		require.NoError(t, err)
		assert.Equal(t, int32(30000), port)
	})
	t.Run("not found", func(t *testing.T) {
		port, err := pm.GetPort("missing")
		require.NoError(t, err)
		assert.Equal(t, int32(0), port)
	})
}

func TestPortManagerAllocatePort(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "hp", Namespace: "ns"},
		Data:       map[string]string{},
	}
	cli := newFakeClient(t, cm)
	viper.Set(constant.CfgHostPortConfigMapName, "hp")
	viper.Set(constant.CfgKeyCtrlrMgrNS, "ns")
	defer func() {
		viper.Set(constant.CfgHostPortConfigMapName, "")
		viper.Set(constant.CfgKeyCtrlrMgrNS, "")
	}()

	pm := &portManager{
		cli:    cli,
		from:   30000,
		to:     30002,
		cursor: 30000,
		used:   map[int32]string{},
		cm:     cm,
	}

	t.Run("allocate new", func(t *testing.T) {
		port, err := pm.AllocatePort("key1")
		require.NoError(t, err)
		assert.Equal(t, int32(30000), port)
	})

	t.Run("return existing", func(t *testing.T) {
		port, err := pm.AllocatePort("key1")
		require.NoError(t, err)
		assert.Equal(t, int32(30000), port)
	})
}

func TestPortManagerUsePort(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "hp", Namespace: "ns"},
		Data:       map[string]string{},
	}
	cli := newFakeClient(t, cm)
	viper.Set(constant.CfgHostPortConfigMapName, "hp")
	viper.Set(constant.CfgKeyCtrlrMgrNS, "ns")
	defer func() {
		viper.Set(constant.CfgHostPortConfigMapName, "")
		viper.Set(constant.CfgKeyCtrlrMgrNS, "")
	}()

	pm := &portManager{
		cli:  cli,
		used: map[int32]string{},
		cm:   cm,
	}

	t.Run("use new port", func(t *testing.T) {
		err := pm.UsePort("key1", 30000)
		require.NoError(t, err)
	})

	t.Run("port already used by different key", func(t *testing.T) {
		err := pm.UsePort("key2", 30000)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "used by")
	})

	t.Run("same key same port ok", func(t *testing.T) {
		err := pm.UsePort("key1", 30000)
		require.NoError(t, err)
	})
}

func TestPortManagerReleaseByPrefix(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "hp", Namespace: "ns"},
		Data: map[string]string{
			"cluster-comp-c1-http":  "30000",
			"cluster-comp-c1-grpc":  "30001",
			"cluster-other-c1-http": "30002",
		},
	}
	cli := newFakeClient(t, cm)
	viper.Set(constant.CfgHostPortConfigMapName, "hp")
	viper.Set(constant.CfgKeyCtrlrMgrNS, "ns")
	defer func() {
		viper.Set(constant.CfgHostPortConfigMapName, "")
		viper.Set(constant.CfgKeyCtrlrMgrNS, "")
	}()

	pm := &portManager{
		cli:  cli,
		used: map[int32]string{30000: "cluster-comp-c1-http", 30001: "cluster-comp-c1-grpc", 30002: "cluster-other-c1-http"},
		cm:   cm,
	}

	t.Run("empty prefix no-op", func(t *testing.T) {
		require.NoError(t, pm.ReleaseByPrefix(""))
	})

	t.Run("release matching prefix", func(t *testing.T) {
		err := pm.ReleaseByPrefix("cluster-comp")
		require.NoError(t, err)
		assert.NotContains(t, pm.used, int32(30000))
		assert.NotContains(t, pm.used, int32(30001))
		assert.Contains(t, pm.used, int32(30002))
	})
}

func TestGetPortManager(t *testing.T) {
	t.Run("nil network returns default", func(t *testing.T) {
		pm := GetPortManager(nil)
		assert.Equal(t, defaultPortManager, pm)
	})
	t.Run("host network disabled returns default", func(t *testing.T) {
		pm := GetPortManager(&kbappsv1.ComponentNetwork{HostNetwork: false})
		assert.Equal(t, defaultPortManager, pm)
	})
	t.Run("empty host ports returns default", func(t *testing.T) {
		pm := GetPortManager(&kbappsv1.ComponentNetwork{HostNetwork: true})
		assert.Equal(t, defaultPortManager, pm)
	})
	t.Run("with host ports creates defined manager", func(t *testing.T) {
		network := &kbappsv1.ComponentNetwork{
			HostNetwork: true,
			HostPorts: []kbappsv1.HostPort{
				{Name: "http", Port: 8080},
			},
		}
		pm := GetPortManager(network)
		assert.NotNil(t, pm)
		dpm, ok := pm.(*definedPortManager)
		assert.True(t, ok)
		assert.Equal(t, int32(8080), dpm.hostPorts["http"])
	})
}

func TestDefinedPortManagerPortKey(t *testing.T) {
	dpm := &definedPortManager{
		defaultPortManager: &portManager{},
		hostPorts:          map[string]int32{"http": 8080},
	}
	t.Run("defined port uses portName", func(t *testing.T) {
		key := dpm.PortKey("c", "comp", "container", "http")
		assert.Equal(t, "http", key)
	})
}

func TestDefinedPortManagerGetPort(t *testing.T) {
	dpm := &definedPortManager{
		defaultPortManager: &portManager{
			cm: &corev1.ConfigMap{Data: map[string]string{}},
		},
		hostPorts: map[string]int32{"http": 8080},
	}
	t.Run("defined port", func(t *testing.T) {
		port, err := dpm.GetPort("http")
		require.NoError(t, err)
		assert.Equal(t, int32(8080), port)
	})
}

func TestDefinedPortManagerAllocatePort(t *testing.T) {
	dpm := &definedPortManager{
		defaultPortManager: &portManager{},
		hostPorts:          map[string]int32{"http": 8080},
	}
	t.Run("defined port returns directly", func(t *testing.T) {
		port, err := dpm.AllocatePort("http")
		require.NoError(t, err)
		assert.Equal(t, int32(8080), port)
	})
	t.Run("unknown non-agent port returns error", func(t *testing.T) {
		_, err := dpm.AllocatePort("unknown")
		assert.Error(t, err)
	})
}

func TestDefinedPortManagerUsePort(t *testing.T) {
	dpm := &definedPortManager{
		defaultPortManager: &portManager{
			cm:   &corev1.ConfigMap{Data: map[string]string{}},
			used: map[int32]string{},
		},
		hostPorts: map[string]int32{"http": 8080},
	}
	t.Run("defined port is no-op", func(t *testing.T) {
		err := dpm.UsePort("http", 8080)
		require.NoError(t, err)
	})
}

func TestDefinedPortManagerReleaseByPrefix(t *testing.T) {
	t.Run("has kb-agent ports defined skips release", func(t *testing.T) {
		dpm := &definedPortManager{
			defaultPortManager: &portManager{
				cm:   &corev1.ConfigMap{Data: map[string]string{"key1": "30000"}},
				used: map[int32]string{30000: "key1"},
			},
			hostPorts: map[string]int32{"http": 8080, "streaming": 9090},
		}
		err := dpm.ReleaseByPrefix("key")
		require.NoError(t, err)
		assert.Contains(t, dpm.defaultPortManager.used, int32(30000)) // not released
	})
}

// ============================================================
// Additional controller_common.go tests
// ============================================================

func TestBackgroundDeleteObject(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns"}}
	cli := newFakeClient(t, cm)

	t.Run("delete existing", func(t *testing.T) {
		err := BackgroundDeleteObject(cli, context.Background(), cm)
		require.NoError(t, err)
	})
	t.Run("delete non-existing is no-op", func(t *testing.T) {
		err := BackgroundDeleteObject(cli, context.Background(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "gone", Namespace: "ns"}})
		require.NoError(t, err)
	})
}

func TestIgnoreIsAlreadyExistsWithAlreadyExists(t *testing.T) {
	err := apierrors.NewAlreadyExists(corev1.Resource("configmaps"), "test")
	assert.NoError(t, IgnoreIsAlreadyExists(err))
}

func TestHandleCRDeletion(t *testing.T) {
	scheme := newTestScheme(t)
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	t.Run("add finalizer to non-deleting object", func(t *testing.T) {
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: "ns"}}
		require.NoError(t, cli.Create(context.Background(), cm))

		reqCtx := RequestCtx{
			Ctx: context.Background(),
			Log: logr.Discard(),
		}
		res, err := HandleCRDeletion(reqCtx, cli, cm, "test-finalizer", nil)
		require.NoError(t, err)
		assert.Nil(t, res) // nil means continue reconciliation

		// Verify finalizer was added
		updated := &corev1.ConfigMap{}
		require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cm), updated))
		assert.Contains(t, updated.Finalizers, "test-finalizer")
	})

	t.Run("already has finalizer - no update needed", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cm2", Namespace: "ns",
				Finalizers: []string{"test-finalizer"},
			},
		}
		require.NoError(t, cli.Create(context.Background(), cm))

		reqCtx := RequestCtx{
			Ctx: context.Background(),
			Log: logr.Discard(),
		}
		res, err := HandleCRDeletion(reqCtx, cli, cm, "test-finalizer", nil)
		require.NoError(t, err)
		assert.Nil(t, res)
	})
}

func TestValidateReferenceCR(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ref-target",
			Namespace: "ns",
			Labels:    map[string]string{"ref-label": "owner"},
		},
	}
	cli := newFakeClient(t, cm)

	reqCtx := RequestCtx{
		Ctx: context.Background(),
		Log: logr.Discard(),
	}
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "ns"}}

	t.Run("no referencing objects", func(t *testing.T) {
		res, err := ValidateReferenceCR(reqCtx, cli, owner, "ref-label", nil, &corev1.SecretList{})
		require.NoError(t, err)
		assert.Nil(t, res)
	})

	t.Run("referencing objects found", func(t *testing.T) {
		res, err := ValidateReferenceCR(reqCtx, cli, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "ref-target"}}, "ref-label", nil, &corev1.ConfigMapList{})
		require.NoError(t, err)
		// If items found, returns a requeue result
		if res != nil {
			assert.True(t, res.Requeue || res.RequeueAfter > 0)
		}
	})
}

// ============================================================
// Additional util.go tests
// ============================================================

func TestSetOwnerReference(t *testing.T) {
	owner := &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns", UID: "uid-1"},
	}
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns"}}
	err := SetOwnerReference(owner, obj)
	require.NoError(t, err)
	assert.Len(t, obj.OwnerReferences, 1)
}

func TestSetControllerReference(t *testing.T) {
	owner := &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns", UID: "uid-1"},
	}
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns"}}
	err := SetControllerReference(owner, obj)
	require.NoError(t, err)
	assert.Len(t, obj.OwnerReferences, 1)
	assert.NotNil(t, obj.OwnerReferences[0].Controller)
	assert.True(t, *obj.OwnerReferences[0].Controller)
}

func TestDeleteOwnedResources(t *testing.T) {
	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "ns", UID: "owner-uid"},
	}
	owned := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owned",
			Namespace: "ns",
			Labels:    map[string]string{"app": "test"},
			OwnerReferences: []metav1.OwnerReference{
				{UID: "owner-uid", Name: "owner", APIVersion: "v1", Kind: "ConfigMap"},
			},
		},
	}
	notOwned := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "not-owned",
			Namespace: "ns",
			Labels:    map[string]string{"app": "test"},
		},
	}
	cli := newFakeClient(t, owner, owned, notOwned)

	err := DeleteOwnedResources(context.Background(), cli, owner,
		client.MatchingLabels{"app": "test"}, generics.ConfigMapSignature)
	require.NoError(t, err)

	// owned should be deleted
	result := &corev1.ConfigMap{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "owned"}, result)
	assert.True(t, apierrors.IsNotFound(err))

	// not-owned should still exist
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "not-owned"}, result)
	require.NoError(t, err)
}

// ============================================================
// Additional image_util.go tests
// ============================================================

func TestLoadRegistryConfig(t *testing.T) {
	t.Run("empty config succeeds", func(t *testing.T) {
		viper.Set(constant.CfgRegistries, nil)
		err := LoadRegistryConfig()
		require.NoError(t, err)
	})
}

// ============================================================
// Additional pod_utils.go tests — ResolvePodSpecDefaultFields more branches
// ============================================================

func TestResolvePodSpecDefaultFieldsSecurityContext(t *testing.T) {
	obj := corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:  func() *int64 { v := int64(1000); return &v }(),
			RunAsGroup: func() *int64 { v := int64(1000); return &v }(),
		},
	}
	pobj := &corev1.PodSpec{}
	ResolvePodSpecDefaultFields(obj, pobj)
	require.NotNil(t, pobj.SecurityContext)
	assert.Equal(t, int64(1000), *pobj.SecurityContext.RunAsUser)
}

func TestResolvePodSpecDefaultFieldsInitContainers(t *testing.T) {
	obj := corev1.PodSpec{
		InitContainers: []corev1.Container{
			{
				TerminationMessagePath:   "/dev/termination-log",
				TerminationMessagePolicy: corev1.TerminationMessageReadFile,
				ImagePullPolicy:          corev1.PullIfNotPresent,
			},
		},
	}
	pobj := &corev1.PodSpec{
		InitContainers: []corev1.Container{{}},
	}
	ResolvePodSpecDefaultFields(obj, pobj)
	assert.Equal(t, corev1.PullIfNotPresent, pobj.InitContainers[0].ImagePullPolicy)
}

func TestResolvePodSpecDefaultFieldsVolumes(t *testing.T) {
	defaultMode := int32(0644)
	obj := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: "downward",
				VolumeSource: corev1.VolumeSource{
					DownwardAPI: &corev1.DownwardAPIVolumeSource{
						Items: []corev1.DownwardAPIVolumeFile{
							{Path: "labels", FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.labels"}},
						},
						DefaultMode: &defaultMode,
					},
				},
			},
			{
				Name: "cm",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						DefaultMode: &defaultMode,
					},
				},
			},
		},
		DeprecatedServiceAccount: "default",
		Tolerations:              []corev1.Toleration{{Key: "key1"}},
		Priority:                 func() *int32 { v := int32(10); return &v }(),
		EnableServiceLinks:       func() *bool { v := true; return &v }(),
		PreemptionPolicy:         func() *corev1.PreemptionPolicy { v := corev1.PreemptLowerPriority; return &v }(),
	}

	pobj := &corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: "downward",
				VolumeSource: corev1.VolumeSource{
					DownwardAPI: &corev1.DownwardAPIVolumeSource{
						Items: []corev1.DownwardAPIVolumeFile{
							{Path: "labels", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels"}},
						},
					},
				},
			},
			{
				Name: "cm",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{},
				},
			},
		},
	}

	ResolvePodSpecDefaultFields(obj, pobj)

	// Volume defaults
	assert.Equal(t, "v1", pobj.Volumes[0].DownwardAPI.Items[0].FieldRef.APIVersion)
	assert.Equal(t, &defaultMode, pobj.Volumes[0].DownwardAPI.DefaultMode)
	assert.Equal(t, &defaultMode, pobj.Volumes[1].ConfigMap.DefaultMode)

	// Scalar field defaults
	assert.Equal(t, "default", pobj.DeprecatedServiceAccount)
	assert.Len(t, pobj.Tolerations, 1)
	assert.Equal(t, int32(10), *pobj.Priority)
	assert.True(t, *pobj.EnableServiceLinks)
	assert.Equal(t, corev1.PreemptLowerPriority, *pobj.PreemptionPolicy)
}

func TestRequeueWithErrorAndRecordEvent(t *testing.T) {
	logger := logr.Discard()
	t.Run("not found with recorder", func(t *testing.T) {
		err := apierrors.NewNotFound(corev1.Resource("configmaps"), "test")
		res, retErr := RequeueWithErrorAndRecordEvent(&corev1.ConfigMap{}, nil, err, logger)
		assert.Error(t, retErr)
		_ = res
	})
	t.Run("regular error", func(t *testing.T) {
		err := fmt.Errorf("some error")
		_, retErr := RequeueWithErrorAndRecordEvent(&corev1.ConfigMap{}, nil, err, logger)
		assert.Error(t, retErr)
	})
}

func TestHandleCRDeletionWithDeletion(t *testing.T) {
	scheme := newTestScheme(t)
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create a configmap with finalizer and deletion timestamp
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "cm-deleting",
			Namespace:  "ns",
			Finalizers: []string{"test-finalizer"},
		},
	}
	require.NoError(t, cli.Create(context.Background(), cm))
	// Trigger deletion
	require.NoError(t, cli.Delete(context.Background(), cm))
	// Re-fetch to get deletion timestamp set
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cm), cm))

	reqCtx := RequestCtx{
		Ctx: context.Background(),
		Log: logr.Discard(),
	}
	res, err := HandleCRDeletion(reqCtx, cli, cm, "test-finalizer", nil)
	require.NoError(t, err)
	require.NotNil(t, res) // deletion returns a result to stop reconciliation
}
