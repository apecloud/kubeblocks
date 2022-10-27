package config

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", func() {
	Context("", func() {
		It("", func() {
			cases := []struct {
				input  string
				output map[string]string
			}{
				{input: "", output: map[string]string{}},
				{input: "a:b,abcd", output: map[string]string{"a": "b"}},
				{input: "a:b:c:d,c:d", output: map[string]string{"a": "b:c:d", "c": "d"}},
				{input: "a:b,c:d", output: map[string]string{"a": "b", "c": "d"}},
			}

			ReadConfig(logger)

			for _, item := range cases {
				Expect(ParseLabels(item.input)).Should(Equal(item.output))
			}
		})
	})
})
