/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package analyzer

import (
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

func GetHostAnalyzer(analyzer *preflightv1beta2.ExtendHostAnalyze) (analyze.HostAnalyzer, bool) {
	switch {
	case analyzer.HostUtility != nil:
		return &AnalyzeHostUtility{analyzer.HostUtility}, true
	case analyzer.ClusterRegion != nil:
		return &AnalyzeClusterRegion{analyzer.ClusterRegion}, true
	default:
		return nil, false
	}
}
