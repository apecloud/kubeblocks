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
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
)

func BuildEnvVarsByCredential(credential *dpv1alpha1.ConnectionCredential) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	if credential == nil {
		return nil
	}
	envVars = append(envVars,
		buildEnvVar(dptypes.DPDBUser, credential.SecretName, credential.UsernameKey),
		buildEnvVar(dptypes.DPDBPassword, credential.SecretName, credential.PasswordKey),
		buildEnvVar(dptypes.DPDBHost, credential.SecretName, credential.HostKey),
		buildEnvVar(dptypes.DPDBPort, credential.SecretName, credential.PortKey),
		buildEnvVar(dptypes.DPDBEndpoint, credential.SecretName, credential.EndpointKey),
	)
	return envVars
}

func buildEnvVar(name, objRefName, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: objRefName,
				},
				Key: key,
			},
		},
	}
}
