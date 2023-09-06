package register

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/fs"
	"log"
	"os"
	//"github.com/cavaliercoder/grab"
)

var global externalConfig
var cacheFiles []fs.DirEntry

type externalConfig struct {
	cache      string      `yaml:"cacheDir"`
	helmCharts []helmChart `yaml:"helmCharts"`
}

type helmChart struct {
	name  string      `yaml:"name"`
	url   string      `yaml:"url"`
	cmd   ClusterType `yaml:"cmd"`
	alias string      `yaml:"alias"`
}

func (h *helmChart) LoadChart() (io.ReadCloser, error) {
	//TODO implement me
	panic("implement me")
}

func (h *helmChart) GetName() string {
	return h.name
}

func (h *helmChart) GetAlias() string {
	return h.alias
}

func (h *helmChart) register(subcmd ClusterType) {
	if _, ok := ClusterTypeCharts[subcmd]; ok {
		panic(fmt.Sprintf("cluster type %s already registered", subcmd))
	}
	ClusterTypeCharts[subcmd] = h

	for _, f := range cacheFiles {
		if f.Name() == h.name {
			return
		}
	}
	//grab.Get(h.url, h.name)
}

var _ chartConfigInterface = &helmChart{}

func init() {
	configFile := "./external_chart.yaml"
	data, err := os.ReadFile(configFile)
	if err != nil {
		panic("Failed to read config file")
	}

	if err = yaml.Unmarshal(data, &global); err != nil {
		log.Fatalf("Failed to unmarshal YAML: %v", err)
	}
	dirFS := os.DirFS(global.cache)
	cacheFiles, err = fs.ReadDir(dirFS, "charts")
	for _, chart := range global.helmCharts {
		chart.register(chart.cmd)
	}
}
