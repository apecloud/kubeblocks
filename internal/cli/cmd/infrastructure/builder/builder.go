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

package builder

import (
	"bufio"
	"embed"
	"encoding/json"
	"strings"

	"github.com/leaanthony/debme"
	"k8s.io/apimachinery/pkg/util/yaml"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

var (
	//go:embed template/*
	cueTemplate embed.FS
)

func newBuildTemplate(templateName string) (string, error) {
	tmplFs, _ := debme.FS(cueTemplate, "template")
	if tmlBytes, err := tmplFs.ReadFile(templateName); err != nil {
		return "", err
	} else {
		return string(tmlBytes), nil
	}
}

func BuildFromTemplate(values *gotemplate.TplValues, templateName string) (string, error) {
	tpl, err := newBuildTemplate(templateName)
	if err != nil {
		return "", err
	}

	engine := gotemplate.NewTplEngine(values, nil, templateName, nil, nil)
	rendered, err := engine.Render(tpl)
	if err != nil {
		return "", err
	}
	return rendered, nil
}

func BuildResourceFromYaml[T any](obj T, bYaml string) (*T, error) {
	var ret map[string]interface{}

	content, err := yaml.NewYAMLReader(bufio.NewReader(strings.NewReader(bYaml))).Read()
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to read the cluster yaml")
	}
	err = yaml.Unmarshal(content, &ret)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to unmarshal the cluster yaml")
	}

	contentToJSON, err := yaml.ToJSON(content)
	if err != nil {
		return nil, cfgcore.WrapError(err, "Unable to convert configuration to json")
	}
	if err := json.Unmarshal(contentToJSON, &obj); err != nil {
		return nil, cfgcore.WrapError(err, "failed to unmarshal the cluster")
	}
	return &obj, nil
}
