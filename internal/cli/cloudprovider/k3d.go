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

package cloudprovider

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/docker/go-connections/nat"
	"github.com/k3d-io/k3d/v5/pkg/actions"
	k3dClient "github.com/k3d-io/k3d/v5/pkg/client"
	config "github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	l "github.com/k3d-io/k3d/v5/pkg/logger"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3d "github.com/k3d-io/k3d/v5/pkg/types"
	"github.com/k3d-io/k3d/v5/pkg/types/fixes"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/version"
)

var (
	// CliDockerNetwork is docker network for k3d cluster when `kbcli playground`
	// all cluster will be created in this network, so they can communicate with each other
	CliDockerNetwork = "k3d-kbcli-playground"

	// K3sImage is k3s image repo
	K3sImage = "rancher/k3s:" + version.K3sImageTag

	// K3dProxyImage is k3d proxy image repo
	K3dProxyImage = "docker.io/apecloud/k3d-proxy:" + version.K3dVersion

	// K3dFixEnv
	KBEnvFix fixes.K3DFixEnv = "KB_FIX_MOUNTS"
)

//go:embed assets/k3d-entrypoint-mount.sh
var k3dMountEntrypoint []byte

// localCloudProvider handles the k3d playground cluster creation and management
type localCloudProvider struct {
	cfg    config.ClusterConfig
	stdout io.Writer
	stderr io.Writer
}

// localCloudProvider should be an implementation of cloud provider
var _ Interface = &localCloudProvider{}

func init() {
	if !klog.V(1).Enabled() {
		// set k3d log level to 'warning' to avoid too much info logs
		l.Log().SetLevel(logrus.WarnLevel)
	}
}

func newLocalCloudProvider(stdout, stderr io.Writer) Interface {
	return &localCloudProvider{
		stdout: stdout,
		stderr: stderr,
	}
}

func (p *localCloudProvider) Name() string {
	return Local
}

// CreateK8sCluster creates a local kubernetes cluster using k3d
func (p *localCloudProvider) CreateK8sCluster(clusterInfo *K8sClusterInfo) error {
	var err error

	if p.cfg, err = buildClusterRunConfig(clusterInfo.ClusterName); err != nil {
		return err
	}

	if err = setUpK3d(context.Background(), &p.cfg); err != nil {
		return errors.Wrapf(err, "failed to create k3d cluster %s", clusterInfo.ClusterName)
	}

	return nil
}

// DeleteK8sCluster removes the k3d cluster
func (p *localCloudProvider) DeleteK8sCluster(clusterInfo *K8sClusterInfo) error {
	var err error
	if clusterInfo == nil {
		clusterInfo, err = p.GetClusterInfo()
		if err != nil {
			return err
		}
	}
	ctx := context.Background()
	clusterName := clusterInfo.ClusterName
	clusters, err := k3dClient.ClusterList(ctx, runtimes.SelectedRuntime)
	if err != nil {
		return errors.Wrap(err, "fail to get k3d cluster list")
	}

	if len(clusters) == 0 {
		return errors.New("no cluster found")
	}

	// find cluster that matches the name
	var cluster *k3d.Cluster
	for _, c := range clusters {
		if c.Name == clusterName {
			cluster = c
			break
		}
	}

	// extra handling to clean up tools nodes
	defer func() {
		if nl, err := k3dClient.NodeList(ctx, runtimes.SelectedRuntime); err == nil {
			toolNode := fmt.Sprintf("k3d-%s-tools", clusterName)
			for _, n := range nl {
				if n.Name == toolNode {
					if err := k3dClient.NodeDelete(ctx, runtimes.SelectedRuntime, n, k3d.NodeDeleteOpts{}); err != nil {
						fmt.Printf("Delete node %s failed.", toolNode)
					}
					break
				}
			}
		}
	}()

	if cluster == nil {
		return fmt.Errorf("k3d cluster %s does not exist", clusterName)
	}

	// delete playground cluster
	if err = k3dClient.ClusterDelete(ctx, runtimes.SelectedRuntime, cluster,
		k3d.ClusterDeleteOpts{SkipRegistryCheck: false}); err != nil {
		return errors.Wrapf(err, "failed to delete playground cluster %s", clusterName)
	}
	return nil
}

