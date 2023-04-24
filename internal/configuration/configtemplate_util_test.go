/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
					Name:        "test1",
					TemplateRef: "tpl1",
					VolumeName:  "test1",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "test2",
					TemplateRef: "tpl2",
					VolumeName:  "test2",
				}}},
			cdTpl: nil,
		},
		want: []appsv1alpha1.ComponentConfigSpec{{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "test1",
				TemplateRef: "tpl1",
				VolumeName:  "test1",
			}}, {
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "test2",
				TemplateRef: "tpl2",
				VolumeName:  "test2",
			}}},
	}, {
		name: "merge_configtpl_test",
		args: args{
			cvTpl: nil,
			cdTpl: []appsv1alpha1.ComponentConfigSpec{{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "test1",
					TemplateRef: "tpl1",
					VolumeName:  "test1",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "test2",
					TemplateRef: "tpl2",
					VolumeName:  "test2",
				}}},
		},
		want: []appsv1alpha1.ComponentConfigSpec{{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "test1",
				TemplateRef: "tpl1",
				VolumeName:  "test1",
			}}, {
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "test2",
				TemplateRef: "tpl2",
				VolumeName:  "test2",
			}}},
	}, {
		name: "merge_configtpl_test",
		args: args{
			cvTpl: []appsv1alpha1.ComponentConfigSpec{{
				// update volume
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "tpl1",
					TemplateRef: "config1_new",
					VolumeName:  "volume1",
				}}, {
				// add volume
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "tpl2",
					TemplateRef: "config2_new",
					VolumeName:  "volume2",
				}}},
			cdTpl: []appsv1alpha1.ComponentConfigSpec{{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "tpl1",
					TemplateRef: "config1",
					VolumeName:  "volume1",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "tpl3",
					TemplateRef: "config3",
					VolumeName:  "volume3",
				}}},
		},
		want: []appsv1alpha1.ComponentConfigSpec{
			{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "tpl1",
					TemplateRef: "config1_new",
					VolumeName:  "volume1",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "tpl2",
					TemplateRef: "config2_new",
					VolumeName:  "volume2",
				}}, {
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "tpl3",
					TemplateRef: "config3",
					VolumeName:  "volume3",
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
				Name:        "tpl1",
				TemplateRef: "cm1",
				VolumeName:  "volum1",
			}}
		tpl2 = appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "tpl2",
				TemplateRef: "cm2",
				VolumeName:  "volum2",
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
				Name:        comType,
				ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl1},
			}},
			aComponents: []appsv1alpha1.ClusterComponentVersion{{
				ComponentDefRef: comType,
				ConfigSpecs:     []appsv1alpha1.ComponentConfigSpec{tpl2},
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
				Name:        comType,
				ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl1},
			}},
			aComponents: []appsv1alpha1.ClusterComponentVersion{{
				ComponentDefRef: comType,
				ConfigSpecs:     []appsv1alpha1.ComponentConfigSpec{tpl2},
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
				Name:        comType,
				ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl1},
			}},
			aComponents: []appsv1alpha1.ClusterComponentVersion{{
				ComponentDefRef: "not exist",
				ConfigSpecs:     []appsv1alpha1.ComponentConfigSpec{tpl2},
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
