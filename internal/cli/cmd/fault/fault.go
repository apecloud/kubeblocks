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

	NamespaceSelector string `json:"namespaceSelector"`

	Label map[string]string `json:"label,omitempty"`
	// GracePeriod waiting time, after which fault injection is performed
	GracePeriod int `json:"gracePeriod"`
	// Duration the duration of the Pod Failure experiment
	Duration string `json:"duration"`
}

func NewFaultCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fault",
		Short: "inject fault.",
	}
	cmd.AddCommand(
		NewFaultPodCmd(f, streams),
		NewNetworkAttackCmd(f, streams),
	)
	return cmd
}

func (o *FaultBaseOptions) BaseValidate() error {
	if o.Label == nil {
		return fmt.Errorf("a valid label is needed, use --label to specify one, run \"kubectl get pod --show-labels\" to show all labels ")
	}

	pattern := regexp.MustCompile(`^\d+(ms|s|m|h)$`)
	if o.Duration != "" && !pattern.MatchString(o.Duration) {
		return fmt.Errorf("invalid duration:%s; input format must be in the form of number + time unit, like 10s, 10m", o.Duration)
	}

	if _, err := strconv.Atoi(o.Value); err != nil {
		return fmt.Errorf("invalid value:%s; must be an integer", o.Value)
	}
	return nil
}

// BaseComplete TODO
func (o *FaultBaseOptions) BaseComplete() error {
	return nil
}
