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

package util

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// --- shouldAllocateNodePorts ---

func TestShouldAllocateNodePorts_NodePort(t *testing.T) {
	svc := &corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort}
	assert.True(t, shouldAllocateNodePorts(svc))
}

func TestShouldAllocateNodePorts_LoadBalancer_Default(t *testing.T) {
	svc := &corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer}
	assert.True(t, shouldAllocateNodePorts(svc))
}

func TestShouldAllocateNodePorts_LoadBalancer_Enabled(t *testing.T) {
	svc := &corev1.ServiceSpec{
		Type:                          corev1.ServiceTypeLoadBalancer,
		AllocateLoadBalancerNodePorts: pointer.Bool(true),
	}
	assert.True(t, shouldAllocateNodePorts(svc))
}

func TestShouldAllocateNodePorts_LoadBalancer_Disabled(t *testing.T) {
	svc := &corev1.ServiceSpec{
		Type:                          corev1.ServiceTypeLoadBalancer,
		AllocateLoadBalancerNodePorts: pointer.Bool(false),
	}
	assert.False(t, shouldAllocateNodePorts(svc))
}

func TestShouldAllocateNodePorts_ClusterIP(t *testing.T) {
	svc := &corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP}
	assert.False(t, shouldAllocateNodePorts(svc))
}

func TestShouldAllocateNodePorts_ExternalName(t *testing.T) {
	svc := &corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName}
	assert.False(t, shouldAllocateNodePorts(svc))
}

// --- ResolveServiceDefaultFields ---

func TestResolveServiceDefaultFields_ClusterIPCarriedOver(t *testing.T) {
	old := &corev1.ServiceSpec{
		ClusterIP:  "10.0.0.1",
		ClusterIPs: []string{"10.0.0.1"},
	}
	new := &corev1.ServiceSpec{}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, "10.0.0.1", new.ClusterIP)
	assert.Equal(t, []string{"10.0.0.1"}, new.ClusterIPs)
}

func TestResolveServiceDefaultFields_ClusterIPNotOverwritten(t *testing.T) {
	old := &corev1.ServiceSpec{ClusterIP: "10.0.0.1"}
	new := &corev1.ServiceSpec{ClusterIP: "10.0.0.2"}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, "10.0.0.2", new.ClusterIP)
}

func TestResolveServiceDefaultFields_TypeCarriedOver(t *testing.T) {
	old := &corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort}
	new := &corev1.ServiceSpec{}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, corev1.ServiceTypeNodePort, new.Type)
}

func TestResolveServiceDefaultFields_SessionAffinityCarriedOver(t *testing.T) {
	old := &corev1.ServiceSpec{SessionAffinity: corev1.ServiceAffinityClientIP}
	new := &corev1.ServiceSpec{}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, corev1.ServiceAffinityClientIP, new.SessionAffinity)
}

func TestResolveServiceDefaultFields_IPFamilyPolicyCarriedOver(t *testing.T) {
	policy := corev1.IPFamilyPolicyPreferDualStack
	old := &corev1.ServiceSpec{IPFamilyPolicy: &policy}
	new := &corev1.ServiceSpec{}
	ResolveServiceDefaultFields(old, new)
	require.NotNil(t, new.IPFamilyPolicy)
	assert.Equal(t, corev1.IPFamilyPolicyPreferDualStack, *new.IPFamilyPolicy)
}

func TestResolveServiceDefaultFields_InternalTrafficPolicyCarriedOver(t *testing.T) {
	itp := corev1.ServiceInternalTrafficPolicyLocal
	old := &corev1.ServiceSpec{InternalTrafficPolicy: &itp}
	new := &corev1.ServiceSpec{}
	ResolveServiceDefaultFields(old, new)
	require.NotNil(t, new.InternalTrafficPolicy)
	assert.Equal(t, corev1.ServiceInternalTrafficPolicyLocal, *new.InternalTrafficPolicy)
}

func TestResolveServiceDefaultFields_ExternalTrafficPolicyCarriedOver(t *testing.T) {
	old := &corev1.ServiceSpec{ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal}
	new := &corev1.ServiceSpec{}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, corev1.ServiceExternalTrafficPolicyLocal, new.ExternalTrafficPolicy)
}

func TestResolveServiceDefaultFields_ExternalTrafficPolicyNotOverwritten(t *testing.T) {
	old := &corev1.ServiceSpec{ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal}
	new := &corev1.ServiceSpec{ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, corev1.ServiceExternalTrafficPolicyCluster, new.ExternalTrafficPolicy)
}

