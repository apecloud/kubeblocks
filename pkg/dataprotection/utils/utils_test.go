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

package utils

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	vsv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestGetBackupStatusTarget(t *testing.T) {
	sourceTargetName := "test-target"
	backupTarget := dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			Name: sourceTargetName,
		},
		SelectedTargetPods: []string{"pod-0"},
	}
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			Target: &backupTarget,
		},
	}
	target := GetBackupStatusTarget(backup, "")
	assert.Equal(t, *target, backupTarget)

	backup.Status.Target = nil
	backup.Status.Targets = []dpv1alpha1.BackupStatusTarget{backupTarget}
	target = GetBackupStatusTarget(backup, sourceTargetName)
	assert.Equal(t, *target, backupTarget)

	target = GetBackupStatusTarget(backup, "test")
	if target != nil {
		assert.Error(t, errors.New("backup status target should be empty"))
	}
}

func TestDataprotectionUtilityHelpers(t *testing.T) {
	assert.Equal(t, "backup-0-data", GetBackupVolumeSnapshotName("backup", "data", 0))
	assert.Equal(t, "backup-data", GetOldBackupVolumeSnapshotName("backup", "data"))

	env := MergeEnv([]corev1.EnvVar{{Name: "A", Value: "1"}, {Name: "B", Value: "2"}}, []corev1.EnvVar{{Name: "B", Value: "override"}, {Name: "C", Value: "3"}})
	assert.Equal(t, map[string]string{"A": "1", "B": "override", "C": "3"}, CovertEnvToMap(env))
	assert.NotContains(t, CovertEnvToMap([]corev1.EnvVar{{Name: "FROM", ValueFrom: &corev1.EnvVarSource{}}}), "FROM")

	assert.Equal(t, "  one\n  two", PrependSpaces("one\ntwo", 2))
	assert.True(t, ExistTargetVolume(&dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data"}}, "data"))
	assert.True(t, ExistTargetVolume(&dpv1alpha1.TargetVolumeInfo{VolumeMounts: []corev1.VolumeMount{{Name: "logs"}}}, "logs"))
	assert.False(t, ExistTargetVolume(&dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data"}}, "logs"))

	assert.Equal(t, dpv1alpha1.BackupTypeFull, GetBackupType(nil, pointer.Bool(true)))
	assert.Empty(t, GetBackupType(nil, pointer.Bool(false)))
	assert.Equal(t, dpv1alpha1.BackupTypeIncremental, GetBackupType(&dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeIncremental}}, pointer.Bool(true)))
}

func TestJobAndPodHelpers(t *testing.T) {
	finished, conditionType, message := IsJobFinished(nil)
	assert.False(t, finished)
	assert.Empty(t, conditionType)
	assert.Empty(t, message)

	finished, conditionType, message = IsJobFinished(&batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
		{Type: batchv1.JobFailed, Status: corev1.ConditionFalse},
		{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
	}}})
	assert.True(t, finished)
	assert.Equal(t, batchv1.JobComplete, conditionType)
	assert.Empty(t, message)

	finished, conditionType, message = IsJobFinished(&batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "Backoff", Message: "failed"},
	}}})
	assert.True(t, finished)
	assert.Equal(t, batchv1.JobFailed, conditionType)
	assert.Equal(t, "Backoff:failed", message)

	pods := &corev1.PodList{Items: []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pod-0"}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}},
	}}
	assert.Equal(t, "pod-0", GetFirstIndexRunningPod(pods).Name)
	assert.Equal(t, "pod-1", GetPodByName(pods, "pod-1").Name)
	assert.Nil(t, GetPodByName(pods, "missing"))

	pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Ports: []corev1.ContainerPort{{Name: "mysql", ContainerPort: 3306}}}}}}
	assert.Equal(t, int32(3306), GetPodFirstContainerPort(pod))
	env, err := GetDPDBPortEnv(pod, &dpv1alpha1.ContainerPort{ContainerName: "db", PortName: "mysql"})
	assert.NoError(t, err)
	assert.Equal(t, "3306", env.Value)
	_, err = GetDPDBPortEnv(pod, &dpv1alpha1.ContainerPort{ContainerName: "db", PortName: "missing"})
	assert.Error(t, err)
}

