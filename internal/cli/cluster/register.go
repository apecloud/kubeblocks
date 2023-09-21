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
	"io"
)

type ClusterType string

func (t ClusterType) String() string {
	return string(t)
}

type chartLoader interface {
	// loadChart loads the chart content during building sub-command
	loadChart() (io.ReadCloser, error)
	// GetChartFileName returns the chart file name, include the extension
	getChartFileName() string
	// getAlias returns the chart alias, this alias will be used as the command alias
	getAlias() string
	// register registers the cluster type as a sub	cmd into create cluster command
	register(subcmd ClusterType) error
}

// ClusterTypeCharts is the map of the cluster type and the chart config
// ClusterType is the type of the cluster, the ClusterType t will be used as sub command name,
// chartLoader is the interface for the chart config, implement this interface to register cluster type.
var ClusterTypeCharts = map[ClusterType]chartLoader{}
