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

package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	_ = dpv1alpha1.AddToScheme(s)
	return s
}

func testClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).Build()
}

// --- AddTolerations ---

func TestAddTolerations_Empty(t *testing.T) {
	viper.Set(constant.CfgKeyCtrlrMgrTolerations, "")
	viper.Set(constant.CfgKeyCtrlrMgrAffinity, "")
	viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, "")
	defer func() {
		viper.Set(constant.CfgKeyCtrlrMgrTolerations, nil)
		viper.Set(constant.CfgKeyCtrlrMgrAffinity, nil)
		viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, nil)
	}()

	podSpec := &corev1.PodSpec{}
	err := AddTolerations(podSpec)
	require.NoError(t, err)
	assert.Nil(t, podSpec.Tolerations)
	assert.Nil(t, podSpec.Affinity)
	assert.Nil(t, podSpec.NodeSelector)
}

func TestAddTolerations_WithTolerations(t *testing.T) {
	viper.Set(constant.CfgKeyCtrlrMgrTolerations, `[{"key":"k","operator":"Equal","value":"v","effect":"NoSchedule"}]`)
	defer viper.Set(constant.CfgKeyCtrlrMgrTolerations, nil)

	podSpec := &corev1.PodSpec{}
	err := AddTolerations(podSpec)
	require.NoError(t, err)
	require.Len(t, podSpec.Tolerations, 1)
	assert.Equal(t, "k", podSpec.Tolerations[0].Key)
}

func TestAddTolerations_WithAffinity(t *testing.T) {
	viper.Set(constant.CfgKeyCtrlrMgrAffinity, `{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"zone","operator":"In","values":["us-east-1"]}]}]}}}`)
	defer viper.Set(constant.CfgKeyCtrlrMgrAffinity, nil)

	podSpec := &corev1.PodSpec{}
	err := AddTolerations(podSpec)
	require.NoError(t, err)
	require.NotNil(t, podSpec.Affinity)
	require.NotNil(t, podSpec.Affinity.NodeAffinity)
}

func TestAddTolerations_WithNodeSelector(t *testing.T) {
	viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, `{"zone":"us-east-1"}`)
	defer viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, nil)

	podSpec := &corev1.PodSpec{}
	err := AddTolerations(podSpec)
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", podSpec.NodeSelector["zone"])
}

func TestAddTolerations_InvalidJSON(t *testing.T) {
	viper.Set(constant.CfgKeyCtrlrMgrTolerations, `not-json`)
	defer viper.Set(constant.CfgKeyCtrlrMgrTolerations, nil)

	podSpec := &corev1.PodSpec{}
	err := AddTolerations(podSpec)
	require.Error(t, err)
}

// --- IsJobFinished ---

func TestIsJobFinished_Nil(t *testing.T) {
	finished, condType, msg := IsJobFinished(nil)
	assert.False(t, finished)
	assert.Equal(t, batchv1.JobConditionType(""), condType)
	assert.Empty(t, msg)
}

func TestIsJobFinished_Complete(t *testing.T) {
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}
	finished, condType, msg := IsJobFinished(job)
	assert.True(t, finished)
	assert.Equal(t, batchv1.JobComplete, condType)
	assert.Empty(t, msg)
}

func TestIsJobFinished_Failed(t *testing.T) {
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded", Message: "too many retries"},
			},
		},
	}
	finished, condType, msg := IsJobFinished(job)
	assert.True(t, finished)
	assert.Equal(t, batchv1.JobFailed, condType)
	assert.Equal(t, "BackoffLimitExceeded:too many retries", msg)
}

func TestIsJobFinished_NotFinished(t *testing.T) {
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionFalse},
			},
		},
	}
	finished, _, _ := IsJobFinished(job)
	assert.False(t, finished)
}

// --- GetAssociatedPodsOfJob ---

func TestGetAssociatedPodsOfJob(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": "test-job"},
		},
	}
	cli := testClient(pod)
	podList, err := GetAssociatedPodsOfJob(context.Background(), cli, "default", "test-job")
	require.NoError(t, err)
	assert.Len(t, podList.Items, 1)
	assert.Equal(t, "test-pod", podList.Items[0].Name)
}

