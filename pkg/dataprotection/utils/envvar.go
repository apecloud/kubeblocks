/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

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

// BuildEnvVarByTargetVars resolves the target pod vars
func BuildEnvVarByTargetVars(ctx context.Context,
	cli client.Client,
	targetPod *corev1.Pod,
	vars []dpv1alpha1.EnvVar) ([]corev1.EnvVar, error) {
	var (
		envVars    []corev1.EnvVar
		envFromMap map[string]*corev1.EnvVar
		err        error
	)
	buildEnvVarByEnvRef := func(envVarRef *dpv1alpha1.EnvVarRef) (*corev1.EnvVar, error) {
		container := intctrlutil.GetPodContainer(targetPod, envVarRef.ContainerName)
		if container == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`can not find container "%s" of the pod "%s"`, envVarRef.ContainerName, targetPod.Name))
		}
		envVar := intctrlutil.BuildVarWithEnv(targetPod, container, envVarRef.EnvName)
		if envVar == nil {
			// if the var not found in container.env, try to find from container.envFrom.
			if envFromMap == nil {
				envFromMap, err = intctrlutil.GetEnvVarsFromEnvFrom(ctx, cli, targetPod.Namespace, container)
				if err != nil {
					return nil, err
				}
			}
			envVar = envFromMap[envVarRef.EnvName]
		}
		return envVar, nil
	}
	for i := range vars {
		envVarRef := vars[i].ValueFrom.EnvVarRef
		var envVar *corev1.EnvVar
		if envVarRef != nil {
			envVar, err = buildEnvVarByEnvRef(envVarRef)
		} else {
			envVar, err = intctrlutil.BuildVarWithFieldPath(targetPod, vars[i].ValueFrom.FieldPath)
		}
		if envVar == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`can not find the env "%s" in the container "%s"`, envVarRef.EnvName, envVarRef.ContainerName))
		}
		envVar.Name = vars[i].Name
		envVars = append(envVars, *envVar)
	}
	return envVars, nil
}
