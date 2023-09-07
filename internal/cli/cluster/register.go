package cluster

import (
	"io"
)

type ClusterType string

func (t ClusterType) String() string {
	return string(t)
}

type chartConfigInterface interface {
	// LoadChart loads the chart from the file system or the url
	loadChart() (io.ReadCloser, error)
	// GetChartFileName returns the chart file name, include the extension
	getChartFileName() string
	// GetAlias returns the chart alias, this alias will be used as the command alias
	getAlias() string
	// register registers the cluster type as a sub	cmd into create cluster command
	register(subcmd ClusterType) error
}

// ClusterTypeCharts is the map of the cluster type and the chart config
// ClusterType is the type of the cluster, the ClusterType t will be used as sub command name,
// chartConfigInterface is the interface for the chart config, implement this interface to register cluster type.
var ClusterTypeCharts = map[ClusterType]chartConfigInterface{}
