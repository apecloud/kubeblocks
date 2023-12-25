/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func BuildEnvByCredential(pod *corev1.Pod, credential *dpv1alpha1.ConnectionCredential) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	if credential == nil {
		envVars = append(envVars, corev1.EnvVar{Name: dptypes.DPDBHost, Value: intctrlutil.BuildPodHostDNS(pod)})
		return envVars
	}
	var hostEnv corev1.EnvVar
	if credential.HostKey == "" {
		hostEnv = corev1.EnvVar{Name: dptypes.DPDBHost, Value: intctrlutil.BuildPodHostDNS(pod)}
	} else {
		hostEnv = buildEnvBySecretKey(dptypes.DPDBHost, credential.SecretName, credential.HostKey)
	}
	envVars = append(envVars, hostEnv)
	if credential.PasswordKey != "" {
		envVars = append(envVars, buildEnvBySecretKey(dptypes.DPDBPassword, credential.SecretName, credential.PasswordKey))
	}
	if credential.UsernameKey != "" {
		envVars = append(envVars, buildEnvBySecretKey(dptypes.DPDBUser, credential.SecretName, credential.UsernameKey))
	}
	if credential.PortKey != "" {
		envVars = append(envVars, buildEnvBySecretKey(dptypes.DPDBPort, credential.SecretName, credential.PortKey))
	}
	return envVars
}

func buildEnvBySecretKey(name, secretName, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: key,
			},
		},
	}
}
