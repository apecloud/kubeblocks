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
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
	"github.com/apecloud/kubeblocks/pkg/cli/util/prompt"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
)

type editConfigOptions struct {
	configOpsOptions

	enableDelete bool
}

var (
	editConfigUse = "edit-config NAME [--component=component-name] [--config-spec=config-spec-name] [--config-file=config-file]"

	editConfigExample = templates.Examples(`
		# update mysql max_connections, cluster name is mycluster
		kbcli cluster edit-config mycluster
	`)
)

func (o *editConfigOptions) Run(fn func() error) error {
	wrapper := o.wrapper
	cfgEditContext := newConfigContext(o.CreateOptions, o.Name, wrapper.ComponentName(), wrapper.ConfigSpecName(), wrapper.ConfigFile())
	if err := cfgEditContext.prepare(); err != nil {
		return err
	}
	reader, err := o.getReaderWrapper()
	if err != nil {
		return err
	}

	editor := editor.NewDefaultEditor([]string{
		"KUBE_EDITOR",
		"EDITOR",
	})
	if err := cfgEditContext.editConfig(editor, reader); err != nil {
		return err
	}

	diff, err := util.GetUnifiedDiffString(cfgEditContext.original, cfgEditContext.edited, "Original", "Current", 3)
	if err != nil {
		return err
	}
	if diff == "" {
		fmt.Println("Edit cancelled, no changes made.")
		return nil
	}
	util.DisplayDiffWithColor(o.IOStreams.Out, diff)

	configSpec := wrapper.ConfigTemplateSpec()
	if configSpec.ConfigConstraintRef != "" {
		return o.runWithConfigConstraints(cfgEditContext, configSpec, fn)
	}

	yes, err := o.confirmReconfigure(fmt.Sprintf(fullRestartConfirmPrompt, printer.BoldRed(o.CfgFile)))
	if err != nil {
		return err
	}
	if !yes {
		return nil
	}

	o.HasPatch = false
	o.FileContent = cfgEditContext.getEdited()
	return fn()
}

func (o *editConfigOptions) runWithConfigConstraints(cfgEditContext *configEditContext, configSpec *appsv1alpha1.ComponentConfigSpec, fn func() error) error {
	oldVersion := map[string]string{
		o.CfgFile: cfgEditContext.getOriginal(),
	}
	newVersion := map[string]string{
		o.CfgFile: cfgEditContext.getEdited(),
	}

	configConstraintKey := client.ObjectKey{
		Namespace: "",
		Name:      configSpec.ConfigConstraintRef,
	}
	configConstraint := appsv1alpha1.ConfigConstraint{}
	if err := util.GetResourceObjectFromGVR(types.ConfigConstraintGVR(), configConstraintKey, o.Dynamic, &configConstraint); err != nil {
		return err
	}
	formatterConfig := configConstraint.Spec.FormatterConfig
	if formatterConfig == nil {
		return core.MakeError("config spec[%s] not support reconfiguring!", configSpec.Name)
	}
	configPatch, fileUpdated, err := core.CreateConfigPatch(oldVersion, newVersion, formatterConfig.Format, configSpec.Keys, true)
	if err != nil {
		return err
	}
	if !fileUpdated && !configPatch.IsModify {
		fmt.Println("No parameters changes made.")
		return nil
	}

	fmt.Fprintf(o.Out, "Config patch(updated parameters): \n%s\n\n", string(configPatch.UpdateConfig[o.CfgFile]))
	if !o.enableDelete {
		if err := core.ValidateConfigPatch(configPatch, configConstraint.Spec.FormatterConfig); err != nil {
			return err
		}
	}

	params := core.GenerateVisualizedParamsList(configPatch, configConstraint.Spec.FormatterConfig, nil)
	// check immutable parameters
	if len(configConstraint.Spec.ImmutableParameters) > 0 {
		if err = util.ValidateParametersModified2(sets.KeySet(fromKeyValuesToMap(params, o.CfgFile)), configConstraint.Spec); err != nil {
			return err
		}
	}

	confirmPrompt, err := generateReconfiguringPrompt(fileUpdated, configPatch, &configConstraint.Spec, o.CfgFile)
	if err != nil {
		return err
	}
	yes, err := o.confirmReconfigure(confirmPrompt)
	if err != nil {
		return err
	}
	if !yes {
		return nil
	}

	validatedData := map[string]string{
		o.CfgFile: cfgEditContext.getEdited(),
	}
	options := validate.WithKeySelector(configSpec.Keys)
	if err = validate.NewConfigValidator(&configConstraint.Spec, options).Validate(validatedData); err != nil {
		return core.WrapError(err, "failed to validate edited config")
	}
	o.KeyValues = fromKeyValuesToMap(params, o.CfgFile)
	return fn()
}

func generateReconfiguringPrompt(fileUpdated bool, configPatch *core.ConfigPatchInfo, cc *appsv1alpha1.ConfigConstraintSpec, fileName string) (string, error) {
	if fileUpdated {
		return restartConfirmPrompt, nil
	}

	dynamicUpdated, err := core.IsUpdateDynamicParameters(cc, configPatch)
	if err != nil {
		return "", nil
	}

	confirmPrompt := confirmApplyReconfigurePrompt
	if !dynamicUpdated || !cfgcm.IsSupportReload(cc.ReloadOptions) {
		confirmPrompt = restartConfirmPrompt
	}
	return confirmPrompt, nil
}

func (o *editConfigOptions) confirmReconfigure(promptStr string) (bool, error) {
	const yesStr = "yes"
	const noStr = "no"

	confirmStr := []string{yesStr, noStr}
	printer.Warning(o.Out, promptStr)
	input, err := prompt.NewPrompt("Please type [Yes/No] to confirm:",
		func(input string) error {
			if !slices.Contains(confirmStr, strings.ToLower(input)) {
				return fmt.Errorf("typed \"%s\" does not match \"%s\"", input, confirmStr)
			}
			return nil
		}, o.In).Run()
	if err != nil {
		return false, err
	}
	return strings.ToLower(input) == yesStr, nil
}

func (o *editConfigOptions) getReaderWrapper() (io.Reader, error) {
	var reader io.Reader
	if o.replaceFile && o.LocalFilePath != "" {
		b, err := os.ReadFile(o.LocalFilePath)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	return reader, nil
}

// NewEditConfigureCmd shows the difference between two configuration version.
func NewEditConfigureCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &editConfigOptions{
		configOpsOptions: configOpsOptions{
			editMode:          true,
			OperationsOptions: newBaseOperationsOptions(f, streams, appsv1alpha1.ReconfiguringType, false),
		}}

	cmd := &cobra.Command{
		Use:               editConfigUse,
		Short:             "Edit the config file of the component.",
		Example:           editConfigExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			util.CheckErr(o.Complete())
			util.CheckErr(o.Validate())
			util.CheckErr(o.Run(o.CreateOptions.Run))
		},
	}
	o.buildReconfigureCommonFlags(cmd, f)
	cmd.Flags().BoolVar(&o.enableDelete, "enable-delete", false, "Boolean flag to enable delete configuration. Default with false.")
	return cmd
}
