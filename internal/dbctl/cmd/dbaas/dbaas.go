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

package dbaas

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/ghodss/yaml"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

const (
	kMonitorParam = "prometheus.enabled=true,grafana.enabled=true,dashboards.enabled=true"
)

type options struct {
	genericclioptions.IOStreams

	cfg       *action.Configuration
	Namespace string
	client    dynamic.Interface
}

type installOptions struct {
	options
	Version string
	Sets    []string
	Monitor bool
}

type addEngineOptions struct {
	options             options
	AppVersionsByte     []byte
	ClusterDefsByte     []byte
	AppVersionsFilePath string
	ClusterDefsFilePath string
}

// NewDbaasCmd creates the dbaas command
func NewDbaasCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dbaas",
		Short: "DBaaS(KubeBlocks) operation commands",
	}
	cmd.AddCommand(
		newInstallCmd(f, streams),
		newUninstallCmd(f, streams),
		newAddEngineCmd(f, streams),
	)
	return cmd
}

func (o *options) complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}

	o.cfg, err = helm.NewActionConfig(o.Namespace, kubeconfig)
	if err != nil {
		return err
	}

	o.client, err = f.DynamicClient()
	return err
}

func (o *installOptions) run() error {
	fmt.Fprintf(o.Out, "Installing KubeBlocks %s\n", o.Version)

	if o.Monitor {
		o.Sets = append(o.Sets, kMonitorParam)
	}

	installer := Installer{
		HelmCfg:   o.cfg,
		Namespace: o.Namespace,
		Version:   o.Version,
		Sets:      o.Sets,
	}

	var notes string
	var err error
	if notes, err = installer.Install(); err != nil {
		return errors.Wrap(err, "Failed to install KubeBlocks")
	}

	fmt.Fprintf(o.Out, `
KubeBlocks %s Install SUCCESSFULLY!

-> Basic commands for cluster:
    dbctl cluster create -h     # help information about creating a database cluster
    dbctl cluster list          # list all database clusters
    dbctl cluster describe <cluster name>  # get cluster information

-> Uninstall DBaaS:
    dbctl dbaas uninstall
`, o.Version)
	fmt.Fprint(o.Out, notes)
	return nil
}

func (o *options) run() error {
	fmt.Fprintln(o.Out, "Uninstalling KubeBlocks")

	installer := Installer{
		HelmCfg:   o.cfg,
		Namespace: o.Namespace,
		client:    o.client,
	}

	if err := installer.Uninstall(); err != nil {
		return errors.Wrap(err, "Failed to uninstall KubeBlocks")
	}

	fmt.Fprintln(o.Out, "Successfully uninstall KubeBlocks")
	return nil
}

func newInstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &installOptions{
		options: options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install KubeBlocks",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(f, cmd))
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().BoolVar(&o.Monitor, "monitor", false, "Set monitor enabled (default false)")
	cmd.Flags().StringVar(&o.Version, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")

	return cmd
}

func newUninstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &options{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall KubeBlocks",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(f, cmd))
			cmdutil.CheckErr(o.run())
		},
	}
	return cmd
}

func newAddEngineCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &addEngineOptions{
		options: options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:   "add-engine",
		Short: "Add a new engine to KubeBlocks",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.AppVersionsFilePath, "app-version", "", "KubeBlocks new engine app version yaml file path")
	cmd.Flags().StringVar(&o.ClusterDefsFilePath, "cluster-definition", "", "KubeBlocks new engine cluster definition yaml file path")

	return cmd
}

func (o *addEngineOptions) Validate() error {
	if o.AppVersionsFilePath == "" && o.ClusterDefsFilePath == "" {
		return fmt.Errorf("a valid appversion yaml file or clusterdefinition yaml file path is needed")
	}
	return nil
}

func (o *addEngineOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var (
		appVersionsByte []byte
		clusterDefsByte []byte
		err             error
	)
	if len(o.AppVersionsFilePath) > 0 {
		if appVersionsByte, err = os.ReadFile(o.AppVersionsFilePath); err != nil {
			return err
		}
		if appVersionsByte, err = yaml.YAMLToJSON(appVersionsByte); err != nil {
			return err
		}
		o.AppVersionsByte = appVersionsByte
	}
	if len(o.ClusterDefsFilePath) > 0 {
		if clusterDefsByte, err = os.ReadFile(o.ClusterDefsFilePath); err != nil {
			return err
		}
		if clusterDefsByte, err = yaml.YAMLToJSON(clusterDefsByte); err != nil {
			return err
		}
		o.ClusterDefsByte = clusterDefsByte
	}
	err = o.options.complete(f, cmd)
	if err != nil {
		return err
	}
	return nil
}

// Run execute command. the options of parameter contain the command flags and args.
func (o *addEngineOptions) Run() error {
	var (
		err             error
		unstructuredObj *unstructured.Unstructured
	)
	if o.ClusterDefsFilePath != "" {
		if err = json.Unmarshal(o.ClusterDefsByte, &unstructuredObj); err != nil {
			return err
		}
		gvr := schema.GroupVersionResource{Group: types.Group, Version: types.Version, Resource: types.ResourceClusterDefinitions}
		if unstructuredObj, err = o.options.client.Resource(gvr).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
			return err
		}
		fmt.Fprintf(o.options.Out, "%s %s created\n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	}
	if o.AppVersionsFilePath != "" {
		if err = json.Unmarshal(o.AppVersionsByte, &unstructuredObj); err != nil {
			return err
		}
		gvr := schema.GroupVersionResource{Group: types.Group, Version: types.Version, Resource: types.ResourceAppVersions}
		if unstructuredObj, err = o.options.client.Resource(gvr).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
			return err
		}
		fmt.Fprintf(o.options.Out, "%s %s created\n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	}
	return nil
}