func TestGetAssociatedPodsOfJob_Empty(t *testing.T) {
	cli := testClient()
	podList, err := GetAssociatedPodsOfJob(context.Background(), cli, "default", "no-such-job")
	require.NoError(t, err)
	assert.Empty(t, podList.Items)
}

// --- RemoveDataProtectionFinalizer ---

func TestRemoveDataProtectionFinalizer_HasFinalizer(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-backup",
			Finalizers: []string{dptypes.DataProtectionFinalizerName},
		},
	}
	cli := testClient(backup)
	err := RemoveDataProtectionFinalizer(context.Background(), cli, backup)
	require.NoError(t, err)

	got := &dpv1alpha1.Backup{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(backup), got))
	assert.False(t, controllerutil.ContainsFinalizer(got, dptypes.DataProtectionFinalizerName))
}

func TestRemoveDataProtectionFinalizer_NoFinalizer(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "test-backup"},
	}
	cli := testClient(backup)
	err := RemoveDataProtectionFinalizer(context.Background(), cli, backup)
	require.NoError(t, err)
}

// --- GetActionSetByName ---

func TestGetActionSetByName_Empty(t *testing.T) {
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}
	as, err := GetActionSetByName(reqCtx, testClient(), "")
	require.NoError(t, err)
	assert.Nil(t, as)
}

func TestGetActionSetByName_Found(t *testing.T) {
	as := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-as"},
	}
	cli := testClient(as)
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}
	got, err := GetActionSetByName(reqCtx, cli, "test-as")
	require.NoError(t, err)
	assert.Equal(t, "test-as", got.Name)
}

func TestGetActionSetByName_NotFound(t *testing.T) {
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}
	_, err := GetActionSetByName(reqCtx, testClient(), "nonexistent")
	require.Error(t, err)
}

// --- GetBackupPolicyByName ---

func TestGetBackupPolicyByName_Found(t *testing.T) {
	bp := &dpv1alpha1.BackupPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-bp", Namespace: "default"},
	}
	cli := testClient(bp)
	reqCtx := intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Req: ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test-bp"}},
	}
	got, err := GetBackupPolicyByName(reqCtx, cli, "test-bp")
	require.NoError(t, err)
	assert.Equal(t, "test-bp", got.Name)
}

func TestGetBackupPolicyByName_NotFound(t *testing.T) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Req: ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default"}},
	}
	_, err := GetBackupPolicyByName(reqCtx, testClient(), "nonexistent")
	require.Error(t, err)
}

// --- GetBackupMethodByName ---

func TestGetBackupMethodByName_Found(t *testing.T) {
	bp := &dpv1alpha1.BackupPolicy{
		Spec: dpv1alpha1.BackupPolicySpec{
			BackupMethods: []dpv1alpha1.BackupMethod{
				{Name: "m1"},
				{Name: "m2"},
			},
		},
	}
	got := GetBackupMethodByName("m2", bp)
	require.NotNil(t, got)
	assert.Equal(t, "m2", got.Name)
}

func TestGetBackupMethodByName_NotFound(t *testing.T) {
	bp := &dpv1alpha1.BackupPolicy{
		Spec: dpv1alpha1.BackupPolicySpec{
			BackupMethods: []dpv1alpha1.BackupMethod{{Name: "m1"}},
		},
	}
	got := GetBackupMethodByName("nonexistent", bp)
	assert.Nil(t, got)
}

// --- GetBackupVolumeSnapshotName / GetOldBackupVolumeSnapshotName ---

func TestGetBackupVolumeSnapshotName(t *testing.T) {
	got := GetBackupVolumeSnapshotName("my-backup", "data", 2)
	assert.Equal(t, "my-backup-2-data", got)
}

func TestGetOldBackupVolumeSnapshotName(t *testing.T) {
	got := GetOldBackupVolumeSnapshotName("my-backup", "data")
	assert.Equal(t, "my-backup-data", got)
}

