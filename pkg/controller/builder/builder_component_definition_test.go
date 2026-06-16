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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var _ = Describe("component definition builder", func() {
	It("should set component definition spec fields", func() {
		updateStrategy := appsv1.BestEffortParallelStrategy
		obj := NewComponentDefinitionBuilder("mysql").
			SetRuntime(&corev1.Container{Name: "mysql", Image: "mysql:8.0"}).
			SetRuntime(&corev1.Container{Name: "mysql", Image: "mysql:8.4"}).
			SetRuntime(&corev1.Container{Name: "metrics", Image: "exporter:1.0"}).
			AddEnv("mysql", corev1.EnvVar{Name: "MYSQL_ROOT_HOST", Value: "%"}).
			AddVolumeMounts("mysql", []corev1.VolumeMount{
				{Name: "data", MountPath: "/old"},
				{Name: "conf", MountPath: "/etc/mysql"},
			}).
			AddVolumeMounts("mysql", []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/mysql"}}).
			AddVar(appsv1.EnvVar{Name: "MYSQL_PORT"}).
			AddVolume("data", true, 80).
			AddService("client", "mysql", 3306, "leader").
			AddServiceExt("metrics", "mysql-metrics", corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Name: "metrics", Port: 9125}},
			}, "").
			SetPolicyRules([]rbacv1.PolicyRule{{Resources: []string{"pods"}, Verbs: []string{"get"}}}).
			SetLabels(map[string]string{"engine": "mysql"}).
			SetReplicasLimit(1, 5).
			SetUpdateStrategy(&updateStrategy).
			AddRole("leader", 10, true).
			GetObject()

		Expect(obj.Name).Should(Equal("mysql"))
		Expect(obj.Spec.Runtime.Containers).Should(HaveLen(2))
		Expect(obj.Spec.Runtime.Containers[0].Name).Should(Equal("mysql"))
		Expect(obj.Spec.Runtime.Containers[0].Image).Should(Equal("mysql:8.4"))
		Expect(obj.Spec.Runtime.Containers[0].Env).Should(Equal([]corev1.EnvVar{{Name: "MYSQL_ROOT_HOST", Value: "%"}}))
		Expect(obj.Spec.Runtime.Containers[0].VolumeMounts).Should(ConsistOf(
			corev1.VolumeMount{Name: "data", MountPath: "/var/lib/mysql"},
			corev1.VolumeMount{Name: "conf", MountPath: "/etc/mysql"},
		))
		Expect(obj.Spec.Runtime.Containers[1]).Should(Equal(corev1.Container{Name: "metrics", Image: "exporter:1.0"}))
		Expect(obj.Spec.Vars).Should(Equal([]appsv1.EnvVar{{Name: "MYSQL_PORT"}}))
		Expect(obj.Spec.Volumes).Should(Equal([]appsv1.ComponentVolume{{Name: "data", NeedSnapshot: true, HighWatermark: 80}}))
		Expect(obj.Spec.Services).Should(Equal([]appsv1.ComponentService{
			{
				Service: appsv1.Service{
					Name:        "client",
					ServiceName: "mysql",
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 3306}},
					},
					RoleSelector: "leader",
				},
			},
			{
				Service: appsv1.Service{
					Name:        "metrics",
					ServiceName: "mysql-metrics",
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{Name: "metrics", Port: 9125}},
					},
				},
			},
		}))
		Expect(obj.Spec.PolicyRules).Should(Equal([]rbacv1.PolicyRule{{Resources: []string{"pods"}, Verbs: []string{"get"}}}))
		Expect(obj.Spec.Labels).Should(Equal(map[string]string{"engine": "mysql"}))
		Expect(obj.Spec.ReplicasLimit).Should(Equal(&appsv1.ReplicasLimit{MinReplicas: 1, MaxReplicas: 5}))
		Expect(obj.Spec.UpdateStrategy).Should(Equal(&updateStrategy))
		Expect(obj.Spec.Roles).Should(Equal([]appsv1.ReplicaRole{{Name: "leader", UpdatePriority: 10, ParticipatesInQuorum: true}}))
	})

	It("should ignore nil runtime and unknown container mutations", func() {
		obj := NewComponentDefinitionBuilder("mysql").
			SetRuntime(nil).
			AddEnv("missing", corev1.EnvVar{Name: "IGNORED"}).
			AddVolumeMounts("missing", []corev1.VolumeMount{{Name: "ignored"}}).
			GetObject()

		Expect(obj.Spec.Runtime.Containers).Should(BeNil())
	})
})
