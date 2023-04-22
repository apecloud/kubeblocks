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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"
)

type ConfigHandler interface {
	OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error
	VolumeHandle(ctx context.Context, event fsnotify.Event) error
	MountPoint() string
}

type ConfigSpecMeta struct {
	*appsv1alpha1.ReloadOptions `json:",inline"`

	ReloadType appsv1alpha1.CfgReloadType       `json:"reloadType"`
	ConfigSpec appsv1alpha1.ComponentConfigSpec `json:"configSpec"`

	// config volume mount path
	TPLConfig  string `json:"tplConfig"`
	MountPoint string `json:"mountPoint"`
	// EngineType string `json:"engineType"`
	// DSN        string `json:"dsn"`

	FormatterConfig appsv1alpha1.FormatterConfig `json:"formatterConfig"`
}

type TPLScriptConfig struct {
	Scripts   string `json:"scripts"`
	FileRegex string `json:"fileRegex"`
	DataType  string `json:"dataType"`
	DSN       string `json:"dsn"`

	FormatterConfig appsv1alpha1.FormatterConfig `json:"formatterConfig"`
}

type configVolumeHandleMeta struct {
	ConfigHandler

	mountPoint string
	reloadType appsv1alpha1.CfgReloadType
	configSpec appsv1alpha1.ComponentTemplateSpec
}

func (s *configVolumeHandleMeta) OnlineUpdate(_ context.Context, _ string, _ map[string]string) error {
	logger.Info("not support online update")
	return nil
}

func (s *configVolumeHandleMeta) VolumeHandle(_ context.Context, _ fsnotify.Event) error {
	logger.Info("not support online update")
	return nil
}

func (s *configVolumeHandleMeta) MountPoint() string {
	return s.mountPoint
}

type multiHandler struct {
	handlers map[string]ConfigHandler
}

func (m *multiHandler) MountPoint() string {
	return ""
}

func (m *multiHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	if handler, ok := m.handlers[name]; ok {
		return handler.OnlineUpdate(ctx, name, updatedParams)
	}
	logger.Error(cfgcore.MakeError("not found handler for config name: %s", name), fmt.Sprintf("all config names: %v", cfgutil.ToSet(m.handlers).AsSlice()))
	return nil
}

func (m *multiHandler) VolumeHandle(ctx context.Context, event fsnotify.Event) error {
	for _, handler := range m.handlers {
		if strings.HasPrefix(event.Name, handler.MountPoint()) {
			return handler.VolumeHandle(ctx, event)
		}
	}
	logger.Error(cfgcore.MakeError("not found handler for config name: %s", event.Name), fmt.Sprintf("all config names: %v", cfgutil.ToSet(m.handlers).AsSlice()))
	return nil
}

type unixSignalHandler struct {
	processName string
	signal      os.Signal

	mountPoint string
}

func (u *unixSignalHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	logger.Info("not support online update")
	return nil
}

func (u *unixSignalHandler) VolumeHandle(ctx context.Context, event fsnotify.Event) error {
	pid, err := findPidFromProcessName(u.processName, ctx)
	if err != nil {
		return err
	}
	logger.V(1).Info(fmt.Sprintf("find pid: %d from process name[%s]", pid, u.processName))
	return sendSignal(pid, u.signal)
}

func (u *unixSignalHandler) MountPoint() string {
	return u.mountPoint
}

func CreateSignalHandler(sig appsv1alpha1.SignalType, processName string, mountPoint string) (ConfigHandler, error) {
	signal, ok := allUnixSignals[sig]
	if !ok {
		err := cfgcore.MakeError("not supported unix signal: %s", sig)
		logger.Error(err, "failed to create signal handler")
		return nil, err
	}
	if processName == "" {
		return nil, fmt.Errorf("process name is empty")
	}
	return &unixSignalHandler{
		processName: processName,
		signal:      signal,
		mountPoint:  mountPoint,
	}, nil
}

type shellCommandHandler struct {
	configVolumeHandleMeta
	command string
	arg     []string
}

func (s *shellCommandHandler) VolumeHandle(ctx context.Context, event fsnotify.Event) error {
	args := make([]string, len(s.arg))
	for i, v := range s.arg {
		args[i] = strings.ReplaceAll(v, "$volume_dir", event.Name)
	}
	command := exec.CommandContext(ctx, s.command, args...)
	stdout, err := cfgutil.ExecShellCommand(command)
	if err == nil {
		logger.V(1).Info(fmt.Sprintf("exec: [%s], result: [%s]", command.String(), stdout))
	}
	return err
}

func createConfigVolumeMeta(configSpecName string, reloadType appsv1alpha1.CfgReloadType, mountPoint string) configVolumeHandleMeta {
	return configVolumeHandleMeta{
		reloadType: reloadType,
		mountPoint: mountPoint,
		configSpec: appsv1alpha1.ComponentTemplateSpec{
			Name: configSpecName,
		},
	}
}