// --- MergeEnv ---

func TestMergeEnv_EmptyTarget(t *testing.T) {
	orig := []corev1.EnvVar{{Name: "A", Value: "1"}}
	got := MergeEnv(orig, nil)
	assert.Len(t, got, 1)
	assert.Equal(t, "1", got[0].Value)
}

func TestMergeEnv_Replace(t *testing.T) {
	orig := []corev1.EnvVar{{Name: "A", Value: "1"}, {Name: "B", Value: "2"}}
	target := []corev1.EnvVar{{Name: "A", Value: "replaced"}}
	got := MergeEnv(orig, target)
	assert.Len(t, got, 2)
	assert.Equal(t, "replaced", got[0].Value)
}

func TestMergeEnv_Append(t *testing.T) {
	orig := []corev1.EnvVar{{Name: "A", Value: "1"}}
	target := []corev1.EnvVar{{Name: "B", Value: "2"}}
	got := MergeEnv(orig, target)
	assert.Len(t, got, 2)
}

// --- SetControllerReference ---

func TestSetControllerReference_NilOwner(t *testing.T) {
	err := SetControllerReference(nil, &corev1.Pod{}, testScheme())
	require.NoError(t, err)
}

func TestSetControllerReference_ValidOwner(t *testing.T) {
	owner := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "default", UID: "uid-1"},
	}
	controlled := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "controlled", Namespace: "default"},
	}
	err := SetControllerReference(owner, controlled, testScheme())
	require.NoError(t, err)
	assert.Len(t, controlled.OwnerReferences, 1)
}

// --- CovertEnvToMap ---

func TestCovertEnvToMap(t *testing.T) {
	env := []corev1.EnvVar{
		{Name: "A", Value: "1"},
		{Name: "B", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
		{Name: "C", Value: "3"},
	}
	got := CovertEnvToMap(env)
	assert.Equal(t, "1", got["A"])
	assert.Equal(t, "3", got["C"])
	_, hasB := got["B"]
	assert.False(t, hasB)
}

func TestCovertEnvToMap_Empty(t *testing.T) {
	got := CovertEnvToMap(nil)
	assert.Empty(t, got)
}

// --- GetBackupType ---

func TestGetBackupType_WithActionSet(t *testing.T) {
	as := &dpv1alpha1.ActionSet{
		Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeFull},
	}
	got := GetBackupType(as, nil)
	assert.Equal(t, dpv1alpha1.BackupTypeFull, got)
}

func TestGetBackupType_SnapshotTrue(t *testing.T) {
	b := true
	got := GetBackupType(nil, &b)
	assert.Equal(t, dpv1alpha1.BackupTypeFull, got)
}

func TestGetBackupType_NilBoth(t *testing.T) {
	got := GetBackupType(nil, nil)
	assert.Equal(t, dpv1alpha1.BackupType(""), got)
}

// --- PrependSpaces ---

func TestPrependSpaces_SingleLine(t *testing.T) {
	got := PrependSpaces("hello", 4)
	assert.Equal(t, "    hello", got)
}

func TestPrependSpaces_MultiLine(t *testing.T) {
	got := PrependSpaces("a\nb\nc", 2)
	assert.Equal(t, "  a\n  b\n  c", got)
}

func TestPrependSpaces_Empty(t *testing.T) {
	got := PrependSpaces("", 3)
	assert.Equal(t, "", got)
}

func TestPrependSpaces_Zero(t *testing.T) {
	got := PrependSpaces("hello", 0)
	assert.Equal(t, "hello", got)
}

// --- GetFirstIndexRunningPod ---

func TestGetFirstIndexRunningPod_Nil(t *testing.T) {
	got := GetFirstIndexRunningPod(nil)
	assert.Nil(t, got)
}

func TestGetFirstIndexRunningPod_NoneAvailable(t *testing.T) {
	podList := &corev1.PodList{
		Items: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-0"}},
		},
	}
	got := GetFirstIndexRunningPod(podList)
	assert.Nil(t, got)
}

