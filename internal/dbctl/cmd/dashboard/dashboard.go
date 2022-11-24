/*
Copyright ApeCloud Inc.

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

package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	cmdpf "k8s.io/kubectl/pkg/cmd/portforward"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/utils/pointer"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
)

const (
	podRunningTimeoutFlag = "pod-running-timeout"
	defaultPodExecTimeout = 60 * time.Second
)

type dashboard struct {
	Name      string
	Port      string
	Namespace string
	Age       string
	Images    string

	// Label used to get the service
	Label string
}

var (
	dashboards = [...]*dashboard{
		{
			Name:  "kubeblocks-grafana",
			Label: "app.kubernetes.io/instance=kubeblocks,app.kubernetes.io/name=grafana",
		},
		{
			Name:  "kubeblocks-prometheus-alertmanager",
			Label: "app=prometheus,component=alertmanager,release=kubeblocks",
		},
		{
			Name:  "kubeblocks-prometheus-server",
			Label: "app=prometheus,component=server,release=kubeblocks",
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
		Short: "List and open the KubeBlocks dashboards",
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
		Use:   "list",
		Short: "List all dashboards",
		Args:  cobra.NoArgs,
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

	// output table
	tbl := uitable.New()
	tbl.AddRow("NAME", "NAMESPACE", "PORT", "IMAGES", "AGE")
	for _, d := range dashboards {
		tbl.AddRow(d.Name, d.Namespace, d.Port, d.Images, d.Age)
	}
	return util.PrintTable(o.Out, tbl)
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
		Use:   "open",
		Short: "open one dashboard",
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
		"The length of time (like 5s, 2m, or 3h, higher than zero) to wait until at least one pod is running",
	)

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
		return fmt.Errorf("failed to find dashboard \"%s\", run \"dbctl dashboard list\" to list all dashboards", o.name)
	}

	if o.localPort == "" {
		o.localPort = dash.Port
	}

	pfArgs := []string{fmt.Sprintf("deployment/%s", o.name), fmt.Sprintf("%s:%s", o.localPort, dash.Port)}
	o.portForwardOptions.Namespace = dash.Namespace
	o.portForwardOptions.Address = []string{"127.0.0.1"}
	return o.portForwardOptions.Complete(newFactory(dash.Namespace), cmd, pfArgs)
}

func (o *openOptions) run() error {
	go func() {
		<-o.portForwardOptions.ReadyChannel
		fmt.Fprintf(o.Out, "Forward successfully! Opening browser ...\n")

		url := "http://127.0.0.1:" + o.localPort
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
	getDeploys := func(client *kubernetes.Clientset, label string) (*appv1.DeploymentList, error) {
		return client.AppsV1().Deployments(metav1.NamespaceAll).
			List(context.TODO(), metav1.ListOptions{
				LabelSelector: label,
			})
	}

	for _, d := range dashboards {
		var dep *appv1.Deployment

		// get all deployments that match the label
		deps, err := getDeploys(client, d.Label)
		if err != nil {
			return err
		}

		// find the dashboard service
		for i, s := range deps.Items {
			if s.Name == d.Name {
				dep = &deps.Items[i]
			}
		}

		if dep == nil {
			continue
		}

		// fill dashboard information
		d.Namespace = dep.Namespace
		d.Age = duration.HumanDuration(time.Since(dep.CreationTimestamp.Time))

		// get ports and images
		var (
			images []string
			ports  []string
		)
		for _, c := range dep.Spec.Template.Spec.Containers {
			images = append(images, c.Image)
			if len(c.Ports) == 0 {
				continue
			}
			ports = append(ports, strconv.FormatInt(int64(c.Ports[0].ContainerPort), 10))
		}

		// now, we only display and use the first port
		if len(ports) > 0 {
			d.Port = ports[0]
		}
		d.Images = strings.Join(images, ",")
	}
	return nil
}

func newFactory(namespace string) cmdutil.Factory {
	cf := genericclioptions.NewConfigFlags(true)
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
