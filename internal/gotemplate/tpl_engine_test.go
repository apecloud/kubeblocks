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

package gotemplate

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

// for test type
type Friend struct {
	Name string
}

var _ = Describe("tpl engine template", func() {

	const (
		defaultNamespace = "testDefault"
	)

	emptyTplEngine := func(values *TplValues, funcs *BuiltInObjectsFunc, tpl string) (string, error) {
		return NewTplEngine(values, funcs, "for_test", nil, nil).Render(tpl)
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	Context("TPL emplate render without built-in object", func() {
		It("Should success with no error", func() {
			f1 := Friend{Name: "test1"}
			f2 := Friend{Name: "test2"}
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
my friend name is {{.Name}}
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

	// A call funcB.1 in B module
	// A call funcC.1 in C module
	// A call funcC.2 in C module
	// funcB.1 call funcB.2 in B module
	// funcB.2 call funcB.1 in C module
	Context("Support export function library", func() {
		It("call function in other module", func() {
			ctrl, k8sMock := testutil.SetupK8sMock()
			defer ctrl.Finish()

			testRenderString := fmt.Sprintf(`
{{- import "%s.moduleB" }}
{{- import "%s.moduleC" }}
{{- $sts := call "getAllStudentMeta" 10 | fromJson }}
{{- $total := call "calTotalStudent" $sts | int }}
{{- $mathAvg := call "calMathAvg" $sts }}
total = {{ $total }}
mathAvg = {{ $mathAvg -}}
`, defaultNamespace, defaultNamespace)

			// func library
			calMathAvg := fmt.Sprintf(`
{{- import "%s.moduleC" }}
{{- $totalMath := call "calTotalMatch" $.arg0 | float64 }}
{{- $totalCount := call "calTotalStudent" $.arg0 | float64 }}
{{- divf $totalMath $totalCount -}}
`, defaultNamespace)
			calTotalMatch := `
{{- $totalScore := 0 }}
{{- range $k, $v := $.arg0 }}
	{{- $totalScore = add $totalScore $v.Math }}
{{- end }}
{{- $totalScore -}}
`
			getAllStudentMeta := `
{{- $sts := dict }}
{{- range $i, $v := until $.arg0 }}
  {{- $grade :=  dict "Math" ( randInt 80 100 ) "Science" ( randInt 70 98 ) }}
  {{- $_ := set $sts (randAlphaNum 10) $grade }}
{{- end }}
{{- $sts | toJson -}}
`
			calTotalStudent := `{{- keys $.arg0 | len -}}`

			expectedRenderedString := `
total = 10
mathAvg = [8-9][0-9]\.?\d*`

			moduleB := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "moduleB",
					Namespace:   defaultNamespace,
					Annotations: map[string]string{GoTemplateLibraryAnnotationKey: "true"},
				},
				Data: map[string]string{
					"calMathAvg":    calMathAvg,
					"calTotalMatch": calTotalMatch,
				},
			}
			moduleC := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "moduleC",
					Namespace:   defaultNamespace,
					Annotations: map[string]string{GoTemplateLibraryAnnotationKey: "true"},
				},
				Data: map[string]string{
					"getAllStudentMeta": getAllStudentMeta,
					"calTotalStudent":   calTotalStudent,
				},
			}

			k8sMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					var ret client.Object
					switch key {
					case client.ObjectKeyFromObject(moduleB):
						ret = moduleB
					case client.ObjectKeyFromObject(moduleC):
						ret = moduleC
					default:
						return cfgcore.MakeError("failed to get cm: %v", key)
					}
					testutil.SetGetReturnedObject(obj, ret)
					return nil
				}).AnyTimes()

			engine := NewTplEngine(&TplValues{}, nil, "for_test", k8sMock, ctx)
			rendered, err := engine.Render(testRenderString)
			Expect(err).Should(Succeed())
			Expect(rendered).Should(MatchRegexp(expectedRenderedString))
		})
	})

	Context("Failed test", func() {
		It("failed to include cm", func() {
			ctrl, k8sMock := testutil.SetupK8sMock()
			defer ctrl.Finish()
			//Annotations: map[string]string{GoTemplateLibraryAnnotationKey: "true"},

			mockCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "moduleB",
					Namespace: defaultNamespace,
				},
				Data: map[string]string{
					"duplicate_fun1": `{{}}`,
					"duplicate_fun2": `{{}}`,
				},
			}

			engine := NewTplEngine(&TplValues{}, nil, "for_test", k8sMock, ctx)
			k8sMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if mockCM != nil {
						testutil.SetGetReturnedObject(obj, mockCM)
						return nil
					}
					return cfgcore.MakeError("get cm failed!")
				}).AnyTimes()

			By("Error cm formatter")
			// error cm formatter
			_, err := engine.Render(`{{ import "xxx" }}`)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("invalid import namespaceName: xxx"))

			By("Error for cm is not func library")
			_, err = engine.Render(`{{ import "xxx.yyy" }}`)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not template functions"))

			By("Error for duplicate function")
			mockCM.Annotations = map[string]string{
				GoTemplateLibraryAnnotationKey: "true"}
			engine.importFuncs["duplicate_fun2"] = functional{}
			_, err = engine.Render(`{{ import "xxx.yyy" }}`)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to import function: duplicate_fun2, from xxx/yyy, function is ready import"))

			By("Error for not exist cm")
			mockCM = nil
			_, err = engine.Render(`{{ import "xxx.not_exist" }}`)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("get cm failed"))
		})

		It("test failed", func() {

			testFailedFunc := `
{{ $testBoundary := 1000 }}
{{- if gt $.testCondition $testBoundary }}
{{- failed "testCondition require <= %d" $testBoundary }}
{{- end }}
`
			engine := NewTplEngine(&TplValues{
				"testCondition": 100,
			}, nil, "for_test", nil, nil)

			_, err := engine.Render(testFailedFunc)
			Expect(err).Should(Succeed())

			engine = NewTplEngine(&TplValues{
				"testCondition": 5000,
			}, nil, "for_test", nil, nil)

			_, err = engine.Render(testFailedFunc)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("testCondition require <= 1000"))
		})
	})

})
