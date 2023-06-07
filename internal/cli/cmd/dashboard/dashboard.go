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

package dashboard

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	cmdpf "k8s.io/kubectl/pkg/cmd/portforward"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/pointer"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	podRunningTimeoutFlag = "pod-running-timeout"
	defaultPodExecTimeout = 60 * time.Second
)

type dashboard struct {
	Name         string
	AddonName    string
	Port         string
	TargetPort   string
	Namespace    string
	CreationTime string

	// Label used to get the service
	Label string
}

var (
	listExample = templates.Examples(`
		# List all dashboards
		kbcli dashboard list
	`)

	openExample = templates.Examples(`
		# Open a dashboard, such as kube-grafana
		kbcli dashboard open kubeblocks-grafana

		# Open a dashboard with a specific local port
		kbcli dashboard open kubeblocks-grafana --port 8080
	`)

	// we do not use the default port to port-forward to avoid conflict with other services
	dashboards = [...]*dashboard{
		{
			Name:       "kubeblocks-grafana",
			AddonName:  "kb-addon-grafana",
			Label:      "app.kubernetes.io/instance=kb-addon-grafana,app.kubernetes.io/name=grafana",
			TargetPort: "13000",
		},
		{
			Name:       "kubeblocks-prometheus-alertmanager",
			AddonName:  "kb-addon-prometheus-alertmanager",
			Label:      "app=prometheus,component=alertmanager,release=kb-addon-prometheus",
			TargetPort: "19093",
		},
		{
			Name:       "kubeblocks-prometheus-server",
			AddonName:  "kb-addon-prometheus-server",
			Label:      "app=prometheus,component=server,release=kb-addon-prometheus",
			TargetPort: "19090",
		},
		{
			Name:       "kubeblocks-nyancat",
			AddonName:  "kb-addon-nyancat",
			Label:      "app.kubernetes.io/instance=kb-addon-nyancat",
			TargetPort: "8087",
		},
	}
)

type listOptions struct {
	genericclioptions.IOStreams
	factory cmdutil.Factory
	client  *kubernetes.Clientset
}

func newListOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *listOptions {
	return &listOptions{
		factory:   f,
		IOStreams: streams,
	}
}

// NewDashboardCmd creates the dashboard command
func NewDashboardCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "List and open the KubeBlocks dashboards.",
	}

	// add subcommands
	cmd.AddCommand(
		newListCmd(f, streams),
		newOpenCmd(f, streams),
	)

	return cmd
}

func newListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newListOptions(f, streams)
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all dashboards.",
		Example: listExample,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(o.run())
		},
	}
	return cmd
}

func (o *listOptions) complete() error {
	var err error
	o.client, err = o.factory.KubernetesClientSet()
	return err
}

// get all dashboard service and print
func (o *listOptions) run() error {
	if err := getDashboardInfo(o.client); err != nil {
		return err
	}

	return printTable(o.Out)
}

func printTable(out io.Writer) error {
	tbl := printer.NewTablePrinter(out)
	tbl.SetHeader("NAME", "NAMESPACE", "PORT", "CREATED-TIME")
	for _, d := range dashboards {
		if d.Namespace == "" {
			continue
		}
		tbl.AddRow(d.Name, d.Namespace, d.TargetPort, d.CreationTime)
	}
	tbl.Print()
	return nil
}

type openOptions struct {
	factory cmdutil.Factory
	genericclioptions.IOStreams
	portForwardOptions *cmdpf.PortForwardOptions

	name      string
	localPort string
}

func newOpenOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *openOptions {
	return &openOptions{
		factory:   f,
		IOStreams: streams,
		portForwardOptions: &cmdpf.PortForwardOptions{
			PortForwarder: &defaultPortForwarder{streams},
		},
	}
}

func newOpenCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newOpenOptions(f, streams)
	cmd := &cobra.Command{
		Use:     "open",
		Short:   "Open one dashboard.",
		Example: openExample,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var names []string
			for _, d := range dashboards {
				names = append(names, d.Name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
			util.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVar(&o.localPort, "port", "", "dashboard local port")
	cmd.Flags().Duration(podRunningTimeoutFlag, defaultPodExecTimeout,
		"The time (like 5s, 2m, or 3h, higher than zero) to wait for at least one pod is running")
	addCharacterFlag(cmd, &characterType)
	return cmd
}

func (o *openOptions) complete(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing dashborad name")
	}

	o.name = args[0]
	client, err := o.factory.KubernetesClientSet()
	if err != nil {
		return err
	}

	if err = getDashboardInfo(client); err != nil {
		return err
	}

	dash := getDashboardByName(o.name)
	if dash == nil {
		return fmt.Errorf("failed to find dashboard \"%s\", run \"kbcli dashboard list\" to list all dashboards", o.name)
	}

	if o.localPort == "" {
		o.localPort = dash.TargetPort
	}

	pfArgs := []string{fmt.Sprintf("svc/%s", dash.AddonName), fmt.Sprintf("%s:%s", o.localPort, dash.Port)}
	o.portForwardOptions.Namespace = dash.Namespace
	o.portForwardOptions.Address = []string{"127.0.0.1"}
	return o.portForwardOptions.Complete(newFactory(dash.Namespace), cmd, pfArgs)
}

func (o *openOptions) run() error {
	url := "http://127.0.0.1:" + o.localPort
	if o.name == "kubeblocks-grafana" {
		err := buildGrafanaDirectURL(&url, characterType)
		if err != nil {
			return err
		}
	}
	go func() {
		<-o.portForwardOptions.ReadyChannel
		fmt.Fprintf(o.Out, "Forward successfully! Opening browser ...\n")
		if err := util.OpenBrowser(url); err != nil {
			fmt.Fprintf(o.ErrOut, "Failed to open browser: %v", err)
		}
	}()

	return o.portForwardOptions.RunPortForward()
}

func getDashboardByName(name string) *dashboard {
	for i, d := range dashboards {
		if d.Name == name {
			return dashboards[i]
		}
	}
	return nil
}

func getDashboardInfo(client *kubernetes.Clientset) error {
	getSvcs := func(client *kubernetes.Clientset, label string) (*corev1.ServiceList, error) {
		return client.CoreV1().Services(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
			LabelSelector: label,
		})
	}

	for _, d := range dashboards {
		var svc *corev1.Service

		// get all services that match the label
		svcs, err := getSvcs(client, d.Label)
		if err != nil {
			return err
		}

		// find the dashboard service
		for i, s := range svcs.Items {
			if s.Name == d.AddonName {
				svc = &svcs.Items[i]
				break
			}
		}

		if svc == nil {
			continue
		}

		// fill dashboard information
		d.Namespace = svc.Namespace
		d.CreationTime = util.TimeFormat(&svc.CreationTimestamp)
		if len(svc.Spec.Ports) > 0 {
			d.Port = fmt.Sprintf("%d", svc.Spec.Ports[0].Port)
			if d.TargetPort == "" {
				d.TargetPort = svc.Spec.Ports[0].TargetPort.String()
			}
		}
	}
	return nil
}

func newFactory(namespace string) cmdutil.Factory {
	cf := util.NewConfigFlagNoWarnings()
	cf.Namespace = pointer.String(namespace)
	return cmdutil.NewFactory(cf)
}

type defaultPortForwarder struct {
	genericclioptions.IOStreams
}

func (f *defaultPortForwarder) ForwardPorts(method string, url *url.URL, opts cmdpf.PortForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(opts.Config)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	pf, err := portforward.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, f.Out, f.ErrOut)
	if err != nil {
		return err
	}
	return pf.ForwardPorts()
}
