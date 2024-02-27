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

package controllerutil

import (
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/common"
)

// GetEnvVarsFromEnvFrom gets the env var by the container envFrom.
func GetEnvVarsFromEnvFrom(ctx context.Context, cli client.Client, podNamespace string, container *corev1.Container) (map[string]*corev1.EnvVar, error) {
	envMap := map[string]*corev1.EnvVar{}
	for _, env := range container.EnvFrom {
		prefix := env.Prefix
		if env.ConfigMapRef != nil {
			configMap := &corev1.ConfigMap{}
			if err := cli.Get(ctx, client.ObjectKey{Name: env.ConfigMapRef.Name, Namespace: podNamespace}, configMap); err != nil {
				return nil, err
			}
			for k, v := range configMap.Data {
				name := prefix + k
				envMap[name] = &corev1.EnvVar{Name: name, Value: v}
			}
		} else if env.SecretRef != nil {
			secret := &corev1.Secret{}
			if err := cli.Get(ctx, client.ObjectKey{Name: env.SecretRef.Name, Namespace: podNamespace}, secret); err != nil {
				return nil, err
			}
			for k := range secret.Data {
				name := prefix + k
				envMap[name] = &corev1.EnvVar{Name: name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: env.SecretRef.Name},
					Key:                  k,
				}}}
			}
		}
	}
	return envMap, nil
}

// BuildVarWithEnv builds the env var by the container env.
func BuildVarWithEnv(targetPod *corev1.Pod, container *corev1.Container, envName string) *corev1.EnvVar {
	for i := range container.Env {
		env := container.Env[i]
		if env.Name != envName {
			continue
		}
		if env.ValueFrom != nil && env.ValueFrom.FieldRef != nil {
			// handle fieldRef
			value, _ := common.GetFieldRef(targetPod, env.ValueFrom)
			return &corev1.EnvVar{Name: envName, Value: value}
		}
		return &env
	}
	return nil
}

// BuildVarWithFieldPath builds the env var by jsonpath of the pod.
func BuildVarWithFieldPath(targetPod *corev1.Pod, fieldPath string) (*corev1.EnvVar, error) {
	path := jsonpath.New("jsonpath")
	if err := path.Parse(fmt.Sprintf("{%s}", fieldPath)); err != nil {
		return nil, fmt.Errorf("failed to parse fieldPath %s", fieldPath)
	}
	buff := bytes.NewBuffer([]byte{})
	if err := path.Execute(buff, targetPod); err != nil {
		return nil, fmt.Errorf("failed to execute fieldPath %s", fieldPath)
	}
	return &corev1.EnvVar{
		Value: buff.String(),
	}, nil
}
