/*
Copyright © 2022 The OpenCli Authors

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

package cluster

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	docker "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	k3dClient "github.com/k3d-io/k3d/v5/pkg/client"
	config "github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3d "github.com/k3d-io/k3d/v5/pkg/types"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"

	//"jihulab.com/infracreate/dbaas-system/opencli/pkg/resources"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/types"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/utils"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/utils/helm"
)

var (
	LocalInstaller Installer = &PlaygroundInstaller{
		ctx: context.Background(),
	}

	dockerCli client.APIClient
	info      = utils.Info
	infof     = utils.Infof
	errf      = utils.Errf
)

const (
	clusterName = "opencli-playground"
	namespace   = "opencli-playground"
	dbCluster   = "playground-dbcluster"
)

type k3dSetupOptions struct {
	dryRun bool
}

var (
	repos = []helm.RepoEntry{
		{
			Name: "prometheus-community",
			Url:  "https://prometheus-community.github.io/helm-charts",
		},
		{
			Name: "mysql-operator",
			Url:  "https://mysql.github.io/mysql-operator/",
		},
	}

	baseCharts = []helm.InstallOpts{
		{
			Name:      "prometheus",
			Chart:     "prometheus-community/kube-prometheus-stack",
			Namespace: namespace,
			Wait:      true,
			Sets: []string{
				"prometheusOperator.admissionWebhooks.patch.image.repository=weidixian/ingress-nginx-kube-webhook-certgen",
				"kube-state-metrics.image.repository=jiamiao442/kube-state-metrics",
			},
		},
	}

	dbCharts = []helm.InstallOpts{
		{
			Name:      "my-mysql-operator",
			Chart:     "mysql-operator/mysql-operator",
			Namespace: namespace,
			Wait:      true,
			Sets:      []string{},
		},
		{
			Name:      dbCluster,
			Chart:     "mysql-operator/mysql-innodbcluster",
			Namespace: namespace,
			Wait:      true,
			Sets: []string{
				"credentials.root.user='root'",
				"credentials.root.password='sakila'",
				"credentials.root.host='%'",
				"serverInstances=1",
				"routerInstances=1",
				"tls.useSelfSigned=true",
			},
		},
	}
)

func init() {
	var err error
	dockerCli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
}

// PlaygroundInstaller will handle the playground cluster creation and management
type PlaygroundInstaller struct {
	ctx context.Context
	cfg config.ClusterConfig
}

// Install install a k3d cluster
func (d *PlaygroundInstaller) Install() error {
	var err error

	d.cfg, err = BuildClusterRunConfig()
	if err != nil {
		return err
	}

	o := k3dSetupOptions{
		dryRun: false,
	}
	err = o.setUpK3d(d.ctx, d.cfg)
	if err != nil {
		return errors.Wrap(err, "failed to setup k3d cluster")
	}
	return nil
}

// Uninstall remove the k3d cluster
func (d *PlaygroundInstaller) Uninstall() error {
	clusters, err := k3dClient.ClusterList(d.ctx, runtimes.SelectedRuntime)
	if err != nil {
		return errors.Wrap(err, "fail to get k3d cluster list")
	}

	if len(clusters) == 0 {
		return errors.New("no cluster found")
	}

	// find playground cluster
	var playgroundCluster *k3d.Cluster
	for _, c := range clusters {
		if c.Name == clusterName {
			playgroundCluster = c
			break
		}
	}

	//	extra handling to cleanup tools nodes
	defer func() {
		if nl, err := k3dClient.NodeList(d.ctx, runtimes.SelectedRuntime); err == nil {
			toolNode := fmt.Sprintf("k3d-%s-tools", clusterName)
			for _, n := range nl {
				if n.Name == toolNode {
					if err := k3dClient.NodeDelete(d.ctx, runtimes.SelectedRuntime, n, k3d.NodeDeleteOpts{}); err != nil {
						utils.Errf("Delete node %s failed.", toolNode)
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
	err = k3dClient.ClusterDelete(d.ctx, runtimes.SelectedRuntime, playgroundCluster, k3d.ClusterDeleteOpts{
		SkipRegistryCheck: false,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to delete playground cluster.")
	}

	// remove playground cluster kubeconfig
	err = utils.RemoveConfig(clusterName)
	if err != nil {
		return errors.Wrap(err, "Failed to remove playground kubeconfig file")
	}

	return nil
}

// GenKubeconfig generate a kubeconfig to access the k3d cluster
func (d *PlaygroundInstaller) GenKubeconfig() error {
	var err error
	var cluster = d.cfg.Cluster.Name

	configPath := utils.ConfigPath(cluster)
	info("Generating kubeconfig into", configPath)

	_, err = k3dClient.KubeconfigGetWrite(context.Background(), runtimes.SelectedRuntime, &d.cfg.Cluster, configPath,
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
	err = ioutil.WriteFile(configPath, []byte(cfgHostContent), 0600)
	if err != nil {
		errf("Fail to re-write host kubeconfig")
	}

	info("Successfully generate kubeconfig at", configPath)
	return nil
}

// SetKubeconfig set kubeconfig environment of cluster
func (d *PlaygroundInstaller) SetKubeconfig() error {
	info("Setting kubeconfig env for opencli playground...")
	return os.Setenv("KUBECONFIG", utils.ConfigPath(d.cfg.Cluster.Name))
}

func (d *PlaygroundInstaller) GetStatus() types.ClusterStatus {
	var status types.ClusterStatus
	images, err := dockerCli.ImageList(d.ctx, docker.ImageListOptions{})

	if err != nil {
		status.K3dImages.Reason = fmt.Sprintf("Failed to get image list:%s", err.Error())
		return status
	}

	for _, image := range images {
		fillK3dImageStatus(image, &status)
	}

	clusters, err := k3dClient.ClusterList(d.ctx, runtimes.SelectedRuntime)
	if err != nil {
		status.K3d.Reason = fmt.Sprintf("Failed to get cluster list: %s", err.Error())
		return status
	}

	status.K3d.K3dCluster = []types.K3dCluster{}
	for _, cluster := range clusters {
		fillK3dCluster(cluster, &status)
	}
	return status
}

func (d *PlaygroundInstaller) InstallDeps() error {
	var err error

	info("Add dependent repos...")
	err = addRepos(repos)
	if err != nil {
		return errors.Wrap(err, "Failed to add dependent repos")
	}

	var wg sync.WaitGroup
	info("Install base charts...")
	err = installCharts(baseCharts, &wg)
	if err != nil {
		return errors.Wrap(err, "Failed to install base charts")
	}

	info("Installing playground database cluster...")
	err = installCharts(dbCharts, &wg)
	if err != nil {
		return errors.Wrap(err, "Failed to install playground database cluster")
	}

	info("Waiting for database cluster to be ready...")
	wg.Wait()

	info("port forward to local host")
	if err = portForward(); err != nil {
		return err
	}
	return nil
}

// BuildClusterRunConfig returns the run-config for the k3d cluster
func BuildClusterRunConfig() (config.ClusterConfig, error) {
	createOpts := buildClusterCreateOpts()
	cluster, err := buildClusterConfig(createOpts)
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

func buildClusterConfig(opts k3d.ClusterCreateOpts) (k3d.Cluster, error) {
	var network = k3d.ClusterNetwork{
		Name:     types.CliDockerNetwork,
		External: false,
	}

	port, err := findAvailablePort(6443)
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
		Image:      types.K3sImage,
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
		utils.CloseQuietly(listener)
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
	lb.Node.Image = types.K3dProxyImage
	lb.Node.Ports = nat.PortMap{
		k3d.DefaultAPIPort: []nat.PortBinding{cluster.KubeAPI.Binding},
	}
	lb.Node.Networks = []string{cluster.Network.Name}
	lb.Node.RuntimeLabels = labels
	lb.Node.Restart = true

	return lb
}

func buildK3sImageDir() (string, error) {
	dir, err := utils.GetCliHomeDir()
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

//func (o k3dSetupOptions) prepareK3sImages() error {
//	info("Preparing K3s images...")
//	embedK3sImage, err := resources.K3sImage.Open("static/k3s/images/k3s-airgap-images.tar.gz")
//	if err != nil {
//		return err
//	}
//	defer utils.CloseQuietly(embedK3sImage)
//
//	k3sImageDir, err := buildK3sImageDir()
//	if err != nil {
//		return err
//	}
//	k3sImagePath := filepath.Join(k3sImageDir, "k3s-airgap-images.tgz")
//	info("saving k3s image airgap install tarball to", k3sImagePath)
//
//	if !o.dryRun {
//		k3sImageFile, err := os.OpenFile(k3sImagePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
//		if err != nil {
//			return err
//		}
//		defer utils.CloseQuietly(k3sImageFile)
//		if _, err := io.Copy(k3sImageFile, embedK3sImage); err != nil {
//			return err
//		}
//	}
//
//	info("Successfully prepare k3s image: ", k3sImagePath)
//	return nil
//}

//func (o k3dSetupOptions) loadK3dImages() error {
//	info("Loading k3d images...")
//	dir, err := resources.K3dImage.ReadDir("static/k3d/images")
//	if err != nil {
//		return err
//	}
//	for _, entry := range dir {
//		file, err := resources.K3dImage.Open(path.Join("static/k3d/images", entry.Name()))
//		if err != nil {
//			return err
//		}
//		name := strings.Split(entry.Name(), ".")[0]
//
//		var (
//			image    = "k3d-image-" + name + "-*.tar.gz"
//			imageTgz string
//			imageTar string
//		)
//
//		if o.dryRun {
//			info("Saving temporary image file:", image)
//		} else {
//			imageTgz, err = utils.SaveToTemp(file, image)
//			if err != nil {
//				return err
//			}
//			unzipCmd := exec.Command("gzip", "-d", imageTgz)
//			output, err := unzipCmd.CombinedOutput()
//			utils.InfoBytes(output)
//			if err != nil {
//				return err
//			}
//			imageTar = strings.TrimSuffix(imageTgz, ".gz")
//		}
//
//		if o.dryRun {
//			info("importing image to docker using temporary file :%s\n", image)
//		} else {
//			importCmd := exec.Command("docker", "image", "load", "-i", imageTar)
//			output, err := importCmd.CombinedOutput()
//			utils.InfoBytes(output)
//			if err != nil {
//				return err
//			}
//		}
//	}
//
//	info("Successfully load k3d images")
//	return nil
//}

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
	//if err := o.prepareK3sImages(); err != nil {
	//	return errors.Wrap(err, "failed to prepare k3s images")
	//}
	//
	//if err := o.loadK3dImages(); err != nil {
	//	return errors.Wrap(err, "failed to load k3d images")
	//}

	if err := o.createCluster(ctx, clusterConfig); err != nil {
		return errors.Wrapf(err, "Failed to create cluster: %s", clusterConfig.Cluster.Name)
	}

	return nil
}

func fillK3dImageStatus(image docker.ImageSummary, status *types.ClusterStatus) {
	if len(image.RepoTags) == 0 {
		return
	}
	for _, tag := range image.RepoTags {
		switch tag {
		case types.K3sImage:
			status.K3dImages.K3s = true
		case types.K3dToolsImage:
			status.K3dImages.K3dTools = true
		case types.K3dProxyImage:
			status.K3dImages.K3dProxy = true
		}
	}
}

func fillK3dCluster(cluster *k3d.Cluster, status *types.ClusterStatus) {
	// Skip cluster that does not match prefix
	if cluster.Name != clusterName {
		return
	}

	c := types.K3dCluster{
		Name:    clusterName,
		Running: true,
	}

	// get k3d cluster kubeconfig
	helm.SetKubeconfig(utils.ConfigPath(clusterName))
	helm.SetNamespace(namespace)
	cfg, err := helm.NewActionConfig()
	if err != nil {
		c.Reason = fmt.Sprintf("Failed to get helm action config: %s", err.Error())
	}
	list := action.NewList(cfg)
	list.SetStateMask()
	releases, err := list.Run()
	if err != nil {
		c.Reason = fmt.Sprintf("Failed to get helm releases: %s", err.Error())
	}

	rs := make(map[string]string)
	for _, release := range releases {
		rs[release.Name] = release.Info.Status.String()
	}
	c.ReleaseStatus = rs

	status.K3d.K3dCluster = append(status.K3d.K3dCluster, c)
}

func addRepos(repos []helm.RepoEntry) error {
	for _, r := range repos {
		if err := r.Add(); err != nil {
			return err
		}
	}
	return nil
}

func installCharts(charts []helm.InstallOpts, wg *sync.WaitGroup) error {
	for _, c := range charts {
		wg.Add(1)
		go func(chart helm.InstallOpts) {
			defer wg.Done()
			if _, err := chart.Install(); err != nil {
				infof("Install chart %s error: %s\n", chart.Name, err.Error())
			}
		}(c)
	}
	return nil
}

func portForward() error {
	return nil
}
