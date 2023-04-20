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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/shirou/gopsutil/v3/process"
	"go.uber.org/zap"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
)

var (
	logger = logr.Discard()
)

func SetLogger(zapLogger *zap.Logger) {
	logger = zapr.NewLogger(zapLogger)
	logger = logger.WithName("configmap_volume_watcher")
}

// findPidFromProcessName gets parent pid
func findPidFromProcessName(processName string) (PID, error) {
	allProcess, err := process.Processes()
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

func CreateSignalHandler(sig appsv1alpha1.SignalType, processName string) (WatchEventHandler, error) {
	signal, ok := allUnixSignals[sig]
	if !ok {
		err := cfgcore.MakeError("not supported unix signal: %s", sig)
		logger.Error(err, "failed to create signal handler")
		return nil, err
	}
	return func(_ context.Context, event fsnotify.Event) error {
		pid, err := findPidFromProcessName(processName)
		if err != nil {
			return err
		}
		logger.V(1).Info(fmt.Sprintf("find pid: %d from process name[%s]", pid, processName))
		return sendSignal(pid, signal)
	}, nil
}

func CreateExecHandler(command string) (WatchEventHandler, error) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return nil, cfgcore.MakeError("invalid command: %s", command)
	}
	cmd := exec.Command(args[0], args[1:]...)
	return func(_ context.Context, _ fsnotify.Event) error {
		stdout, err := cfgutil.ExecShellCommand(cmd)
		if err == nil {
			logger.V(1).Info(fmt.Sprintf("exec: [%s], result: [%s]", command, stdout))
		}
		return err
	}, nil
}

func IsValidUnixSignal(sig appsv1alpha1.SignalType) bool {
	_, ok := allUnixSignals[sig]
	return ok
}

func CreateTPLScriptHandler(tplScripts string, dirs []string, fileRegex string, backupPath string, formatConfig *appsv1alpha1.FormatterConfig, dataType string, dsn string) (WatchEventHandler, error) {
	logger.V(1).Info(fmt.Sprintf("config file regex: %s", fileRegex))
	logger.V(1).Info(fmt.Sprintf("config file reload script: %s", tplScripts))
	if _, err := os.Stat(tplScripts); err != nil {
		return nil, err
	}
	tplContent, err := os.ReadFile(tplScripts)
	if err != nil {
		return nil, err
	}
	if err := checkTPLScript(tplScripts, string(tplContent)); err != nil {
		return nil, err
	}
	filter, err := createFileRegex(fileRegex)
	if err != nil {
		return nil, err
	}
	if err := backupConfigFiles(dirs, filter, backupPath); err != nil {
		return nil, err
	}
	return func(ctx context.Context, event fsnotify.Event) error {
		var (
			lastVersion = []string{backupPath}
			currVersion = []string{filepath.Dir(event.Name)}
		)
		currFiles, err := scanConfigFiles(currVersion, filter)
		if err != nil {
			return err
		}
		lastFiles, err := scanConfigFiles(lastVersion, filter)
		if err != nil {
			return err
		}
		updatedParams, err := createUpdatedParamsPatch(currFiles, lastFiles, formatConfig)
		if err != nil {
			return err
		}
		if err := wrapGoTemplateRun(ctx, tplScripts, string(tplContent), updatedParams, formatConfig, dataType, dsn); err != nil {
			return err
		}
		return backupLastConfigFiles(currFiles, backupPath)
	}, nil
}
