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

package component

import (
	"path/filepath"
	"strings"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	renderTask = "render"
)

func NewRenderTask(compName, uid string, replicas []string, synthesizedComp *SynthesizedComponent, files map[string][]string) (map[string]string, error) {
	if len(synthesizedComp.FileTemplates) == 0 {
		return nil, nil
	}

	task := proto.Task{
		Instance:            compName,
		Task:                renderTask,
		UID:                 uid,
		Replicas:            strings.Join(replicas, ","),
		NotifyAtFinish:      false,
		ReportPeriodSeconds: 0,
		Render: &proto.RenderTask{
			Templates: []proto.RenderTaskFileTemplate{},
		},
	}
	for _, tpl := range synthesizedComp.FileTemplates {
		task.Render.Templates = append(task.Render.Templates, proto.RenderTaskFileTemplate{
			Name:      tpl.Name,
			Files:     templateFiles(synthesizedComp, tpl.Name, files[tpl.Name]),
			Variables: tpl.Variables,
		})
	}
	return buildKBAgentTaskEnv(task)
}

func templateFiles(synthesizedComp *SynthesizedComponent, tpl string, files []string) []string {
	result := make([]string, 0)
	for _, f := range files {
		fullPath := absoluteTemplateFilePath(synthesizedComp, tpl, f)
		if len(fullPath) > 0 {
			result = append(result, fullPath)
		}
	}
	return result
}

func absoluteTemplateFilePath(synthesizedComp *SynthesizedComponent, tpl, file string) string {
	var volName, mountPath string
	for _, fileTpl := range synthesizedComp.FileTemplates {
		if fileTpl.Name == tpl {
			volName = fileTpl.VolumeName
			break
		}
	}
	if volName == "" {
		return "" // has no volumes specified
	}

	for _, container := range synthesizedComp.PodSpec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volName {
				mountPath = mount.MountPath
				break
			}
		}
		if mountPath != "" {
			break
		}
	}
	if mountPath == "" {
		return "" // the template is not mounted, ignore it
	}

	return filepath.Join(mountPath, file)
}