func CreateExecHandler(command string, mountPoint string) (ConfigHandler, error) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return nil, cfgcore.MakeError("invalid command: %s", command)
	}

	shellTrigger := &shellCommandHandler{
		command:                args[0],
		arg:                    args[1:],
		configVolumeHandleMeta: createConfigVolumeMeta("", appsv1alpha1.ShellType, mountPoint),
	}
	return shellTrigger, nil
}

type tplScriptHandler struct {
	configVolumeHandleMeta

	tplContent      string
	fileFilter      regexFilter
	engineType      string
	dsn             string
	formatterConfig appsv1alpha1.FormatterConfig
	backupPath      string
}

func (u *tplScriptHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	logger.V(1).Info(fmt.Sprintf("online update[%v]", updatedParams))
	return wrapGoTemplateRun(ctx,
		u.configVolumeHandleMeta.configSpec.Name,
		u.tplContent,
		updatedParams,
		&u.formatterConfig,
		u.engineType, u.dsn)
}

func (u *tplScriptHandler) VolumeHandle(ctx context.Context, event fsnotify.Event) error {
	var (
		lastVersion = []string{u.backupPath}
		currVersion = []string{filepath.Dir(event.Name)}
	)
	currFiles, err := scanConfigFiles(currVersion, u.fileFilter)
	if err != nil {
		return err
	}
	lastFiles, err := scanConfigFiles(lastVersion, u.fileFilter)
	if err != nil {
		return err
	}
	updatedParams, err := createUpdatedParamsPatch(currFiles, lastFiles, &u.formatterConfig)
	if err != nil {
		return err
	}
	if err := u.OnlineUpdate(ctx, "", updatedParams); err != nil {
		return err
	}
	return backupLastConfigFiles(currFiles, u.backupPath)
}

func CreateTPLScriptHandler(name, configPath string, dirs []string, backupPath string) (ConfigHandler, error) {
	if _, err := os.Stat(configPath); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	tplConfig := TPLScriptConfig{}
	if err := yaml.Unmarshal(b, &tplConfig); err != nil {
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
	if err := backupConfigFiles(dirs, filter, backupPath); err != nil {
		return nil, err
	}
	tplHandler := &tplScriptHandler{
		configVolumeHandleMeta: createConfigVolumeMeta(name, appsv1alpha1.TPLScriptType, dirs[0]),
		tplContent:             string(tplContent),
		fileFilter:             filter,
		engineType:             tplConfig.DataType,
		dsn:                    tplConfig.DSN,
		backupPath:             backupPath,
		formatterConfig:        tplConfig.FormatterConfig,
	}
	return tplHandler, nil
}

func CreateCombinedHandler(config string, backupPath string) (ConfigHandler, error) {
	shellHandler := func(shellTrigger *appsv1alpha1.ShellTrigger, mountPoint string) (ConfigHandler, error) {
		if shellTrigger == nil {
			return nil, cfgcore.MakeError("shell trigger is nil")
		}
		return CreateExecHandler(shellTrigger.Exec, mountPoint)
	}
	signalHandler := func(signalTrigger *appsv1alpha1.UnixSignalTrigger, mountPoint string) (ConfigHandler, error) {
		if signalTrigger == nil {
			return nil, cfgcore.MakeError("signal trigger is nil")
		}
		return CreateSignalHandler(signalTrigger.Signal, signalTrigger.ProcessName, mountPoint)
	}
	tplHandler := func(tplTrigger *appsv1alpha1.TPLScriptTrigger, configMeta ConfigSpecMeta) (ConfigHandler, error) {
		if tplTrigger != nil {
			return nil, cfgcore.MakeError("tpl trigger is nil")
		}
		return CreateTPLScriptHandler(
			configMeta.ConfigSpec.Name,
			configMeta.TPLConfig,
			[]string{configMeta.MountPoint},
			filepath.Join(backupPath, configMeta.ConfigSpec.Name),
		)
	}

	var handlerMetas []ConfigSpecMeta
	err := json.Unmarshal([]byte(config), &handlerMetas)
	if err != nil {
		return nil, err
	}
	mhandler := &multiHandler{
		handlers: make(map[string]ConfigHandler, len(handlerMetas)),
	}

	var h ConfigHandler
	for _, configMeta := range handlerMetas {
		switch configMeta.ReloadType {
		default:
			return nil, fmt.Errorf("not support reload type: %s", configMeta.ReloadType)
		case appsv1alpha1.ShellType:
			h, err = shellHandler(configMeta.ReloadOptions.ShellTrigger, configMeta.MountPoint)
		case appsv1alpha1.UnixSignalType:
			h, err = signalHandler(configMeta.ReloadOptions.UnixSignalTrigger, configMeta.MountPoint)
		case appsv1alpha1.TPLScriptType:
			h, err = tplHandler(configMeta.ReloadOptions.TPLScriptTrigger, configMeta)
		}
		if err != nil {
			return nil, err
		}
		mhandler.handlers[configMeta.ConfigSpec.Name] = h
	}
	return mhandler, nil
}
