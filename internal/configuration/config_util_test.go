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

package configuration

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("config_util", func() {

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("MergeAndValidateConfiguration", func() {
		It("Should success with no error", func() {
			type args struct {
				configConstraint v1alpha1.ConfigConstraintSpec
				baseCfg          map[string]string
				updatedParams    []ParamPairs
			}

			configConstraintObj := testapps.NewCustomizedObj("resources/mysql_config_template.yaml",
				&v1alpha1.ConfigConstraint{}, func(cc *v1alpha1.ConfigConstraint) {
					if ccContext, err := testdata.GetTestDataFileContent("cue_testdata/pg14.cue"); err == nil {
						cc.Spec.ConfigurationSchema = &v1alpha1.CustomParametersValidation{
							CUE: string(ccContext),
						}
					}
					cc.Spec.FormatterConfig = &v1alpha1.FormatterConfig{
						Format: v1alpha1.Properties,
					}
				})

			cfgContext, err := testdata.GetTestDataFileContent("cue_testdata/pg14.conf")
			Expect(err).Should(Succeed())

			tests := []struct {
				name    string
				args    args
				want    map[string]string
				wantErr bool
			}{{
				name: "pg1_merge",
				args: args{
					configConstraint: configConstraintObj.Spec,
					baseCfg:          map[string]string{"key": string(cfgContext)},
					updatedParams: []ParamPairs{
						{
							Key: "key",
							UpdatedParams: map[string]interface{}{
								"max_connections": "200",
								"shared_buffers":  "512M",
							},
						},
					},
				},
				want: map[string]string{
					"max_connections": "200",
					"shared_buffers":  "512M",
				},
			}}
			for _, tt := range tests {
				got, err := MergeAndValidateConfiguration(tt.args.configConstraint, tt.args.baseCfg, tt.args.updatedParams)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))

				option := CfgOption{
					Type:    CfgTplType,
					CfgType: tt.args.configConstraint.FormatterConfig.Format,
				}

				patch, err := CreateMergePatch(&K8sConfig{
					Configurations: tt.args.baseCfg,
				}, &K8sConfig{
					Configurations: got,
				}, option)
				Expect(err).Should(Succeed())

				var patchJSON map[string]string
				Expect(json.Unmarshal(patch.UpdateConfig["key"], &patchJSON)).Should(Succeed())
				Expect(patchJSON).Should(BeEquivalentTo(tt.want))
			}
		})
	})
})
