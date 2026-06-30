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

package controllerutil

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("image util test", func() {
	imageList := [][]string{
		// original image name, registry, namespace, image name, tag and digest
		{"busybox", "docker.io", "library", "busybox", ""},
		{"apecloud/busybox:1.28", "docker.io", "apecloud", "busybox", ":1.28"},
		{"foo.io/a/b/busybox", "foo.io", "a/b", "busybox", ""},
		{
			"registry.k8s.io/pause:latest@sha256:1ff6c18fbef2045af6b9c16bf034cc421a29027b800e4f9b68ae9b1cb3e9ae07",
			"registry.k8s.io", "", "pause", ":latest@sha256:1ff6c18fbef2045af6b9c16bf034cc421a29027b800e4f9b68ae9b1cb3e9ae07"},
	}

	AfterEach(func() {
		// reset config
		registriesConfigInstance = &registriesConfig{}
	})

	It("parses image", func() {
		for _, image := range imageList {
			host, ns, repository, remainder, err := parseImageName(image[0])
			// fmt.Println(host, ns, repository, remainder)
			Expect(err).NotTo(HaveOccurred())
			Expect(host).To(Equal(image[1]))
			Expect(ns).To(Equal(image[2]))
			Expect(repository).To(Equal(image[3]))
			Expect(remainder).To(Equal(image[4]))
		}

		_, _, _, _, err := parseImageName("/invalid")
		Expect(err).To(HaveOccurred())
	})

	It("detects only KB-managed tools image updates", func() {
		oldToolsImage := viper.GetString(constant.KBToolsImage)
		defer viper.Set(constant.KBToolsImage, oldToolsImage)
		viper.Set(constant.KBToolsImage, "docker.io/apecloud/kubeblocks-tools:1.0.0")

		basePod := &corev1.Pod{
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:    "init-kbagent",
						Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
						Command: []string{"cp"},
					},
					{
						Name:    "install-config-manager-tool",
						Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
						Command: []string{"install"},
					},
				},
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "docker.io/apecloud/redis:7.2",
					},
					{
						Name:    "kbagent",
						Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
						Command: []string{"/bin/kbagent"},
					},
					{
						Name:    "kbagent-worker",
						Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
						Command: []string{"/bin/kbagent"},
						Args:    []string{"--server=false"},
					},
					{
						Name:    "config-manager",
						Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
						Command: []string{"/bin/config-manager"},
					},
				},
			},
		}

		newPod := basePod.DeepCopy()
		for i := range newPod.Spec.InitContainers {
			newPod.Spec.InitContainers[i].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
		}
		for i := range newPod.Spec.Containers {
			if newPod.Spec.Containers[i].Name != "app" {
				newPod.Spec.Containers[i].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
			}
		}
		changed, ok := OnlyKBManagedContainerImageChanged(basePod.Spec.InitContainers, newPod.Spec.InitContainers)
		Expect(ok).Should(BeTrue())
		Expect(changed).Should(BeTrue())
		changed, ok = OnlyKBManagedContainerImageChanged(basePod.Spec.Containers, newPod.Spec.Containers)
		Expect(ok).Should(BeTrue())
		Expect(changed).Should(BeTrue())

		appChangedPod := basePod.DeepCopy()
		appChangedPod.Spec.Containers[0].Image = "docker.io/apecloud/redis:7.4"
		_, ok = OnlyKBManagedContainerImageChanged(basePod.Spec.Containers, appChangedPod.Spec.Containers)
		Expect(ok).Should(BeFalse())

		customAgentPod := basePod.DeepCopy()
		customAgentPod.Spec.Containers[1].Image = "docker.io/example/custom-agent:1.1.0"
		_, ok = OnlyKBManagedContainerImageChanged(basePod.Spec.Containers, customAgentPod.Spec.Containers)
		Expect(ok).Should(BeFalse())

		agentEnvChangedPod := basePod.DeepCopy()
		agentEnvChangedPod.Spec.Containers[1].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
		agentEnvChangedPod.Spec.Containers[1].Env = []corev1.EnvVar{{Name: "EXTRA", Value: "true"}}
		_, ok = OnlyKBManagedContainerImageChanged(basePod.Spec.Containers, agentEnvChangedPod.Spec.Containers)
		Expect(ok).Should(BeFalse())

		removedContainerPod := basePod.DeepCopy()
		removedContainerPod.Spec.Containers = removedContainerPod.Spec.Containers[:len(removedContainerPod.Spec.Containers)-1]
		_, ok = OnlyKBManagedContainerImageChanged(basePod.Spec.Containers, removedContainerPod.Spec.Containers)
		Expect(ok).Should(BeFalse())
	})

	It("only expands image when config does not exist", func() {
		for _, image := range imageList {
			newImage := ReplaceImageRegistry(image[0])
			if image[2] == "" {
				Expect(newImage).To(Equal(fmt.Sprintf("%v/%v%v", image[1], image[3], image[4])))
			} else {
				Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", image[1], image[2], image[3], image[4])))
			}
		}
	})

	It("replaces image when default config exists", func() {
		registriesConfigInstance = &registriesConfig{
			DefaultRegistry: "foo.bar",
		}
		for _, image := range imageList {
			newImage := ReplaceImageRegistry(image[0])
			if image[2] == "" {
				Expect(newImage).To(Equal(fmt.Sprintf("%v/%v%v", "foo.bar", image[3], image[4])))
			} else {
				Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", "foo.bar", image[2], image[3], image[4])))
			}
		}

		registriesConfigInstance = &registriesConfig{
			DefaultRegistry:  "foo.bar",
			DefaultNamespace: "test",
		}
		for _, image := range imageList {
			newImage := ReplaceImageRegistry(image[0])
			Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", "foo.bar", "test", image[3], image[4])))
		}
	})

	It("replaces image when registry/namespace config exists", func() {
		registriesConfigInstance = &registriesConfig{
			DefaultRegistry:  "foo.bar",
			DefaultNamespace: "default",
			RegistryConfig: []registryConfig{
				{
					From: "docker.io",
					To:   "bar.io",
					NamespaceMapping: map[string]string{
						"library":  "foo",
						"apecloud": "",
					},
				},
				{
					From: "foo.io",
					To:   "foo.bar",
					NamespaceMapping: map[string]string{
						"a/b": "foo",
					},
				},
				{
					From: "registry.k8s.io",
					To:   "k8s.bar",
					NamespaceMapping: map[string]string{
						"": "k8s",
					},
				},
			},
		}
		expectedImage := []string{
			"bar.io/foo/busybox",
			"bar.io/busybox:1.28",
			"foo.bar/foo/busybox",
			"k8s.bar/k8s/pause:latest@sha256:1ff6c18fbef2045af6b9c16bf034cc421a29027b800e4f9b68ae9b1cb3e9ae07",
		}
		for i, image := range imageList {
			newImage := ReplaceImageRegistry(image[0])
			Expect(newImage).To(Equal(expectedImage[i]))
		}
	})

	It("replaces image w/ or w/o RegistryDefaultNamespace", func() {
		registriesConfigInstance = &registriesConfig{
			DefaultRegistry:  "foo.bar",
			DefaultNamespace: "default",
			RegistryConfig: []registryConfig{
				{
					From:             "docker.io",
					To:               "bar.io",
					DefaultNamespace: "docker",
				},
				{
					From: "foo.io",
					To:   "foo.bar",
				},
			},
		}
		expectedImage := []string{
			"bar.io/docker/busybox",
			"bar.io/docker/busybox:1.28",
			"foo.bar/a/b/busybox",
			"foo.bar/default/pause:latest@sha256:1ff6c18fbef2045af6b9c16bf034cc421a29027b800e4f9b68ae9b1cb3e9ae07",
		}
		for i, image := range imageList {
			newImage := ReplaceImageRegistry(image[0])
			Expect(newImage).To(Equal(expectedImage[i]))
		}
	})

	It("matches pod status images", func() {
		tests := []struct {
			name          string
			specImage     string
			statusImage   string
			statusImageID string
			expected      bool
		}{
			{
				name:        "tagless spec accepts status tag and registry",
				specImage:   "nginx",
				statusImage: "docker.io/nginx:latest@sha256:0f37a86c04f8",
				expected:    true,
			},
			{
				name:          "digest spec matches imageID digest when status image is tag",
				specImage:     "docker.io/nginx@sha256:0f37a86c04f8",
				statusImage:   "docker.io/nginx:latest",
				statusImageID: "docker.io/nginx@sha256:0f37a86c04f8",
				expected:      true,
			},
			{
				name:          "digest spec matches imageID digest when status image is local ID",
				specImage:     "docker.io/nginx@sha256:0f37a86c04f8",
				statusImage:   "sha256:runtime-local-image-id",
				statusImageID: "docker.io/nginx@sha256:0f37a86c04f8",
				expected:      true,
			},
			{
				name:          "digest spec rejects different imageID digest even if status image matches",
				specImage:     "docker.io/nginx@sha256:0f37a86c04f8",
				statusImage:   "docker.io/nginx@sha256:0f37a86c04f8",
				statusImageID: "docker.io/nginx@sha256:different",
				expected:      false,
			},
			{
				name:          "digest spec rejects missing imageID digest",
				specImage:     "docker.io/nginx@sha256:0f37a86c04f8",
				statusImage:   "docker.io/nginx@sha256:0f37a86c04f8",
				statusImageID: "",
				expected:      false,
			},
			{
				name:          "tag spec rejects different status image tag even with imageID",
				specImage:     "docker.io/nginx:1.0.0",
				statusImage:   "docker.io/nginx:latest",
				statusImageID: "docker.io/nginx@sha256:0f37a86c04f8",
				expected:      false,
			},
			{
				name:        "tag spec accepts same tag with rewritten registry",
				specImage:   "192.168.173.140:6451/apecloud/redis:8.4.0",
				statusImage: "172.31.255.3:5000/apecloud/redis:8.4.0",
				expected:    true,
			},
			{
				name:        "tag spec rejects different basename",
				specImage:   "192.168.173.140:6451/apecloud/redis:8.4.0",
				statusImage: "172.31.255.3:5000/apecloud/redis-stack:8.4.0",
				expected:    false,
			},
			{
				name:        "tagless spec rejects different basename",
				specImage:   "apecloud/redis",
				statusImage: "docker.io/apecloud/redis-stack:latest",
				expected:    false,
			},
		}
		for _, tt := range tests {
			By(tt.name)
			Expect(MatchContainerImageInStatus(tt.specImage, tt.statusImage, tt.statusImageID)).Should(Equal(tt.expected))
		}
	})

	It("matches pod spec images", func() {
		tests := []struct {
			name     string
			oldImage string
			newImage string
			expected bool
		}{
			{
				name:     "exact image matches",
				oldImage: "apecloud/redis:8.4.0",
				newImage: "apecloud/redis:8.4.0",
				expected: true,
			},
			{
				name:     "registry-only rewrite matches",
				oldImage: "172.31.255.3:5000/apecloud/redis:8.4.0",
				newImage: "192.168.173.140:6451/apecloud/redis:8.4.0",
				expected: true,
			},
			{
				name:     "init image registry-only rewrite matches",
				oldImage: "172.31.255.3:5000/apecloud/kbagent:1.0.3-beta.5",
				newImage: "192.168.173.140:6451/apecloud/kbagent:1.0.3-beta.5",
				expected: true,
			},
			{
				name:     "tag mismatch fails",
				oldImage: "172.31.255.3:5000/apecloud/redis:8.4.0",
				newImage: "192.168.173.140:6451/apecloud/redis:8.4.1",
				expected: false,
			},
			{
				name:     "digest mismatch fails",
				oldImage: "172.31.255.3:5000/apecloud/redis:8.4.0@sha256:old",
				newImage: "192.168.173.140:6451/apecloud/redis:8.4.0@sha256:new",
				expected: false,
			},
			{
				name:     "digest missing on one side fails",
				oldImage: "172.31.255.3:5000/apecloud/redis:8.4.0@sha256:old",
				newImage: "192.168.173.140:6451/apecloud/redis:8.4.0",
				expected: false,
			},
			{
				name:     "tag missing on one side fails",
				oldImage: "172.31.255.3:5000/apecloud/redis:8.4.0",
				newImage: "192.168.173.140:6451/apecloud/redis",
				expected: false,
			},
			{
				name:     "basename mismatch fails",
				oldImage: "172.31.255.3:5000/apecloud/redis:8.4.0",
				newImage: "192.168.173.140:6451/apecloud/redis-stack:8.4.0",
				expected: false,
			},
			{
				name:     "tagless registry-only rewrite matches",
				oldImage: "172.31.255.3:5000/apecloud/redis",
				newImage: "192.168.173.140:6451/apecloud/redis",
				expected: true,
			},
		}
		for _, tt := range tests {
			By(tt.name)
			Expect(EqualContainerImageInSpec(tt.oldImage, tt.newImage)).Should(Equal(tt.expected))
		}
	})
})
