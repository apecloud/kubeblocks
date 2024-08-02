/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package core

import (
	"testing"

	"github.com/stretchr/testify/require"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestGetConfigTemplatesFromComponent(t *testing.T) {
	var (
		comName     = "replicats_name"
		comType     = "replicats"
		cComponents = []appsv1alpha1.ClusterComponentSpec{
			{
				Name:            comName,
				ComponentDefRef: comType,
			},
		}
		tpl = appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "tpl1",
				TemplateRef: "cm1",
				VolumeName:  "volum1",
			},
		}
	)

	type args struct {
		cComponents []appsv1alpha1.ClusterComponentSpec
		dComponents []appsv1alpha1.ClusterComponentDefinition
		comName     string
	}
	tests := []struct {
		name    string
		args    args
		want    []appsv1alpha1.ComponentConfigSpec
		wantErr bool
	}{
		{
			name: "normal_test",
			args: args{
				comName:     comName,
				cComponents: cComponents,
				dComponents: []appsv1alpha1.ClusterComponentDefinition{{
					Name:        comType,
					ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl},
				}},
			},
			want: []appsv1alpha1.ComponentConfigSpec{
				tpl,
			},
			wantErr: false,
		},
		{
			name: "failed_test",
			args: args{
				comName:     "not exist component",
				cComponents: cComponents,
				dComponents: []appsv1alpha1.ClusterComponentDefinition{{
					Name:        comType,
					ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl},
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
					ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{tpl},
				}},
			},
			want: []appsv1alpha1.ComponentConfigSpec{
				tpl,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		got, err := GetConfigTemplatesFromComponent(
			tt.args.cComponents,
			tt.args.dComponents,
			tt.args.comName)
		require.Equal(t, err != nil, tt.wantErr)
		require.EqualValues(t, got, tt.want)
	}
}

func TestIsSupportConfigFileReconfigure(t *testing.T) {
	mockTemplateSpec := func() appsv1alpha1.ComponentTemplateSpec {
		return appsv1alpha1.ComponentTemplateSpec{
			Name:        "tpl",
			TemplateRef: "config",
			VolumeName:  "volume",
		}
	}

	type args struct {
		configTemplateSpec appsv1alpha1.ComponentConfigSpec
		configFileKey      string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "not_key_test",
		args: args{
			configTemplateSpec: appsv1alpha1.ComponentConfigSpec{
				ConfigConstraintRef:   "cc",
				ComponentTemplateSpec: mockTemplateSpec(),
			},
			configFileKey: "test_config",
		},
		want: true,
	}, {
		name: "not_exit_key_test",
		args: args{
			configTemplateSpec: appsv1alpha1.ComponentConfigSpec{
				ConfigConstraintRef:   "cc",
				ComponentTemplateSpec: mockTemplateSpec(),
				Keys:                  []string{"config1", "config2"},
			},
			configFileKey: "test_config",
		},
		want: false,
	}, {
		name: "exit_key_test",
		args: args{
			configTemplateSpec: appsv1alpha1.ComponentConfigSpec{
				ConfigConstraintRef:   "cc",
				ComponentTemplateSpec: mockTemplateSpec(),
				Keys:                  []string{"test_config", "config2"},
			},
			configFileKey: "test_config",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportConfigFileReconfigure(tt.args.configTemplateSpec, tt.args.configFileKey); got != tt.want {
				t.Errorf("IsSupportConfigFileReconfigure() = %v, want %v", got, tt.want)
			}
		})
	}
}
