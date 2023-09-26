package monitor

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	yamlv2 "gopkg.in/yaml.v2"
	"gopkg.in/yaml.v3"
)

var _ = Describe("monitor_controller", func() {

	It("should generate config correctly from config yaml", func() {
		Eventually(func(g Gomega) {
			valMap := map[string]any{
				"meta.transport": "http",
				//"meta.allow_native_password": false,
				//"meta.endpoint":              "http://",
				"meta.password": "labels[\"pass\"]",
			}
			tplName := "test.cue"
			bytes, err := buildFromCUEForOTel(tplName, valMap, "output")
			node := yaml.Node{}
			err = node.Encode(bytes)

			Expect(err).ShouldNot(HaveOccurred())
			slice := yamlv2.MapSlice{}
			err = yamlv2.Unmarshal(bytes, &slice)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(bytes).ShouldNot(BeNil())

		}).Should(Succeed())
	})

	It("should generate config correctly from config yaml", func() {
		Eventually(func(g Gomega) {

		}).Should(Succeed())
	})
})
