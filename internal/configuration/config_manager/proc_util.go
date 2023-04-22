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
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v3/process"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

// findPidFromProcessName gets parent pid
func findPidFromProcessName(processName string, ctx ...context.Context) (PID, error) {
	var fctx context.Context

	if len(ctx) == 0 {
		fctx = context.TODO()
	} else {
		fctx = ctx[0]
	}
	allProcess, err := process.ProcessesWithContext(fctx)
	if err != nil {
		return InvalidPID, err
	}

	psGraph := map[PID]int32{}
	for _, proc := range allProcess {
		name, err := proc.Name()
		// OS X getting the name of the system process may fail,
		// because OS X Process.Name function depends on sysctl and elevated permissions
		if err != nil {
			logger.Error(err, fmt.Sprintf("failed to get process name from pid[%d], and pass", proc.Pid))
			continue
		}
		if name != processName {
			continue
		}
		ppid, err := proc.Ppid()
		if err != nil {
			return InvalidPID, cfgcore.WrapError(err, "failed to get parent pid from pid[%d]", proc.Pid)
		}
		psGraph[PID(proc.Pid)] = ppid
	}

	for key, value := range psGraph {
		if _, ok := psGraph[PID(value)]; !ok {
			return key, nil
		}
	}

	return InvalidPID, cfgcore.MakeError("cannot find pid of process name: [%s]", processName)
}

func IsValidUnixSignal(sig appsv1alpha1.SignalType) bool {
	_, ok := allUnixSignals[sig]
	return ok
}