func TestResolveServiceDefaultFields_IPFamiliesCarriedOver_Empty(t *testing.T) {
	old := &corev1.ServiceSpec{IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}}
	new := &corev1.ServiceSpec{}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}, new.IPFamilies)
}

func TestResolveServiceDefaultFields_IPFamiliesCarriedOver_SingleNonSingleStack(t *testing.T) {
	policy := corev1.IPFamilyPolicyPreferDualStack
	old := &corev1.ServiceSpec{IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}}
	new := &corev1.ServiceSpec{
		IPFamilies:     []corev1.IPFamily{corev1.IPv4Protocol},
		IPFamilyPolicy: &policy,
	}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}, new.IPFamilies)
}

func TestResolveServiceDefaultFields_IPFamiliesKept_SingleStack(t *testing.T) {
	policy := corev1.IPFamilyPolicySingleStack
	old := &corev1.ServiceSpec{IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}}
	new := &corev1.ServiceSpec{
		IPFamilies:     []corev1.IPFamily{corev1.IPv6Protocol},
		IPFamilyPolicy: &policy,
	}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, []corev1.IPFamily{corev1.IPv6Protocol}, new.IPFamilies)
}

func TestResolveServiceDefaultFields_NodePortCarriedOver(t *testing.T) {
	old := &corev1.ServiceSpec{
		Type: corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{
			{Port: 80, NodePort: 30080, TargetPort: intstr.FromInt(8080)},
		},
	}
	new := &corev1.ServiceSpec{
		Type: corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{
			{Port: 80},
		},
	}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, int32(30080), new.Ports[0].NodePort)
}

func TestResolveServiceDefaultFields_NodePortNotCarriedForClusterIP(t *testing.T) {
	old := &corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{Port: 80, NodePort: 30080},
		},
	}
	new := &corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{
			{Port: 80},
		},
	}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, int32(0), new.Ports[0].NodePort)
}

func TestResolveServiceDefaultFields_TargetPortCarriedOver(t *testing.T) {
	old := &corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{Port: 80, TargetPort: intstr.FromInt(8080)},
		},
	}
	new := &corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{Port: 80},
		},
	}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, intstr.FromInt(8080), new.Ports[0].TargetPort)
}

func TestResolveServiceDefaultFields_TargetPortNotOverwritten(t *testing.T) {
	old := &corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{Port: 80, TargetPort: intstr.FromInt(8080)},
		},
	}
	new := &corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{Port: 80, TargetPort: intstr.FromInt(9090)},
		},
	}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, intstr.FromInt(9090), new.Ports[0].TargetPort)
}

func TestResolveServiceDefaultFields_NewPortAdded(t *testing.T) {
	old := &corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{Port: 80, TargetPort: intstr.FromInt(8080)},
		},
	}
	new := &corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{Port: 80},
			{Port: 443},
		},
	}
	ResolveServiceDefaultFields(old, new)
	// Port 80 gets TargetPort from old, port 443 stays as-is
	assert.Equal(t, intstr.FromInt(8080), new.Ports[0].TargetPort)
	assert.Equal(t, intstr.IntOrString{}, new.Ports[1].TargetPort)
}

func TestResolveServiceDefaultFields_EmptySpecs(t *testing.T) {
	old := &corev1.ServiceSpec{}
	new := &corev1.ServiceSpec{}
	ResolveServiceDefaultFields(old, new)
	assert.Equal(t, &corev1.ServiceSpec{}, new)
}

// --- SendWarningEventWithError ---

func TestSendWarningEventWithError_NilError(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"}}
	SendWarningEventWithError(recorder, pod, "TestReason", nil)
	assert.Empty(t, recorder.Events)
}

func TestSendWarningEventWithError_RequeueError(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"}}
	err := intctrlutil.NewRequeueError(time.Second, "retry later")
	SendWarningEventWithError(recorder, pod, "TestReason", err)
	assert.Empty(t, recorder.Events)
}

func TestSendWarningEventWithError_RegularError(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"}}
	SendWarningEventWithError(recorder, pod, "TestReason", fmt.Errorf("something failed"))
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, "Warning")
		assert.Contains(t, event, "TestReason")
		assert.Contains(t, event, "something failed")
	default:
		t.Fatal("expected a warning event")
	}
}

