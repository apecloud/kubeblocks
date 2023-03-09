/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
