package fault

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/apecloud/kubeblocks/internal/cli/util"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type FaultBaseOptions struct {
	Action string `json:"action"`

	Mode string `json:"mode"`

	Value string `json:"value"`

	NamespaceSelector []string `json:"namespaceSelector"`

	Label map[string]string `json:"label"`

	Duration string `json:"duration"`
}

func NewFaultCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fault",
		Short: "inject faults to pod.",
	}
	cmd.AddCommand(
		NewPodChaosCmd(f, streams),
		NewNetworkChaosCmd(f, streams),
		NewIOChaosCmd(f, streams),
		NewStressChaosCmd(f, streams),
		NewDNSChaosCmd(f, streams),
	)
	return cmd
}

// TODO : Add the completion function of other flags.
func registerFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	var formatsWithDesc = map[string]string{
		"JSON": "Output result in JSON format",
		"YAML": "Output result in YAML format",
	}
	util.CheckErr(cmd.RegisterFlagCompletionFunc("output",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var names []string
			for format, desc := range formatsWithDesc {
				if strings.HasPrefix(format, toComplete) {
					names = append(names, fmt.Sprintf("%s\t%s", format, desc))
				}
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		}))
}

func (o *FaultBaseOptions) BaseValidate() error {
	pattern := regexp.MustCompile(`^\d+(ms|s|m|h)$`)
	if o.Duration != "" && !pattern.MatchString(o.Duration) {
		return fmt.Errorf("invalid duration:%s; input format must be in the form of number + time unit, like 10s, 10m", o.Duration)
	}

	if o.Value == "" && (o.Mode == "fixed" || o.Mode == "fixed-percent" || o.Mode == "random-max-percent") {
		return fmt.Errorf("you must use --value to specify an integer")
	}

	if _, err := strconv.Atoi(o.Value); o.Value != "" && err != nil {
		return fmt.Errorf("invalid value:%s; must be an integer", o.Value)
	}
	return nil
}

// BaseComplete TODO : Add the completion function of other flags.
func (o *FaultBaseOptions) BaseComplete() error {
	return nil
}
