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
		cvTpl []appsv1alpha1.ComponentConfigSpec
		cdTpl []appsv1alpha1.ComponentConfigSpec
	}
	tests := []struct {
		name string
		args args
		want []appsv1alpha1.ComponentConfigSpec
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
			cvTpl: []appsv1alpha1.ComponentConfigSpec{{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "test1",
					ConfigTemplateRef: "tpl1",
					VolumeName:        "test1",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "test2",
					ConfigTemplateRef: "tpl2",
					VolumeName:        "test2",
				}}},
			cdTpl: nil,
		},
		want: []appsv1alpha1.ComponentConfigSpec{{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:              "test1",
				ConfigTemplateRef: "tpl1",
				VolumeName:        "test1",
			}}, {
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:              "test2",
				ConfigTemplateRef: "tpl2",
				VolumeName:        "test2",
			}}},
	}, {
		name: "merge_configtpl_test",
		args: args{
			cvTpl: nil,
			cdTpl: []appsv1alpha1.ComponentConfigSpec{{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "test1",
					ConfigTemplateRef: "tpl1",
					VolumeName:        "test1",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "test2",
					ConfigTemplateRef: "tpl2",
					VolumeName:        "test2",
				}}},
		},
		want: []appsv1alpha1.ComponentConfigSpec{{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:              "test1",
				ConfigTemplateRef: "tpl1",
				VolumeName:        "test1",
			}}, {
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:              "test2",
				ConfigTemplateRef: "tpl2",
				VolumeName:        "test2",
			}}},
	}, {
		name: "merge_configtpl_test",
		args: args{
			cvTpl: []appsv1alpha1.ComponentConfigSpec{{
				// update volume
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "tpl1",
					ConfigTemplateRef: "config1_new",
					VolumeName:        "volume1",
				}}, {
				// add volume
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "tpl2",
					ConfigTemplateRef: "config2_new",
					VolumeName:        "volume2",
				}}},
			cdTpl: []appsv1alpha1.ComponentConfigSpec{{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "tpl1",
					ConfigTemplateRef: "config1",
					VolumeName:        "volume1",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "tpl3",
					ConfigTemplateRef: "config3",
					VolumeName:        "volume3",
				}}},
		},
		want: []appsv1alpha1.ComponentConfigSpec{
			{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "tpl1",
					ConfigTemplateRef: "config1_new",
					VolumeName:        "volume1",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "tpl2",
					ConfigTemplateRef: "config2_new",
					VolumeName:        "volume2",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:              "tpl3",
					ConfigTemplateRef: "config3",
					VolumeName:        "volume3",
				}}},
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
		tpl1 = appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:              "tpl1",
				ConfigTemplateRef: "cm1",
				VolumeName:        "volum1",
			}}
		tpl2 = appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:              "tpl2",
				ConfigTemplateRef: "cm2",
				VolumeName:        "volum2",
			}}
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
		want    []appsv1alpha1.ComponentConfigSpec
		wantErr bool
	}{{
		name: "normal_test",
		args: args{
			comName:     comName,
			cComponents: cComponents,
			dComponents: []appsv1alpha1.ClusterComponentDefinition{{
				Name:                 comType,
				ComponentConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl1},
			}},
			aComponents: []appsv1alpha1.ClusterComponentVersion{{
				ComponentDefRef:      comType,
				ComponentConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl2},
			}},
		},
		want: []appsv1alpha1.ComponentConfigSpec{
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
				Name:                 comType,
				ComponentConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl1},
			}},
			aComponents: []appsv1alpha1.ClusterComponentVersion{{
				ComponentDefRef:      comType,
				ComponentConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl2},
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
				Name:                 comType,
				ComponentConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl1},
			}},
			aComponents: []appsv1alpha1.ClusterComponentVersion{{
				ComponentDefRef:      "not exist",
				ComponentConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl2},
			},
			}},
		want: []appsv1alpha1.ComponentConfigSpec{
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