func TestFakeClientHelpers(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))

	ctx := context.Background()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns", Labels: map[string]string{"app": "db"}}}
	jobPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "job-pod", Namespace: "ns", Labels: map[string]string{"job-name": "backup-job"}}}
	backupPolicy := &dpv1alpha1.BackupPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "ns"},
		Spec:       dpv1alpha1.BackupPolicySpec{BackupMethods: []dpv1alpha1.BackupMethod{{Name: "full", SnapshotVolumes: pointer.Bool(true)}}},
	}
	actionSet := &dpv1alpha1.ActionSet{ObjectMeta: metav1.ObjectMeta{Name: "action-set"}, Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeFull}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod, jobPod, backupPolicy, actionSet).Build()
	reqCtx := intctrlutil.RequestCtx{Ctx: ctx, Req: ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "ns", Name: "req"}}}

	pods, err := GetPodListByLabelSelector(reqCtx, cli, &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}})
	assert.NoError(t, err)
	assert.Len(t, pods.Items, 1)

	jobPods, err := GetAssociatedPodsOfJob(ctx, cli, "ns", "backup-job")
	assert.NoError(t, err)
	assert.Len(t, jobPods.Items, 1)

	gotPolicy, err := GetBackupPolicyByName(reqCtx, cli, "policy")
	assert.NoError(t, err)
	assert.Equal(t, "policy", gotPolicy.Name)
	assert.Equal(t, "full", GetBackupMethodByName("full", gotPolicy).Name)
	assert.Nil(t, GetBackupMethodByName("missing", gotPolicy))

	gotActionSet, err := GetActionSetByName(reqCtx, cli, "action-set")
	assert.NoError(t, err)
	assert.Equal(t, dpv1alpha1.BackupTypeFull, gotActionSet.Spec.BackupType)
	gotType, err := GetBackupTypeByMethodName(reqCtx, cli, "full", gotPolicy)
	assert.NoError(t, err)
	assert.Equal(t, dpv1alpha1.BackupTypeFull, gotType)
	gotType, err = GetBackupTypeByMethodName(reqCtx, cli, "missing", gotPolicy)
	assert.NoError(t, err)
	assert.Empty(t, gotType)

	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "ns", Finalizers: []string{dptypes.DataProtectionFinalizerName}}}
	assert.NoError(t, cli.Create(ctx, cm))
	assert.NoError(t, RemoveDataProtectionFinalizer(ctx, cli, cm))
	assert.Empty(t, cm.Finalizers)
}

func TestBuildEnvByTargetAndParameters(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "ns"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Ports: []corev1.ContainerPort{{Name: "mysql", ContainerPort: 3306}}}}},
	}

	envs, err := BuildEnvByTarget(pod, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, dptypes.DPDBHost, envs[0].Name)
	assert.Equal(t, dptypes.DPDBPort, envs[1].Name)
	assert.Equal(t, "3306", envs[1].Value)

	envs, err = BuildEnvByTarget(pod, &dpv1alpha1.ConnectionCredential{
		SecretName:  "conn",
		HostKey:     "host",
		PortKey:     "port",
		UsernameKey: "user",
		PasswordKey: "password",
	}, nil)
	assert.NoError(t, err)
	assert.Len(t, envs, 4)
	for _, env := range envs {
		assert.Equal(t, "conn", env.ValueFrom.SecretKeyRef.Name)
	}

	params := BuildEnvByParameters([]dpv1alpha1.ParameterPair{{Name: "P1", Value: "v1"}})
	assert.Equal(t, []corev1.EnvVar{{Name: "P1", Value: "v1"}}, params)
}

