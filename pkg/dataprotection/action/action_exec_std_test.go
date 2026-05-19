/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestExecAction_Validate_MissingPodName(t *testing.T) {
	e := &ExecAction{
		Namespace: "default",
		Command:   []string{"ls"},
	}
	err := e.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pod name")
}

func TestExecAction_Validate_MissingNamespace(t *testing.T) {
	e := &ExecAction{
		PodName: "pod-0",
		Command: []string{"ls"},
	}
	err := e.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace")
}

func TestExecAction_Validate_MissingCommand(t *testing.T) {
	e := &ExecAction{
		PodName:   "pod-0",
		Namespace: "default",
	}
	err := e.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command")
}

func TestExecAction_Validate_Success(t *testing.T) {
	e := &ExecAction{
		PodName:   "pod-0",
		Namespace: "default",
		Command:   []string{"ls", "-la"},
	}
	err := e.validate()
	assert.NoError(t, err)
}

func TestExecAction_BuildPodSpec(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	viper.Set(constant.KBImagePullPolicy, "IfNotPresent")
	t.Cleanup(func() {
		viper.Set(constant.KBToolsImage, "")
		viper.Set(constant.KBImagePullPolicy, "")
	})

	e := &ExecAction{
		JobAction: JobAction{
			Name: "exec-test",
		},
		PodName:            "target-pod",
		Namespace:          "ns1",
		Command:            []string{"sh", "-c", "echo hello"},
		Container:          "main",
		ServiceAccountName: "sa-backup",
	}

	spec := e.buildPodSpec()
	require.NotNil(t, spec)

	assert.Equal(t, corev1.RestartPolicyNever, spec.RestartPolicy)
	assert.Equal(t, "sa-backup", spec.ServiceAccountName)
	require.Len(t, spec.Containers, 1)

	c := spec.Containers[0]
	assert.Equal(t, "exec-test", c.Name)
	assert.Equal(t, "apecloud/kubeblocks-tools:latest", c.Image)
	assert.Equal(t, corev1.PullIfNotPresent, c.ImagePullPolicy)
	assert.Equal(t, []string{"kubectl"}, c.Command)

	expectedArgs := []string{"-n", "ns1", "exec", "target-pod", "-c", "main", "--", "sh", "-c", "echo hello"}
	assert.Equal(t, expectedArgs, c.Args)

	require.Len(t, spec.Tolerations, 1)
	assert.Equal(t, corev1.TolerationOpExists, spec.Tolerations[0].Operator)

	require.NotNil(t, spec.Affinity)
	require.NotNil(t, spec.NodeSelector)
}

func TestExecAction_Execute_ValidationFails(t *testing.T) {
	e := &ExecAction{} // empty, validate will fail
	status, err := e.Execute(ActionContext{})
	require.Error(t, err)
	assert.Nil(t, status)
}
