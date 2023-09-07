package cluster

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/downloader"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var ClusterList []*ClusterTypes

var chartsCacheDir string
var cacheFiles []fs.DirEntry

var chartsDownloaders *downloader.ChartDownloader

// ClusterTypes gets the helm chart from the url and
type ClusterTypes struct {
	Name  ClusterType `yaml:"name"`
	Url   string      `yaml:"helmChartUrl"`
	Alias string      `yaml:"alias"`
}

func (h *ClusterTypes) getCMD() ClusterType {
	return h.Name
}

func (h *ClusterTypes) loadChart() (io.ReadCloser, error) {
	return os.Open(filepath.Join(chartsCacheDir, h.getChartFileName()))
}

func (h *ClusterTypes) getChartFileName() string {
	return path.Base(h.Url)
}

func (h *ClusterTypes) getAlias() string {
	return h.Alias
}

func (h *ClusterTypes) register(subcmd ClusterType) error {
	if _, ok := ClusterTypeCharts[subcmd]; ok {
		panic(fmt.Sprintf("cluster type %s already registered", subcmd))
	}
	ClusterTypeCharts[subcmd] = h

	for _, f := range cacheFiles {
		if f.Name() == h.getChartFileName() {
			return nil
		}
	}

	fmt.Printf("can't find the %s in config, please use 'kbcli cluster pull %s --url %s' first", h.Name.String(), h.Name.String(), h.Url)
	return nil
}

var _ chartConfigInterface = &ClusterTypes{}

func init() {

	homeDir, err := util.GetCliHomeDir()
	contents, err := os.ReadFile(filepath.Join(homeDir, types.CliClusterTypeConfigs))
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf(err.Error())
			return
		}
	}
	err = yaml.Unmarshal(contents, &ClusterList)
	if err != nil {
		fmt.Printf(err.Error())
		return
	}

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
	for _, chart := range ClusterList {
		err = chart.register(chart.getCMD())
		if err != nil {
			fmt.Printf("Failed to register chart %s: %s", chart.Name, err.Error())
		}
	}
}
