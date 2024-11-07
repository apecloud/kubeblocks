/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/gotemplate"
)

type configVolumeHandleMeta struct {
	ConfigHandler

	mountPoint []string
	reloadType parametersv1alpha1.DynamicReloadType
	configSpec appsv1alpha1.ComponentTemplateSpec

	formatterConfig *parametersv1alpha1.FileFormatConfig
}

func (s *configVolumeHandleMeta) OnlineUpdate(_ context.Context, _ string, _ map[string]string) error {
	return cfgcore.MakeError("not support online update")
}

func (s *configVolumeHandleMeta) VolumeHandle(_ context.Context, _ fsnotify.Event) error {
	logger.Info("not support online update")
	return nil
}

func (s *configVolumeHandleMeta) prepare(backupPath string, filter regexFilter, event fsnotify.Event) (map[string]string, []string, error) {
	var (
		lastVersion = []string{backupPath}
		currVersion = []string{event.Name}
	)

	logger.Info(fmt.Sprintf("prepare for config update: %s %s", s.mountPoint, event.Name))
	currFiles, err := scanConfigFiles(currVersion, filter)
	if err != nil {
		return nil, nil, err
	}
	lastFiles, err := scanConfigFiles(lastVersion, filter)
	if err != nil {
		return nil, nil, err
	}
	updatedParams, err := createUpdatedParamsPatch(currFiles, lastFiles, s.formatterConfig)
	if err != nil {
		return nil, nil, err
	}
	logger.Info(fmt.Sprintf("config update patch: %v", updatedParams))
	return updatedParams, currFiles, nil
}

func (s *configVolumeHandleMeta) MountPoint() []string {
	return s.mountPoint
}

type multiHandler struct {
	handlers map[string]ConfigHandler
}

func (m *multiHandler) MountPoint() []string {
	var mountPoints []string
	for _, handler := range m.handlers {
		mountPoints = append(mountPoints, handler.MountPoint()...)
	}
	return mountPoints
}

func (m *multiHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	if handler, ok := m.handlers[name]; ok {
		return handler.OnlineUpdate(ctx, name, updatedParams)
	}
	logger.Error(cfgcore.MakeError("not found handler for config name: %s", name), fmt.Sprintf("all config names: %v", cfgutil.ToSet(m.handlers).AsSlice()))
	return cfgcore.MakeError("not found handler for config name: %s", name)
}

func isOwnerEvent(volumeDirs []string, event fsnotify.Event) bool {
	for _, dir := range volumeDirs {
		if strings.HasPrefix(event.Name, dir) {
			return true
		}
	}
	return false
}

