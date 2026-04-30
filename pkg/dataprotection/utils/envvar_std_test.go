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
)

// --- BuildEnvByTarget ---

func TestBuildEnvByTarget_NoCredential(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Subdomain: "svc",
			Containers: []corev1.Container{
				{
					Ports: []corev1.ContainerPort{{ContainerPort: 5432}},
				},
			},
		},
	}
	envs, err := BuildEnvByTarget(pod, nil, nil)
	require.NoError(t, err)
	require.Len(t, envs, 2)
	assert.Equal(t, dptypes.DPDBHost, envs[0].Name)
	assert.Equal(t, dptypes.DPDBPort, envs[1].Name)
	assert.Equal(t, "5432", envs[1].Value)
}

func TestBuildEnvByTarget_WithCredential_AllKeys(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Subdomain: "svc",
			Containers: []corev1.Container{
				{Ports: []corev1.ContainerPort{{ContainerPort: 3306}}},
			},
		},
	}
	cred := &dpv1alpha1.ConnectionCredential{
		SecretName:  "cred-secret",
		UsernameKey: "user",
		PasswordKey: "pass",
		HostKey:     "host",
		PortKey:     "port",
	}
	envs, err := BuildEnvByTarget(pod, cred, nil)
	require.NoError(t, err)

	envMap := map[string]corev1.EnvVar{}
	for _, e := range envs {
		envMap[e.Name] = e
	}
	// host from secret
	assert.NotNil(t, envMap[dptypes.DPDBHost].ValueFrom)
	assert.Equal(t, "host", envMap[dptypes.DPDBHost].ValueFrom.SecretKeyRef.Key)
	// port from secret
	assert.NotNil(t, envMap[dptypes.DPDBPort].ValueFrom)
	assert.Equal(t, "port", envMap[dptypes.DPDBPort].ValueFrom.SecretKeyRef.Key)
	// user/password from secret
	assert.Equal(t, "user", envMap[dptypes.DPDBUser].ValueFrom.SecretKeyRef.Key)
	assert.Equal(t, "pass", envMap[dptypes.DPDBPassword].ValueFrom.SecretKeyRef.Key)
}

func TestBuildEnvByTarget_WithCredential_NoHostKey(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Subdomain: "svc",
			Containers: []corev1.Container{
				{Ports: []corev1.ContainerPort{{ContainerPort: 3306}}},
			},
		},
	}
	cred := &dpv1alpha1.ConnectionCredential{
		SecretName:  "cred-secret",
		PasswordKey: "pass",
	}
	envs, err := BuildEnvByTarget(pod, cred, nil)
	require.NoError(t, err)

	envMap := map[string]corev1.EnvVar{}
	for _, e := range envs {
		envMap[e.Name] = e
	}
	// host from pod DNS (no secret)
	assert.Nil(t, envMap[dptypes.DPDBHost].ValueFrom)
	// port from pod (no PortKey)
	assert.Equal(t, "3306", envMap[dptypes.DPDBPort].Value)
}

func TestBuildEnvByTarget_WithContainerPort(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Subdomain: "svc",
			Containers: []corev1.Container{
				{
					Name:  "mysql",
					Ports: []corev1.ContainerPort{{Name: "mysql", ContainerPort: 3306}},
				},
			},
		},
	}
	cp := &dpv1alpha1.ContainerPort{ContainerName: "mysql", PortName: "mysql"}
	envs, err := BuildEnvByTarget(pod, nil, cp)
	require.NoError(t, err)

	envMap := map[string]corev1.EnvVar{}
	for _, e := range envs {
		envMap[e.Name] = e
	}
	assert.Equal(t, "3306", envMap[dptypes.DPDBPort].Value)
}

// --- buildEnvBySecretKey ---

func TestBuildEnvBySecretKey(t *testing.T) {
	env := buildEnvBySecretKey("MY_ENV", "my-secret", "my-key")
	assert.Equal(t, "MY_ENV", env.Name)
	require.NotNil(t, env.ValueFrom)
	require.NotNil(t, env.ValueFrom.SecretKeyRef)
	assert.Equal(t, "my-secret", env.ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "my-key", env.ValueFrom.SecretKeyRef.Key)
}

// --- BuildEnvByParameters ---

func TestBuildEnvByParameters_Empty(t *testing.T) {
	envs := BuildEnvByParameters(nil)
	assert.Empty(t, envs)
}

func TestBuildEnvByParameters_Multiple(t *testing.T) {
	params := []dpv1alpha1.ParameterPair{
		{Name: "K1", Value: "V1"},
		{Name: "K2", Value: "V2"},
	}
	envs := BuildEnvByParameters(params)
	require.Len(t, envs, 2)
	assert.Equal(t, "K1", envs[0].Name)
	assert.Equal(t, "V1", envs[0].Value)
	assert.Equal(t, "K2", envs[1].Name)
	assert.Equal(t, "V2", envs[1].Value)
}
