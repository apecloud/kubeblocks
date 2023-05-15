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

package fault

import (
	"fmt"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var faultNetWorkExample = templates.Examples(`
	# Isolate all pods network under the default namespace from the outside world, including the k8s internal network.
	kbcli fault network partition

	# The specified pod is isolated from the k8s external network "kubeblocks.io".
	kbcli fault network partition --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-1 --external-targets=kubeblocks.io
	
	# Isolate the network between two pods.
	kbcli fault network partition --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-1 --target-label=statefulset.kubernetes.io/pod-name=mycluster-mysql-2
	
	// Like the partition command, the target can be specified through --target-label or --external-targets. The pod only has obstacles in communicating with this target. If the target is not specified, all communication will be blocked.
	# Block all pod communication under the default namespace, resulting in a 50% packet loss rate.
	kbcli fault network loss --loss=50
	
	# Block the specified pod communication, so that the packet loss rate is 50%.
	kbcli fault network loss --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --loss=50
	
	kbcli fault network corrupt --corrupt=50

	# Blocks specified pod communication with a 50% packet corruption rate.
	kbcli fault network corrupt --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --corrupt=50
	
	kbcli fault network duplicate --duplicate=50

	# Block specified pod communication so that the packet repetition rate is 50%.
	kbcli fault network duplicate --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --duplicate=50
	
	kbcli fault network delay --latency=10s

	# Block the communication of the specified pod, causing its network delay for 10s.
	kbcli fault network delay --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --latency=10s
`)

type NetworkChaosOptions struct {
	// Specify the network direction
	Direction string `json:"direction"`
	// Indicates a network target outside of Kubernetes, which can be an IPv4 address or a domain name,
	// such as "www.baidu.com". Only works with direction: to.
	ExternalTargets []string `json:"externalTargets,omitempty"`

	TargetMode  string `json:"targetMode,omitempty"`
	TargetValue string `json:"targetValue"`
	// Specifies the labels that target Pods come with.
	TargetLabelSelectors map[string]string `json:"targetLabelSelectors,omitempty"`
	// Specifies the namespaces to which target Pods belong.
	TargetNamespaceSelectors []string `json:"targetNamespaceSelectors"`

	// The percentage of packet loss
	Loss string `json:"loss,omitempty"`
	// The percentage of packet corruption
	Corrupt string `json:"corrupt,omitempty"`
	// The percentage of packet duplication
	Duplicate string `json:"duplicate,omitempty"`
	// The latency of delay
	Latency string `json:"latency,omitempty"`
	// The jitter of delay
	Jitter string `json:"jitter"`

	// The correlation of loss or corruption or duplication or delay
	Correlation string `json:"correlation"`

	// Bandwidth command
	Rate     string `json:"rate,omitempty"`
	Limit    uint32 `json:"limit"`
	Buffer   uint32 `json:"buffer"`
	Peakrate uint64 `json:"peakrate"`
	Minburst uint32 `json:"minburst"`

	FaultBaseOptions
}

func NewNetworkChaosOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, action string) *NetworkChaosOptions {
	o := &NetworkChaosOptions{
		FaultBaseOptions: FaultBaseOptions{CreateOptions: create.CreateOptions{
			Factory:         f,
			IOStreams:       streams,
			CueTemplateName: CueTemplateNetworkChaos,
			GVR:             GetGVR(Group, Version, ResourceNetworkChaos),
		},
			Action: action,
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewNetworkChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network chaos.",
	}
	cmd.AddCommand(
		NewPartitionCmd(f, streams),
		NewLossCmd(f, streams),
		NewDelayCmd(f, streams),
		NewDuplicateCmd(f, streams),
		NewCorruptCmd(f, streams),
		NewBandwidthCmd(f, streams),
		NewDNSChaosCmd(f, streams),
		NewHTTPChaosCmd(f, streams),
	)
	return cmd
}

func NewPartitionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNetworkChaosOptions(f, streams, string(v1alpha1.PartitionAction))

	cmd := o.NewCobraCommand(Partition, PartitionShort)

	o.AddCommonFlag(cmd)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewLossCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNetworkChaosOptions(f, streams, string(v1alpha1.LossAction))

	cmd := o.NewCobraCommand(Loss, LossShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringVar(&o.Loss, "loss", "", `Indicates the probability of a packet error occurring. Value range: [0, 100].`)
	cmd.Flags().StringVarP(&o.Correlation, "correlation", "c", "0", `Indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100].`)

	util.CheckErr(cmd.MarkFlagRequired("loss"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewDelayCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNetworkChaosOptions(f, streams, string(v1alpha1.DelayAction))

	cmd := o.NewCobraCommand(Delay, DelayShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringVar(&o.Latency, "latency", "", `the length of time to delay.`)
	cmd.Flags().StringVar(&o.Jitter, "jitter", "0ms", `the variation range of the delay time.`)
	cmd.Flags().StringVarP(&o.Correlation, "correlation", "c", "0", `Indicates the probability of a packet error occurring. Value range: [0, 100].`)

	util.CheckErr(cmd.MarkFlagRequired("latency"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewDuplicateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNetworkChaosOptions(f, streams, string(v1alpha1.DuplicateAction))

	cmd := o.NewCobraCommand(Duplicate, DuplicateShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringVar(&o.Duplicate, "duplicate", "", `the probability of a packet being repeated. Value range: [0, 100].`)
	cmd.Flags().StringVarP(&o.Correlation, "correlation", "c", "0", `Indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100].`)

	util.CheckErr(cmd.MarkFlagRequired("duplicate"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewCorruptCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNetworkChaosOptions(f, streams, string(v1alpha1.CorruptAction))

	cmd := o.NewCobraCommand(Corrupt, CorruptShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringVar(&o.Corrupt, "corrupt", "", `Indicates the probability of a packet error occurring. Value range: [0, 100].`)
	cmd.Flags().StringVarP(&o.Correlation, "correlation", "c", "0", `Indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100].`)

	util.CheckErr(cmd.MarkFlagRequired("corrupt"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewBandwidthCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNetworkChaosOptions(f, streams, string(v1alpha1.BandwidthAction))

	cmd := o.NewCobraCommand(Bandwidth, BandwidthShort)

	o.AddCommonFlag(cmd)

	cmd.Flags().StringVar(&o.Rate, "rate", "", `the rate at which the bandwidth is limited. For example : 10 bps/kbps/mbps/gbps.`)
	cmd.Flags().Uint32Var(&o.Limit, "limit", 1, `the number of bytes waiting in the queue.`)
	cmd.Flags().Uint32Var(&o.Buffer, "buffer", 1, `the maximum number of bytes that can be sent instantaneously.`)
	cmd.Flags().Uint64Var(&o.Peakrate, "peakrate", 0, `the maximum consumption rate of the bucket.`)
	cmd.Flags().Uint32Var(&o.Minburst, "minburst", 0, `the size of the peakrate bucket.`)

	util.CheckErr(cmd.MarkFlagRequired("rate"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func (o *NetworkChaosOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultNetWorkExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Run())
		},
	}
}

func (o *NetworkChaosOptions) AddCommonFlag(cmd *cobra.Command) {
	o.FaultBaseOptions.AddCommonFlag(cmd)

	cmd.Flags().StringVar(&o.Direction, "direction", "to", `You can select "to"" or "from"" or "both"".`)
	cmd.Flags().StringArrayVarP(&o.ExternalTargets, "external-target", "e", nil, "a network target outside of Kubernetes, which can be an IPv4 address or a domain name,\n\t such as \"www.baidu.com\". Only works with direction: to.")
	cmd.Flags().StringVar(&o.TargetMode, "target-mode", "", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.TargetValue, "target-value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringToStringVar(&o.TargetLabelSelectors, "target-label", nil, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringArrayVar(&o.TargetNamespaceSelectors, "target-ns-fault", []string{"default"}, `Specifies the namespace into which you want to inject faults.`)
}

func (o *NetworkChaosOptions) Validate() error {
	if o.TargetValue == "" && (o.TargetMode == "fixed" || o.TargetMode == "fixed-percent" || o.TargetMode == "random-max-percent") {
		return fmt.Errorf("you must use --value to specify an integer")
	}

	if ok, err := IsInteger(o.TargetValue); !ok {
		return err
	}

	if ok, err := IsInteger(o.Loss); !ok {
		return err
	}

	if ok, err := IsInteger(o.Corrupt); !ok {
		return err
	}

	if ok, err := IsInteger(o.Duplicate); !ok {
		return err
	}

	if ok, err := IsInteger(o.Correlation); !ok {
		return err
	}

	if ok, err := IsRegularMatch(o.Latency); !ok {
		return err
	}

	if ok, err := IsRegularMatch(o.Jitter); !ok {
		return err
	}

	return o.BaseValidate()
}

func (o *NetworkChaosOptions) Complete() error {
	return o.BaseComplete()
}

func (o *NetworkChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.NetworkChaos{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, c); err != nil {
		return err
	}

	data, e := runtime.DefaultUnstructuredConverter.ToUnstructured(c)
	if e != nil {
		return e
	}
	obj.SetUnstructuredContent(data)
	return nil
}
