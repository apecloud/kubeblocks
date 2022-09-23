/*
Copyright © 2022 The dbctl Authors

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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	k3dClient "github.com/k3d-io/k3d/v5/pkg/client"
	config "github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3d "github.com/k3d-io/k3d/v5/pkg/types"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

var (
	info = util.Info
	errf = util.Errf
)

type k3dSetupOptions struct {
	dryRun bool
}

// Installer will handle the playground cluster creation and management
type Installer struct {
	cfg config.ClusterConfig

	Ctx         context.Context
	Namespace   string
	Kubeconfig  string
	ClusterName string
	DBCluster   string
	wesql       Wesql
}

// Install install a k3d cluster
func (i *Installer) Install() error {
	var err error

	i.cfg, err = BuildClusterRunConfig(i.ClusterName)
	if err != nil {
		return err
	}

	o := k3dSetupOptions{
		dryRun: false,
	}

	err = o.setUpK3d(i.Ctx, i.cfg)
	if err != nil {
		return errors.Wrap(err, "failed to setup k3d cluster")
	}
	return nil
}

// Uninstall remove the k3d cluster
func (i *Installer) Uninstall() error {
	clusters, err := k3dClient.ClusterList(i.Ctx, runtimes.SelectedRuntime)
	if err != nil {
		return errors.Wrap(err, "fail to get k3d cluster list")
	}

	if len(clusters) == 0 {
		return errors.New("no cluster found")
	}

	// find playground cluster
	var playgroundCluster *k3d.Cluster
	for _, c := range clusters {
		if c.Name == i.ClusterName {
			playgroundCluster = c
			break
		}
	}

	//	extra handling to clean up tools nodes
	defer func() {
		if nl, err := k3dClient.NodeList(i.Ctx, runtimes.SelectedRuntime); err == nil {
			toolNode := fmt.Sprintf("k3d-%s-tools", i.ClusterName)
			for _, n := range nl {
				if n.Name == toolNode {
					if err := k3dClient.NodeDelete(i.Ctx, runtimes.SelectedRuntime, n, k3d.NodeDeleteOpts{}); err != nil {
						util.Errf("Delete node %s failed.", toolNode)
					}
					break
				}
			}
		}
	}()

	if playgroundCluster == nil {
		return fmt.Errorf("no playground cluster")
	}

	// delete playground cluster
	err = k3dClient.ClusterDelete(i.Ctx, runtimes.SelectedRuntime, playgroundCluster, k3d.ClusterDeleteOpts{
		SkipRegistryCheck: false,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to delete playground cluster.")
	}

	// remove playground cluster kubeconfig
	err = util.RemoveConfig(i.ClusterName)
	if err != nil {
		return errors.Wrap(err, "Failed to remove playground kubeconfig file")
	}

	return nil
}

// GenKubeconfig generate a kubeconfig to access the k3d cluster
func (i *Installer) GenKubeconfig() error {
	var err error
	var cluster = i.cfg.Cluster.Name

	configPath := util.ConfigPath(cluster)
	info("Generating kubeconfig into", configPath)

	_, err = k3dClient.KubeconfigGetWrite(context.Background(), runtimes.SelectedRuntime, &i.cfg.Cluster, configPath,
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
	err = os.WriteFile(configPath, []byte(cfgHostContent), 0600)
	if err != nil {
		errf("Fail to re-write host kubeconfig")
	}

	info("Successfully generate kubeconfig at", configPath)
	return nil
}

// SetKubeconfig set kubeconfig environment of cluster
func (i *Installer) SetKubeconfig() error {
	info("Setting kubeconfig env for dbctl playground...")
	return os.Setenv("KUBECONFIG", util.ConfigPath(i.cfg.Cluster.Name))
}

func (i *Installer) InstallDeps() error {
	var err error

	info("Add dependent repos...")
	err = addRepos(i.wesql.GetRepos())
	if err != nil {
		return errors.Wrap(err, "Failed to add dependent repos")
	}

	err = installCharts(i)
	if err != nil {
		return errors.Wrap(err, "Failed to install playground database cluster")
	}
	info("Waiting for database cluster to be ready...")
	time.Sleep(10 * time.Second)

	return nil
}

func (i *Installer) PrintGuide(cloudProvider string, hostIP string) error {
	info := types.PlaygroundInfo{
		HostIP:        hostIP,
		CloudProvider: cloudProvider,
		DBCluster:     i.DBCluster,
		DBPort:        "3306",
		DBNamespace:   "default",
		Namespace:     i.Namespace,
		ClusterName:   i.ClusterName,
		GrafanaSvc:    "prometheus-grafana",
		GrafanaPort:   "9100",
		GrafanaUser:   "admin",
		GrafanaPasswd: "prom-operator",
		Version:       version.Version,
	}
	return util.PrintPlaygroundGuide(info)
}

// BuildClusterRunConfig returns the run-config for the k3d cluster
func BuildClusterRunConfig(clusterName string) (config.ClusterConfig, error) {
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
	k3sImageDir, err := buildK3sImageDir()
	if err != nil {
		errf("Failed to create k3s image dir: %v", err)
	}

	serverNode := k3d.Node{
		Name:       k3dClient.GenerateNodeName(clusterConfig.Name, k3d.ServerRole, 0),
		Role:       k3d.ServerRole,
		Image:      K3sImage,
		ServerOpts: k3d.ServerOpts{},
		Volumes:    []string{k3sImageDir + ":/var/lib/rancher/k3s/agent/images/"},
	}

	nodes = append(nodes, &serverNode)

	clusterConfig.Nodes = nodes
	clusterConfig.ServerLoadBalancer.Config.Ports[fmt.Sprintf("%s.tcp", k3d.DefaultAPIPort)] = append(clusterConfig.ServerLoadBalancer.Config.Ports[fmt.Sprintf("%s.tcp", k3d.DefaultAPIPort)], serverNode.Name)

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

func buildK3sImageDir() (string, error) {
	dir, err := util.GetCliHomeDir()
	if err != nil {
		return "", err
	}
	k3sImagesDir := filepath.Join(dir, "playground", "k3s")
	if err := os.MkdirAll(k3sImagesDir, 0700); err != nil {
		return "", err
	}
	return k3sImagesDir, nil
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

func (o k3dSetupOptions) createCluster(ctx context.Context, cluster config.ClusterConfig) error {
	info("Launching k3d cluster:", cluster.Cluster.Name)
	if !o.dryRun {
		l, err := k3dClient.ClusterList(ctx, runtimes.SelectedRuntime)
		if err != nil {
			return err
		}
		for _, c := range l {
			if c.Name == cluster.Name {
				if c, err := k3dClient.ClusterGet(ctx, runtimes.SelectedRuntime, c); err == nil {
					info("Detected an existing cluster:", c.Name, ";", c)
					return nil
				}
				break
			}
		}
		if err := k3dClient.ClusterRun(ctx, runtimes.SelectedRuntime, &cluster); err != nil {
			return err
		}
	}

	info("Successfully created k3d cluster.")
	return nil
}

func (o k3dSetupOptions) setUpK3d(ctx context.Context, clusterConfig config.ClusterConfig) error {
	if err := o.createCluster(ctx, clusterConfig); err != nil {
		return errors.Wrapf(err, "Failed to create cluster: %s", clusterConfig.Cluster.Name)
	}

	return nil
}

func addRepos(repos []repo.Entry) error {
	for _, r := range repos {
		if err := helm.AddRepo(&r); err != nil {
			return err
		}
	}
	return nil
}

func installCharts(i *Installer) error {
	install := func(cs []helm.InstallOpts) error {
		cfg, err := helm.NewActionConfig("", util.ConfigPath(i.ClusterName))
		if err != nil {
			return err
		}

		for _, c := range cs {
			if err = c.Install(cfg); err != nil {
				return err
			}
		}
		return nil
	}

	info("Installing playground database cluster...")
	charts := i.wesql.GetBaseCharts(i.Namespace)
	err := install(charts)
	if err != nil {
		return err
	}

	// install database cluster to default namespace
	charts = i.wesql.GetDBCharts(i.Namespace, i.DBCluster)
	err = install(charts)
	if err != nil {
		return err
	}
	return nil
}
