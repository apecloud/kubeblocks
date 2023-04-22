/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

func TestFindParentPidFromProcessName(t *testing.T) {
	processName := getProcName()
	fmt.Printf("current test program name: %s\n", processName)
	pid, err := findPidFromProcessName(processName)
	require.Nil(t, err)
	require.Equal(t, PID(os.Getpid()), pid)
}

func getProcName() string {
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
