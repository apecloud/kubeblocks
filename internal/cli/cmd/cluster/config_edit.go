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
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
)

type editConfigOptions struct {
	configOpsOptions

	// config file replace
	replaceFile bool
}

var (
	editConfigUse = "edit-config NAME [--component=component-name] [--config-spec=config-spec-name] [--config-file=config-file]"

	editConfigExample = templates.Examples(`
		# update mysql max_connections, cluster name is mycluster
		kbcli cluster edit-config mycluster --component=mysql --config-spec=mysql-3node-tpl --config-file=my.cnf 
	`)
)

func (o *editConfigOptions) Run(fn func(info *cfgcore.ConfigPatchInfo, cc *appsv1alpha1.ConfigConstraintSpec) error) error {
	wrapper := o.wrapper
	cfgEditContext := newConfigContext(o.CreateOptions, o.Name, wrapper.ComponentName(), wrapper.ConfigSpecName(), wrapper.ConfigFile())
	if err := cfgEditContext.prepare(); err != nil {
		return err
	}

	editor := editor.NewDefaultEditor([]string{
		"KUBE_EDITOR",
		"EDITOR",
	})
	if err := cfgEditContext.editConfig(editor); err != nil {
		return err
	}

	diff, err := cfgEditContext.getUnifiedDiffString()
	if err != nil {
		return err
	}
	if diff == "" {
		fmt.Println("Edit cancelled, no changes made.")
		return nil
	}

	displayDiffWithColor(o.IOStreams.Out, diff)

	oldVersion := map[string]string{
		o.CfgFile: cfgEditContext.getOriginal(),
	}
	newVersion := map[string]string{
		o.CfgFile: cfgEditContext.getEdited(),
	}

	configSpec := wrapper.ConfigTemplateSpec()
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
		return cfgcore.MakeError("config spec[%s] not support reconfigure!", wrapper.ConfigSpecName())
	}
	configPatch, _, err := cfgcore.CreateConfigPatch(oldVersion, newVersion, formatterConfig.Format, configSpec.Keys, false)
	if err != nil {
		return err
	}
	if !configPatch.IsModify {
		fmt.Println("No parameters changes made.")
		return nil
	}

	fmt.Fprintf(o.Out, "Config patch(updated parameters): \n%s\n\n", string(configPatch.UpdateConfig[o.CfgFile]))

	dynamicUpdated, err := cfgcore.IsUpdateDynamicParameters(&configConstraint.Spec, configPatch)
	if err != nil {
		return nil
	}

	confirmPrompt := confirmApplyReconfigurePrompt
	if !dynamicUpdated || !cfgcm.IsSupportReload(configConstraint.Spec.ReloadOptions) {
		confirmPrompt = restartConfirmPrompt
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
	options := cfgcore.WithKeySelector(wrapper.ConfigTemplateSpec().Keys)
	if err = cfgcore.NewConfigValidator(&configConstraint.Spec, options).Validate(validatedData); err != nil {
		return cfgcore.WrapError(err, "failed to validate edited config")
	}
	return fn(configPatch, &configConstraint.Spec)
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

// NewEditConfigureCmd shows the difference between two configuration version.
func NewEditConfigureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
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
			util.CheckErr(o.Run(func(info *cfgcore.ConfigPatchInfo, cc *appsv1alpha1.ConfigConstraintSpec) error {
				// generate patch for config
				formatterConfig := cc.FormatterConfig
				params := cfgcore.GenerateVisualizedParamsList(info, formatterConfig, nil)
				o.KeyValues = fromKeyValuesToMap(params, o.CfgFile)
				return o.CreateOptions.Run()
			}))
		},
	}
	o.buildReconfigureCommonFlags(cmd)
	cmd.Flags().BoolVar(&o.replaceFile, "replace", false, "Boolean flag to enable replacing config file. Default with false.")
	return cmd
}
