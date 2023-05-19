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
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
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

type configOpsOptions struct {
	*OperationsOptions

	editMode bool
	wrapper  *configWrapper

	// Reconfiguring options
	ComponentName string
	LocalFilePath string   `json:"localFilePath"`
	Parameters    []string `json:"parameters"`
}

var (
	createReconfigureExample = templates.Examples(`
		# update component params 
		kbcli cluster configure mycluster --component=mysql --config-spec=mysql-3node-tpl --config-file=my.cnf --set max_connections=1000,general_log=OFF

		# if only one component, and one config spec, and one config file, simplify the searching process of configure. e.g:
		# update mysql max_connections, cluster name is mycluster
		kbcli cluster configure mycluster --set max_connections=2000
	`)
)

func (o *configOpsOptions) Complete() error {
	if o.Name == "" {
		return makeMissingClusterNameErr()
	}

	if !o.editMode {
		if err := o.validateReconfigureOptions(); err != nil {
			return err
		}
	}

	wrapper, err := newConfigWrapper(o.CreateOptions, o.Name, o.ComponentName, o.CfgTemplateName, o.CfgFile, o.KeyValues)
	if err != nil {
		return err
	}

	o.wrapper = wrapper
	return wrapper.AutoFillRequiredParam()
}

func (o *configOpsOptions) validateReconfigureOptions() error {
	if o.LocalFilePath != "" && o.CfgFile == "" {
		return cfgcore.MakeError("config file is required when using --local-file")
	}
	if o.LocalFilePath != "" {
		b, err := os.ReadFile(o.LocalFilePath)
		if err != nil {
			return err
		}
		o.FileContent = string(b)
	} else {
		kvs, err := o.parseUpdatedParams()
		if err != nil {
			return err
		}
		o.KeyValues = kvs
	}
	return nil
}

// Validate command flags or args is legal
func (o *configOpsOptions) Validate() error {
	if err := o.wrapper.ValidateRequiredParam(); err != nil {
		return err
	}

	o.CfgFile = o.wrapper.ConfigFile()
	o.CfgTemplateName = o.wrapper.ConfigSpecName()
	o.ComponentNames = []string{o.wrapper.ComponentName()}

	if o.editMode {
		return nil
	}
	if err := o.validateConfigParams(o.wrapper.ConfigTemplateSpec()); err != nil {
		return err
	}
	o.printConfigureTips()
	return nil
}

func (o *configOpsOptions) validateConfigParams(tpl *appsv1alpha1.ComponentConfigSpec) error {
	configConstraintKey := client.ObjectKey{
		Namespace: "",
		Name:      tpl.ConfigConstraintRef,
	}
	configConstraint := appsv1alpha1.ConfigConstraint{}
	if err := util.GetResourceObjectFromGVR(types.ConfigConstraintGVR(), configConstraintKey, o.Dynamic, &configConstraint); err != nil {
		return err
	}

	var err error
	var newConfigData map[string]string
	if o.FileContent != "" {
		newConfigData = map[string]string{o.CfgFile: o.FileContent}
	} else {
		newConfigData, err = cfgcore.MergeAndValidateConfigs(configConstraint.Spec, map[string]string{o.CfgFile: ""}, tpl.Keys, []cfgcore.ParamPairs{{
			Key:           o.CfgFile,
			UpdatedParams: cfgcore.FromStringMap(o.KeyValues),
		}})
	}
	if err != nil {
		return err
	}
	return o.checkChangedParamsAndDoubleConfirm(&configConstraint.Spec, newConfigData, tpl)
}

