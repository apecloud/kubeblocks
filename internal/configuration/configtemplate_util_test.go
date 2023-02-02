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

/*
Copyright ApeCloud Inc.

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("Reconfigure RollingPolicy", func() {

	Context("MergeConfigTemplates test", func() {
		It("Should success without error", func() {
			type args struct {
				avTpl []dbaasv1alpha1.ConfigTemplate
				cdTpl []dbaasv1alpha1.ConfigTemplate
			}
			tests := []struct {
				name string
				args args
				want []dbaasv1alpha1.ConfigTemplate
			}{{
				name: "merge_configtpl_test",
				args: args{
					avTpl: nil,
					cdTpl: nil,
				},
				want: nil,
			}, {
				name: "merge_configtpl_test",
				args: args{
					avTpl: []dbaasv1alpha1.ConfigTemplate{
						{
							Name:         "test1",
							ConfigTplRef: "tpl1",
							VolumeName:   "test1",
						},
						{
							Name:         "test2",
							ConfigTplRef: "tpl2",
							VolumeName:   "test2",
						},
					},
					cdTpl: nil,
				},
				want: []dbaasv1alpha1.ConfigTemplate{
					{
						Name:         "test1",
						ConfigTplRef: "tpl1",
						VolumeName:   "test1",
					},
					{
						Name:         "test2",
						ConfigTplRef: "tpl2",
						VolumeName:   "test2",
					},
				},
			}, {
				name: "merge_configtpl_test",
				args: args{
					avTpl: nil,
					cdTpl: []dbaasv1alpha1.ConfigTemplate{
						{
							Name:         "test1",
							ConfigTplRef: "tpl1",
							VolumeName:   "test1",
						},
						{
							Name:         "test2",
							ConfigTplRef: "tpl2",
							VolumeName:   "test2",
						},
					},
				},
				want: []dbaasv1alpha1.ConfigTemplate{
					{
						Name:         "test1",
						ConfigTplRef: "tpl1",
						VolumeName:   "test1",
					},
					{
						Name:         "test2",
						ConfigTplRef: "tpl2",
						VolumeName:   "test2",
					},
				},
			}, {
				name: "merge_configtpl_test",
				args: args{
					avTpl: []dbaasv1alpha1.ConfigTemplate{
						// update volume
						{
							Name:         "test_new_v1",
							ConfigTplRef: "tpl_new_v1",
							VolumeName:   "test1",
						},
						// add volume
						{
							Name:         "test_new_v2",
							ConfigTplRef: "tpl_new_v2",
							VolumeName:   "test2",
						},
					},
					cdTpl: []dbaasv1alpha1.ConfigTemplate{
						{
							Name:         "test1",
							ConfigTplRef: "tpl1",
							VolumeName:   "test1",
						},
						{
							Name:         "test3",
							ConfigTplRef: "tpl3",
							VolumeName:   "test3",
						},
					},
				},
				want: []dbaasv1alpha1.ConfigTemplate{
					{
						Name:         "test_new_v1",
						ConfigTplRef: "tpl_new_v1",
						VolumeName:   "test1",
					},
					{
						Name:         "test_new_v2",
						ConfigTplRef: "tpl_new_v2",
						VolumeName:   "test2",
					},
					{
						Name:         "test3",
						ConfigTplRef: "tpl3",
						VolumeName:   "test3",
					},
				},
			}}

			for _, tt := range tests {
				got := MergeConfigTemplates(tt.args.avTpl, tt.args.cdTpl)
				Expect(got).Should(BeEquivalentTo(tt.want))
			}
		})
	})

	Context("getConfigTemplatesFromComponent test", func() {
		It("Should success without error", func() {
			var (
				comName     = "replicats_name"
				comType     = "replicats"
				cComponents = []dbaasv1alpha1.ClusterComponent{
					{
						Name: comName,
						Type: comType,
					},
				}
				tpl1 = dbaasv1alpha1.ConfigTemplate{
					Name:         "tpl1",
					ConfigTplRef: "cm1",
					VolumeName:   "volum1",
				}
				tpl2 = dbaasv1alpha1.ConfigTemplate{
					Name:         "tpl2",
					ConfigTplRef: "cm2",
					VolumeName:   "volum2",
				}
			)

			type args struct {
				cComponents []dbaasv1alpha1.ClusterComponent
				dComponents []dbaasv1alpha1.ClusterDefinitionComponent
				aComponents []dbaasv1alpha1.ClusterVersionComponent
				comName     string
			}
			tests := []struct {
				name    string
				args    args
				want    []dbaasv1alpha1.ConfigTemplate
				wantErr bool
			}{{
				name: "normal_test",
				args: args{
					comName:     comName,
					cComponents: cComponents,
					dComponents: []dbaasv1alpha1.ClusterDefinitionComponent{
						{
							TypeName: comType,
							ConfigSpec: &dbaasv1alpha1.ConfigurationSpec{
								ConfigTemplateRefs: []dbaasv1alpha1.ConfigTemplate{tpl1},
							},
						},
					},
					aComponents: []dbaasv1alpha1.ClusterVersionComponent{
						{
							Type:               comType,
							ConfigTemplateRefs: []dbaasv1alpha1.ConfigTemplate{tpl2},
						},
					},
				},
				want: []dbaasv1alpha1.ConfigTemplate{
					tpl2,
					tpl1,
				},
				wantErr: false,
			}, {
				name: "failed_test",
				args: args{
					comName:     "not exist component",
					cComponents: cComponents,
					dComponents: []dbaasv1alpha1.ClusterDefinitionComponent{
						{
							TypeName: comType,
							ConfigSpec: &dbaasv1alpha1.ConfigurationSpec{
								ConfigTemplateRefs: []dbaasv1alpha1.ConfigTemplate{tpl1},
							},
						},
					},
					aComponents: []dbaasv1alpha1.ClusterVersionComponent{
						{
							Type:               comType,
							ConfigTemplateRefs: []dbaasv1alpha1.ConfigTemplate{tpl2},
						},
					},
				},
				want:    nil,
				wantErr: true,
			}, {
				name: "not_exist_and_not_failed",
				args: args{
					comName:     comName,
					cComponents: cComponents,
					dComponents: []dbaasv1alpha1.ClusterDefinitionComponent{
						{
							TypeName: comType,
							ConfigSpec: &dbaasv1alpha1.ConfigurationSpec{
								ConfigTemplateRefs: []dbaasv1alpha1.ConfigTemplate{tpl1},
							},
						},
					},
					aComponents: []dbaasv1alpha1.ClusterVersionComponent{
						{
							Type:               "not exist",
							ConfigTemplateRefs: []dbaasv1alpha1.ConfigTemplate{tpl2},
						},
					},
				},
				want: []dbaasv1alpha1.ConfigTemplate{
					tpl1,
				},
				wantErr: false,
			}}

			for _, tt := range tests {
				got, err := GetConfigTemplatesFromComponent(
					tt.args.cComponents,
					tt.args.dComponents,
					tt.args.aComponents,
					tt.args.comName)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
				Expect(got).Should(BeEquivalentTo(tt.want))
			}
		})
	})

})