func TestGetFirstIndexRunningPod_ReturnsFirst(t *testing.T) {
	now := metav1.Now()
	podList := &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-1"},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: now},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-0"},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: now},
					},
				},
			},
		},
	}
	got := GetFirstIndexRunningPod(podList)
	require.NotNil(t, got)
	assert.Equal(t, "pod-0", got.Name)
}

// --- GetPodByName ---

func TestGetPodByName_Nil(t *testing.T) {
	assert.Nil(t, GetPodByName(nil, "x"))
}

func TestGetPodByName_Found(t *testing.T) {
	podList := &corev1.PodList{
		Items: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-0"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
		},
	}
	got := GetPodByName(podList, "pod-1")
	require.NotNil(t, got)
	assert.Equal(t, "pod-1", got.Name)
}

func TestGetPodByName_NotFound(t *testing.T) {
	podList := &corev1.PodList{
		Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "pod-0"}}},
	}
	assert.Nil(t, GetPodByName(podList, "nonexistent"))
}

// --- GetPodFirstContainerPort ---

func TestGetPodFirstContainerPort_HasPort(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Ports: []corev1.ContainerPort{{ContainerPort: 3306}}},
			},
		},
	}
	assert.Equal(t, int32(3306), GetPodFirstContainerPort(pod))
}

func TestGetPodFirstContainerPort_NoPorts(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{}},
		},
	}
	assert.Equal(t, int32(0), GetPodFirstContainerPort(pod))
}

// --- GetDPDBPortEnv ---

func TestGetDPDBPortEnv_NilContainerPort(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Ports: []corev1.ContainerPort{{ContainerPort: 5432}}},
			},
		},
	}
	env, err := GetDPDBPortEnv(pod, nil)
	require.NoError(t, err)
	assert.Equal(t, dptypes.DPDBPort, env.Name)
	assert.Equal(t, "5432", env.Value)
}

func TestGetDPDBPortEnv_NamedPort(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "mysql",
					Ports: []corev1.ContainerPort{
						{Name: "mysql-port", ContainerPort: 3306},
					},
				},
			},
		},
	}
	cp := &dpv1alpha1.ContainerPort{ContainerName: "mysql", PortName: "mysql-port"}
	env, err := GetDPDBPortEnv(pod, cp)
	require.NoError(t, err)
	assert.Equal(t, "3306", env.Value)
}

func TestGetDPDBPortEnv_NotFound(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "other", Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}}},
			},
		},
	}
	cp := &dpv1alpha1.ContainerPort{ContainerName: "mysql", PortName: "mysql-port"}
	_, err := GetDPDBPortEnv(pod, cp)
	require.Error(t, err)
}

// --- ExistTargetVolume ---

func TestExistTargetVolume_InVolumes(t *testing.T) {
	tv := &dpv1alpha1.TargetVolumeInfo{
		Volumes: []string{"data", "log"},
	}
	assert.True(t, ExistTargetVolume(tv, "data"))
	assert.False(t, ExistTargetVolume(tv, "wal"))
}

func TestExistTargetVolume_InVolumeMounts(t *testing.T) {
	tv := &dpv1alpha1.TargetVolumeInfo{
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
		},
	}
	assert.True(t, ExistTargetVolume(tv, "data"))
	assert.False(t, ExistTargetVolume(tv, "wal"))
}

// --- GetBackupTargets ---

func TestGetBackupTargets_MethodTarget(t *testing.T) {
	target := dpv1alpha1.BackupTarget{Name: "t1"}
	method := &dpv1alpha1.BackupMethod{Target: &target}
	policy := &dpv1alpha1.BackupPolicy{}
	got := GetBackupTargets(policy, method)
	require.Len(t, got, 1)
	assert.Equal(t, "t1", got[0].Name)
}

func TestGetBackupTargets_MethodTargets(t *testing.T) {
	method := &dpv1alpha1.BackupMethod{
		Targets: []dpv1alpha1.BackupTarget{{Name: "t1"}, {Name: "t2"}},
	}
	policy := &dpv1alpha1.BackupPolicy{}
	got := GetBackupTargets(policy, method)
	assert.Len(t, got, 2)
}

