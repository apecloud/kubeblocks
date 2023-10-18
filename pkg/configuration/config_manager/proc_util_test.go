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

package configmanager

import (
	"fmt"
	"os"
	"testing"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func init() {
	var zapLog, _ = zap.NewDevelopment()
	SetLogger(zapLog)
}

func TestFindParentPidByProcessName(t *testing.T) {
	processName := findCurrProcName()
	fmt.Printf("current test program name: %s\n", processName)
	pid, err := findParentPIDByProcessName(processName)
	require.Nil(t, err)
	require.Equal(t, PID(os.Getpid()), pid)
}

func findCurrProcName() string {
	pid := int32(os.Getpid())
	procs, _ := process.Processes()
	for _, proc := range procs {
		if pid == proc.Pid {
			name, _ := proc.Name()
			return name
		}
	}
	return ""
}