func (m *multiHandler) VolumeHandle(ctx context.Context, event fsnotify.Event) error {
	for _, handler := range m.handlers {
		if isOwnerEvent(handler.MountPoint(), event) {
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

func (u *unixSignalHandler) OnlineUpdate(ctx context.Context, _ string, updatedParams map[string]string) error {
	return cfgcore.MakeError("not support online update, param: %v", updatedParams)
}

func (u *unixSignalHandler) VolumeHandle(ctx context.Context, _ fsnotify.Event) error {
	pid, err := findParentPIDByProcessName(u.processName, ctx)
	if err != nil {
		return err
	}
	logger.V(1).Info(fmt.Sprintf("find pid: %d from process name[%s]", pid, u.processName))
	return sendSignal(pid, u.signal)
}

func (u *unixSignalHandler) MountPoint() []string {
	return []string{u.mountPoint}
}

func CreateSignalHandler(sig parametersv1alpha1.SignalType, processName string, mountPoint string) (ConfigHandler, error) {
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

	downwardAPITrigger    bool
	downwardAPIMountPoint []string
	downwardAPIHandler    map[string]ConfigHandler

	backupPath string
	configMeta *ConfigSpecInfo
	filter     regexFilter

	isBatchReload      bool
	batchInputTemplate string
}

func generateBatchStdinData(ctx context.Context, updatedParams map[string]string, batchInputTemplate string) (string, error) {
	tplValues := gotemplate.TplValues{}
	for k, v := range updatedParams {
		tplValues[k] = v
	}
	engine := gotemplate.NewTplEngine(&tplValues, nil, "render-batch-input-parameters", nil, ctx)
	stdinStr, err := engine.Render(batchInputTemplate)
	return strings.TrimSpace(stdinStr) + "\n", err
}

func (s *shellCommandHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	logger.V(1).Info(fmt.Sprintf("online update[%v]", updatedParams))
	logger.Info(fmt.Sprintf("updated parameters: %v", updatedParams))
	args := make([]string, len(s.arg))
	copy(args, s.arg)
	return s.execHandler(ctx, updatedParams, args...)
}

func doBatchReloadAction(ctx context.Context, updatedParams map[string]string, fn ActionCallback, batchInputTemplate string, commandName string, args ...string) error {
	// If there are any errors, try to check them before all steps.
	batchStdinStr, err := generateBatchStdinData(ctx, updatedParams, batchInputTemplate)
	if err != nil {
		logger.Error(err, "cannot generate batch stdin data")
		return err
	}

	command := exec.CommandContext(ctx, commandName, args...)
	stdin, err := command.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "cannot create a pipe connecting to the STDIN of the command")
	}

	go func() {
		defer stdin.Close()
		if _, err := io.WriteString(stdin, batchStdinStr); err != nil {
			logger.Error(err, "cannot write batch stdin data into STDIN stream")
		}
	}()

	stdout, err := cfgutil.ExecShellCommand(command)
	if fn != nil {
		fn(stdout, err)
	}
	logger.Info("do batch reload action",
		"command", command.String(),
		"stdin", batchStdinStr,
		"stdout", stdout,
		"error", err,
	)
	return err
}

// ActionCallback is a callback function for testcase.
type ActionCallback func(output string, err error)

func doReloadAction(ctx context.Context, updatedParams map[string]string, fn ActionCallback, commandName string, args ...string) error {
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
	if s.isBatchReload && s.batchInputTemplate != "" {
		return doBatchReloadAction(ctx, updatedParams, nil, s.batchInputTemplate, s.command, args...)
	}
	return doReloadAction(ctx, updatedParams, nil, s.command, args...)
}

func (s *shellCommandHandler) VolumeHandle(ctx context.Context, event fsnotify.Event) error {
	commonHandle := func(args []string) error {
		command := exec.CommandContext(ctx, s.command, args...)
		stdout, err := cfgutil.ExecShellCommand(command)
		logger.Info(fmt.Sprintf("exec: [%s], stdout: [%s], stderr:%v", command.String(), stdout, err))
		return err
	}

	if mountPoint, ok := s.downwardAPIVolume(event); ok {
		return s.processDownwardAPI(ctx, mountPoint, event)
	}
	args := make([]string, len(s.arg))
	for i, v := range s.arg {
		args[i] = strings.ReplaceAll(v, "$volume_dir", event.Name)
	}
	if s.isDownwardAPITrigger() {
		return commonHandle(args)
	}

	logger.V(1).Info(fmt.Sprintf("mountpoint change trigger: [%s], %s", s.mountPoint, event.Name))
	updatedParams, files, err := s.checkAndPrepareUpdate(event)
	if err != nil {
		return err
	}
	if len(updatedParams) == 0 {
		logger.Info("not parameter updated, skip")
		return nil
	}

	logger.Info(fmt.Sprintf("updated parameters: %v", updatedParams))
	if err := s.execHandler(ctx, updatedParams, args...); err != nil {
		return err
	}
	if len(files) != 0 {
		return backupLastConfigFiles(files, s.backupPath)
	}
	return nil
}

func (s *shellCommandHandler) MountPoint() []string {
	return append(s.mountPoint, s.downwardAPIMountPoint...)
}

func (s *shellCommandHandler) checkAndPrepareUpdate(event fsnotify.Event) (map[string]string, []string, error) {
	if s.configMeta == nil || s.backupPath == "" {
		return nil, nil, nil
	}
	updatedParams, files, err := s.prepare(s.backupPath, s.filter, event)
	if err != nil {
		return nil, nil, err
	}
	return updatedParams, files, nil
}

func (s *shellCommandHandler) downwardAPIVolume(event fsnotify.Event) (string, bool) {
	for _, v := range s.downwardAPIMountPoint {
		if strings.HasPrefix(event.Name, v) {
			return v, true
		}
	}
	return "", false
}

func (s *shellCommandHandler) processDownwardAPI(ctx context.Context, mountPoint string, event fsnotify.Event) error {
	if handle, ok := s.downwardAPIHandler[mountPoint]; ok {
		return handle.VolumeHandle(ctx, event)
	}
	logger.Info(fmt.Sprintf("not found downward api command, and pass. path: %s", event.Name))
	return nil
}

func (s *shellCommandHandler) isDownwardAPITrigger() bool {
	return s.downwardAPITrigger
}

func createConfigVolumeMeta(configSpecName string, reloadType parametersv1alpha1.DynamicReloadType, mountPoint []string, formatterConfig *parametersv1alpha1.FileFormatConfig) configVolumeHandleMeta {
	return configVolumeHandleMeta{
		reloadType: reloadType,
		mountPoint: mountPoint,
		configSpec: appsv1alpha1.ComponentTemplateSpec{
			Name: configSpecName,
		},
		formatterConfig: formatterConfig,
	}
}

func isShellCommand(configMeta *ConfigSpecInfo) bool {
	return configMeta != nil &&
		configMeta.ReloadAction != nil &&
		configMeta.ReloadAction.ShellTrigger != nil
}

func isBatchReloadMode(shellAction *parametersv1alpha1.ShellTrigger) bool {
	return shellAction.BatchReload != nil && *shellAction.BatchReload
}

func isValidBatchReload(shellAction *parametersv1alpha1.ShellTrigger) bool {
	return isBatchReloadMode(shellAction) && len(shellAction.BatchParamsFormatterTemplate) > 0
}

func isBatchReload(configMeta *ConfigSpecInfo) bool {
	return isShellCommand(configMeta) && isBatchReloadMode(configMeta.ReloadAction.ShellTrigger)
}

func getBatchInputTemplate(configMeta *ConfigSpecInfo) string {
	if !isShellCommand(configMeta) {
		return ""
	}

	shellAction := configMeta.ReloadAction.ShellTrigger
	if isValidBatchReload(shellAction) {
		return shellAction.BatchParamsFormatterTemplate
	}
	return ""
}

func CreateExecHandler(command []string, mountPoint string, configMeta *ConfigSpecInfo, backupPath string) (ConfigHandler, error) {
	if len(command) == 0 {
		return nil, cfgcore.MakeError("invalid command: %s", command)
	}
	filter, err := createFileRegex(fromConfigSpecInfo(configMeta))
	if err != nil {
		return nil, err
	}

	var formatterConfig *parametersv1alpha1.FileFormatConfig
	if backupPath != "" && configMeta != nil && configMeta.ReloadAction != nil {
		if err := checkAndBackup(*configMeta, []string{configMeta.MountPoint}, filter, backupPath); err != nil {
			return nil, err
		}
		formatterConfig = &configMeta.FormatterConfig
	}
	handler, err := createDownwardHandler(configMeta)
	if err != nil {
		return nil, err
	}

	shellTrigger := &shellCommandHandler{
		command:    command[0],
		arg:        command[1:],
		backupPath: backupPath,
		configMeta: configMeta,
		filter:     filter,
		// for downward api watch
		downwardAPIMountPoint:  cfgutil.ToSet(handler).AsSlice(),
		downwardAPIHandler:     handler,
		configVolumeHandleMeta: createConfigVolumeMeta(configMeta.ConfigSpec.Name, parametersv1alpha1.ShellType, []string{mountPoint}, formatterConfig),
		isBatchReload:          isBatchReload(configMeta),
		batchInputTemplate:     getBatchInputTemplate(configMeta),
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

func createDownwardHandler(meta *ConfigSpecInfo) (map[string]ConfigHandler, error) {
	if meta == nil || len(meta.DownwardAPIOptions) == 0 {
		return nil, nil
	}

	handlers := make(map[string]ConfigHandler)
	for _, field := range meta.DownwardAPIOptions {
		mockConfigSpec := &ConfigSpecInfo{ConfigSpec: appsv1.ComponentTemplateSpec{
			Name:       strings.Join([]string{meta.ConfigSpec.Name, field.Name}, "."),
			VolumeName: field.MountPoint,
		}}
		h, err := CreateExecHandler(field.Command, field.MountPoint, mockConfigSpec, "")
		if err != nil {
			return nil, err
		}
		if execHandler := h.(*shellCommandHandler); execHandler != nil {
			execHandler.downwardAPITrigger = true
		}
		handlers[field.MountPoint] = h
	}
	return handlers, nil
}

type tplScriptHandler struct {
	configVolumeHandleMeta

	tplScripts string
	tplContent string
	fileFilter regexFilter
	engineType string
	dsn        string
	backupPath string
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

func (u *tplScriptHandler) VolumeHandle(ctx context.Context, event fsnotify.Event) error {
	if !isOwnerEvent(u.MountPoint(), event) {
		logger.Info(fmt.Sprintf("ignore event: %s, current watch volume: %s", event.String(), u.mountPoint))
		return nil
	}
	updatedParams, files, err := u.prepare(u.backupPath, u.fileFilter, event)
	if err != nil {
		return err
	}
	if err := u.OnlineUpdate(ctx, event.Name, updatedParams); err != nil {
		return err
	}
	return backupLastConfigFiles(files, u.backupPath)
}

func CreateTPLScriptHandler(name, configPath string, dirs []string, backupPath string) (ConfigHandler, error) {
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
		configVolumeHandleMeta: createConfigVolumeMeta(name, parametersv1alpha1.TPLScriptType, dirs, &tplConfig.FormatterConfig),
		tplContent:             string(tplContent),
		tplScripts:             tplScripts,
		fileFilter:             filter,
		engineType:             tplConfig.DataType,
		dsn:                    dsn,
		backupPath:             backupPath,
	}
	return tplHandler, nil
}

func CreateCombinedHandler(config string, backupPath string) (ConfigHandler, error) {
	shellHandler := func(configMeta ConfigSpecInfo, backupPath string) (ConfigHandler, error) {
		if configMeta.ShellTrigger == nil {
			return nil, cfgcore.MakeError("shell trigger is nil")
		}
		shellTrigger := configMeta.ShellTrigger
		return CreateExecHandler(shellTrigger.Command, configMeta.MountPoint, &configMeta, filepath.Join(backupPath, configMeta.ConfigSpec.Name))
	}
	signalHandler := func(signalTrigger *parametersv1alpha1.UnixSignalTrigger, mountPoint string) (ConfigHandler, error) {
		if signalTrigger == nil {
			return nil, cfgcore.MakeError("signal trigger is nil")
		}
		return CreateSignalHandler(signalTrigger.Signal, signalTrigger.ProcessName, mountPoint)
	}
	tplHandler := func(tplTrigger *parametersv1alpha1.TPLScriptTrigger, configMeta ConfigSpecInfo, backupPath string) (ConfigHandler, error) {
		if tplTrigger == nil {
			return nil, cfgcore.MakeError("tpl trigger is nil")
		}
		return CreateTPLScriptHandler(
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
		if backupPath != "" {
			tmpPath = filepath.Join(backupPath, configMeta.ConfigSpec.Name)
		}
		switch configMeta.ReloadType {
		default:
			return nil, fmt.Errorf("not support reload type: %s", configMeta.ReloadType)
		case parametersv1alpha1.ShellType:
			h, err = shellHandler(configMeta, tmpPath)
		case parametersv1alpha1.UnixSignalType:
			h, err = signalHandler(configMeta.ReloadAction.UnixSignalTrigger, configMeta.MountPoint)
		case parametersv1alpha1.TPLScriptType:
			h, err = tplHandler(configMeta.ReloadAction.TPLScriptTrigger, configMeta, tmpPath)
		}
		if err != nil {
			return nil, err
		}
		hkey := configMeta.ConfigSpec.Name
		if configMeta.ConfigFile == "" {
			hkey = hkey + "/" + configMeta.ConfigFile
		}
		mHandler.handlers[hkey] = h
	}
	return mHandler, nil
}
