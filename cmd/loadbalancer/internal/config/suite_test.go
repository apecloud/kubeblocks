/*
Copyright ApeCloud, Inc.

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

package config

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func init() {
	viper.AutomaticEnv()
}

var (
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
		o.TimeEncoder = zapcore.ISO8601TimeEncoder
	})
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
