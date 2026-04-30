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

package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

// --- checkLocalFileExist ---

func TestCheckLocalFileExist_Exists(t *testing.T) {
	f, err := os.CreateTemp("", "reconf-exist-*.txt")
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())

	exists, err := checkLocalFileExist(f.Name())
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCheckLocalFileExist_NotExists(t *testing.T) {
	exists, err := checkLocalFileExist("/tmp/non-existent-file-abc123xyz")
	require.NoError(t, err)
	assert.False(t, exists)
}

// --- checkReconfigureCreated ---

func TestCheckReconfigureCreated_EmptyParam(t *testing.T) {
	req := &proto.ActionRequest{
		Action:     "reconfigure",
		Parameters: map[string]string{},
	}
	assert.NoError(t, checkReconfigureCreated(req))
}

func TestCheckReconfigureCreated_FileExists(t *testing.T) {
	f, err := os.CreateTemp("", "reconf-created-*.txt")
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())

	req := &proto.ActionRequest{
		Action: "reconfigure",
		Parameters: map[string]string{
			configFilesCreated: f.Name(),
		},
	}
	assert.NoError(t, checkReconfigureCreated(req))
}

func TestCheckReconfigureCreated_FileNotExists(t *testing.T) {
	req := &proto.ActionRequest{
		Action: "reconfigure",
		Parameters: map[string]string{
			configFilesCreated: "/tmp/non-existent-file-def456uvw",
		},
	}
	err := checkReconfigureCreated(req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, proto.ErrPreconditionFailed))
}

// --- checkReconfigureRemoved ---

func TestCheckReconfigureRemoved_EmptyParam(t *testing.T) {
	req := &proto.ActionRequest{
		Action:     "reconfigure",
		Parameters: map[string]string{},
	}
	assert.NoError(t, checkReconfigureRemoved(req))
}

func TestCheckReconfigureRemoved_FileStillExists(t *testing.T) {
	f, err := os.CreateTemp("", "reconf-removed-*.txt")
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())

	req := &proto.ActionRequest{
		Action: "reconfigure",
		Parameters: map[string]string{
			configFilesRemoved: f.Name(),
		},
	}
	err = checkReconfigureRemoved(req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, proto.ErrPreconditionFailed))
}

func TestCheckReconfigureRemoved_FileActuallyRemoved(t *testing.T) {
	req := &proto.ActionRequest{
		Action: "reconfigure",
		Parameters: map[string]string{
			configFilesRemoved: "/tmp/non-existent-file-ghi789",
		},
	}
	assert.NoError(t, checkReconfigureRemoved(req))
}

// --- checkReconfigure with udf-reconfigure prefix ---

func TestCheckReconfigure_UDFReconfigure(t *testing.T) {
	req := &proto.ActionRequest{
		Action:     "udf-reconfigure-myapp",
		Parameters: map[string]string{},
	}
	assert.NoError(t, checkReconfigure(context.Background(), req))
}

// --- checkReconfigure Created + Removed combined ---

func TestCheckReconfigure_Created_MultipleFiles(t *testing.T) {
	f1, err := os.CreateTemp("", "reconf-multi1-*.txt")
	require.NoError(t, err)
	f1.Close()
	defer os.Remove(f1.Name())

	f2, err := os.CreateTemp("", "reconf-multi2-*.txt")
	require.NoError(t, err)
	f2.Close()
	defer os.Remove(f2.Name())

	req := &proto.ActionRequest{
		Action: "reconfigure",
		Parameters: map[string]string{
			configFilesCreated: f1.Name() + "," + f2.Name(),
		},
	}
	assert.NoError(t, checkReconfigureCreated(req))
}

// --- checkLocalFileUpToDate ---

func TestCheckLocalFileUpToDate_Match(t *testing.T) {
	content := "test-content-for-checksum"
	f, err := os.CreateTemp("", "reconf-uptodate-*.txt")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())

	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	assert.NoError(t, checkLocalFileUpToDate(f.Name(), checksum))
}

func TestCheckLocalFileUpToDate_Mismatch(t *testing.T) {
	f, err := os.CreateTemp("", "reconf-uptodate-*.txt")
	require.NoError(t, err)
	_, _ = f.WriteString("actual-content")
	f.Close()
	defer os.Remove(f.Name())

	err = checkLocalFileUpToDate(f.Name(), "wrong-checksum")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not up-to-date")
}
