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
	"os"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"

	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
)

var allUnixSignals = map[string]os.Signal{
	"SIGHUP":  syscall.SIGHUP,  // reload signal for mysql 8.x.xxx
	"SIGTERM": syscall.SIGTERM, // shutdown signal
	"SIGINT":  syscall.SIGINT,
	"SIGKILL": syscall.SIGKILL,
	"SIGSEGV": syscall.SIGSEGV,
	"SIGQUIT": syscall.SIGQUIT,
	"SIGUSR1": syscall.SIGUSR1,
	"SIGUSR2": syscall.SIGUSR2,
}

// findParentPidFromProcessName get parent pid
func findParentPidFromProcessName(processName string) (PID, error) {
	allProcess, err := process.Processes()
	if err != nil {
		return INVALID_PID, err
	}

	psGraph := map[PID]int32{}
	for _, proc := range allProcess {
		name, err := proc.Name()
		if err != nil {
			return INVALID_PID, cfgutil.WrapError(err, "failed to get process name from pid[%d]", proc.Pid)
		}
		if name != processName {
			continue
		}
		ppid, err := proc.Ppid()
		if err != nil {
			return INVALID_PID, cfgutil.WrapError(err, "failed to get parent pid from pid[%d]", proc.Pid)
		}
		psGraph[PID(proc.Pid)] = ppid
	}

	for key, value := range psGraph {
		if _, ok := psGraph[PID(value)]; !ok {
			return key, nil
		}
	}

	return INVALID_PID, cfgutil.MakeError("not find pid fo process name: [%s]", processName)
}

func CreateSignalHandler(sig string, processName string) WatchEventHandler {
	signal, ok := allUnixSignals[strings.ToUpper(sig)]
	if !ok {
		logrus.Fatalf("not support unix signal: %s", signal)
	}
	return func(event fsnotify.Event) error {
		pid, err := findParentPidFromProcessName(processName)
		if err != nil {
			return err
		}
		logrus.Tracef("find pid: %d from process name[%s]", pid, processName)
		return sendSignal(pid, signal)
	}
}

func IsValidUnixSignal(sig string) bool {
	_, ok := allUnixSignals[strings.ToUpper(sig)]
	return ok
}
