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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// --- InjectDatasafedWithPVC ---

func TestInjectDatasafedWithPVC_Basic(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "restore"}},
	}
	InjectDatasafedWithPVC(podSpec, "my-pvc", "/backupdata", "")

	// Should add PVC volume
	foundPVC := false
	for _, v := range podSpec.Volumes {
		if v.Name == "dp-backup-data" && v.PersistentVolumeClaim != nil {
			assert.Equal(t, "my-pvc", v.PersistentVolumeClaim.ClaimName)
			foundPVC = true
		}
	}
	assert.True(t, foundPVC, "PVC volume not found")

	// Should add local backend env
	foundEnv := false
	for _, env := range podSpec.Containers[0].Env {
		if env.Name == dptypes.DPDatasafedLocalBackendPath {
			assert.Equal(t, "/backupdata", env.Value)
			foundEnv = true
		}
	}
	assert.True(t, foundEnv, "local backend path env not found")

	// Should have init container for datasafed installer
	assert.NotEmpty(t, podSpec.InitContainers)
	assert.Equal(t, "dp-copy-datasafed", podSpec.InitContainers[0].Name)
}

func TestInjectDatasafedWithPVC_WithKopiaRepoPath(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "restore"}},
	}
	InjectDatasafedWithPVC(podSpec, "my-pvc", "/backupdata", "/kopia/repo")

	foundKopia := false
	for _, env := range podSpec.Containers[0].Env {
		if env.Name == dptypes.DPDatasafedKopiaRepoRoot {
			assert.Equal(t, "/kopia/repo", env.Value)
			foundKopia = true
		}
	}
	assert.True(t, foundKopia, "kopia repo root env not found")
}

// --- InjectDatasafedWithConfig ---

func TestInjectDatasafedWithConfig_Basic(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "restore"}},
	}
	InjectDatasafedWithConfig(podSpec, "my-secret", "")

	// Should add secret volume
	foundSecret := false
	for _, v := range podSpec.Volumes {
		if v.Name == "dp-datasafed-config" && v.Secret != nil {
			assert.Equal(t, "my-secret", v.Secret.SecretName)
			foundSecret = true
		}
	}
	assert.True(t, foundSecret, "secret volume not found")

	// Should have volume mount at config path
	foundMount := false
	for _, vm := range podSpec.Containers[0].VolumeMounts {
		if vm.Name == "dp-datasafed-config" {
			assert.Equal(t, datasafedConfigMountPath, vm.MountPath)
			assert.True(t, vm.ReadOnly)
			foundMount = true
		}
	}
	assert.True(t, foundMount, "config volume mount not found")

	// Should have init container
	assert.NotEmpty(t, podSpec.InitContainers)
}

func TestInjectDatasafedWithConfig_WithKopiaRepoPath(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "restore"}},
	}
	InjectDatasafedWithConfig(podSpec, "my-secret", "/kopia/repo")

	foundKopia := false
	for _, env := range podSpec.Containers[0].Env {
		if env.Name == dptypes.DPDatasafedKopiaRepoRoot {
			assert.Equal(t, "/kopia/repo", env.Value)
			foundKopia = true
		}
	}
	assert.True(t, foundKopia, "kopia repo root env not found")
}

// --- injectEncryptionEnvs ---

func TestInjectEncryptionEnvs_Nil(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c1"}},
	}
	injectEncryptionEnvs(podSpec, nil)
	assert.Empty(t, podSpec.Containers[0].Env)
}

func TestInjectEncryptionEnvs_Valid(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c1"}},
	}
	config := &dpv1alpha1.EncryptionConfig{
		Algorithm: "aes-256-ctr",
		PassPhraseSecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "enc-secret"},
			Key:                  "passphrase",
		},
	}
	injectEncryptionEnvs(podSpec, config)
	require.Len(t, podSpec.Containers[0].Env, 2)
	assert.Equal(t, dptypes.DPDatasafedEncryptionAlgorithm, podSpec.Containers[0].Env[0].Name)
	assert.Equal(t, "aes-256-ctr", podSpec.Containers[0].Env[0].Value)
	assert.Equal(t, dptypes.DPDatasafedEncryptionPassPhrase, podSpec.Containers[0].Env[1].Name)
	require.NotNil(t, podSpec.Containers[0].Env[1].ValueFrom)
}

// --- InjectDatasafed (dispatch) ---

