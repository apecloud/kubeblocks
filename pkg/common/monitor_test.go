/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestFromContainerPort(t *testing.T) {
	container := &corev1.Container{
		Name: "metrics",
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: 8080,
			},
		},
	}

	type args struct {
		exporter  Exporter
		container *corev1.Container
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "port name",
		args: args{
			exporter: Exporter{
				Exporter: appsv1alpha1.Exporter{
					ContainerName: "metrics",
					ScrapePath:    "/metrics",
					ScrapePort:    "http",
				},
			},
			container: container,
		},
		want: "8080",
	}, {
		name: "port number",
		args: args{
			exporter: Exporter{
				Exporter: appsv1alpha1.Exporter{
					ContainerName: "metrics",
					ScrapePath:    "/metrics",
					ScrapePort:    "8080",
				},
			},
			container: container,
		},
		want: "8080",
	}, {
		name: "empty port test",
		args: args{
			exporter: Exporter{
				Exporter: appsv1alpha1.Exporter{
					ContainerName: "metrics",
					ScrapePath:    "/metrics",
				},
			},
			container: container,
		},
		want: "8080",
	}, {
		name: "compatible with port name",
		args: args{
			exporter: Exporter{
				TargetPort: func() *intstr.IntOrString {
					r := intstr.FromString("http")
					return &r
				}(),
			},
			container: container,
		},
		want: "8080",
	}, {
		name: "compatible with port number",
		args: args{
			exporter: Exporter{
				TargetPort: func() *intstr.IntOrString {
					r := intstr.FromInt32(8080)
					return &r
				}(),
			},
			container: container,
		},
		want: "8080",
	}, {
		name: "invalid port",
		args: args{exporter: Exporter{}},
		want: "",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromContainerPort(tt.args.exporter, tt.args.container); got != tt.want {
				t.Errorf("FromContainerPort() = %v, want %v", got, tt.want)
			}
		})
	}
}
