/*
Copyright ApeCloud, Inc.

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

package cloudprovider

import (
	"context"
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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/version"
)

var (
	// CliDockerNetwork is docker network for k3d cluster when `kbcli playground`
	// all cluster will be created in this network, so they can communicate with each other
	CliDockerNetwork = "k3d-kbcli-playground"

	// K3sImage is k3s image repo
	K3sImage = "rancher/k3s:" + version.K3sImageTag

	// K3dToolsImage is k3d tools image repo
	K3dToolsImage = "docker.io/apecloud/k3d-tools:" + version.K3dVersion

	// K3dProxyImage is k3d proxy image repo
	K3dProxyImage = "docker.io/apecloud/k3d-proxy:" + version.K3dVersion
)

// localCloudProvider will handle the k3d playground cluster creation and management
type localCloudProvider struct {
	cfg config.ClusterConfig
	ctx context.Context

	stdout io.Writer
	stderr io.Writer
}

// localCloudProvider should be an implementation of cloud provider
var _ Interface = &localCloudProvider{}

func NewLocalCloudProvider(stdout, stderr io.Writer) *localCloudProvider {
	return &localCloudProvider{
		ctx:    context.Background(),
		stdout: stdout,
		stderr: stderr,
	}
}

func (p *localCloudProvider) VerboseLog(v bool) {
	if !v {
		// set k3d log level to warning to avoid so much info log
		l.Log().SetLevel(logrus.WarnLevel)
	}
}

func (p *localCloudProvider) Name() string {
	return Local
}

// CreateK8sCluster create a local kubernetes cluster using k3d
func (p *localCloudProvider) CreateK8sCluster(name string, init bool) error {
	var err error

	if p.cfg, err = buildClusterRunConfig(name); err != nil {
		return err
	}

	if err = setUpK3d(p.ctx, &p.cfg); err != nil {
		return errors.Wrapf(err, "failed to create k3d cluster %s", name)
	}

	return p.UpdateKubeconfig(name)
}

// DeleteK8sCluster remove the k3d cluster
func (p *localCloudProvider) DeleteK8sCluster(name string) error {
	clusters, err := k3dClient.ClusterList(p.ctx, runtimes.SelectedRuntime)
	if err != nil {
		return errors.Wrap(err, "fail to get k3d cluster list")
	}

	if len(clusters) == 0 {
		return errors.New("no cluster found")
	}

	// find cluster that matches the name
	var cluster *k3d.Cluster
	for _, c := range clusters {
		if c.Name == name {
			cluster = c
			break
		}
	}

	//	extra handling to clean up tools nodes
	defer func() {
		if nl, err := k3dClient.NodeList(p.ctx, runtimes.SelectedRuntime); err == nil {
			toolNode := fmt.Sprintf("k3d-%s-tools", name)
			for _, n := range nl {
				if n.Name == toolNode {
					if err := k3dClient.NodeDelete(p.ctx, runtimes.SelectedRuntime, n, k3d.NodeDeleteOpts{}); err != nil {
						fmt.Printf("Delete node %s failed.", toolNode)
					}
					break
				}
			}
		}
	}()

	if cluster == nil {
		return fmt.Errorf("k3d cluster %s does not exist", name)
	}

	// delete playground cluster
	if err = k3dClient.ClusterDelete(p.ctx, runtimes.SelectedRuntime, cluster,
		k3d.ClusterDeleteOpts{SkipRegistryCheck: false}); err != nil {
		return errors.Wrapf(err, "failed to delete playground cluster %s", name)
	}

	// remove cluster info from kubeconfig
	return k3dClient.KubeconfigRemoveClusterFromDefaultConfig(p.ctx, cluster)
}

// UpdateKubeconfig generate a kubeconfig to access the k3d cluster
func (p *localCloudProvider) UpdateKubeconfig(name string) error {
	var err error

	configPath := util.ConfigPath("config")
	_, err = k3dClient.KubeconfigGetWrite(p.ctx, runtimes.SelectedRuntime, &p.cfg.Cluster, configPath,
		&k3dClient.WriteKubeConfigOptions{UpdateExisting: true, OverwriteExisting: false, UpdateCurrentContext: true})
	if err != nil {
		return errors.Wrapf(err, "failed to generate kubeconfig for cluster %s", name)
	}

	_cfgContent, err := os.ReadFile(configPath)
	if err != nil {
		return errors.Wrap(err, "read kubeconfig")
	}

	var (
		hostToReplace string
		kubeConfig    = string(_cfgContent)
	)

	switch {
	case strings.Contains(kubeConfig, "0.0.0.0"):
		hostToReplace = "0.0.0.0"
	case strings.Contains(kubeConfig, "host.docker.internal"):
		hostToReplace = "host.docker.internal"
	default:
		return errors.Wrap(err, "unrecognized kubeconfig format")
	}

	// Replace host config with loop back address
	cfgHostContent := strings.ReplaceAll(kubeConfig, hostToReplace, "127.0.0.1")
	if err = os.WriteFile(configPath, []byte(cfgHostContent), 0600); err != nil {
		fmt.Println("Fail to re-write host kubeconfig")
	}
	return nil
}

func (p *localCloudProvider) GetExistedClusters() ([]string, error) {
	clusters, err := k3dClient.ClusterList(p.ctx, runtimes.SelectedRuntime)
	if err != nil {
		return nil, errors.Wrap(err, "fail to get k3d cluster list")
	}

	names := make([]string, len(clusters))
	for i, c := range clusters {
		names[i] = c.Name
	}
	return names, nil
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
				fmt.Printf(" Detected an existing cluster: %s", c.Name)
				return nil
			}
			break
		}
	}

	// exec "mount --make-rshared /" to fix csi driver plugins crash
	cluster.ClusterCreateOpts.NodeHooks = append(cluster.ClusterCreateOpts.NodeHooks, k3d.NodeHook{
		Stage: k3d.LifecycleStagePostStart,
		Action: actions.ExecAction{
			Runtime: runtimes.SelectedRuntime,
			Command: []string{
				"sh", "-c", "mount --make-rshared /",
			},
			Retries:     0,
			Description: "Inject 'mount --make-rshared /' for csi driver",
		},
	})

	if err := k3dClient.ClusterRun(ctx, runtimes.SelectedRuntime, cluster); err != nil {
		return err
	}

	return nil
}
