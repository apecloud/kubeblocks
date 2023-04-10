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
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

type configOpsOptions struct {
	*OperationsOptions

	editMode bool
	wrapper  *configWrapper

	// Reconfiguring options
	ComponentName string
	URLPath       string   `json:"urlPath"`
	Parameters    []string `json:"parameters"`
}

var (
	createReconfigureExample = templates.Examples(`
		# update component params 
		kbcli cluster configure <cluster-name> --component=<component-name> --config-spec=<config-spec-name> --config-file=<config-file> --set max_connections=1000,general_log=OFF

		# update mysql max_connections, cluster name is mycluster
		kbcli cluster configure mycluster --component=mysql --config-spec=mysql-3node-tpl --config-file=my.cnf --set max_connections=2000
	`)
)

func (o *configOpsOptions) Complete() error {
	if o.Name == "" {
		return makeMissingClusterNameErr()
	}

	if !o.editMode {
		kvs, err := o.parseUpdatedParams()
		if err != nil {
			return err
		}
		o.KeyValues = kvs
	}

	wrapper, err := newConfigWrapper(o.BaseOptions, o.Name, o.ComponentName, o.CfgTemplateName, o.CfgFile, o.KeyValues)
	if err != nil {
		return err
	}

	o.wrapper = wrapper
	return wrapper.AutoFillRequiredParam()
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
	if err := o.validateConfigParams(o.wrapper.ConfigSpec()); err != nil {
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

	newConfigData, err := cfgcore.MergeAndValidateConfigs(configConstraint.Spec, map[string]string{o.CfgFile: ""}, tpl.Keys, []cfgcore.ParamPairs{{
		Key:           o.CfgFile,
		UpdatedParams: cfgcore.FromStringMap(o.KeyValues),
	}})
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

	configPatch, _, err := cfgcore.CreateConfigPatch(mockEmptyData(data), data, cc.FormatterConfig.Format, tpl.Keys, false)
	if err != nil {
		return err
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
	const confirmStr = "yes"
	printer.Warning(o.Out, restartConfirmPrompt)
	_, err := prompt.NewPrompt(fmt.Sprintf("Please type \"%s\" to confirm:", confirmStr),
		func(input string) error {
			if input != confirmStr {
				return fmt.Errorf("typed \"%s\" does not match \"%s\"", input, confirmStr)
			}
			return nil
		}, o.In).Run()
	return err
}

func (o *configOpsOptions) parseUpdatedParams() (map[string]string, error) {
	if len(o.Parameters) == 0 && len(o.URLPath) == 0 {
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

// buildCommonFlags build common flags for operations command
func (o *configOpsOptions) buildReconfigureCommonFlags(cmd *cobra.Command) {
	o.buildCommonFlags(cmd)
	cmd.Flags().StringSliceVar(&o.Parameters, "set", nil, "Specify updated parameter list. For details about the parameters, refer to kbcli sub command: 'kbcli cluster describe-config'.")
	cmd.Flags().StringVar(&o.ComponentName, "component", "", "Specify the name of Component to be updated. If the cluster has only one component, unset the parameter.")
	cmd.Flags().StringVar(&o.CfgTemplateName, "config-spec", "", "Specify the name of the configuration template to be updated (e.g. for apecloud-mysql: --config-spec=mysql-3node-tpl). What templates or configure files are available for this cluster can refer to kbcli sub command: 'kbcli cluster describe-config'.")
	cmd.Flags().StringVar(&o.CfgFile, "config-file", "", "Specify the name of the configuration file to be updated (e.g. for mysql: --config-file=my.cnf). What templates or configure files are available for this cluster can refer to kbcli sub command: 'kbcli cluster describe-config'.")
}

// NewReconfigureCmd creates a Reconfiguring command
func NewReconfigureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &configOpsOptions{
		editMode:          false,
		OperationsOptions: newBaseOperationsOptions(streams, appsv1alpha1.ReconfiguringType, false),
	}
	inputs := buildOperationsInputs(f, o.OperationsOptions)
	inputs.Use = "configure"
	inputs.Short = "Reconfigure parameters with the specified components in the cluster."
	inputs.Example = createReconfigureExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildReconfigureCommonFlags(cmd)
	}

	inputs.Complete = o.Complete
	inputs.Validate = o.Validate
	return create.BuildCommand(inputs)
}
