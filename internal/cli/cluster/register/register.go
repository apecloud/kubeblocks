package register

import (
	"embed"
	"fmt"
	"io"
)

type ClusterType string

func (t ClusterType) String() string {
	return string(t)
}

type chartConfigInterface interface {
	// LoadChart loads the chart from the file system or the url
	LoadChart() (io.ReadCloser, error)
	// GetName returns the chart file name, include the extension
	GetName() string
	// GetAlias returns the chart alias, this alias will be used as the command alias
	GetAlias() string
	// register registers the cluster type as a sub	cmd into parent cmd
	register(subcmd ClusterType)
}

// ClusterTypeCharts is the map of the cluster type and the chart config
// ClusterType is the type of the cluster, the ClusterType t will be used as sub command name,
// chartConfigInterface is the interface for the chart config, implement this interface to register cluster type.
var ClusterTypeCharts = map[ClusterType]chartConfigInterface{}

// embedConfig is the interface for the go embed chart
type embedConfig struct {
	chartFS embed.FS
	// chart file name, include the extension
	name string
	// chart alias, this alias will be used as the command alias
	alias string
}

var _ chartConfigInterface = &embedConfig{}

func (e *embedConfig) register(subcmd ClusterType) {
	if _, ok := ClusterTypeCharts[subcmd]; ok {
		panic(fmt.Sprintf("cluster type %s already registered", subcmd))
	}
	ClusterTypeCharts[subcmd] = e
}

func (e *embedConfig) GetAlias() string {
	return e.alias
}

func (e *embedConfig) LoadChart() (io.ReadCloser, error) {
	return e.chartFS.Open(fmt.Sprintf("charts/%s", e.name))
}

func (e *embedConfig) GetName() string {
	return e.name
}

type urlConfig struct {
	chartUrl string

	// chart alias, this alias will be used as the command alias
	alias string
}
