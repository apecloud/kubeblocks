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
	"bytes"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	pkgcollector "github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	cliutil "github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

const (
	ClusterRegionTitle = "Cluster Region"
	ClusterRegionPath  = "host-collectors/extend/region_name.json"
)

var RestConfigFn = func() (*rest.Config, error) {
	// create rest config of target k8s cluster
	return cmdutil.NewFactory(cmdutil.NewMatchVersionFlags(cliutil.NewConfigFlagNoWarnings())).ToRESTConfig()
}

type ClusterRegionInfo struct {
	RegionName string `json:"regionName"`
}

type CollectClusterRegion struct {
	HostCollector *preflightv1beta2.ClusterRegion
	BundlePath    string
}

func (c *CollectClusterRegion) Title() string {
	return util.TitleOrDefault(c.HostCollector.HostCollectorMeta, ClusterRegionTitle)
}

func (c *CollectClusterRegion) IsExcluded() (bool, error) {
	return util.IsExcluded(c.HostCollector.Exclude)
}

func (c *CollectClusterRegion) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	b, err := doCollect(RestConfigFn, c.HostCollector.ProviderName)
	if err != nil {
		return nil, err
	}
	output := pkgcollector.NewResult()
	_ = output.SaveResult(c.BundlePath, ClusterRegionPath, bytes.NewBuffer(b))
	return output, nil
}

func doCollect(fn func() (*rest.Config, error), providerName string) ([]byte, error) {
	kubeConfig, err := fn()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get kube config")
	}
	regionName := ResolveRegionNameByEndPoint(providerName, kubeConfig.Host)
	info := ClusterRegionInfo{regionName}
	b, err := json.Marshal(info)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal cluster region info")
	}
	return b, nil
}

func ResolveRegionNameByEndPoint(providerName, endPoint string) string {
	var regionName string
	switch {
	case strings.EqualFold(providerName, "eks"):
		if len(endPoint) > 0 {
			// eks endpoint format: https://xxx.yl4.cn-northwest-1.eks.amazonaws.com.cn
			strList := strings.Split(endPoint, ".")
			if len(strList) == 7 {
				regionName = strList[2]
			}
		}
	default:
	}
	return regionName
}
