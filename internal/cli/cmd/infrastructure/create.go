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
	"os"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/spf13/cobra"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/constant"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/tasks"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/configuration/container"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"
)

type createOptions struct {
	clusterOptions
	version types.InfraVersionInfo

	criType      string
	debug        bool
	sandBoxImage string

	securityEnhancement bool
	outputKubeconfig    string
}

var createExamples = templates.Examples(`
	# Create kubernetes cluster with specified config yaml
	kbcli infra create -c cluster.yaml

    # example cluster.yaml
	cat cluster.yaml
metadata:
  name: kb-k8s-test-cluster
user:
  name: user1
  privateKeyPath: ~/.ssh/test.pem
nodes:
  - name: kb-infra-node-0
    address: 1.1.1.1
    internalAddress: 10.128.0.19
  - name: kb-infra-node-1
    address: 1.1.1.2
    internalAddress: 10.128.0.20
  - name: kb-infra-node-2
    address: 1.1.1.3
    internalAddress: 10.128.0.21
    options:
      hugePageFeature:
        hugePageSize: 10GB
roleGroup:
  etcd:
    - kb-infra-node-0
    - kb-infra-node-1
    - kb-infra-node-2
  master:
    - kb-infra-node-0
  worker:
    - kb-infra-node-1
    - kb-infra-node-2

kubernetes:
  containerManager: containerd
  # apis/kubeadm/types.Networking
  networking:
    plugin: cilium
    dnsDomain: cluster.local
    podSubnet: 10.233.64.0/18
    serviceSubnet: 10.233.0.0/18
  controlPlaneEndpoint:
    domain: lb.kubeblocks.local
    port: 6443
  cri:
    containerRuntimeType: "containerd"
    containerRuntimeEndpoint: "unix:///run/containerd/containerd.sock"
    sandBoxImage: "k8s.gcr.io/pause:3.8"
addons:
  - name: openebs
    namespace: kube-blocks
    sources:
      chart:
        name: openebs
        version: 3.7.0
        repo: https://openebs.github.io/charts
        options:
          values:
            - "localprovisioner.basePath=/mnt/disks"
            - "localprovisioner.hostpathClass.isDefaultClass=true"
`)

func (o *createOptions) Run() error {
	const minKubernetesVersion = "v1.24.0"

	v, err := versionutil.ParseSemantic(o.version.KubernetesVersion)
	if err != nil {
		return err
	}
	c, err := v.Compare(minKubernetesVersion)
	if err != nil {
		return err
	}
	if c < 0 {
		return cfgcore.MakeError("kubernetes version must be greater than %s", minKubernetesVersion)
	}

	o.Cluster.Kubernetes.AutoDefaultFill()
	o.version = o.Version
	o.checkAndSetDefaultVersion()
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
	checkAndUpdateZone()
	pipelineRunner := tasks.NewPipelineRunner("CreateCluster", NewCreatePipeline(o), runtime)
	if err := pipelineRunner.Do(o.IOStreams.Out); err != nil {
		return err
	}
	fmt.Fprintf(o.IOStreams.Out, "Kubernetes Installation is complete.\n\n")
	return nil
}

func checkAndUpdateZone() {
	const ZoneName = "KKZONE"
	if location, _ := util.GetIPLocation(); location == "CN" {
		os.Setenv(ZoneName, "cn")
	}
	fmt.Printf("current zone: %s\n", os.Getenv(ZoneName))
}

func NewCreateKubernetesCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := &createOptions{
		clusterOptions: clusterOptions{
			IOStreams: streams,
		}}
	o.checkAndSetDefaultVersion()
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
	o.Version = o.version
	return cmd
}

func (o *createOptions) buildCreateInfraFlags(cmd *cobra.Command) {
	buildCommonFlags(cmd, &o.clusterOptions)
	cmd.Flags().StringVarP(&o.version.KubernetesVersion, "version", "", o.version.KubernetesVersion, fmt.Sprintf("Specify install kubernetes version. default version is %s", o.version.KubernetesVersion))
	cmd.Flags().StringVarP(&o.sandBoxImage, "sandbox-image", "", constant.DefaultSandBoxImage, "Specified sandbox-image will not be used by the cri. [option]")
	cmd.Flags().StringVarP(&o.criType, "container-runtime", "", string(container.ContainerdType), "Specify kubernetes container runtime. default is containerd")
	cmd.Flags().BoolVarP(&o.debug, "debug", "", false, "set debug mode")
	cmd.Flags().StringVarP(&o.outputKubeconfig, "output-kubeconfig", "", tasks.GetDefaultConfig(), "Specified output kubeconfig. [option]")
}

func (o *createOptions) checkAndSetDefaultVersion() {
	if o.version.KubernetesVersion == "" {
		o.version.KubernetesVersion = constant.DefaultK8sVersion
	}
	if o.version.EtcdVersion == "" {
		o.version.EtcdVersion = constant.DefaultEtcdVersion
	}
	if o.version.ContainerVersion == "" {
		o.version.ContainerVersion = constant.DefaultContainerdVersion
	}
	if o.version.HelmVersion == "" {
		o.version.HelmVersion = constant.DefaultHelmVersion
	}
	if o.version.CRICtlVersion == "" {
		o.version.CRICtlVersion = constant.DefaultCRICtlVersion
	}
	if o.version.CniVersion == "" {
		o.version.CniVersion = constant.DefaultCniVersion
	}
	if o.version.RuncVersion == "" {
		o.version.RuncVersion = constant.DefaultRuncVersion
	}
}
