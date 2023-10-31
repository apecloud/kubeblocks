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
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v3/process"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
)

// findParentPIDByProcessName gets parent pid
func findParentPIDByProcessName(processName string, ctx ...context.Context) (PID, error) {
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
