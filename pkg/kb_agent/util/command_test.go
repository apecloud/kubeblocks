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

package util

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/stretchr/testify/assert"
)

func TestGetGlobalSharedEnvs(t *testing.T) {
	// Set up test environment
	expectedEnvs := []string{
		constant.KBEnvPodFQDN + "=value1",
		constant.KBEnvServicePort + "=value2",
		constant.KBEnvServiceUser + "=value3",
		constant.KBEnvServicePassword + "=value4",
	}
	os.Clearenv()
	for _, env := range expectedEnvs {
		parts := strings.SplitN(env, "=", 2)
		os.Setenv(parts[0], parts[1])
	}

	// Call the function
	envs, err := GetGlobalSharedEnvs()

	// Check the results
	assert.NoError(t, err)
	assert.ElementsMatch(t, expectedEnvs, envs)
}

func TestExecCommand(t *testing.T) {
	// Set up test environment
	ctx := context.Background()
	command := []string{"binary not exists"}
	envs := []string{"ENV_VAR=value"}

	// Call the function
	_, err := ExecCommand(ctx, command, envs)

	// Check the results
	assert.Error(t, err)
}
