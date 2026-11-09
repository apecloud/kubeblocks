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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateVolumeRestorePolicy(t *testing.T) {
	for _, policy := range []string{"", "Parallel"} {
		require.NoError(t, ValidateVolumeRestorePolicy(policy, VolumeRestorePolicyParameterKey))
	}
	err := ValidateVolumeRestorePolicy("Serial", VolumeRestorePolicyParameterKey)
	require.ErrorContains(t, err, "Serial is no longer supported")
	err = ValidateVolumeRestorePolicy("invalid", VolumeRestorePolicyParameterKey)
	require.ErrorContains(t, err, "invalid volume restore policy")
}
