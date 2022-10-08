/*
Copyright 2022.

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

package controllerutil

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// for test type
type Friend struct {
	Fname string
}

var _ = Describe("tpl engine template", func() {

	emptyTplEngine := func(values *TplValues, funcs *BuiltinObjectsFunc, tpl string) (string, error) {
		return NewTplEngine(values, funcs).Render(tpl)
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	Context("TPL emplate render without built-in object", func() {
		It("Should success with no error", func() {
			f1 := Friend{Fname: "test1"}
			f2 := Friend{Fname: "test2"}
			pp := TplValues{
				"UserName":   "user@@test",
				"Emails":     []string{"test1@gmail.com", "test2@gmail.com"},
				"Friends":    []*Friend{&f1, &f2},
				"MemorySize": 100,
			}

			tplString := `hello {{.UserName}}!
cal test: {{ add ( div ( mul .MemorySize 88 ) 100 ) 6 7 }}
{{ range .Emails }}
an email {{ . }}
{{- end }}
{{ with .Friends }}
{{- range . }}
my friend name is {{.Fname}}
{{- end }}
{{ end }}`

			expectString := `hello user@@test!
cal test: 101

an email test1@gmail.com
an email test2@gmail.com

my friend name is test1
my friend name is test2
`

			context, err := emptyTplEngine(&pp, nil, tplString)
			Expect(err).NotTo(HaveOccurred())
			Expect(context).To(Equal(expectString))
		})
	})

})
