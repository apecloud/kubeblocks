package cluster

import (
	"fmt"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/downloader"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

type externalConfig struct {
	Instances []clusterTypes `yaml:"clusterTypes"`
}

var chartsCacheDir string
var cacheFiles []fs.DirEntry

var chartsDownloaders *downloader.ChartDownloader

// clusterTypes gets the helm chart from the url and
type clusterTypes struct {
	Name  ClusterType `yaml:"name"`
	Url   string      `yaml:"helmChartUrl"`
	Alias string      `yaml:"alias"`

	fileName string
}

func (h *clusterTypes) getCMD() ClusterType {
	return h.Name
}

func (h *clusterTypes) loadChart() (io.ReadCloser, error) {
	return os.Open(filepath.Join(chartsCacheDir, h.Name.String()))
}

func (h *clusterTypes) getChartFileName() string {
	panic("imp")
}

func (h *clusterTypes) getAlias() string {
	return h.Alias
}

func (h *clusterTypes) register(subcmd ClusterType) error {
	if _, ok := ClusterTypeCharts[subcmd]; ok {
		panic(fmt.Sprintf("cluster type %s already registered", subcmd))
	}
	ClusterTypeCharts[subcmd] = h

	for _, f := range cacheFiles {
		if f.Name() == h.getChartFileName() {
			return nil
		}
	}

	_, _, err := chartsDownloaders.DownloadTo(h.Url, "", chartsCacheDir)
	if err != nil {
		return err
	}
	//fmt.Println(to)
	return nil
}

var _ chartConfigInterface = &clusterTypes{}

// fixme: there are so many panics in this file in the init function, which will cause the whole program to crash
func initt() {

	var ecc externalConfig
	viper.Unmarshal(&ecc)
	homeDir, err := util.GetCliHomeDir()
	//if err != nil {
	//	fmt.Printf("Failed to get cli home dir")
	//	return
	//}
	////configFile := "external_chart.yaml"
	//data, err := os.ReadFile(filepath.Join(homeDir, types.CliConfigName))
	//if err != nil {
	//	fmt.Printf("Failed to read config file %s: %s", filepath.Join(homeDir, types.CliConfigName), err.Error())
	//	return
	//}

	//if err = yaml.Unmarshal(data, &econfigs); err != nil {
	//	fmt.Printf("Failed to unmarshal config file %s: %s", filepath.Join(homeDir, types.CliConfigName), err.Error())
	//	return
	//}
	chartsCacheDir = filepath.Join(homeDir, types.CliChartsCache)
	dirFS := os.DirFS(homeDir)
	cacheFiles, err = fs.ReadDir(dirFS, "charts")
	//_, err := fs.Stat(dirFS, "charts")
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(chartsCacheDir, 0755)
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
	chartsDownloaders, err = helm.NewDownloader(helm.NewConfig("default", "", "", false))
	if err != nil {
		fmt.Printf("Failed to create downloader: %s", err.Error())
		return
	}
	for _, chart := range ecc.Instances {
		err = chart.register(chart.getCMD())
		if err != nil {
			fmt.Printf("Failed to register chart %s: %s", chart.Name, err.Error())
		}
	}
}