func TestSendWarningEventWithError_ControllerError(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"}}
	controllerErr := intctrlutil.NewError(intctrlutil.ErrorTypeNotFound, "resource gone")
	SendWarningEventWithError(recorder, pod, "OriginalReason", controllerErr)
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, "Warning")
		assert.Contains(t, event, string(intctrlutil.ErrorTypeNotFound))
		assert.Contains(t, event, "resource gone")
	default:
		t.Fatal("expected a warning event")
	}
}

// --- GetRestorePassword ---

func TestGetRestorePassword_EmptyAnnotation(t *testing.T) {
	annotations := map[string]string{}
	assert.Empty(t, GetRestorePassword(annotations, "comp1"))
}

func TestGetRestorePassword_InvalidJSON(t *testing.T) {
	annotations := map[string]string{
		constant.RestoreFromBackupAnnotationKey: "not-json",
	}
	assert.Empty(t, GetRestorePassword(annotations, "comp1"))
}

func TestGetRestorePassword_MissingComponent(t *testing.T) {
	annotations := map[string]string{
		constant.RestoreFromBackupAnnotationKey: `{"other-comp":{"connectionPassword":"pwd"}}`,
	}
	assert.Empty(t, GetRestorePassword(annotations, "comp1"))
}

func TestGetRestorePassword_MissingPasswordKey(t *testing.T) {
	annotations := map[string]string{
		constant.RestoreFromBackupAnnotationKey: `{"comp1":{"someOtherKey":"val"}}`,
	}
	assert.Empty(t, GetRestorePassword(annotations, "comp1"))
}

func TestGetRestorePassword_ValidPassword(t *testing.T) {
	encryptor := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey))
	encrypted, err := encryptor.Encrypt([]byte("my-secret"))
	require.NoError(t, err)

	annotations := map[string]string{
		constant.RestoreFromBackupAnnotationKey: fmt.Sprintf(`{"comp1":{"%s":"%s"}}`, constant.ConnectionPassword, encrypted),
	}
	result := GetRestorePassword(annotations, "comp1")
	assert.Equal(t, "my-secret", result)
}

// --- MockReader.Get ---

func TestMockReader_Get_Hit(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"},
	}
	reader := &MockReader{Objects: []client.Object{pod}}

	got := &corev1.Pod{}
	err := reader.Get(t.Context(), client.ObjectKey{Name: "pod-1", Namespace: "default"}, got)
	require.NoError(t, err)
	assert.Equal(t, "pod-1", got.Name)
}

func TestMockReader_Get_Miss(t *testing.T) {
	reader := &MockReader{Objects: []client.Object{}}
	got := &corev1.Pod{}
	err := reader.Get(t.Context(), client.ObjectKey{Name: "missing", Namespace: "default"}, got)
	assert.Error(t, err)
}

// --- MockReader.List ---

func TestMockReader_List_Empty(t *testing.T) {
	reader := &MockReader{}
	list := &corev1.PodList{}
	err := reader.List(t.Context(), list)
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestMockReader_List_WithObjects(t *testing.T) {
	pod1 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"}}
	pod2 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "default"}}
	reader := &MockReader{Objects: []client.Object{pod1, pod2}}

	list := &corev1.PodList{}
	err := reader.List(t.Context(), list)
	require.NoError(t, err)
	assert.Len(t, list.Items, 2)
}

func TestMockReader_List_WithLabelSelector(t *testing.T) {
	pod1 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Labels: map[string]string{"app": "web"}}}
	pod2 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Labels: map[string]string{"app": "api"}}}
	reader := &MockReader{Objects: []client.Object{pod1, pod2}}

	list := &corev1.PodList{}
	err := reader.List(t.Context(), list, client.MatchingLabels{"app": "web"})
	require.NoError(t, err)
	assert.Len(t, list.Items, 1)
	assert.Equal(t, "pod-1", list.Items[0].Name)
}

func TestMockReader_List_MixedTypes(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc-1"}}
	reader := &MockReader{Objects: []client.Object{pod, svc}}

	list := &corev1.PodList{}
	err := reader.List(t.Context(), list)
	require.NoError(t, err)
	assert.Len(t, list.Items, 1)
	assert.Equal(t, "pod-1", list.Items[0].Name)
}

// --- RequeueDuration ---

func TestRequeueDuration(t *testing.T) {
	assert.Equal(t, time.Millisecond*1000, RequeueDuration)
}
