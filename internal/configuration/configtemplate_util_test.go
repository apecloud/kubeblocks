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
	"testing"

	"github.com/stretchr/testify/require"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestMergeConfigTemplates(t *testing.T) {
	type args struct {
		cvTpl []appsv1alpha1.ConfigTemplate
		cdTpl []appsv1alpha1.ConfigTemplate
	}
	tests := []struct {
		name string
		args args
		want []appsv1alpha1.ConfigTemplate
	}{{
		name: "merge_configtpl_test",
		args: args{
			cvTpl: nil,
			cdTpl: nil,
		},
		want: nil,
	}, {
		name: "merge_configtpl_test",
		args: args{
			cvTpl: []appsv1alpha1.ConfigTemplate{
				{
					Name:         "test1",
					ConfigTplRef: "tpl1",
					VolumeName:   "test1",
				}, {
					Name:         "test2",
					ConfigTplRef: "tpl2",
					VolumeName:   "test2",
				}},
			cdTpl: nil,
		},
		want: []appsv1alpha1.ConfigTemplate{
			{
				Name:         "test1",
				ConfigTplRef: "tpl1",
				VolumeName:   "test1",
			}, {
				Name:         "test2",
				ConfigTplRef: "tpl2",
				VolumeName:   "test2",
			}},
	}, {
		name: "merge_configtpl_test",
		args: args{
			cvTpl: nil,
			cdTpl: []appsv1alpha1.ConfigTemplate{{
				Name:         "test1",
				ConfigTplRef: "tpl1",
				VolumeName:   "test1",
			}, {
				Name:         "test2",
				ConfigTplRef: "tpl2",
				VolumeName:   "test2",
			}},
		},
		want: []appsv1alpha1.ConfigTemplate{
			{
				Name:         "test1",
				ConfigTplRef: "tpl1",
				VolumeName:   "test1",
			}, {
				Name:         "test2",
				ConfigTplRef: "tpl2",
				VolumeName:   "test2",
			}},
	}, {
		name: "merge_configtpl_test",
		args: args{
			cvTpl: []appsv1alpha1.ConfigTemplate{{
				// update volume
				Name:         "tpl1",
				ConfigTplRef: "config1_new",
				VolumeName:   "volume1",
			}, {
				// add volume
				Name:         "tpl2",
				ConfigTplRef: "config2_new",
				VolumeName:   "volume2",
			}},
			cdTpl: []appsv1alpha1.ConfigTemplate{{
				Name:         "tpl1",
				ConfigTplRef: "config1",
				VolumeName:   "volume1",
			}, {
				Name:         "tpl3",
				ConfigTplRef: "config3",
				VolumeName:   "volume3",
			}},
		},
		want: []appsv1alpha1.ConfigTemplate{
			{
				Name:         "tpl1",
				ConfigTplRef: "config1_new",
				VolumeName:   "volume1",
			}, {
				Name:         "tpl2",
				ConfigTplRef: "config2_new",
				VolumeName:   "volume2",
			}, {
				Name:         "tpl3",
				ConfigTplRef: "config3",
				VolumeName:   "volume3",
			}},
	}}

	for _, tt := range tests {
		got := MergeConfigTemplates(tt.args.cvTpl, tt.args.cdTpl)
		require.EqualValues(t, tt.want, got)
	}
}

func TestGetConfigTemplatesFromComponent(t *testing.T) {
	var (
		comName     = "replicats_name"
		comType     = "replicats"
		cComponents = []appsv1alpha1.ClusterComponentSpec{{
			Name:            comName,
			ComponentDefRef: comType,
		}}
		tpl1 = appsv1alpha1.ConfigTemplate{
			Name:         "tpl1",
			ConfigTplRef: "cm1",
			VolumeName:   "volum1",
		}
		tpl2 = appsv1alpha1.ConfigTemplate{
			Name:         "tpl2",
			ConfigTplRef: "cm2",
			VolumeName:   "volum2",
		}
	)

	type args struct {
		cComponents []appsv1alpha1.ClusterComponentSpec
		dComponents []appsv1alpha1.ClusterComponentDefinition
		aComponents []appsv1alpha1.ClusterComponentVersion
		comName     string
	}
	tests := []struct {
		name    string
		args    args
		want    []appsv1alpha1.ConfigTemplate
		wantErr bool
	}{{
		name: "normal_test",
		args: args{
			comName:     comName,
			cComponents: cComponents,
			dComponents: []appsv1alpha1.ClusterComponentDefinition{{
				Name: comType,
				ConfigSpec: &appsv1alpha1.ConfigurationSpec{
					ConfigTemplateRefs: []appsv1alpha1.ConfigTemplate{tpl1},
				},
			}},
			aComponents: []appsv1alpha1.ClusterComponentVersion{{
				ComponentDefRef:    comType,
				ConfigTemplateRefs: []appsv1alpha1.ConfigTemplate{tpl2},
			}},
		},
		want: []appsv1alpha1.ConfigTemplate{
			tpl2,
			tpl1,
		},
		wantErr: false,
	}, {
		name: "failed_test",
		args: args{
			comName:     "not exist component",
			cComponents: cComponents,
			dComponents: []appsv1alpha1.ClusterComponentDefinition{{
				Name: comType,
				ConfigSpec: &appsv1alpha1.ConfigurationSpec{
					ConfigTemplateRefs: []appsv1alpha1.ConfigTemplate{tpl1},
				},
			}},
			aComponents: []appsv1alpha1.ClusterComponentVersion{{
				ComponentDefRef:    comType,
				ConfigTemplateRefs: []appsv1alpha1.ConfigTemplate{tpl2},
			}},
		},
		want:    nil,
		wantErr: true,
	}, {
		name: "not_exist_and_not_failed",
		args: args{
			comName:     comName,
			cComponents: cComponents,
			dComponents: []appsv1alpha1.ClusterComponentDefinition{{
				Name: comType,
				ConfigSpec: &appsv1alpha1.ConfigurationSpec{
					ConfigTemplateRefs: []appsv1alpha1.ConfigTemplate{tpl1},
				},
			}},
			aComponents: []appsv1alpha1.ClusterComponentVersion{{
				ComponentDefRef:    "not exist",
				ConfigTemplateRefs: []appsv1alpha1.ConfigTemplate{tpl2},
			},
			}},
		want: []appsv1alpha1.ConfigTemplate{
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
		require.Equal(t, err != nil, tt.wantErr)
		require.EqualValues(t, got, tt.want)
	}
}
