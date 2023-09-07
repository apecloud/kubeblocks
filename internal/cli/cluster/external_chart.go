package cluster

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var chartsCacheDir string
var cacheFiles []fs.DirEntry

type clusterConfig []*TypeInstance

// GlobalClusterChartConfig is kbcli global cluster chart config
var GlobalClusterChartConfig clusterConfig

// ReadConfigs read the config from $HOME/.kbcli/clusterTypes
func (c *clusterConfig) ReadConfigs() error {
	homeDir, err := util.GetCliHomeDir()
	if err != nil {
		return err
	}
	contents, err := os.ReadFile(filepath.Join(homeDir, types.CliClusterTypeConfigs))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		contents = []byte{}
	}
	err = yaml.Unmarshal(contents, c)
	if err != nil {
		return err
	}
	return nil
}

// WriteConfigs write current config into $HOME/.kbcli/clusterTypes
func (c *clusterConfig) WriteConfigs() error {
	homeDir, err := util.GetCliHomeDir()
	if err != nil {
		return err
	}
	newConfig, err := yaml.Marshal(*c)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(homeDir, types.CliClusterTypeConfigs), newConfig, 0666)
}

// AddConfig add a new cluster type instance into current config
func (c *clusterConfig) AddConfig(add *TypeInstance) {
	*c = append(*c, add)
}

// RemoveConfig remove a ClusterType from current config
func (c *clusterConfig) RemoveConfig(name ClusterType) {
	tempList := *c
	for i, chart := range tempList {
		if chart.Name == name {
			*c = append((*c)[:i], (*c)[i+1:]...)
			break
		}
	}
}

// RegisterCMD will register all cluster type instances in the config and auto clear the register failed instances
// and rewrite config
func (c *clusterConfig) RegisterCMD() {
	var needRemove []ClusterType
	for _, config := range *c {
		if err := config.register(config.Name); err != nil {
			fmt.Println(err.Error())
			needRemove = append(needRemove, config.Name)
		}
	}
	for _, name := range needRemove {
		c.RemoveConfig(name)
	}
	if err := c.WriteConfigs(); err != nil {
		fmt.Printf("Warning: auto clear kbcli cluster chart config failed %s", err.Error())
	}
}

// TypeInstance reference to a cluster type instance in config
type TypeInstance struct {
	Name  ClusterType `yaml:"name"`
	Url   string      `yaml:"helmChartUrl"`
	Alias string      `yaml:"alias"`
}

func (h *TypeInstance) loadChart() (io.ReadCloser, error) {
	return os.Open(filepath.Join(chartsCacheDir, h.getChartFileName()))
}

func (h *TypeInstance) getChartFileName() string {
	return path.Base(h.Url)
}

func (h *TypeInstance) getAlias() string {
	return h.Alias
}

func (h *TypeInstance) register(subcmd ClusterType) error {
	if len(subcmd.String()) == 0 {
		subcmd = h.Name
	}
	if _, ok := ClusterTypeCharts[subcmd]; ok {
		return fmt.Errorf("cluster type %s already registered", subcmd)
	}
	ClusterTypeCharts[subcmd] = h

	for _, f := range cacheFiles {
		if f.Name() == h.getChartFileName() {
			return nil
		}
	}
	return fmt.Errorf("Can't find the %s in config, please use 'kbcli cluster pull %s --url %s' first\n", h.Name.String(), h.Name.String(), h.Url)
}

var _ chartConfigInterface = &TypeInstance{}

func init() {
	err := GlobalClusterChartConfig.ReadConfigs()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// check charts cache dir
	homeDir, _ := util.GetCliHomeDir()
	chartsCacheDir = filepath.Join(homeDir, types.CliChartsCache)
	dirFS := os.DirFS(homeDir)
	cacheFiles, err = fs.ReadDir(dirFS, "charts")
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(chartsCacheDir, 0777)
			if err != nil {
				fmt.Printf("Failed to create charts cache dir %s: %s", chartsCacheDir, err.Error())
				return
			}
			cacheFiles = []fs.DirEntry{}
		} else {
			fmt.Printf("Failed to read charts cache dir %s: %s", chartsCacheDir, err.Error())
			return
		}
	}
	GlobalClusterChartConfig.RegisterCMD()
}
