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

package core

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestGetInstanceName(t *testing.T) {
	require.Equal(t, "config.kubeblocks.io/tpl-template-test", GenerateTPLUniqLabelKeyWithConfig("template-test"))
	require.Equal(t, "config.kubeblocks.io/constraints-test", GenerateConstraintsUniqLabelKeyWithConfig("test"))
	require.Equal(t, "mytest-mysql-config-template", GetComponentCfgName("mytest", "mysql", "config-template"))
	require.Equal(t, "mytest-envfrom", GenerateEnvFromName("mytest"))
	require.Equal(t, "mytest-mysql", GenerateComponentConfigurationName("mytest", "mysql"))
	require.Equal(t, "config.kubeblocks.io/revision-reconcile-phase-100", GenerateRevisionPhaseKey("100"))
	require.Equal(t, "mycluster-mysql-config", GetInstanceCMName(&appsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mycluster",
		}},
		&appsv1alpha1.ComponentTemplateSpec{
			Name:        "mysql-config",
			TemplateRef: "mysql-template-config",
		}))
}

func TestWrapError(t *testing.T) {
	require.Nil(t, WrapError(nil, ""))
	require.Equal(t, WrapError(MakeError("reason: not expected"), "failed to test"), MakeError("failed to test: [reason: not expected]"))
}
