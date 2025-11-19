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

package configmanager

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/util"
)

func CreateCombinedHandler(config string) (ConfigHandler, error) {
	shellHandler := func(configMeta ConfigSpecInfo, backupPath string) (ConfigHandler, error) {
		if configMeta.ShellTrigger == nil {
			return nil, cfgcore.MakeError("shell trigger is nil")
		}
		shellTrigger := configMeta.ShellTrigger
		return createExecHandler(shellTrigger.Command, &configMeta, filepath.Join(backupPath, configMeta.ConfigSpec.Name))
	}
	tplHandler := func(tplTrigger *parametersv1alpha1.TPLScriptTrigger, configMeta ConfigSpecInfo, backupPath string) (ConfigHandler, error) {
		if tplTrigger == nil {
			return nil, cfgcore.MakeError("tpl trigger is nil")
		}
		return createTPLScriptHandler(
			configMeta.ConfigSpec.Name,
			configMeta.TPLConfig,
			[]string{configMeta.MountPoint},
			backupPath,
		)
	}

	var h ConfigHandler
	var handlerMetas []ConfigSpecInfo
	err := cfgutil.FromYamlConfig(config, &handlerMetas)
	if err != nil {
		return nil, err
	}
	mHandler := &multiHandler{
		handlers: make(map[string]ConfigHandler, len(handlerMetas)),
	}

	tmpPath := ""
	for _, configMeta := range handlerMetas {
		switch configMeta.ReloadType {
		case parametersv1alpha1.ShellType:
			h, err = shellHandler(configMeta, tmpPath)
		case parametersv1alpha1.TPLScriptType:
			h, err = tplHandler(configMeta.ReloadAction.TPLScriptTrigger, configMeta, tmpPath)
		default:
			return nil, fmt.Errorf("not support reload type: %s", configMeta.ReloadType)
		}
		if err != nil {
			return nil, err
		}
		hkey := configMeta.ConfigSpec.Name
		if configMeta.ConfigFile != "" {
			hkey = hkey + "/" + configMeta.ConfigFile
		}
		mHandler.handlers[hkey] = h
	}
	return mHandler, nil
}

type multiHandler struct {
	handlers map[string]ConfigHandler
}

func (m *multiHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	if handler, ok := m.handlers[name]; ok {
		return handler.OnlineUpdate(ctx, name, updatedParams)
	}
	logger.Error(cfgcore.MakeError("not found handler for config name: %s", name), fmt.Sprintf("all config names: %v", cfgutil.ToSet(m.handlers).AsSlice()))
	return cfgcore.MakeError("not found handler for config name: %s", name)
}

type shellCommandHandler struct {
	command    string
	arg        []string
	backupPath string
	configMeta *ConfigSpecInfo
}

func (s *shellCommandHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	logger.V(1).Info(fmt.Sprintf("online update[%v]", updatedParams))
	logger.Info(fmt.Sprintf("updated parameters: %v", updatedParams))
	args := make([]string, len(s.arg))
	copy(args, s.arg)
	return s.execHandler(ctx, updatedParams, args...)
}

type actionCallback func(output string, err error)

func doReloadAction(ctx context.Context, updatedParams map[string]string, fn actionCallback, commandName string, args ...string) error {
	commonHandle := func(args []string) error {
		command := exec.CommandContext(ctx, commandName, args...)
		stdout, err := cfgutil.ExecShellCommand(command)
		if fn != nil {
			fn(stdout, err)
		}
		logger.Info("do reload action",
			"command", command.String(),
			"stdout", stdout,
			"err", err,
		)
		return err
	}
	volumeHandle := func(baseCMD []string, paramName, paramValue string) error {
		args := make([]string, len(baseCMD))
		copy(args, baseCMD)
		args = append(args, paramName, paramValue)
		return commonHandle(args)
	}
	for key, value := range updatedParams {
		if err := volumeHandle(args, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *shellCommandHandler) execHandler(ctx context.Context, updatedParams map[string]string, args ...string) error {
	return doReloadAction(ctx, updatedParams, nil, s.command, args...)
}

func createExecHandler(command []string, configMeta *ConfigSpecInfo, backupPath string) (ConfigHandler, error) {
	if len(command) == 0 {
		return nil, cfgcore.MakeError("invalid command: %s", command)
	}
	filter, err := createFileRegex(fromConfigSpecInfo(configMeta))
	if err != nil {
		return nil, err
	}

	if backupPath != "" && configMeta != nil && configMeta.ReloadAction != nil {
		if err := checkAndBackup(*configMeta, []string{configMeta.MountPoint}, filter, backupPath); err != nil {
			return nil, err
		}
	}

	shellTrigger := &shellCommandHandler{
		command:    command[0],
		arg:        command[1:],
		backupPath: backupPath,
		configMeta: configMeta,
	}
	return shellTrigger, nil
}

func checkAndBackup(configMeta ConfigSpecInfo, dirs []string, filter regexFilter, backupPath string) error {
	if isSyncReloadAction(configMeta) {
		return nil
	}
	if err := backupConfigFiles(dirs, filter, backupPath); err != nil {
		return err
	}
	return nil
}

func fromConfigSpecInfo(meta *ConfigSpecInfo) string {
	if meta == nil || len(meta.ConfigFile) == 0 {
		return ""
	}
	return meta.ConfigFile
}

type tplScriptHandler struct {
	tplScripts      string
	tplContent      string
	engineType      string
	dsn             string
	backupPath      string
	formatterConfig *parametersv1alpha1.FileFormatConfig
}

func (u *tplScriptHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	logger.V(1).Info(fmt.Sprintf("online update[%v]", updatedParams), "file", name)
	return wrapGoTemplateRun(ctx,
		u.tplScripts,
		u.tplContent,
		updatedParams,
		u.formatterConfig,
		u.engineType, u.dsn)
}

func createTPLScriptHandler(name, configPath string, dirs []string, backupPath string) (ConfigHandler, error) {
	tplConfig := TPLScriptConfig{}
	if err := cfgutil.FromYamlConfig(configPath, &tplConfig); err != nil {
		return nil, err
	}

	tplScripts := filepath.Join(filepath.Dir(configPath), tplConfig.Scripts)
	tplContent, err := os.ReadFile(tplScripts)
	if err != nil {
		return nil, err
	}
	if err := checkTPLScript(tplScripts, string(tplContent)); err != nil {
		return nil, err
	}
	filter, err := createFileRegex(tplConfig.FileRegex)
	if err != nil {
		return nil, err
	}
	dsn := tplConfig.DSN
	if dsn != "" {
		dsn, err = renderDSN(dsn)
		if err != nil {
			return nil, err
		}
	}
	if err := backupConfigFiles(dirs, filter, backupPath); err != nil {
		return nil, err
	}
	tplHandler := &tplScriptHandler{
		tplContent:      string(tplContent),
		tplScripts:      tplScripts,
		engineType:      tplConfig.DataType,
		dsn:             dsn,
		backupPath:      backupPath,
		formatterConfig: &tplConfig.FormatterConfig,
	}
	return tplHandler, nil
}
