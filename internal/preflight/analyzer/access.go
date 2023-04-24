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

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/collect"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

const ClusterVersionPath = "cluster-info/cluster_version.json"

type AnalyzeClusterAccess struct {
	analyzer *preflightv1beta2.ClusterAccessAnalyze
}

func (a *AnalyzeClusterAccess) Title() string {
	return util.TitleOrDefault(a.analyzer.AnalyzeMeta, "Cluster Access")
}

func (a *AnalyzeClusterAccess) IsExcluded() (bool, error) {
	return util.IsExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClusterAccess) Analyze(getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) ([]*analyze.AnalyzeResult, error) {
	isAccess := true
	collected, err := getFile(ClusterVersionPath)
	if err != nil {
		isAccess = false
	} else {
		if err := json.Unmarshal(collected, &collect.ClusterVersion{}); err != nil {
			isAccess = false
		}
	}

	result := analyze.AnalyzeResult{
		Title: a.Title(),
	}
	for _, outcome := range a.analyzer.Outcomes {
		if outcome.Fail != nil && !isAccess {
			result.IsFail = true
			result.Message = outcome.Fail.Message
			result.URI = outcome.Fail.URI
		} else if outcome.Pass != nil && isAccess {
			result.IsPass = true
			result.Message = outcome.Pass.Message
			result.URI = outcome.Pass.URI
		}
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*analyze.AnalyzeResult{&result}, nil
}
