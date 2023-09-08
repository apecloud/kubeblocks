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

package cluster

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// CliClusterChartConfig is $HOME/.kbcli/cluster_types in default
var CliClusterChartConfig string

// CliChartsCacheDir is $HOME/.kbcli/charts in default
var CliChartsCacheDir string

type clusterConfig []*TypeInstance

// GlobalClusterChartConfig is kbcli global cluster chart config reference to CliClusterChartConfig
var GlobalClusterChartConfig clusterConfig
var cacheFiles []fs.DirEntry

// ReadConfigs read the config from configPath
func (c *clusterConfig) ReadConfigs(configPath string) error {
	contents, err := os.ReadFile(configPath)
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

// WriteConfigs write current config into configPath
func (c *clusterConfig) WriteConfigs(configPath string) error {
	newConfig, err := yaml.Marshal(*c)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, newConfig, 0666)
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
func (c *clusterConfig) Len() int {
	return len(*c)
}

// RegisterCMD will register all cluster type instances in the config c and auto clear the register failed instances
// and rewrite config
func RegisterCMD(c clusterConfig, configPath string) {
	var needRemove []ClusterType
	for _, config := range c {
		if err := config.register(config.Name); err != nil {
			fmt.Println(err.Error())
			needRemove = append(needRemove, config.Name)
		}
	}
	for _, name := range needRemove {
		c.RemoveConfig(name)
	}
	if err := c.WriteConfigs(configPath); err != nil {
		fmt.Printf("Warning: auto clear kbcli cluster chart config failed %s\n", err.Error())
	}
}

// TypeInstance reference to a cluster type instance in config
type TypeInstance struct {
	Name  ClusterType `yaml:"name"`
	URL   string      `yaml:"helmChartUrl"`
	Alias string      `yaml:"alias"`
}

func (h *TypeInstance) loadChart() (io.ReadCloser, error) {
	return os.Open(filepath.Join(CliChartsCacheDir, h.getChartFileName()))
}

func (h *TypeInstance) getChartFileName() string {
	return path.Base(h.URL)
}

func (h *TypeInstance) getAlias() string {
	return h.Alias
}

func (h *TypeInstance) register(subcmd ClusterType) error {
	if _, ok := ClusterTypeCharts[subcmd]; ok {
		return fmt.Errorf("cluster type %s already registered", subcmd)
	}
	ClusterTypeCharts[subcmd] = h

	for _, f := range cacheFiles {
		if f.Name() == h.getChartFileName() {
			return nil
		}
	}
	return fmt.Errorf("can't find the %s in cache, please use 'kbcli cluster pull %s --url %s' first", h.Name.String(), h.Name.String(), h.URL)
}

var _ chartConfigInterface = &TypeInstance{}

func init() {
	homeDir, _ := util.GetCliHomeDir()
	CliClusterChartConfig = filepath.Join(homeDir, types.CliClusterTypeConfigs)
	CliChartsCacheDir = filepath.Join(homeDir, types.CliChartsCache)

	err := GlobalClusterChartConfig.ReadConfigs(CliClusterChartConfig)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// check charts cache dir
	dirFS := os.DirFS(homeDir)
	cacheFiles, err = fs.ReadDir(dirFS, "charts")
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(CliChartsCacheDir, 0777)
			if err != nil {
				fmt.Printf("Failed to create charts cache dir %s: %s", CliChartsCacheDir, err.Error())
				return
			}
			cacheFiles = []fs.DirEntry{}
		} else {
			fmt.Printf("Failed to read charts cache dir %s: %s", CliChartsCacheDir, err.Error())
			return
		}
	}
	RegisterCMD(GlobalClusterChartConfig, CliClusterChartConfig)
}
