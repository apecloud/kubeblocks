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

package infrastructure

import (
	"fmt"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/pipeline"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/tasks"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/configuration/container"
)

type createOptions struct {
	clusterOptions
	version types.InfraVersionInfo

	criType      string
	debug        bool
	sandBoxImage string

	autoRenewCerts      bool
	securityEnhancement bool
}

var createExamples = `
`

func (o *createOptions) Run() error {
	cluster, err := createClusterWithOptions(buildTemplateParams(o))
	if err != nil {
		return err
	}

	yes, err := o.confirm(fmt.Sprintf("install kubernetes using version: %v", o.version.KubernetesVersion))
	if err != nil {
		return err
	}
	if !yes {
		return nil
	}

	runtime := &common.KubeRuntime{
		BaseRuntime: connector.NewBaseRuntime(o.clusterName, connector.NewDialer(), o.debug, false),
		Cluster:     cluster,
		ClusterName: o.clusterName,
	}
	syncClusterNodeRole(cluster, runtime)

	pipeline := pipeline.Pipeline{
		Name:    "CreateCluster",
		Modules: NewCreatePipeline(o),
		Runtime: runtime,
	}
	if err := pipeline.Start(); err != nil {
		return err
	}
	fmt.Fprintf(o.IOStreams.Out, "Kubernetes Installation is complete.\n\n")
	return nil
}

func NewCreateKubernetesCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &createOptions{
		clusterOptions: clusterOptions{
			IOStreams: streams,
		}}
	o.setDefaultVersion()
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "create kubernetes cluster.",
		Example: createExamples,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete())
			util.CheckErr(o.Validate())
			util.CheckErr(o.Run())
		},
	}
	o.buildCreateInfraFlags(cmd)
	return cmd
}

func (o *createOptions) buildCreateInfraFlags(cmd *cobra.Command) {
	buildCommonFlags(cmd, &o.clusterOptions)
	cmd.Flags().StringVarP(&o.version.KubernetesVersion, "version", "", o.version.KubernetesVersion, fmt.Sprintf("Specify install kubernetes version. default version is %s", o.version.KubernetesVersion))
	cmd.Flags().StringVarP(&o.criType, "container-runtime", "", string(container.ContainerdType), "Specify kubernetes container runtime. default is containerd")
	cmd.Flags().BoolVarP(&o.debug, "debug", "", false, "set debug mode")

}

func (o *createOptions) setDefaultVersion() {
	o.version.KubernetesVersion = tasks.DefaultK8sVersion
	o.version.EtcdVersion = tasks.DefaultEtcdVersion
	o.version.ContainerVersion = tasks.DefaultContainerdVersion
	o.version.HelmVersion = tasks.DefaultHelmVersion
	o.version.CRICtlVersion = tasks.DefaultCRICtlVersion
	o.version.CniVersion = tasks.DefaultCniVersion
	o.version.RuncVersion = tasks.DefaultRuncVersion
}