func (p *localCloudProvider) GetKubeConfig() (string, error) {
	ctx := context.Background()
	cluster := &k3d.Cluster{Name: types.K3dClusterName}
	kubeConfig, err := k3dClient.KubeconfigGet(ctx, runtimes.SelectedRuntime, cluster)
	if err != nil {
		return "", err
	}
	cfgBytes, err := clientcmd.Write(*kubeConfig)
	if err != nil {
		return "", err
	}

	var (
		hostToReplace string
		cfgStr        = string(cfgBytes)
	)

	switch {
	case strings.Contains(cfgStr, "0.0.0.0"):
		hostToReplace = "0.0.0.0"
	case strings.Contains(cfgStr, "host.docker.internal"):
		hostToReplace = "host.docker.internal"
	default:
		return "", errors.Wrap(err, "unrecognized k3d kubeconfig format")
	}

	// replace host config with loop back address
	return strings.ReplaceAll(cfgStr, hostToReplace, "127.0.0.1"), nil
}

func (p *localCloudProvider) GetClusterInfo() (*K8sClusterInfo, error) {
	kubeConfig, err := p.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	return &K8sClusterInfo{
		CloudProvider: p.Name(),
		ClusterName:   types.K3dClusterName,
		KubeConfig:    kubeConfig,
		Region:        "",
	}, nil
}

// buildClusterRunConfig returns the run-config for the k3d cluster
func buildClusterRunConfig(clusterName string) (config.ClusterConfig, error) {
	createOpts := buildClusterCreateOpts()
	cluster, err := buildClusterConfig(clusterName, createOpts)
	if err != nil {
		return config.ClusterConfig{}, err
	}
	kubeconfigOpts := buildKubeconfigOptions()
	runConfig := config.ClusterConfig{
		Cluster:           cluster,
		ClusterCreateOpts: createOpts,
		KubeconfigOpts:    kubeconfigOpts,
	}

	return runConfig, nil
}

func buildClusterCreateOpts() k3d.ClusterCreateOpts {
	clusterCreateOpts := k3d.ClusterCreateOpts{
		GlobalLabels:        map[string]string{},
		GlobalEnv:           []string{},
		DisableLoadBalancer: false,
	}

	for k, v := range k3d.DefaultRuntimeLabels {
		clusterCreateOpts.GlobalLabels[k] = v
	}

	return clusterCreateOpts
}

func buildClusterConfig(clusterName string, opts k3d.ClusterCreateOpts) (k3d.Cluster, error) {
	var network = k3d.ClusterNetwork{
		Name:     CliDockerNetwork,
		External: false,
	}

	port, err := findAvailablePort(6444)
	if err != nil {
		panic(err)
	}

	// build opts to access the Kubernetes API
	kubeAPIOpts := k3d.ExposureOpts{
		PortMapping: nat.PortMapping{
			Port: k3d.DefaultAPIPort,
			Binding: nat.PortBinding{
				HostIP:   k3d.DefaultAPIHost,
				HostPort: port,
			},
		},
		Host: k3d.DefaultAPIHost,
	}

	// build cluster config
	clusterConfig := k3d.Cluster{
		Name:    clusterName,
		Network: network,
		KubeAPI: &kubeAPIOpts,
	}

	// build nodes
	var nodes []*k3d.Node

	// build load balancer node
	clusterConfig.ServerLoadBalancer = buildLoadbalancer(clusterConfig, opts)
	nodes = append(nodes, clusterConfig.ServerLoadBalancer.Node)

	// build k3d node
	serverNode := k3d.Node{
		Name:       k3dClient.GenerateNodeName(clusterConfig.Name, k3d.ServerRole, 0),
		Role:       k3d.ServerRole,
		Image:      K3sImage,
		ServerOpts: k3d.ServerOpts{},
		Args:       []string{"--disable=metrics-server", "--disable=traefik", "--disable=local-storage"},
	}

	nodes = append(nodes, &serverNode)

	clusterConfig.Nodes = nodes
	clusterConfig.ServerLoadBalancer.Config.Ports[fmt.Sprintf("%s.tcp", k3d.DefaultAPIPort)] =
		append(clusterConfig.ServerLoadBalancer.Config.Ports[fmt.Sprintf("%s.tcp", k3d.DefaultAPIPort)], serverNode.Name)

	// other configurations
	portWithFilter, err := buildPortWithFilters()
	if err != nil {
		return clusterConfig, errors.Wrap(err, "failed to build http ports")
	}

	err = k3dClient.TransformPorts(context.Background(), runtimes.SelectedRuntime, &clusterConfig, []config.PortWithNodeFilters{portWithFilter})
	if err != nil {
		return clusterConfig, errors.Wrap(err, "failed to transform ports")
	}

	return clusterConfig, nil
}

