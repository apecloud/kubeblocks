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

	"github.com/fsnotify/fsnotify"
	"k8s.io/apimachinery/pkg/util/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
)

type ConfigHandler interface {
	OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error
	VolumeHandle(ctx context.Context, event fsnotify.Event) error
	MountPoint() []string
}

type ConfigSpecMeta struct {
	*appsv1alpha1.ReloadOptions `json:",inline"`

	ReloadType appsv1alpha1.CfgReloadType       `json:"reloadType"`
	ConfigSpec appsv1alpha1.ComponentConfigSpec `json:"configSpec"`

	ToolConfigs        []appsv1alpha1.ToolConfig
	DownwardAPIOptions []appsv1alpha1.DownwardAPIOption

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

	mountPoint []string
	reloadType appsv1alpha1.CfgReloadType
	configSpec appsv1alpha1.ComponentTemplateSpec

	formatterConfig *appsv1alpha1.FormatterConfig
}

func (s *configVolumeHandleMeta) OnlineUpdate(_ context.Context, _ string, _ map[string]string) error {
	logger.Info("not support online update")
	return nil
}

func (s *configVolumeHandleMeta) VolumeHandle(_ context.Context, _ fsnotify.Event) error {
	logger.Info("not support online update")
	return nil
}

func (s *configVolumeHandleMeta) prepare(backupPath string, filter regexFilter, event fsnotify.Event) (map[string]string, []string, error) {
	var (
		lastVersion = []string{backupPath}
		currVersion = []string{filepath.Dir(event.Name)}
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
	return nil
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

func (u *unixSignalHandler) MountPoint() []string {
	return []string{u.mountPoint}
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

	downwardAPICommand    []string
	downwardAPIMountPoint []string

	backupPath string
	configMeta *ConfigSpecMeta
	filter     regexFilter
}

func (s *shellCommandHandler) VolumeHandle(ctx context.Context, event fsnotify.Event) error {
	if mountPoint, ok := s.downwardAPIVolume(event); ok {
		return s.processDownwardAPI(ctx, mountPoint)
	}
	args := make([]string, len(s.arg))
	for i, v := range s.arg {
		args[i] = strings.ReplaceAll(v, "$volume_dir", event.Name)
	}

	logger.V(1).Info(fmt.Sprintf("mountpoint change trigger: [%s], %s", s.mountPoint, event.Name))
	updatedParams, files, err := s.checkAndPrepareUpdate(event)
	if err != nil {
		return err
	}
	if len(updatedParams) != 0 {
		args = append(args, cfgutil.ToArgs(updatedParams)...)
	}

	command := exec.CommandContext(ctx, s.command, args...)
	stdout, err := cfgutil.ExecShellCommand(command)
	logger.V(1).Info(fmt.Sprintf("exec: [%s], stdout: [%s], stderr:%v", command.String(), stdout, err))

	if err == nil && len(files) != 0 {
		return backupLastConfigFiles(files, s.backupPath)
	}
	return err
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

func (s *shellCommandHandler) processDownwardAPI(ctx context.Context, mountPoint string) error {
	if len(s.downwardAPICommand) == 0 {
		logger.Info("not found downward api command, and pass.")
		return nil
	}

	args := s.downwardAPICommand[1:]
	args = append(args, mountPoint)
	command := exec.CommandContext(ctx, s.downwardAPICommand[0], args...)
	stdout, err := cfgutil.ExecShellCommand(command)
	logger.V(1).Info(fmt.Sprintf("exec: [%s], stdout: [%s], stderr:%v", command.String(), stdout, err))
	return err
}

func createConfigVolumeMeta(configSpecName string, reloadType appsv1alpha1.CfgReloadType, mountPoint []string, formatterConfig *appsv1alpha1.FormatterConfig) configVolumeHandleMeta {
	return configVolumeHandleMeta{
		reloadType: reloadType,
		mountPoint: mountPoint,
		configSpec: appsv1alpha1.ComponentTemplateSpec{
			Name: configSpecName,
		},
		formatterConfig: formatterConfig,
	}
}

func CreateExecHandler(command string, mountPoint string, configMeta *ConfigSpecMeta, backupPath string) (ConfigHandler, error) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return nil, cfgcore.MakeError("invalid command: %s", command)
	}
	filter, err := createFileRegex(fromConfigSpecMeta(configMeta))
	if err != nil {
		return nil, err
	}

	var formatterConfig *appsv1alpha1.FormatterConfig
	if backupPath != "" && configMeta != nil {
		if err := backupConfigFiles([]string{configMeta.MountPoint}, filter, backupPath); err != nil {
			return nil, err
		}
		formatterConfig = &configMeta.FormatterConfig
	}
	shellTrigger := &shellCommandHandler{
		command:                args[0],
		arg:                    args[1:],
		backupPath:             backupPath,
		configMeta:             configMeta,
		filter:                 filter,
		downwardAPICommand:     fromDownwardCommand(configMeta),
		downwardAPIMountPoint:  fromDownwardMountPoint(configMeta),
		configVolumeHandleMeta: createConfigVolumeMeta("", appsv1alpha1.ShellType, []string{mountPoint}, formatterConfig),
	}
	return shellTrigger, nil
}

func fromConfigSpecMeta(meta *ConfigSpecMeta) string {
	if meta == nil || len(meta.ConfigSpec.Keys) == 0 {
		return ""
	}
	if len(meta.ConfigSpec.Keys) == 1 {
		return meta.ConfigSpec.Keys[0]
	}
	return "( " + strings.Join(meta.ConfigSpec.Keys, " | ") + " )"
}

func fromDownwardMountPoint(meta *ConfigSpecMeta) []string {
	if meta == nil || len(meta.DownwardAPIOptions) == 0 {
		return nil
	}
	var mountPoints []string
	for _, field := range meta.DownwardAPIOptions {
		mountPoints = append(mountPoints, field.MountPoint)
	}
	return mountPoints
}

func fromDownwardCommand(meta *ConfigSpecMeta) []string {
	if meta == nil || meta.ReloadOptions == nil || meta.ReloadOptions.ShellTrigger == nil {
		return nil
	}
	return meta.ReloadOptions.ShellTrigger.DownwardAPIExec
}

type tplScriptHandler struct {
	configVolumeHandleMeta

	tplContent string
	fileFilter regexFilter
	engineType string
	dsn        string
	backupPath string
}

func (u *tplScriptHandler) OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error {
	logger.V(1).Info(fmt.Sprintf("online update[%v]", updatedParams))
	return wrapGoTemplateRun(ctx,
		u.configVolumeHandleMeta.configSpec.Name,
		u.tplContent,
		updatedParams,
		u.formatterConfig,
		u.engineType, u.dsn)
}

func (u *tplScriptHandler) VolumeHandle(ctx context.Context, event fsnotify.Event) error {
	if isOwnerEvent(u.MountPoint(), event) {
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
		configVolumeHandleMeta: createConfigVolumeMeta(name, appsv1alpha1.TPLScriptType, dirs, &tplConfig.FormatterConfig),
		tplContent:             string(tplContent),
		fileFilter:             filter,
		engineType:             tplConfig.DataType,
		dsn:                    tplConfig.DSN,
		backupPath:             backupPath,
	}
	return tplHandler, nil
}

func CreateCombinedHandler(config string, backupPath string) (ConfigHandler, error) {
	shellHandler := func(configMeta ConfigSpecMeta, backupPath string) (ConfigHandler, error) {
		if configMeta.ShellTrigger == nil {
			return nil, cfgcore.MakeError("shell trigger is nil")
		}
		shellTrigger := configMeta.ShellTrigger
		return CreateExecHandler(shellTrigger.Exec, configMeta.MountPoint, &configMeta, filepath.Join(backupPath, configMeta.ConfigSpec.Name))
	}
	signalHandler := func(signalTrigger *appsv1alpha1.UnixSignalTrigger, mountPoint string) (ConfigHandler, error) {
		if signalTrigger == nil {
			return nil, cfgcore.MakeError("signal trigger is nil")
		}
		return CreateSignalHandler(signalTrigger.Signal, signalTrigger.ProcessName, mountPoint)
	}
	tplHandler := func(tplTrigger *appsv1alpha1.TPLScriptTrigger, configMeta ConfigSpecMeta, backupPath string) (ConfigHandler, error) {
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

	var handlerMetas []ConfigSpecMeta
	err := json.Unmarshal([]byte(config), &handlerMetas)
	if err != nil {
		return nil, err
	}

	var h ConfigHandler
	mhandler := &multiHandler{
		handlers: make(map[string]ConfigHandler, len(handlerMetas)),
	}

	for _, configMeta := range handlerMetas {
		tmpPath := filepath.Join(backupPath, configMeta.ConfigSpec.Name)
		switch configMeta.ReloadType {
		default:
			return nil, fmt.Errorf("not support reload type: %s", configMeta.ReloadType)
		case appsv1alpha1.ShellType:
			h, err = shellHandler(configMeta, tmpPath)
		case appsv1alpha1.UnixSignalType:
			h, err = signalHandler(configMeta.ReloadOptions.UnixSignalTrigger, configMeta.MountPoint)
		case appsv1alpha1.TPLScriptType:
			h, err = tplHandler(configMeta.ReloadOptions.TPLScriptTrigger, configMeta, tmpPath)
		}
		if err != nil {
			return nil, err
		}
		mhandler.handlers[configMeta.ConfigSpec.Name] = h
	}
	return mhandler, nil
}
