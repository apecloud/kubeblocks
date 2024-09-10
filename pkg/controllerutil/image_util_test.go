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
	"fmt"

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
		registriesConfig = &RegistriesConfig{}
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

	It("only expands image when config does not exist", func() {
		for _, image := range imageList {
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			if image[2] == "" {
				Expect(newImage).To(Equal(fmt.Sprintf("%v/%v%v", image[1], image[3], image[4])))
			} else {
				Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", image[1], image[2], image[3], image[4])))
			}
		}
	})

	It("replaces image when default config exists", func() {
		registriesConfig = &RegistriesConfig{
			DefaultRegistry: "foo.bar",
		}
		for _, image := range imageList {
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			if image[2] == "" {
				Expect(newImage).To(Equal(fmt.Sprintf("%v/%v%v", "foo.bar", image[3], image[4])))
			} else {
				Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", "foo.bar", image[2], image[3], image[4])))
			}
		}

		registriesConfig = &RegistriesConfig{
			DefaultRegistry:  "foo.bar",
			DefaultNamespace: "test",
		}
		for _, image := range imageList {
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", "foo.bar", "test", image[3], image[4])))
		}
	})

	It("replaces image when registry/namespace config exists", func() {
		registriesConfig = &RegistriesConfig{
			DefaultRegistry:  "foo.bar",
			DefaultNamespace: "default",
			RegistryConfig: []RegistryConfig{
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
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(newImage).To(Equal(expectedImage[i]))
		}
	})

	It("replaces image w/ or w/o RegistryDefaultNamespace", func() {
		registriesConfig = &RegistriesConfig{
			DefaultRegistry:  "foo.bar",
			DefaultNamespace: "default",
			RegistryConfig: []RegistryConfig{
				{
					From:                     "docker.io",
					To:                       "bar.io",
					RegistryDefaultNamespace: "docker",
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
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(newImage).To(Equal(expectedImage[i]))
		}
	})
})
