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
