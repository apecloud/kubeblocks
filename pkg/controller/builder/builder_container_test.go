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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("container builder", func() {
	It("should work well", func() {
		const name = "foo"
		commands := []string{
			name,
			"--bar",
		}
		args := []string{
			"arg1",
			"arg2",
		}
		env := []corev1.EnvVar{
			{
				Name:  name,
				Value: "bar",
			},
			{
				Name: "hello",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		}
		image := "foo:latest"
		policy := corev1.PullAlways
		mounts := []corev1.VolumeMount{
			{
				Name:      name,
				MountPath: "/data/foo",
			},
			{
				Name:      "bar",
				ReadOnly:  true,
				MountPath: "/log/bar",
			},
		}
		user := int64(0)
		ctx := corev1.SecurityContext{
			RunAsUser: &user,
		}

		resourceQuantityValue := func(value string) resource.Quantity {
			quantity, _ := resource.ParseQuantity(value)
			return quantity
		}
		resources := corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resourceQuantityValue("0.5"),
				corev1.ResourceMemory: resourceQuantityValue("500m"),
			},
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resourceQuantityValue("0.5"),
				corev1.ResourceMemory: resourceQuantityValue("500m"),
			},
		}
		ports := []corev1.ContainerPort{
			{
				Name:          name,
				ContainerPort: 12345,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "bar",
				ContainerPort: 54321,
				Protocol:      corev1.ProtocolUDP,
			},
		}
		readinessProbe := corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{},
				},
			},
		}
		startupProbe := corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(12345),
				},
			},
		}
		livenessProbe := corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{},
				},
			},
		}
		container := NewContainerBuilder(name).
			AddCommands(commands...).
			AddArgs(args...).
			AddEnv(env...).
			SetImage(image).
			SetImagePullPolicy(policy).
			AddVolumeMounts(mounts...).
			SetSecurityContext(ctx).
			SetResources(resources).
			AddPorts(ports...).
			SetReadinessProbe(readinessProbe).
			SetStartupProbe(startupProbe).
			SetLivenessProbe(livenessProbe).
			GetObject()

		Expect(container.Name).Should(Equal(name))
		Expect(container.Command).Should(Equal(commands))
		Expect(container.Args).Should(Equal(args))
		Expect(container.Env).Should(Equal(env))
		Expect(container.Image).Should(Equal(image))
		Expect(container.ImagePullPolicy).Should(Equal(policy))
		Expect(container.VolumeMounts).Should(Equal(mounts))
		Expect(container.SecurityContext).ShouldNot(BeNil())
		Expect(*container.SecurityContext).Should(Equal(ctx))
		Expect(container.Resources).Should(Equal(resources))
		Expect(container.Ports).Should(Equal(ports))
		Expect(container.ReadinessProbe).ShouldNot(BeNil())
		Expect(*container.ReadinessProbe).Should(Equal(readinessProbe))
		Expect(container.StartupProbe).ShouldNot(BeNil())
		Expect(*container.StartupProbe).Should(Equal(startupProbe))
		Expect(container.LivenessProbe).ShouldNot(BeNil())
		Expect(*container.LivenessProbe).Should(Equal(livenessProbe))
	})
})
