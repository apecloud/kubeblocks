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
	"os/exec"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/util"
)

func CreateCombinedHandler(config string) (ConfigHandler, error) {
	newShellHandler := func(configMeta ConfigSpecInfo) (ConfigHandler, error) {
		if configMeta.ShellTrigger == nil {
			return nil, cfgcore.MakeError("shell trigger is nil")
		}
		shellTrigger := configMeta.ShellTrigger
		return createShellHandler(shellTrigger.Command, &configMeta)
	}

	var h ConfigHandler
	var handlerMetas []ConfigSpecInfo
	err := cfgutil.FromYamlConfig(config, &handlerMetas)
	if err != nil {
		return nil, err
	}
	mHandler := &combinedHandler{
		handlers: make(map[string]ConfigHandler, len(handlerMetas)),
	}

	for _, configMeta := range handlerMetas {
		switch configMeta.ReloadType {
		case parametersv1alpha1.ShellType:
			h, err = newShellHandler(configMeta)
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

type combinedHandler struct {
	handlers map[string]ConfigHandler
}

func (m *combinedHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	if handler, ok := m.handlers[name]; ok {
		return handler.OnlineUpdate(ctx, name, updatedParams)
	}
	logger.Error(cfgcore.MakeError("not found handler for config name: %s", name), fmt.Sprintf("all config names: %v", cfgutil.ToSet(m.handlers).AsSlice()))
	return cfgcore.MakeError("not found handler for config name: %s", name)
}

type shellHandler struct {
	command    string
	arg        []string
	configMeta *ConfigSpecInfo
}

func (s *shellHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	logger.V(1).Info(fmt.Sprintf("online update[%v]", updatedParams))
	logger.Info(fmt.Sprintf("updated parameters: %v", updatedParams))
	args := make([]string, len(s.arg))
	copy(args, s.arg)
	return doReloadAction(ctx, updatedParams, nil, s.command, args...)
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

func createShellHandler(command []string, configMeta *ConfigSpecInfo) (ConfigHandler, error) {
	if len(command) == 0 {
		return nil, cfgcore.MakeError("invalid command: %s", command)
	}
	return &shellHandler{
		command:    command[0],
		arg:        command[1:],
		configMeta: configMeta,
	}, nil
}
