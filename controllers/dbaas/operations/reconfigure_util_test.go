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

package operations

import (
	"context"
	"os"
	"reflect"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	mock_client "github.com/apecloud/kubeblocks/controllers/dbaas/configuration/policy/mocks"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

var _ = Describe("Reconfigure RollingPolicy", func() {

	var (
		mockClient *mock_client.MockClient
		ctrl       *gomock.Controller
	)

	setup := func() (*gomock.Controller, *mock_client.MockClient) {
		ctrl := gomock.NewController(GinkgoT())
		client := mock_client.NewMockClient(ctrl)
		return ctrl, client
	}

	setExpectedObject := func(out client.Object, obj client.Object) {
		outVal := reflect.ValueOf(out)
		objVal := reflect.ValueOf(obj)
		reflect.Indirect(outVal).Set(reflect.Indirect(objVal))
	}

	mockCfgTplObj := func(tpl dbaasv1alpha1.ConfigTemplate) (*corev1.ConfigMap, *dbaasv1alpha1.ConfigurationTemplate) {
		By("By assure an cm obj")
		configmapYAML, err := os.ReadFile("./testdata/configcm.yaml")
		Expect(err).Should(BeNil())
		Expect(configmapYAML).ShouldNot(BeNil())
		configTemplateYaml, err := os.ReadFile("./testdata/configtpl.yaml")
		Expect(err).Should(BeNil())
		Expect(configTemplateYaml).ShouldNot(BeNil())

		cfgCM := &corev1.ConfigMap{}
		cfgTpl := &dbaasv1alpha1.ConfigurationTemplate{}
		Expect(yaml.Unmarshal(configmapYAML, cfgCM)).Should(Succeed())
		Expect(yaml.Unmarshal(configTemplateYaml, cfgTpl)).Should(Succeed())

		cfgCM.Name = tpl.ConfigTplRef
		cfgCM.Namespace = tpl.Namespace
		cfgTpl.Name = tpl.ConfigConstraintRef
		cfgTpl.Namespace = tpl.Namespace
		return cfgCM, cfgTpl
	}

	BeforeEach(func() {
		ctrl, mockClient = setup()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		ctrl.Finish()
	})

	_ = mockClient

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
				got, err := getConfigTemplatesFromComponent(
					tt.args.cComponents,
					tt.args.dComponents,
					tt.args.aComponents,
					tt.args.comName)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
				Expect(got).Should(BeEquivalentTo(tt.want))
			}
		})
	})

	Context("updateCfgParams test", func() {
		It("Should success without error", func() {
			tpl := dbaasv1alpha1.ConfigTemplate{
				Name:                "for_test",
				ConfigTplRef:        "cm_obj",
				ConfigConstraintRef: "cfg_constraint_obj",
			}
			updatedCfg := dbaasv1alpha1.Configuration{
				Keys: []dbaasv1alpha1.ParameterConfig{
					{
						Parameters: []dbaasv1alpha1.ParameterPair{
							{
								Key:   "x1",
								Value: "y1",
							},
							{
								Key:   "x2",
								Value: "y2",
							},
						},
					},
				},
			}
			diffCfg := `{"mysqld":{"x1":"y1","x2":"y2"}}`

			cmObj, tplObj := mockCfgTplObj(tpl)
			mockK8sObjs := map[client.ObjectKey][]struct {
				object client.Object
				err    error
			}{
				// for cm
				client.ObjectKeyFromObject(cmObj): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get cm object"),
				}, {
					object: cmObj,
					err:    nil,
				}},
				// for tpl
				client.ObjectKeyFromObject(tplObj): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get tpl object"),
				}, {
					object: tplObj,
					err:    nil,
				}},
			}
			accessCounter := map[client.ObjectKey]int{}

			mockClient.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					tests, ok := mockK8sObjs[key]
					if !ok {
						return cfgcore.MakeError("not exist")
					}
					index := accessCounter[key]
					tt := tests[index]
					if tt.err == nil {
						// mock data
						setExpectedObject(obj, tt.object)
					}
					if index < len(tests)-1 {
						accessCounter[key]++
					}
					return tt.err
				}).AnyTimes()

			mockClient.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					cm, _ := obj.(*corev1.ConfigMap)
					cmObj.Data = cm.Data
					return nil
				}).AnyTimes()

			By("CM object failed.")
			// mock failed
			_, err := updateCfgParams(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, mockClient, "test")
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cm object"))

			By("TPL object failed.")
			// mock failed
			_, err = updateCfgParams(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, mockClient, "test")
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get tpl object"))

			By("update validate failed.")
			// check diff
			failed, err := updateCfgParams(dbaasv1alpha1.Configuration{
				Keys: []dbaasv1alpha1.ParameterConfig{{
					Parameters: []dbaasv1alpha1.ParameterPair{
						{
							Key:   "innodb_autoinc_lock_mode",
							Value: "100", // invalid value
						},
					},
				}},
			}, tpl, client.ObjectKeyFromObject(cmObj), ctx, mockClient, "test")
			Expect(failed).Should(BeTrue())
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring(`
mysqld.innodb_autoinc_lock_mode: conflicting values 0 and 100:
    9:36
    12:18
mysqld.innodb_autoinc_lock_mode: conflicting values 1 and 100:
    9:40
    12:18
mysqld.innodb_autoinc_lock_mode: conflicting values 2 and 100:
    9:44
    12:18
`))

			By("normal update.")
			{
				oldConfig := cmObj.Data
				_, err := updateCfgParams(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, mockClient, "test")
				Expect(err).Should(Succeed())
				option := cfgcore.CfgOption{
					Type:    cfgcore.CfgTplType,
					CfgType: dbaasv1alpha1.INI,
					Log:     log.FromContext(context.Background()),
				}
				diff, err := cfgcore.CreateMergePatch(&cfgcore.K8sConfig{
					CfgKey:         client.ObjectKeyFromObject(cmObj),
					Configurations: oldConfig,
				}, &cfgcore.K8sConfig{
					CfgKey:         client.ObjectKeyFromObject(cmObj),
					Configurations: cmObj.Data,
				}, option)
				Expect(err).Should(Succeed())
				Expect(diff.IsModify).Should(BeTrue())
				Expect(diff.UpdateConfig["my.cnf"]).Should(BeEquivalentTo(diffCfg))
			}
		})
	})

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
})