func TestBackupPolicyAndScheduleHelpers(t *testing.T) {
	policies := &dpv1alpha1.BackupPolicyList{Items: []dpv1alpha1.BackupPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "default", Annotations: map[string]string{dptypes.DefaultBackupPolicyAnnotationKey: "true"}},
			Status:     dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
			Spec:       dpv1alpha1.BackupPolicySpec{BackupMethods: []dpv1alpha1.BackupMethod{{Name: "snapshot", SnapshotVolumes: pointer.Bool(true)}, {Name: "logical"}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "named", Annotations: map[string]string{}},
			Status:     dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
			Spec:       dpv1alpha1.BackupPolicySpec{BackupMethods: []dpv1alpha1.BackupMethod{{Name: "named-method"}}},
		},
	}}
	defaultMethod, methods := GetBackupMethodsFromBackupPolicy(policies, "")
	assert.Equal(t, "snapshot", defaultMethod)
	assert.Contains(t, methods, "logical")

	defaultMethod, methods = GetBackupMethodsFromBackupPolicy(policies, "named")
	assert.Empty(t, defaultMethod)
	assert.Contains(t, methods, "named-method")

	assert.NoError(t, ValidateScheduleNames([]dpv1alpha1.SchedulePolicy{{Name: "daily", BackupMethod: "full"}, {BackupMethod: "log"}}))
	assert.Error(t, ValidateScheduleNames([]dpv1alpha1.SchedulePolicy{{Name: "same", BackupMethod: "full"}, {Name: "same", BackupMethod: "log"}}))
}

func TestDatasafedInjectionHelpers(t *testing.T) {
	oldImage := viper.GetString("DATASAFED_IMAGE")
	defer viper.Set("DATASAFED_IMAGE", oldImage)
	viper.Set("DATASAFED_IMAGE", "example/datasafed:test")

	podSpec := &corev1.PodSpec{Containers: []corev1.Container{{Name: "main"}}}
	InjectDatasafedWithPVC(podSpec, "repo-pvc", "/repo", "/kopia")
	assert.Len(t, podSpec.InitContainers, 1)
	assert.Equal(t, "repo-pvc", podSpec.Volumes[0].PersistentVolumeClaim.ClaimName)
	assert.Contains(t, CovertEnvToMap(podSpec.Containers[0].Env), dptypes.DPDatasafedKopiaRepoRoot)

	podSpec = &corev1.PodSpec{Containers: []corev1.Container{{Name: "main"}}}
	InjectDatasafed(podSpec, &dpv1alpha1.BackupRepo{
		Spec:   dpv1alpha1.BackupRepoSpec{AccessMethod: dpv1alpha1.AccessMethodTool},
		Status: dpv1alpha1.BackupRepoStatus{ToolConfigSecretName: "tool-secret"},
	}, "/repo", &dpv1alpha1.EncryptionConfig{
		Algorithm:              "AES256",
		PassPhraseSecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "enc"}, Key: "key"},
	}, "/kopia")
	assert.Equal(t, "tool-secret", podSpec.Volumes[0].Secret.SecretName)
	envMap := CovertEnvToMap(podSpec.Containers[0].Env)
	assert.Equal(t, "AES256", envMap[dptypes.DPDatasafedEncryptionAlgorithm])
	assert.True(t, strings.Contains(podSpec.InitContainers[0].Command[2], "/bin/datasafed"))
}

func TestValidationAndComparisonHelpers(t *testing.T) {
	schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"name":  {Type: "string"},
			"count": {Type: "integer"},
		},
	}
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "as"},
		Spec: dpv1alpha1.ActionSetSpec{
			Backup:           &dpv1alpha1.BackupActionSpec{WithParameters: []string{"name", "count"}},
			ParametersSchema: &dpv1alpha1.ActionSetParametersSchema{OpenAPIV3Schema: schema},
		},
	}
	assert.NoError(t, ValidateParameters(actionSet, []dpv1alpha1.ParameterPair{{Name: "name", Value: "kb"}, {Name: "count", Value: "2"}}, true))
	assert.Error(t, ValidateParameters(actionSet, []dpv1alpha1.ParameterPair{{Name: "unknown", Value: "x"}}, true))
	assert.Error(t, ValidateParameters(nil, []dpv1alpha1.ParameterPair{{Name: "name", Value: "kb"}}, true))

	start := metav1.NewTime(time.Now())
	later := metav1.NewTime(start.Add(time.Hour))
	assert.True(t, CompareWithBackupStopTime(
		dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Status: dpv1alpha1.BackupStatus{CompletionTimestamp: &start}},
		dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "b"}, Status: dpv1alpha1.BackupStatus{CompletionTimestamp: &later}},
	))
	assert.True(t, CompareWithBackupStopTime(
		dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Status: dpv1alpha1.BackupStatus{CompletionTimestamp: &start}},
		dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
	))
	assert.False(t, CompareWithBackupStopTime(dpv1alpha1.Backup{}, dpv1alpha1.Backup{}))

	targets := GetBackupTargets(
		&dpv1alpha1.BackupPolicy{Spec: dpv1alpha1.BackupPolicySpec{Targets: []dpv1alpha1.BackupTarget{{Name: "policy-target"}}}},
		&dpv1alpha1.BackupMethod{Targets: []dpv1alpha1.BackupTarget{{Name: "method-target"}}},
	)
	assert.Equal(t, "method-target", targets[0].Name)
}

