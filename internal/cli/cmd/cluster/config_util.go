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

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"

	"github.com/apecloud/kubeblocks/internal/cli/create"
)

type configEditContext struct {
	create.BaseOptions

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
		return makeNotFoundConfigFileErr(c.configKey, c.configSpecName, cfgcore.ToSet(cmObj.Data).AsSlice())
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

func newConfigContext(baseOptions create.BaseOptions, clusterName, componentName, configSpec, file string) *configEditContext {
	return &configEditContext{
		BaseOptions:    baseOptions,
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
