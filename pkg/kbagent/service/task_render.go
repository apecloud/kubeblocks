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

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

type renderTask struct {
	logger logr.Logger
	task   *proto.RenderTask
}

var _ task = &renderTask{}

func (s *renderTask) run(ctx context.Context) (chan error, error) {
	for _, tpl := range s.task.Templates {
		if err := s.renderTemplate(tpl); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (s *renderTask) status(ctx context.Context, event *proto.TaskEvent) {
}

func (s *renderTask) renderTemplate(tpl proto.RenderTaskFileTemplate) error {
	variables := util.EnvL2M(os.Environ())
	for k, v := range tpl.Variables {
		variables[k] = v // override
	}

	for _, f := range tpl.Files {
		if err := s.renderFile(tpl.Name, f, variables); err != nil {
			return err
		}
	}
	return nil
}

func (s *renderTask) renderFile(name, path string, variables map[string]string) error {
	data, err := s.readFile(path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	rendered, err := s.renderFileData(name, data, variables)
	if err != nil {
		return err
	}

	return s.writeFile(path, rendered)
}

func (s *renderTask) readFile(path string) (string, error) {
	var (
		f    *os.File
		info os.FileInfo
		err  error
	)

	info, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if info.Size() == 0 {
		return "", nil
	}

	f, err = os.Open(path)
	if err != nil {
		return "", err
	}

	buf := make([]byte, info.Size()+1)
	_, err = f.Read(buf)
	if err != nil {
		return "", err
	}

	if err = f.Close(); err != nil {
		return "", err
	}

	return string(buf), nil
}

func (s *renderTask) writeFile(path string, data string) error {
	tmpPath := filepath.Join("tmp", fmt.Sprintf("%d.tmp", time.Now().UnixMicro()))
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(data))
	if err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (s *renderTask) renderFileData(name, data string, variables map[string]string) (string, error) {
	tpl, err := template.New(name).Option("missingkey=error").Funcs(sprig.TxtFuncMap()).Parse(data)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err = tpl.Execute(&buf, variables); err != nil {
		return "", err
	}
	return buf.String(), nil
}
