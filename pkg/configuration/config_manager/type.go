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

	"github.com/fsnotify/fsnotify"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

type ConfigHandler interface {
	OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error
	VolumeHandle(ctx context.Context, event fsnotify.Event) error
	MountPoint() []string
}

type ConfigSpecInfo struct {
	*parametersv1alpha1.ReloadAction `json:",inline"`

	ReloadType      parametersv1alpha1.DynamicReloadType `json:"reloadType"`
	ConfigSpec      appsv1.ComponentFileTemplate         `json:"configSpec"`
	FormatterConfig parametersv1alpha1.FileFormatConfig  `json:"formatterConfig"`
	ConfigFile      string                               `json:"configFile"`

	DownwardAPIOptions []parametersv1alpha1.DownwardAPIChangeTriggeredAction `json:"downwardAPIOptions"`

	// config volume mount path
	MountPoint string `json:"mountPoint"`
	TPLConfig  string `json:"tplConfig"`
}

type ConfigSpecMeta struct {
	ConfigSpecInfo `json:",inline"`

	ScriptConfig   []parametersv1alpha1.ScriptConfig
	ToolsImageSpec *parametersv1alpha1.ToolsSetup
}

type TPLScriptConfig struct {
	Scripts   string `json:"scripts"`
	FileRegex string `json:"fileRegex"`
	DataType  string `json:"dataType"`
	DSN       string `json:"dsn"`

	FormatterConfig parametersv1alpha1.FileFormatConfig `json:"formatterConfig"`
}
