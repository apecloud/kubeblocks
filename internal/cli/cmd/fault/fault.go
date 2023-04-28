package fault

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type FaultBaseOptions struct {
	Action string `json:"action"`

	Mode string `json:"mode"`
	// Value The number and percentage of fault injection pods
	Value string `json:"value"`

	NamespaceSelector []string `json:"namespaceSelector"`

	Label map[string]string `json:"label"`

	// Duration the duration of the Pod Failure experiment
	Duration string `json:"duration"`
}

func NewFaultCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fault",
		Short: "inject fault.",
	}
	cmd.AddCommand(
		NewPodChaosCmd(f, streams),
		NewNetworkChaosCmd(f, streams),
		NewIOChaosCmd(f, streams),
	)
	return cmd
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

// BaseComplete TODO
func (o *FaultBaseOptions) BaseComplete() error {
	return nil
}