func TestAddTolerationsFromViper(t *testing.T) {
	oldTolerations := viper.GetString(constant.CfgKeyCtrlrMgrTolerations)
	oldAffinity := viper.GetString(constant.CfgKeyCtrlrMgrAffinity)
	oldNodeSelector := viper.GetString(constant.CfgKeyCtrlrMgrNodeSelector)
	defer func() {
		viper.Set(constant.CfgKeyCtrlrMgrTolerations, oldTolerations)
		viper.Set(constant.CfgKeyCtrlrMgrAffinity, oldAffinity)
		viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, oldNodeSelector)
	}()
	viper.Set(constant.CfgKeyCtrlrMgrTolerations, `[{"key":"dedicated","operator":"Equal","value":"dp","effect":"NoSchedule"}]`)
	viper.Set(constant.CfgKeyCtrlrMgrAffinity, `{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"disk","operator":"In","values":["ssd"]}]}]}}}`)
	viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, `{"node":"dp"}`)

	podSpec := &corev1.PodSpec{}
	assert.NoError(t, AddTolerations(podSpec))
	assert.Equal(t, "dedicated", podSpec.Tolerations[0].Key)
	assert.Equal(t, "dp", podSpec.NodeSelector["node"])

	viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, `{invalid`)
	assert.Error(t, AddTolerations(&corev1.PodSpec{}))
}

func TestSetControllerReferenceNilOwner(t *testing.T) {
	assert.NoError(t, SetControllerReference(nil, &corev1.ConfigMap{}, runtime.NewScheme()))
	assert.NoError(t, SetControllerReference((*corev1.ConfigMap)(nil), &corev1.ConfigMap{}, runtime.NewScheme()))
}

func TestVolumeSnapshotEnabledWithFakeClient(t *testing.T) {
	oldSupportsVolumeSnapshotV1 := supportsVolumeSnapshotV1
	defer func() { supportsVolumeSnapshotV1 = oldSupportsVolumeSnapshotV1 }()
	supportsVolumeSnapshotV1 = pointer.Bool(true)

	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))
	assert.NoError(t, vsv1.AddToScheme(scheme))
	assert.NoError(t, vsv1beta1.AddToScheme(scheme))

	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{}}}}}}
	ok, err := VolumeSnapshotEnabled(context.Background(), fake.NewClientBuilder().WithScheme(scheme).Build(), pod, []string{"config"})
	assert.False(t, ok)
	assert.Error(t, err)

	ok, err = VolumeSnapshotEnabled(context.Background(), fake.NewClientBuilder().WithScheme(scheme).Build(), pod, []string{"missing"})
	assert.False(t, ok)
	assert.Error(t, err)

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv"}, Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "driver"}}}}
	vsc := &vsv1.VolumeSnapshotClass{ObjectMeta: metav1.ObjectMeta{Name: "class"}, Driver: "driver"}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pv, vsc).Build()
	ok, err = IsVolumeSnapshotEnabled(context.Background(), cli, "pv")
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = IsVolumeSnapshotEnabled(context.Background(), cli, "")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestEventsToString(t *testing.T) {
	events := &corev1.EventList{Items: []corev1.Event{{
		ObjectMeta:     metav1.ObjectMeta{Name: "event"},
		Type:           corev1.EventTypeWarning,
		Reason:         "Failed",
		Message:        "backup failed",
		Count:          1,
		LastTimestamp:  metav1.Now(),
		FirstTimestamp: metav1.Now(),
	}}}
	got := EventsToString(events)
	assert.Contains(t, got, "Failed")
	assert.Contains(t, got, "backup failed")
}

func TestPeriodicalEnqueueSourceString(t *testing.T) {
	assert.Contains(t, (&PeriodicalEnqueueSource{objList: &corev1.PodList{}}).String(), "PodList")
	assert.Equal(t, "periodical enqueue source: unknown type", (&PeriodicalEnqueueSource{}).String())
}
