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

package cluster

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/pmezard/go-difflib/difflib"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
)

type configEditContext struct {
	create.CreateOptions

	clusterName    string
	componentName  string
	configSpecName string
	configKey      string

	original string
	edited   string
}

func (c *configEditContext) getOriginal() string {
	return c.original
}

func (c *configEditContext) getEdited() string {
	return c.edited
}

func (c *configEditContext) prepare() error {
	cmObj := corev1.ConfigMap{}
	cmKey := client.ObjectKey{
		Name:      cfgcore.GetComponentCfgName(c.clusterName, c.componentName, c.configSpecName),
		Namespace: c.Namespace,
	}
	if err := util.GetResourceObjectFromGVR(types.ConfigmapGVR(), cmKey, c.Dynamic, &cmObj); err != nil {
		return err
	}

	val, ok := cmObj.Data[c.configKey]
	if !ok {
		return makeNotFoundConfigFileErr(c.configKey, c.configSpecName, cfgutil.ToSet(cmObj.Data).AsSlice())
	}

	c.original = val
	return nil
}

func (c *configEditContext) editConfig(editor editor.Editor) error {
	edited, _, err := editor.LaunchTempFile(fmt.Sprintf("%s-edit-", filepath.Base(c.configKey)), "", bytes.NewBufferString(c.original))
	if err != nil {
		return err
	}

	c.edited = string(edited)
	return nil
}

func (c *configEditContext) getUnifiedDiffString() (string, error) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(c.original),
		B:        difflib.SplitLines(c.edited),
		FromFile: "Original",
		ToFile:   "Current",
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(diff)
}

func newConfigContext(baseOptions create.CreateOptions, clusterName, componentName, configSpec, file string) *configEditContext {
	return &configEditContext{
		CreateOptions:  baseOptions,
		clusterName:    clusterName,
		componentName:  componentName,
		configSpecName: configSpec,
		configKey:      file,
	}
}

func displayDiffWithColor(out io.Writer, diffText string) {
	for _, line := range difflib.SplitLines(diffText) {
		switch {
		case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
			line = color.HiYellowString(line)
		case strings.HasPrefix(line, "@@"):
			line = color.HiBlueString(line)
		case strings.HasPrefix(line, "-"):
			line = color.RedString(line)
		case strings.HasPrefix(line, "+"):
			line = color.GreenString(line)
		}
		fmt.Fprint(out, line)
	}
}

func fromKeyValuesToMap(params []cfgcore.VisualizedParam, file string) map[string]string {
	result := make(map[string]string)
	for _, param := range params {
		if param.Key != file {
			continue
		}
		for _, kv := range param.Parameters {
			result[kv.Key] = kv.Value
		}
	}
	return result
}
