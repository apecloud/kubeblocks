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

	"github.com/fsnotify/fsnotify"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ConfigHandler interface {
	OnlineUpdate(ctx context.Context, name string, updatedParams map[string]string) error
	VolumeHandle(ctx context.Context, event fsnotify.Event) error
	MountPoint() []string
}

type ConfigSpecInfo struct {
	*appsv1alpha1.ReloadOptions `json:",inline"`

	ReloadType      appsv1alpha1.CfgReloadType       `json:"reloadType"`
	ConfigSpec      appsv1alpha1.ComponentConfigSpec `json:"configSpec"`
	FormatterConfig appsv1alpha1.FormatterConfig     `json:"formatterConfig"`

	DownwardAPIOptions []appsv1alpha1.DownwardAPIOption `json:"downwardAPIOptions"`

	// config volume mount path
	MountPoint string `json:"mountPoint"`
	TPLConfig  string `json:"tplConfig"`
}

type ConfigSpecMeta struct {
	ConfigSpecInfo `json:",inline"`

	ScriptConfig   []appsv1alpha1.ScriptConfig
	ToolsImageSpec *appsv1alpha1.ToolsImageSpec
}

type TPLScriptConfig struct {
	Scripts   string `json:"scripts"`
	FileRegex string `json:"fileRegex"`
	DataType  string `json:"dataType"`
	DSN       string `json:"dsn"`

	FormatterConfig appsv1alpha1.FormatterConfig `json:"formatterConfig"`
}

type ConfigLazyRenderedMeta struct {
	*appsv1alpha1.ComponentConfigSpec `json:",inline"`

	// secondary template path
	Templates       []string                     `json:"templates"`
	FormatterConfig appsv1alpha1.FormatterConfig `json:"formatterConfig"`
}
