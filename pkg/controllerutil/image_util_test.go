package controllerutil

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("image util test", func() {
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
		registriesConf = registriesConfig{}
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
	})

	It("only expands image when config does not exist", func() {
		for _, image := range imageList {
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", image[1], image[2], image[3], image[4])))
		}
	})

	It("replaces image when default config exists", func() {
		registriesConf = registriesConfig{
			DefaultRegistry: "foo.bar",
		}
		for _, image := range imageList {
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", "foo.bar", image[2], image[3], image[4])))
		}

		registriesConf = registriesConfig{
			DefaultRegistry:  "foo.bar",
			DefaultNamespace: "test",
		}
		for _, image := range imageList {
			newImage, err := ReplaceImageRegistry(image[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(newImage).To(Equal(fmt.Sprintf("%v/%v/%v%v", "foo.bar", "test", image[3], image[4])))
		}
	})
})
