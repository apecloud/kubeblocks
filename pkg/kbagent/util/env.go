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

package util

import (
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	kbEnvNamespace = "KB_AGENT_NAMESPACE"
	kbEnvPodName   = "KB_AGENT_POD_NAME"
	kbEnvPodUID    = "KB_AGENT_POD_UID"
	kbEnvNodeName  = "KB_AGENT_NODE_NAME"
)

func EnvM2L(m map[string]string) []string {
	l := make([]string, 0)
	for k, v := range m {
		l = append(l, k+"="+v)
	}
	return l
}

func EnvL2M(l []string) map[string]string {
	m := make(map[string]string, 0)
	for _, p := range l {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			m[kv[0]] = kv[1]
		}
		if len(kv) == 1 {
			m[kv[0]] = ""
		}
	}
	return m
}

func DefaultEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: kbEnvNamespace,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.namespace",
				},
			},
		},
		{
			Name: kbEnvPodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.name",
				},
			},
		},
		{
			Name: kbEnvPodUID,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.uid",
				},
			},
		},
		{
			Name: kbEnvNodeName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "spec.nodeName",
				},
			},
		},
	}
}

func namespace() string {
	return os.Getenv(kbEnvNamespace)
}

func podName() string {
	return os.Getenv(kbEnvPodName)
}

func podUID() string {
	return os.Getenv(kbEnvPodUID)
}

func nodeName() string {
	return os.Getenv(kbEnvNodeName)
}
