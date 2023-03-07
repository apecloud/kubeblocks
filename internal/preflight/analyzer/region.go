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

package analyzer

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	corev1 "k8s.io/api/core/v1"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

const (
	ClusterRegionPath = constants.CLUSTER_RESOURCES_DIR + "/" + constants.CLUSTER_RESOURCES_NODES + ".json"
	RegionLabelName   = "topology.kubernetes.io/region"
)

type AnalyzeClusterRegion struct {
	analyzer *preflightv1beta2.ClusterRegion
}

func (a *AnalyzeClusterRegion) Title() string {
	return util.TitleOrDefault(a.analyzer.AnalyzeMeta, "Cluster Region")
}

func (a *AnalyzeClusterRegion) IsExcluded() (bool, error) {
	return util.IsExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClusterRegion) Analyze(getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) ([]*analyzer.AnalyzeResult, error) {
	collected, err := getFile(ClusterRegionPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of nodes.json")
	}

	var nodes corev1.NodeList
	if err := json.Unmarshal(collected, &nodes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal node list")
	}

	result := &analyzer.AnalyzeResult{
		Title: a.Title(),
	}
	if len(nodes.Items) == 0 {
		result.IsWarn = true
		result.Message = "can't obtain regionName because no compute node in target cluster"
		return []*analyzer.AnalyzeResult{result}, nil
	}

	regionName := nodes.Items[0].Labels[RegionLabelName]
	isMatched := false
	for _, region := range a.analyzer.RegionList {
		if strings.EqualFold(region, regionName) {
			isMatched = true
			break
		}
	}
	for _, outcome := range a.analyzer.Outcomes {
		switch {
		case isMatched && outcome.Pass != nil:
			result.IsPass = true
			result.Message = outcome.Pass.Message + ", current cluster region is " + regionName
			result.URI = outcome.Pass.URI
		case !isMatched && outcome.Fail != nil:
			// just return warning info even if outcome.Fail is set
			result.IsWarn = true
			result.Message = outcome.Fail.Message + ", current cluster region is " + regionName
			result.URI = outcome.Fail.URI
		case !isMatched && outcome.Warn != nil:
			result.IsWarn = true
			result.Message = outcome.Warn.Message + ", current cluster region is " + regionName
			result.URI = outcome.Warn.URI
		default:
		}
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*analyzer.AnalyzeResult{result}, nil
}