func (o *configOpsOptions) checkChangedParamsAndDoubleConfirm(cc *appsv1alpha1.ConfigConstraintSpec, data map[string]string, tpl *appsv1alpha1.ComponentConfigSpec) error {
	mockEmptyData := func(m map[string]string) map[string]string {
		r := make(map[string]string, len(data))
		for key := range m {
			r[key] = ""
		}
		return r
	}

	if !cfgcm.IsSupportReload(cc.ReloadOptions) {
		return o.confirmReconfigureWithRestart()
	}

	configPatch, restart, err := cfgcore.CreateConfigPatch(mockEmptyData(data), data, cc.FormatterConfig.Format, tpl.Keys, o.FileContent != "")
	if err != nil {
		return err
	}
	if restart {
		return o.confirmReconfigureWithRestart()
	}

	dynamicUpdated, err := cfgcore.IsUpdateDynamicParameters(cc, configPatch)
	if err != nil {
		return nil
	}
	if dynamicUpdated {
		return nil
	}
	return o.confirmReconfigureWithRestart()
}

func (o *configOpsOptions) confirmReconfigureWithRestart() error {
	if o.autoApprove {
		return nil
	}
	const confirmStr = "yes"
	printer.Warning(o.Out, restartConfirmPrompt)
	_, err := prompt.NewPrompt(fmt.Sprintf("Please type \"%s\" to confirm:", confirmStr),
		func(input string) error {
			if input != confirmStr {
				return fmt.Errorf("typed \"%s\" not match \"%s\"", input, confirmStr)
			}
			return nil
		}, o.In).Run()
	return err
}

func (o *configOpsOptions) parseUpdatedParams() (map[string]string, error) {
	if len(o.Parameters) == 0 && len(o.LocalFilePath) == 0 {
		return nil, cfgcore.MakeError(missingUpdatedParametersErrMessage)
	}

	keyValues := make(map[string]string)
	for _, param := range o.Parameters {
		pp := strings.Split(param, ",")
		for _, p := range pp {
			fields := strings.SplitN(p, "=", 2)
			if len(fields) != 2 {
				return nil, cfgcore.MakeError("updated parameter format: key=value")
			}
			keyValues[fields[0]] = fields[1]
		}
	}
	return keyValues, nil
}

func (o *configOpsOptions) printConfigureTips() {
	fmt.Println("Will updated configure file meta:")
	printer.PrintLineWithTabSeparator(
		printer.NewPair("  ConfigSpec", printer.BoldYellow(o.CfgTemplateName)),
		printer.NewPair("  ConfigFile", printer.BoldYellow(o.CfgFile)),
		printer.NewPair("ComponentName", o.ComponentName),
		printer.NewPair("ClusterName", o.Name))
}

// buildReconfigureCommonFlags build common flags for reconfigure command
func (o *configOpsOptions) buildReconfigureCommonFlags(cmd *cobra.Command) {
	o.addCommonFlags(cmd)
	cmd.Flags().StringSliceVar(&o.Parameters, "set", nil, "Specify parameters list to be updated. For more details, refer to 'kbcli cluster describe-config'.")
	cmd.Flags().StringVar(&o.ComponentName, "component", "", "Specify the name of Component to be updated. If the cluster has only one component, unset the parameter.")
	cmd.Flags().StringVar(&o.CfgTemplateName, "config-spec", "", "Specify the name of the configuration template to be updated (e.g. for apecloud-mysql: --config-spec=mysql-3node-tpl). "+
		"For available templates and configs, refer to: 'kbcli cluster describe-config'.")
	cmd.Flags().StringVar(&o.CfgFile, "config-file", "", "Specify the name of the configuration file to be updated (e.g. for mysql: --config-file=my.cnf). "+
		"For available templates and configs, refer to: 'kbcli cluster describe-config'.")
	cmd.Flags().StringVar(&o.LocalFilePath, "local-file", "", "Specify the local configuration file to be updated.")
}

// NewReconfigureCmd creates a Reconfiguring command
func NewReconfigureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &configOpsOptions{
		editMode:          false,
		OperationsOptions: newBaseOperationsOptions(f, streams, appsv1alpha1.ReconfiguringType, false),
	}
	cmd := &cobra.Command{
		Use:               "configure NAME --set key=value[,key=value] [--component=component-name] [--config-spec=config-spec-name] [--config-file=config-file]",
		Short:             "Configure parameters with the specified components in the cluster.",
		Example:           createReconfigureExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.buildReconfigureCommonFlags(cmd)
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before reconfiguring the cluster")
	return cmd
}
