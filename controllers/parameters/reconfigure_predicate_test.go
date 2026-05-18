/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package parameters

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestCheckConfigurationObjectWithReason(t *testing.T) {
	requiredLabels := map[string]string{
		constant.AppInstanceLabelKey:                 "cluster",
		constant.KBAppComponentLabelKey:              "mysql",
		constant.CMConfigurationTemplateNameLabelKey: "mysql-config",
		constant.CMConfigurationSpecProviderLabelKey: "mysql-config",
		constant.CMConfigurationTypeLabelKey:         constant.ConfigInstanceType,
	}

	tests := []struct {
		name         string
		mutate       func(*corev1.ConfigMap)
		wantOK       bool
		wantReasonIn string
	}{
		{
			name:         "missing labels",
			mutate:       func(cm *corev1.ConfigMap) { cm.Labels = nil },
			wantReasonIn: "missing labels",
		},
		{
			name: "missing required label",
			mutate: func(cm *corev1.ConfigMap) {
				delete(cm.Labels, constant.CMConfigurationTemplateNameLabelKey)
			},
			wantReasonIn: constant.CMConfigurationTemplateNameLabelKey,
		},
		{
			name: "wrong configuration type",
			mutate: func(cm *corev1.ConfigMap) {
				cm.Labels[constant.CMConfigurationTypeLabelKey] = "template"
			},
			wantReasonIn: constant.ConfigInstanceType,
		},
		{
			name: "disabled",
			mutate: func(cm *corev1.ConfigMap) {
				cm.Annotations = map[string]string{
					constant.DisableUpgradeInsConfigurationAnnotationKey: "true",
				}
			},
			wantReasonIn: constant.DisableUpgradeInsConfigurationAnnotationKey,
		},
		{
			name: "valid",
			mutate: func(cm *corev1.ConfigMap) {
				cm.Annotations = map[string]string{
					constant.DisableUpgradeInsConfigurationAnnotationKey: "false",
				}
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: "default",
					Labels:    map[string]string{},
				},
			}
			for k, v := range requiredLabels {
				cm.Labels[k] = v
			}
			if tt.mutate != nil {
				tt.mutate(cm)
			}

			gotOK, gotReason := checkConfigurationObjectWithReason(cm)
			if gotOK != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v, reason=%q", tt.wantOK, gotOK, gotReason)
			}
			if !tt.wantOK && !strings.Contains(gotReason, tt.wantReasonIn) {
				t.Fatalf("expected reason to contain %q, got %q", tt.wantReasonIn, gotReason)
			}
		})
	}
}
