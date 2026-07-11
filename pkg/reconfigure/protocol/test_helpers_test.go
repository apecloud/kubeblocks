/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func testDigest(input string) string {
	digest := sha256.Sum256([]byte(input))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func requireStateEqual(t *testing.T, expected, actual FenceState) {
	t.Helper()
	require.Equal(t, canonicalStateBytes(t, expected), canonicalStateBytes(t, actual))
}

func canonicalStateBytes(t *testing.T, state FenceState) []byte {
	t.Helper()
	data, err := CanonicalFenceStateBytes(state)
	require.NoError(t, err)
	return data
}
