/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package restore

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestRestoreJobEnvCompositionPreservesActionSetOverrideAndInjectsTargetIdentity(t *testing.T) {
	const (
		rabbitNodeName  = "RABBITMQ_NODENAME"
		backupPriority  = "BACKUP_OVER_ACTION"
		restorePriority = "RESTORE_OVER_BACKUP"
	)

	backupSet := BackupActionSet{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: "backup"},
			Status: dpv1alpha1.BackupStatus{
				BackupMethod: &dpv1alpha1.BackupMethod{Env: []corev1.EnvVar{
					{Name: backupPriority, Value: "backup"},
					{Name: restorePriority, Value: "backup"},
				}},
			},
		},
		ActionSet: &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{Env: []corev1.EnvVar{
			{Name: rabbitNodeName, Value: "rabbit@$(DP_TARGET_POD_NAME).$(K8S_SERVICE_NAME).$(POD_NAMESPACE)"},
			{Name: backupPriority, Value: "action-set"},
			{Name: restorePriority, Value: "action-set"},
		}}},
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{Name: "restore", UID: "restore-uid"},
		Spec:       dpv1alpha1.RestoreSpec{Env: []corev1.EnvVar{{Name: restorePriority, Value: "restore"}}},
	}
	targetPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "rabbitmq-2",
			Labels: map[string]string{constant.RoleLabelKey: "secondary"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "rabbitmq",
			EnvFrom: []corev1.EnvFromSource{{
				Prefix:       "WORKLOAD_",
				ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "rabbitmq-env"}},
			}},
			Env: []corev1.EnvVar{
				{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
				{Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
				{Name: "CLUSTER_COMPONENT_NAME", Value: "rabbitmq"},
				{Name: "K8S_SERVICE_NAME", Value: "$(CLUSTER_COMPONENT_NAME)-headless"},
				{Name: rabbitNodeName, Value: "rabbit@$(POD_NAME).$(K8S_SERVICE_NAME).$(POD_NAMESPACE)"},
				{Name: backupPriority, Value: "workload"},
				{Name: restorePriority, Value: "workload"},
			},
		}}},
	}

	job := newRestoreJobBuilder(restore, backupSet, nil, dpv1alpha1.PostReady).
		setImage("busybox").
		addCommonEnv("source-rabbitmq-0").
		addTargetPodAndCredentialEnv(targetPod, nil, &dpv1alpha1.BackupTarget{}).
		build()

	env := job.Spec.Template.Spec.Containers[0].Env
	requireUniqueEnvNames(t, env)
	require.Equal(t, "rabbit@$(DP_TARGET_POD_NAME).$(K8S_SERVICE_NAME).$(POD_NAMESPACE)", envValue(t, env, rabbitNodeName))
	require.Equal(t, "backup", envValue(t, env, backupPriority))
	require.Equal(t, "restore", envValue(t, env, restorePriority))
	require.Equal(t, targetPod.Name, envValue(t, env, dptypes.DPTargetPodName))
	require.Equal(t, "secondary", envValue(t, env, dptypes.DPTargetPodRole))
	require.Equal(t, targetPod.Spec.Containers[0].EnvFrom, job.Spec.Template.Spec.Containers[0].EnvFrom)

	// Kubernetes expands only variables that appear earlier in the explicit
	// env list. The framework-provided target identity and the inherited
	// workload dependencies must therefore precede the ActionSet override.
	require.Less(t, envIndex(t, env, dptypes.DPTargetPodName), envIndex(t, env, rabbitNodeName))
	require.Less(t, envIndex(t, env, "K8S_SERVICE_NAME"), envIndex(t, env, rabbitNodeName))
	require.Less(t, envIndex(t, env, "POD_NAMESPACE"), envIndex(t, env, rabbitNodeName))
}

func TestRestoreJobEnvCompositionKeepsSourceAndCurrentTargetIdentitiesDistinct(t *testing.T) {
	backupSet := BackupActionSet{
		Backup:    &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup"}},
		ActionSet: &dpv1alpha1.ActionSet{},
	}
	restore := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "restore", UID: "restore-uid"}}
	for i, role := range []string{"primary", "secondary", "learner"} {
		targetName := "target-rabbitmq-" + string(rune('0'+i))
		sourceName := "source-rabbitmq-" + string(rune('0'+i))
		t.Run(targetName, func(t *testing.T) {
			targetPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   targetName,
					Labels: map[string]string{constant.RoleLabelKey: role},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "rabbitmq"}}},
			}

			job := newRestoreJobBuilder(restore, backupSet, nil, dpv1alpha1.PostReady).
				setImage("busybox").
				addCommonEnv(sourceName).
				addTargetPodAndCredentialEnv(targetPod, nil, &dpv1alpha1.BackupTarget{}).
				build()

			env := job.Spec.Template.Spec.Containers[0].Env
			requireUniqueEnvNames(t, env)
			require.Equal(t, targetName, envValue(t, env, dptypes.DPTargetPodName))
			require.Equal(t, role, envValue(t, env, dptypes.DPTargetPodRole))
			require.NotEqual(t, sourceName, envValue(t, env, dptypes.DPTargetPodName))
		})
	}
}

func requireUniqueEnvNames(t *testing.T, env []corev1.EnvVar) {
	t.Helper()
	seen := make(map[string]struct{}, len(env))
	for _, item := range env {
		_, exists := seen[item.Name]
		require.Falsef(t, exists, "duplicate env name %q", item.Name)
		seen[item.Name] = struct{}{}
	}
}

func envIndex(t *testing.T, env []corev1.EnvVar, name string) int {
	t.Helper()
	for i := range env {
		if env[i].Name == name {
			return i
		}
	}
	require.FailNowf(t, "missing env", "env %q was not found", name)
	return -1
}

func envValue(t *testing.T, env []corev1.EnvVar, name string) string {
	t.Helper()
	return env[envIndex(t, env, name)].Value
}
