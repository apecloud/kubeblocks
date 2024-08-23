package controllerutil

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("image util test", func() {
	imageList := [][]string{
		[]string{"busybox", "docker.io", "library", "busybox", ""},
		[]string{"apecloud/busybox:1.28", "docker.io", "apecloud", "busybox", ":1.28"},
		[]string{"foo.io/a/b/busybox", "foo.io", "a/b", "busybox", ""},
		[]string{
			"registry.k8s.io/pause:latest@sha256:1ff6c18fbef2045af6b9c16bf034cc421a29027b800e4f9b68ae9b1cb3e9ae07",
			"registry.k8s.io", "", "pause", ":latest@sha256:1ff6c18fbef2045af6b9c16bf034cc421a29027b800e4f9b68ae9b1cb3e9ae07"},
	}

	AfterEach(func() {
		// reset config
		registriesConfig = RegistriesConfig{}
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
		registriesConfig = RegistriesConfig{
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

		registriesConfig = RegistriesConfig{
			DefaultRegistry:  "foo.bar",
			DefaultNamespace: "test",
		}
		for _, image := range imageList {
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", "foo.bar", "test", image[3], image[4])))
		}
	})

	It("replaces image when single registry config exists", func() {
		registriesConfig = RegistriesConfig{
			DefaultRegistry: "foo.bar",
			Registries: []RegistryConfig{
				{
					From: "docker.io",
					To:   "bar.io",
					Namespaces: []RegistryNamespaceConfig{
						{From: "library", To: "foo"},
					},
				},
				{
					From: "foo.io",
					Namespaces: []RegistryNamespaceConfig{
						{From: "a/b", To: "foo"},
					},
				},
			},
		}
		expectedImage := []string{
			"bar.io/foo/busybox",
			"bar.io/apecloud/busybox:1.28",
			"foo.bar/foo/busybox",
			"foo.bar/pause:latest@sha256:1ff6c18fbef2045af6b9c16bf034cc421a29027b800e4f9b68ae9b1cb3e9ae07",
		}
		for i, image := range imageList {
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(newImage).To(Equal(expectedImage[i]))
		}
	})
})