func TestGetBackupTargets_PolicyTarget(t *testing.T) {
	target := dpv1alpha1.BackupTarget{Name: "pt"}
	method := &dpv1alpha1.BackupMethod{}
	policy := &dpv1alpha1.BackupPolicy{
		Spec: dpv1alpha1.BackupPolicySpec{Target: &target},
	}
	got := GetBackupTargets(policy, method)
	require.Len(t, got, 1)
	assert.Equal(t, "pt", got[0].Name)
}

func TestGetBackupTargets_PolicyTargets(t *testing.T) {
	method := &dpv1alpha1.BackupMethod{}
	policy := &dpv1alpha1.BackupPolicy{
		Spec: dpv1alpha1.BackupPolicySpec{
			Targets: []dpv1alpha1.BackupTarget{{Name: "pt1"}, {Name: "pt2"}},
		},
	}
	got := GetBackupTargets(policy, method)
	assert.Len(t, got, 2)
}

func TestGetBackupTargets_Empty(t *testing.T) {
	got := GetBackupTargets(&dpv1alpha1.BackupPolicy{}, &dpv1alpha1.BackupMethod{})
	assert.Empty(t, got)
}

// --- ValidateParameters ---

func TestValidateParameters_EmptyParams(t *testing.T) {
	err := ValidateParameters(nil, nil, true)
	require.NoError(t, err)
}

func TestValidateParameters_NilActionSet(t *testing.T) {
	params := []dpv1alpha1.ParameterPair{{Name: "k", Value: "v"}}
	err := ValidateParameters(nil, params, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actionSet is empty")
}

func TestValidateParameters_UndeclaredParam(t *testing.T) {
	as := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			Backup: &dpv1alpha1.BackupActionSpec{
				WithParameters: []string{"allowed"},
			},
		},
	}
	params := []dpv1alpha1.ParameterPair{{Name: "notallowed", Value: "v"}}
	err := ValidateParameters(as, params, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undeclared")
}

func TestValidateParameters_TooManyParams(t *testing.T) {
	as := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			Backup: &dpv1alpha1.BackupActionSpec{
				WithParameters: []string{"a"},
			},
		},
	}
	params := []dpv1alpha1.ParameterPair{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	err := ValidateParameters(as, params, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undeclared")
}

func TestValidateParameters_NilSchema(t *testing.T) {
	as := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			Backup: &dpv1alpha1.BackupActionSpec{
				WithParameters: []string{"key"},
			},
		},
	}
	params := []dpv1alpha1.ParameterPair{{Name: "key", Value: "val"}}
	err := ValidateParameters(as, params, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parametersSchema is invalid")
}

func TestValidateParameters_Valid(t *testing.T) {
	as := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			Backup: &dpv1alpha1.BackupActionSpec{
				WithParameters: []string{"name"},
			},
			ParametersSchema: &dpv1alpha1.ActionSetParametersSchema{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"name": {Type: "string"},
					},
				},
			},
		},
	}
	params := []dpv1alpha1.ParameterPair{{Name: "name", Value: "hello"}}
	err := ValidateParameters(as, params, true)
	require.NoError(t, err)
}

func TestValidateParameters_RestorePath(t *testing.T) {
	as := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			Restore: &dpv1alpha1.RestoreActionSpec{
				WithParameters: []string{"name"},
			},
			ParametersSchema: &dpv1alpha1.ActionSetParametersSchema{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"name": {Type: "string"},
					},
				},
			},
		},
	}
	params := []dpv1alpha1.ParameterPair{{Name: "name", Value: "hello"}}
	err := ValidateParameters(as, params, false)
	require.NoError(t, err)
}

// --- CompareWithBackupStopTime ---

func TestCompareWithBackupStopTime_BothZero(t *testing.T) {
	a := dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	b := dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "b"}}
	assert.False(t, CompareWithBackupStopTime(a, b))
}

