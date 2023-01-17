/*
Copyright ApeCloud Inc.

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

package configmap

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/shirou/gopsutil/v3/process"
	"go.uber.org/zap"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
)

var (
	logger = logr.Discard()
)

func SetLogger(zapLogger *zap.Logger) {
	logger = zapr.NewLogger(zapLogger)
	logger = logger.WithName("configmap_volume_watcher")
}

// findParentPidFromProcessName get parent pid
func findParentPidFromProcessName(processName string) (PID, error) {
	allProcess, err := process.Processes()
	if err != nil {
		return InvalidPID, err
	}

	psGraph := map[PID]int32{}
	for _, proc := range allProcess {
		name, err := proc.Name()
		// OS X getting the name of the system process sometimes fails,
		// because OS X Process.Name function depends on sysctl,
		// the function requires elevated permissions.
		if err != nil {
			logger.Error(err, fmt.Sprintf("failed to get process name from pid[%d], and pass", proc.Pid))
			continue
		}
		if name != processName {
			continue
		}
		ppid, err := proc.Ppid()
		if err != nil {
			return InvalidPID, cfgutil.WrapError(err, "failed to get parent pid from pid[%d]", proc.Pid)
		}
		psGraph[PID(proc.Pid)] = ppid
	}

	for key, value := range psGraph {
		if _, ok := psGraph[PID(value)]; !ok {
			return key, nil
		}
	}

	return InvalidPID, cfgutil.MakeError("not find pid fo process name: [%s]", processName)
}

func CreateSignalHandler(sig dbaasv1alpha1.SignalType, processName string) WatchEventHandler {
	signal, ok := allUnixSignals[sig]
	if !ok {
		logger.Error(cfgutil.MakeError("not support unix signal: %s", signal), "failed to create signal handler")
	}
	return func(event fsnotify.Event) error {
		pid, err := findParentPidFromProcessName(processName)
		if err != nil {
			return err
		}
		logger.Info(fmt.Sprintf("find pid: %d from process name[%s]", pid, processName))
		return sendSignal(pid, signal)
	}
}

func IsValidUnixSignal(sig dbaasv1alpha1.SignalType) bool {
	_, ok := allUnixSignals[sig]
	return ok
}
