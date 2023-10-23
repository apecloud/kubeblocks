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

package types

import (
	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"helm.sh/helm/v3/pkg/cli/values"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/constant"
)

type InfraVersionInfo struct {
	KubernetesVersion string `json:"kubernetesVersion"`
	EtcdVersion       string `json:"etcdVersion"`
	ContainerVersion  string `json:"containerVersion"`
	CRICtlVersion     string `json:"crictlVersion"`
	RuncVersion       string `json:"runcVersion"`
	CniVersion        string `json:"cniVersion"`
	HelmVersion       string `json:"helmVersion"`
}

type PluginMeta struct {
	Name      string        `json:"name,omitempty"`
	Namespace string        `json:"namespace,omitempty"`
	Sources   PluginSources `json:"sources,omitempty"`
}

type PluginSources struct {
	Chart *HelmChart `json:"chart"`
	Yaml  *Yaml      `json:"yaml"`
}

type HelmChart struct {
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Repo         string         `json:"repo"`
	Path         string         `json:"path"`
	ValueOptions values.Options `json:"options"`
}

type Yaml struct {
	Path []string `json:"path,omitempty"`
}

type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	User  ClusterUser   `json:"user"`
	Nodes []ClusterNode `json:"nodes"`

	RoleGroup RoleGroup    `json:"roleGroup"`
	Addons    []PluginMeta `json:"addons"`

	// for kubeadm configuration
	Kubernetes Kubernetes `json:"kubernetes"`

	Version InfraVersionInfo `json:"version"`
}

type RoleGroup struct {
	ETCD   []string `json:"etcd"`
	Master []string `json:"master"`
	Worker []string `json:"worker"`
}

type ClusterNode struct {
	Name            string       `json:"name"`
	Address         string       `json:"address"`
	InternalAddress string       `json:"internalAddress"`
	NodeOptions     *NodeOptions `json:"options"`
}

type NodeOptions struct {
	HugePageFeature *HugePageFeature `json:"hugePageFeature"`
}

type Kubernetes struct {
	// ClusterName string `json:"clusterName"`
	// DNSDomain string `json:"dnsDomain"`
	ProxyMode string `json:"proxyMode"`

	Networking Networking       `json:"networking"`
	CRI        ContainerRuntime `json:"cri"`

	ControlPlaneEndpoint ControlPlaneEndpoint `json:"controlPlaneEndpoint"`
	APIServer            APIServer            `json:"apiServer"`
	Scheduler            Scheduler            `json:"scheduler"`
}

type Networking struct {
	// using network plugin, default is calico
	Plugin string `json:"plugin"`

	// apis/kubeadm/types.Networking
	ServiceSubnet string `json:"serviceSubnet"`
	PodSubnet     string `json:"podSubnet"`
	DNSDomain     string `json:"dnsDomain"`
}

type ContainerRuntime struct {
	ContainerRuntimeType     string `json:"containerRuntimeType"`
	ContainerRuntimeEndpoint string `json:"containerRuntimeEndpoint"`
	SandBoxImage             string `json:"sandBoxImage"`
}

type ControlPlaneComponent struct {
	// apiserver extra args
	ExtraArgs map[string]string `json:"extraArgs"`
}

type APIServer struct {
	ControlPlaneComponent `json:",inline"`
}

type Scheduler struct {
	ControlPlaneComponent `json:",inline"`
}

type ControlPlaneEndpoint struct {
	Domain string `json:"domain"`
	Port   int    `json:"port"`

	// TODO support apiserver loadbalancer
	LoadBalancer string `json:"loadBalancer"`
}

type ClusterUser struct {
	// user name
	Name string `json:"name"`
	// sudo password
	Password string `json:"password"`
	// ssh privateKey
	PrivateKey     string `json:"privateKey"`
	PrivateKeyPath string `json:"privateKeyPath"`
}

func (g *RoleGroup) IsValidate() bool {
	return len(g.ETCD) > 0 && len(g.Master) > 0 && len(g.Worker) > 0
}

func (k *Kubernetes) AutoDefaultFill() {
	// if k.ClusterName == "" {
	//	k.ClusterName = constant.DefaultK8sClusterName
	// }
	// if k.DNSDomain == "" {
	//	k.DNSDomain = constant.DefaultK8sDNSDomain
	// }
	if k.ProxyMode == "" {
		k.ProxyMode = constant.DefaultK8sProxyMode
	}

	fillNetworkField(&k.Networking)
	fillContainerRuntimeField(&k.CRI)
	fillAPIServerField(&k.APIServer)
	fillSchedulerField(&k.Scheduler)
	fillControlPlaneField(&k.ControlPlaneEndpoint)
}

func fillContainerRuntimeField(c *ContainerRuntime) {
	if c.ContainerRuntimeType == "" {
		c.ContainerRuntimeType = kubekeyapiv1alpha2.Conatinerd
		c.ContainerRuntimeEndpoint = kubekeyapiv1alpha2.DefaultContainerdEndpoint
	}
	if c.SandBoxImage == "" {
		c.SandBoxImage = constant.DefaultSandBoxImage
	}
}

func fillControlPlaneField(c *ControlPlaneEndpoint) {
	if c.Port == 0 {
		c.Port = constant.DefaultAPIServerPort
	}
	if c.Domain == "" {
		c.Domain = constant.DefaultAPIDNSDomain
	}
}

func fillSchedulerField(s *Scheduler) {
}

func fillAPIServerField(a *APIServer) {
}

func fillNetworkField(n *Networking) {
	if n.Plugin == "" {
		n.Plugin = constant.DefaultNetworkPlugin
	}
	if n.DNSDomain == "" {
		n.DNSDomain = constant.DefaultK8sDNSDomain
	}
	if n.ServiceSubnet == "" {
		n.ServiceSubnet = kubekeyapiv1alpha2.DefaultServiceCIDR
	}
	if n.PodSubnet == "" {
		n.PodSubnet = kubekeyapiv1alpha2.DefaultPodsCIDR
	}
}