func TestCompareWithBackupStopTime_IZero(t *testing.T) {
	a := dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	t1 := metav1.NewTime(time.Now())
	b := dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "b"},
		Status:     dpv1alpha1.BackupStatus{CompletionTimestamp: &t1},
	}
	assert.False(t, CompareWithBackupStopTime(a, b))
}

func TestCompareWithBackupStopTime_JZero(t *testing.T) {
	t1 := metav1.NewTime(time.Now())
	a := dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "a"},
		Status:     dpv1alpha1.BackupStatus{CompletionTimestamp: &t1},
	}
	b := dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "b"}}
	assert.True(t, CompareWithBackupStopTime(a, b))
}

func TestCompareWithBackupStopTime_EqualTimes(t *testing.T) {
	now := metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	a := dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "aaa"},
		Status:     dpv1alpha1.BackupStatus{CompletionTimestamp: &now},
	}
	b := dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bbb"},
		Status:     dpv1alpha1.BackupStatus{CompletionTimestamp: &now},
	}
	assert.True(t, CompareWithBackupStopTime(a, b))
	assert.False(t, CompareWithBackupStopTime(b, a))
}

func TestCompareWithBackupStopTime_DifferentTimes(t *testing.T) {
	t1 := metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	t2 := metav1.NewTime(time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC))
	a := dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "a"},
		Status:     dpv1alpha1.BackupStatus{CompletionTimestamp: &t1},
	}
	b := dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "b"},
		Status:     dpv1alpha1.BackupStatus{CompletionTimestamp: &t2},
	}
	assert.True(t, CompareWithBackupStopTime(a, b))
	assert.False(t, CompareWithBackupStopTime(b, a))
}

// --- VolumeSnapshotEnabled ---

func TestVolumeSnapshotEnabled_NilPod(t *testing.T) {
	ok, err := VolumeSnapshotEnabled(context.Background(), testClient(), nil, nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVolumeSnapshotEnabled_NonPVCVolume(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		},
	}
	_, err := VolumeSnapshotEnabled(context.Background(), testClient(), pod, []string{"data"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not PersistentVolumeClaim")
}

func TestVolumeSnapshotEnabled_NoMatchingVolume(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{Name: "other", VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pvc-1"},
				}},
			},
		},
	}
	_, err := VolumeSnapshotEnabled(context.Background(), testClient(), pod, []string{"data"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can not find any volume")
}

// --- GetBackupTypeByMethodName ---

func TestGetBackupTypeByMethodName_MethodNotFound(t *testing.T) {
	bp := &dpv1alpha1.BackupPolicy{
		Spec: dpv1alpha1.BackupPolicySpec{
			BackupMethods: []dpv1alpha1.BackupMethod{{Name: "m1"}},
		},
	}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}
	got, err := GetBackupTypeByMethodName(reqCtx, testClient(), "nonexistent", bp)
	require.NoError(t, err)
	assert.Equal(t, dpv1alpha1.BackupType(""), got)
}

func TestGetBackupTypeByMethodName_WithActionSet(t *testing.T) {
	as := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-as"},
		Spec:       dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeFull},
	}
	bp := &dpv1alpha1.BackupPolicy{
		Spec: dpv1alpha1.BackupPolicySpec{
			BackupMethods: []dpv1alpha1.BackupMethod{
				{Name: "m1", ActionSetName: "test-as"},
			},
		},
	}
	cli := testClient(as)
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}
	got, err := GetBackupTypeByMethodName(reqCtx, cli, "m1", bp)
	require.NoError(t, err)
	assert.Equal(t, dpv1alpha1.BackupTypeFull, got)
}

// --- GetPodListByLabelSelector ---

func TestGetPodListByLabelSelector(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "mysql"},
		},
	}
	cli := testClient(pod)
	reqCtx := intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Req: ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default"}},
	}
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "mysql"},
	}
	podList, err := GetPodListByLabelSelector(reqCtx, cli, selector)
	require.NoError(t, err)
	assert.Len(t, podList.Items, 1)
}
