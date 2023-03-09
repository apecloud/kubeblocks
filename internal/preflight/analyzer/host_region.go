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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	kbcollector "github.com/apecloud/kubeblocks/internal/preflight/collector"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

type AnalyzeClusterRegion struct {
	analyzer *preflightv1beta2.ClusterRegionAnalyze
}

func (a *AnalyzeClusterRegion) Title() string {
	return util.TitleOrDefault(a.analyzer.AnalyzeMeta, "Cluster Region")
}

func (a *AnalyzeClusterRegion) IsExcluded() (bool, error) {
	return util.IsExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClusterRegion) Analyze(getFile func(string) ([]byte, error)) ([]*analyzer.AnalyzeResult, error) {
	collected, err := getFile(kbcollector.ClusterRegionPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of region_name.json")
	}
	regionInfo := kbcollector.ClusterRegionInfo{}
	if err := json.Unmarshal(collected, &regionInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal region info")
	}

	isMatched := false
	regionName := regionInfo.RegionName
	for _, expectedRegionName := range a.analyzer.RegionNames {
		if strings.EqualFold(regionName, expectedRegionName) {
			isMatched = true
			break
		}
	}

	strRegionInfo := fmt.Sprintf(" target cluster region is %s.", regionName)
	result := &analyzer.AnalyzeResult{
		Title: a.Title(),
	}
	for _, outcome := range a.analyzer.Outcomes {
		switch {
		case isMatched && outcome.Pass != nil:
			result.IsPass = true
			result.Message = outcome.Pass.Message + strRegionInfo
			result.URI = outcome.Pass.URI
		case !isMatched && outcome.Warn != nil:
			result.IsWarn = true
			result.Message = outcome.Warn.Message + strRegionInfo
			result.URI = outcome.Warn.URI
		case !isMatched && outcome.Fail != nil:
			// just return warning info even if outcome.Fail is set
			result.IsWarn = true
			result.Message = outcome.Fail.Message + strRegionInfo
			result.URI = outcome.Fail.URI
		default:
		}
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*analyzer.AnalyzeResult{result}, nil
}
