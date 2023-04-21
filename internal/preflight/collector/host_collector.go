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

package collector

import (
	"github.com/replicatedhq/troubleshoot/pkg/collect"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

func GetExtendHostCollector(kbCollector *preflightv1beta2.ExtendHostCollect, bundlePath string) (collect.HostCollector, bool) {
	switch {
	case kbCollector.HostUtility != nil:
		return &CollectHostUtility{HostCollector: kbCollector.HostUtility, BundlePath: bundlePath}, true
	case kbCollector.ClusterRegion != nil:
		return &CollectClusterRegion{HostCollector: kbCollector.ClusterRegion, BundlePath: bundlePath}, true
	default:
		return nil, false
	}
}
