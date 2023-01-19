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

package playground

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/docker/go-connections/nat"
	k3dClient "github.com/k3d-io/k3d/v5/pkg/client"
	config "github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	l "github.com/k3d-io/k3d/v5/pkg/logger"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3d "github.com/k3d-io/k3d/v5/pkg/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// installer will handle the playground cluster creation and management
type installer struct {
	cfg         config.ClusterConfig
	ctx         context.Context
	clusterName string
	genericclioptions.IOStreams
}

func (i *installer) verboseLog(v bool) {
	if !v {
		// set k3d log level to warning to avoid so much info log
		l.Log().SetLevel(logrus.WarnLevel)
	}
}

// Install install a k3d cluster
func (i *installer) install() error {
	var err error

	if i.cfg, err = buildClusterRunConfig(i.clusterName); err != nil {
		return err
	}

	if err = setUpK3d(i.ctx, &i.cfg); err != nil {
		return errors.Wrap(err, "failed to setup k3d cluster")
	}
	return nil
}

// uninstall remove the k3d cluster
func (i *installer) uninstall() error {
	clusters, err := k3dClient.ClusterList(i.ctx, runtimes.SelectedRuntime)
	if err != nil {
		return errors.Wrap(err, "fail to get k3d cluster list")
	}

	if len(clusters) == 0 {
		return errors.New("no cluster found")
	}

	// find playground cluster
	var playgroundCluster *k3d.Cluster
	for _, c := range clusters {
		if c.Name == i.clusterName {
			playgroundCluster = c
			break
		}
	}

	//	extra handling to clean up tools nodes
	defer func() {
		if nl, err := k3dClient.NodeList(i.ctx, runtimes.SelectedRuntime); err == nil {
			toolNode := fmt.Sprintf("k3d-%s-tools", i.clusterName)
			for _, n := range nl {
				if n.Name == toolNode {
					if err := k3dClient.NodeDelete(i.ctx, runtimes.SelectedRuntime, n, k3d.NodeDeleteOpts{}); err != nil {
						fmt.Printf("Delete node %s failed.", toolNode)
					}
					break
				}
			}
		}
	}()

	if playgroundCluster == nil {
		return fmt.Errorf("playground k3d cluster does not exist")
	}

	// delete playground cluster
	if err = k3dClient.ClusterDelete(i.ctx, runtimes.SelectedRuntime, playgroundCluster,
		k3d.ClusterDeleteOpts{SkipRegistryCheck: false}); err != nil {
		return errors.Wrap(err, "failed to delete playground cluster")
	}

	// remove playground cluster kubeconfig
	if err = util.RemoveConfig(i.clusterName); err != nil {
		return errors.Wrap(err, "failed to remove playground kubeconfig file")
	}

	return nil
}

// genKubeconfig generate a kubeconfig to access the k3d cluster
func (i *installer) genKubeconfig() error {
	var err error

	configPath := util.ConfigPath(i.clusterName)
	_, err = k3dClient.KubeconfigGetWrite(i.ctx, runtimes.SelectedRuntime, &i.cfg.Cluster, configPath,
		&k3dClient.WriteKubeConfigOptions{UpdateExisting: true, OverwriteExisting: false, UpdateCurrentContext: true})
	if err != nil {
		return errors.Wrap(err, "failed to generate kubeconfig")
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

	if err := k3dClient.ClusterRun(ctx, runtimes.SelectedRuntime, cluster); err != nil {
		return err
	}

	return nil
}
