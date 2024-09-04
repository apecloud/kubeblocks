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

package component

import (
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	kbagent "github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("kb-agent", func() {
	var (
		synthesizedComp *SynthesizedComponent
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	kbAgentContainer := func() *corev1.Container {
		for _, c := range synthesizedComp.PodSpec.Containers {
			if c.Name == kbagent.ContainerName {
				return &c
			}
		}
		return nil
	}

	kbAgentInitContainer := func() *corev1.Container {
		for _, c := range synthesizedComp.PodSpec.InitContainers {
			if c.Name == kbagent.InitContainerName {
				return &c
			}
		}
		return nil
	}

	Context("build kb-agent", func() {
		BeforeEach(func() {
			synthesizedComp = &SynthesizedComponent{
				PodSpec: &corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-kbagent",
							Image: "test-kbagent-image",
						},
					},
				},
				LifecycleActions: &appsv1alpha1.ComponentLifecycleActions{
					PostProvision: &appsv1alpha1.Action{
						Exec: &appsv1alpha1.ExecAction{
							Command: []string{"echo", "hello"},
						},
						TimeoutSeconds: 5,
						RetryPolicy: &appsv1alpha1.RetryPolicy{
							MaxRetries:    5,
							RetryInterval: 10,
						},
						PreCondition: &[]appsv1alpha1.PreConditionType{appsv1alpha1.ComponentReadyPreConditionType}[0],
					},
					RoleProbe: &appsv1alpha1.Probe{
						Action: appsv1alpha1.Action{
							Exec: &appsv1alpha1.ExecAction{
								Command: []string{"echo", "hello"},
							},
							TimeoutSeconds: 5,
						},
						InitialDelaySeconds: 5,
						PeriodSeconds:       1,
						SuccessThreshold:    3,
						FailureThreshold:    3,
					},
				},
			}
		})

		It("nil", func() {
			synthesizedComp.LifecycleActions = nil

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())
			Expect(kbAgentContainer()).Should(BeNil())
		})

		It("port", func() {
			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())

			c := kbAgentContainer()
			Expect(c).ShouldNot(BeNil())
			Expect(c.Ports).Should(HaveLen(1))
			Expect(c.Ports[0].ContainerPort).Should(Equal(int32(kbAgentDefaultPort)))
		})

		It("port - in use", func() {
			synthesizedComp.PodSpec.Containers[0].Ports = []corev1.ContainerPort{
				{
					ContainerPort: kbAgentDefaultPort,
				},
			}
			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())

			c := kbAgentContainer()
			Expect(c).ShouldNot(BeNil())
			Expect(c.Ports).Should(HaveLen(1))
			Expect(c.Ports[0].ContainerPort).Should(Equal(int32(kbAgentDefaultPort + 1)))
		})

		It("startup env", func() {
			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())

			c := kbAgentContainer()
			Expect(c).ShouldNot(BeNil())
			Expect(c.Env).Should(HaveLen(2))
		})

		It("action env", func() {
			env := []corev1.EnvVar{
				{
					Name:  "NOW",
					Value: time.Now().String(),
				},
				{
					Name: "POD_IP",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "status.podIP",
						},
					},
				},
			}
			synthesizedComp.LifecycleActions.PostProvision.Exec.Env = env

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())

			c := kbAgentContainer()
			Expect(c).ShouldNot(BeNil())
			Expect(c.Env).Should(HaveLen(4))
			Expect(reflect.DeepEqual(c.Env[0], env[0])).Should(BeTrue())
			Expect(reflect.DeepEqual(c.Env[1], env[1])).Should(BeTrue())
		})

		It("custom image", func() {
			image := "custom-image"
			synthesizedComp.LifecycleActions.PostProvision.Exec.Image = image

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())

			ic := kbAgentInitContainer()
			Expect(ic).ShouldNot(BeNil())

			c := kbAgentContainer()
			Expect(c).ShouldNot(BeNil())
			Expect(c.Image).Should(Equal(image))
			Expect(c.Command[0]).Should(Equal(kbAgentCommandOnSharedMount))
			Expect(c.VolumeMounts).Should(HaveLen(1))
			Expect(c.VolumeMounts[0]).Should(Equal(sharedVolumeMount))
		})

		It("custom image - two same images", func() {
			image := "custom-image"
			synthesizedComp.LifecycleActions.PostProvision.Exec.Image = image
			synthesizedComp.LifecycleActions.RoleProbe.Exec.Image = image

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())
		})

		It("custom image - two different images", func() {
			image1 := "custom-image1"
			image2 := "custom-image2"
			synthesizedComp.LifecycleActions.PostProvision.Exec.Image = image1
			synthesizedComp.LifecycleActions.RoleProbe.Exec.Image = image2

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("only one exec image is allowed in lifecycle actions"))
		})

		It("custom container", func() {
			container := synthesizedComp.PodSpec.Containers[0]
			synthesizedComp.LifecycleActions.PostProvision.Exec.Container = container.Name

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())

			ic := kbAgentInitContainer()
			Expect(ic).Should(BeNil())

			c := kbAgentContainer()
			Expect(c).ShouldNot(BeNil())
			Expect(c.Image).Should(Equal(viperx.GetString(constant.KBToolsImage)))
			Expect(c.Command[0]).Should(Equal(kbAgentCommand))
			Expect(c.VolumeMounts).Should(HaveLen(0))
		})

		It("custom container - volume mounts", func() {
			synthesizedComp.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "test-volume",
					MountPath: "/test",
				},
			}
			container := synthesizedComp.PodSpec.Containers[0]
			synthesizedComp.LifecycleActions.PostProvision.Exec.Container = container.Name

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())

			c := kbAgentContainer()
			Expect(c).ShouldNot(BeNil())
			Expect(c.VolumeMounts).Should(HaveLen(1))
			Expect(c.VolumeMounts[0]).Should(Equal(container.VolumeMounts[0]))
		})

		It("custom container - two same containers", func() {
			container := synthesizedComp.PodSpec.Containers[0]
			synthesizedComp.LifecycleActions.PostProvision.Exec.Container = container.Name
			synthesizedComp.LifecycleActions.RoleProbe.Exec.Container = container.Name

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())
		})

		It("custom container - two different containers", func() {
			synthesizedComp.PodSpec.Containers = append(synthesizedComp.PodSpec.Containers, corev1.Container{
				Name: "test-kbagent-1024",
			})
			container := synthesizedComp.PodSpec.Containers[0]
			container1 := synthesizedComp.PodSpec.Containers[1]
			synthesizedComp.LifecycleActions.PostProvision.Exec.Container = container.Name
			synthesizedComp.LifecycleActions.RoleProbe.Exec.Container = container1.Name

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("only one exec container is allowed in lifecycle actions"))
		})

		It("custom container - not defined", func() {
			name := "not-defined"
			synthesizedComp.LifecycleActions.PostProvision.Exec.Container = name

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring(fmt.Sprintf("exec container %s not found", name)))
		})

		It("custom image & container", func() {
			synthesizedComp.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "test-volume",
					MountPath: "/test",
				},
			}
			container := synthesizedComp.PodSpec.Containers[0]
			synthesizedComp.LifecycleActions.PostProvision.Exec.Image = container.Image
			synthesizedComp.LifecycleActions.PostProvision.Exec.Container = container.Name

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())

			ic := kbAgentInitContainer()
			Expect(ic).ShouldNot(BeNil())

			c := kbAgentContainer()
			Expect(c).ShouldNot(BeNil())
			Expect(c).ShouldNot(BeNil())
			Expect(c.Image).Should(Equal(container.Image))
			Expect(c.Command[0]).Should(Equal(kbAgentCommandOnSharedMount))
			Expect(c.VolumeMounts).Should(HaveLen(2))
			Expect(c.VolumeMounts[0]).Should(Equal(sharedVolumeMount))
			Expect(c.VolumeMounts[1]).Should(Equal(container.VolumeMounts[0]))
		})

		It("custom image & container - different images", func() {
			image := "custom-image"
			synthesizedComp.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "test-volume",
					MountPath: "/test",
				},
			}
			container := synthesizedComp.PodSpec.Containers[0]
			Expect(image).ShouldNot(Equal(container.Image))
			synthesizedComp.LifecycleActions.PostProvision.Exec.Image = image
			synthesizedComp.LifecycleActions.PostProvision.Exec.Container = container.Name

			err := buildKBAgentContainer(synthesizedComp)
			Expect(err).Should(BeNil())

			ic := kbAgentInitContainer()
			Expect(ic).ShouldNot(BeNil())

			c := kbAgentContainer()
			Expect(c.Image).Should(Equal(image))
			Expect(c.Command[0]).Should(Equal(kbAgentCommandOnSharedMount))
			Expect(c.VolumeMounts).Should(HaveLen(2))
			Expect(c.VolumeMounts[0]).Should(Equal(sharedVolumeMount))
			Expect(c.VolumeMounts[1]).Should(Equal(container.VolumeMounts[0]))
		})

		// TODO: host-network
	})
})
