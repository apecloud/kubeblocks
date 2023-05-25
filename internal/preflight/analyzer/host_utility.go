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

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	kbcollector "github.com/apecloud/kubeblocks/internal/preflight/collector"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

type AnalyzeHostUtility struct {
	hostAnalyzer *preflightv1beta2.HostUtilityAnalyze
}

func (a *AnalyzeHostUtility) Title() string {
	return util.TitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Host "+a.hostAnalyzer.CollectorName+" Utility Info")
}

func (a *AnalyzeHostUtility) IsExcluded() (bool, error) {
	return util.IsExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostUtility) utilityPath() string {
	if a.hostAnalyzer.CollectorName != "" {
		return a.hostAnalyzer.CollectorName
	}
	return kbcollector.DefaultHostUtilityPath
}

func (a *AnalyzeHostUtility) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*analyzer.AnalyzeResult, error) {
	fullPath := fmt.Sprintf(kbcollector.UtilityPathFormat, a.utilityPath())
	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get collected file name: %s", fullPath)
	}

	utilityInfo := kbcollector.HostUtilityInfo{}
	if err := json.Unmarshal(collected, &utilityInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal utility info")
	}
	// generate analyzer result
	result := analyzer.AnalyzeResult{
		Title: a.Title(),
	}
	for _, outcome := range a.hostAnalyzer.Outcomes {
		switch {
		case outcome.Pass != nil && len(utilityInfo.Error) == 0 && len(utilityInfo.Path) > 0:
			result.IsPass = true
			result.Message = outcome.Pass.Message + fmt.Sprintf(". Utility %s Path is %s", utilityInfo.Name, utilityInfo.Path)
			result.URI = outcome.Pass.URI
		case outcome.Warn != nil && len(utilityInfo.Error) > 0 && len(utilityInfo.Path) == 0:
			result.IsWarn = true
			result.Message = outcome.Warn.Message
			result.URI = outcome.Warn.URI
		case outcome.Fail != nil && len(utilityInfo.Error) > 0 && len(utilityInfo.Path) == 0:
			// return warning info even if outcome.Fail is set
			result.IsWarn = true
			result.Message = outcome.Fail.Message
			result.URI = outcome.Fail.URI
		default:
		}
	}
	return []*analyzer.AnalyzeResult{&result}, nil
}