func TestInjectDatasafed_AccessByMount(t *testing.T) {
	repo := &dpv1alpha1.BackupRepo{
		Spec: dpv1alpha1.BackupRepoSpec{
			AccessMethod: dpv1alpha1.AccessMethodMount,
		},
		Status: dpv1alpha1.BackupRepoStatus{
			BackupPVCName: "repo-pvc",
		},
	}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "restore"}},
	}
	InjectDatasafed(podSpec, repo, "/backupdata", nil, "")

	foundPVC := false
	for _, v := range podSpec.Volumes {
		if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == "repo-pvc" {
			foundPVC = true
		}
	}
	assert.True(t, foundPVC)
}

func TestInjectDatasafed_AccessByTool(t *testing.T) {
	repo := &dpv1alpha1.BackupRepo{
		Spec: dpv1alpha1.BackupRepoSpec{
			AccessMethod: dpv1alpha1.AccessMethodTool,
		},
		Status: dpv1alpha1.BackupRepoStatus{
			ToolConfigSecretName: "tool-secret",
		},
	}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "restore"}},
	}
	InjectDatasafed(podSpec, repo, "/backupdata", nil, "")

	foundSecret := false
	for _, v := range podSpec.Volumes {
		if v.Secret != nil && v.Secret.SecretName == "tool-secret" {
			foundSecret = true
		}
	}
	assert.True(t, foundSecret)
}

func TestInjectDatasafed_WithEncryption(t *testing.T) {
	repo := &dpv1alpha1.BackupRepo{
		Spec: dpv1alpha1.BackupRepoSpec{
			AccessMethod: dpv1alpha1.AccessMethodMount,
		},
		Status: dpv1alpha1.BackupRepoStatus{
			BackupPVCName: "repo-pvc",
		},
	}
	config := &dpv1alpha1.EncryptionConfig{
		Algorithm: "aes-256-ctr",
		PassPhraseSecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "enc-secret"},
			Key:                  "passphrase",
		},
	}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "restore"}},
	}
	InjectDatasafed(podSpec, repo, "/backupdata", config, "")

	foundEncAlg := false
	for _, env := range podSpec.Containers[0].Env {
		if env.Name == dptypes.DPDatasafedEncryptionAlgorithm {
			foundEncAlg = true
		}
	}
	assert.True(t, foundEncAlg, "encryption env not injected")
}

// --- injectDatasafedInstaller ---

func TestInjectDatasafedInstaller_DefaultImage(t *testing.T) {
	viper.Set(datasafedImageEnv, "")
	defer viper.Set(datasafedImageEnv, nil)

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c1"}},
	}
	injectDatasafedInstaller(podSpec)

	require.NotEmpty(t, podSpec.InitContainers)
	assert.Equal(t, "dp-copy-datasafed", podSpec.InitContainers[0].Name)
	assert.Contains(t, podSpec.InitContainers[0].Image, "datasafed")
}

func TestInjectDatasafedInstaller_CustomImage(t *testing.T) {
	viper.Set(datasafedImageEnv, "custom/datasafed:v1")
	defer viper.Set(datasafedImageEnv, nil)

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c1"}},
	}
	injectDatasafedInstaller(podSpec)

	require.NotEmpty(t, podSpec.InitContainers)
	assert.Contains(t, podSpec.InitContainers[0].Image, "datasafed")
}

// --- injectElements ---

func TestInjectElements_MultipleContainers(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "c1"},
			{Name: "c2"},
		},
	}
	vol := corev1.Volume{Name: "vol1"}
	vm := corev1.VolumeMount{Name: "vol1", MountPath: "/data"}
	env := corev1.EnvVar{Name: "KEY", Value: "VALUE"}

	injectElements(podSpec, []corev1.Volume{vol}, []corev1.VolumeMount{vm}, []corev1.EnvVar{env})

	assert.Len(t, podSpec.Volumes, 1)
	for _, c := range podSpec.Containers {
		assert.Len(t, c.VolumeMounts, 1)
		assert.Len(t, c.Env, 1)
	}
}

// --- toSlice ---

func TestToSlice(t *testing.T) {
	s := toSlice("a", "b", "c")
	assert.Equal(t, []string{"a", "b", "c"}, s)
}

func TestToSlice_Single(t *testing.T) {
	s := toSlice(42)
	assert.Equal(t, []int{42}, s)
}