func findAvailablePort(start int) (string, error) {
	for i := start; i < 65535; i++ {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", i))
		if err != nil {
			continue
		}
		util.CloseQuietly(listener)
		return strconv.Itoa(i), nil
	}
	return "", errors.New("can not find any available port")
}

func buildLoadbalancer(cluster k3d.Cluster, opts k3d.ClusterCreateOpts) *k3d.Loadbalancer {
	lb := k3d.NewLoadbalancer()

	labels := map[string]string{}
	if opts.GlobalLabels == nil && len(opts.GlobalLabels) == 0 {
		labels = opts.GlobalLabels
	}

	lb.Node.Name = fmt.Sprintf("%s-%s-serverlb", k3d.DefaultObjectNamePrefix, cluster.Name)
	lb.Node.Image = K3dProxyImage
	lb.Node.Ports = nat.PortMap{
		k3d.DefaultAPIPort: []nat.PortBinding{cluster.KubeAPI.Binding},
	}
	lb.Node.Networks = []string{cluster.Network.Name}
	lb.Node.RuntimeLabels = labels
	lb.Node.Restart = true

	return lb
}

func buildPortWithFilters() (config.PortWithNodeFilters, error) {
	var port config.PortWithNodeFilters

	hostPort, err := findAvailablePort(8090)
	if err != nil {
		return port, err
	}
	port.Port = fmt.Sprintf("%s:80", hostPort)
	port.NodeFilters = []string{"loadbalancer"}

	return port, nil
}

func buildKubeconfigOptions() config.SimpleConfigOptionsKubeconfig {
	opts := config.SimpleConfigOptionsKubeconfig{
		UpdateDefaultKubeconfig: true,
		SwitchCurrentContext:    true,
	}
	return opts
}

func setUpK3d(ctx context.Context, cluster *config.ClusterConfig) error {
	// add fix Envs
	if err := os.Setenv(string(KBEnvFix), "1"); err != nil {
		return err
	}
	fixes.FixEnvs = append(fixes.FixEnvs, KBEnvFix)

	l, err := k3dClient.ClusterList(ctx, runtimes.SelectedRuntime)
	if err != nil {
		return err
	}

	if cluster == nil {
		return errors.New("failed to create cluster")
	}

	for _, c := range l {
		if c.Name == cluster.Name {
			if c, err := k3dClient.ClusterGet(ctx, runtimes.SelectedRuntime, c); err == nil {
				klog.V(1).Infof("Detected an existing cluster: %s", c.Name)
				return nil
			}
			break
		}
	}

	// exec "mount --make-rshared /" to fix csi driver plugins crash
	cluster.ClusterCreateOpts.NodeHooks = append(cluster.ClusterCreateOpts.NodeHooks, k3d.NodeHook{
		Stage: k3d.LifecycleStagePreStart,
		Action: actions.WriteFileAction{
			Runtime:     runtimes.SelectedRuntime,
			Content:     k3dMountEntrypoint,
			Dest:        "/bin/k3d-entrypoint-mount.sh",
			Mode:        0744,
			Description: "Write entrypoint script for mount shared fix",
		},
	})

	if err := k3dClient.ClusterRun(ctx, runtimes.SelectedRuntime, cluster); err != nil {
		return err
	}

	return nil
}
