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
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

const (
	MissingOutcomeMessage = "there is a missing outcome message"
	IncorrectOutcomeType  = "there is an incorrect outcome type"
	PassType              = "Pass"
	WarnType              = "Warn"
	FailType              = "Fail"
)

func newAnalyzeResult(title string, resultType string, outcomes []*troubleshoot.Outcome) *analyze.AnalyzeResult {
	for _, outcome := range outcomes {
		if outcome == nil {
			continue
		}
		switch resultType {
		case PassType:
			if outcome.Pass != nil {
				return newPassAnalyzeResult(title, outcome)
			}
		case WarnType:
			if outcome.Warn != nil {
				return newWarnAnalyzeResult(title, outcome)
			}
		case FailType:
			if outcome.Fail != nil {
				return newFailAnalyzeResult(title, outcome)
			}
		default:
			return newFailedResultWithMessage(title, IncorrectOutcomeType)
		}
	}
	return newFailedResultWithMessage(title, MissingOutcomeMessage)
}

func newFailAnalyzeResult(title string, outcome *troubleshoot.Outcome) *analyze.AnalyzeResult {
	return &analyze.AnalyzeResult{
		Title:   title,
		IsFail:  true,
		Message: outcome.Fail.Message,
		URI:     outcome.Fail.URI,
	}
}

func newWarnAnalyzeResult(title string, outcome *troubleshoot.Outcome) *analyze.AnalyzeResult {
	return &analyze.AnalyzeResult{
		Title:   title,
		IsWarn:  true,
		Message: outcome.Warn.Message,
		URI:     outcome.Warn.URI,
	}
}

func newPassAnalyzeResult(title string, outcome *troubleshoot.Outcome) *analyze.AnalyzeResult {
	return &analyze.AnalyzeResult{
		Title:   title,
		IsPass:  true,
		Message: outcome.Pass.Message,
		URI:     outcome.Pass.URI,
	}
}

func newFailedResultWithMessage(title, message string) *analyze.AnalyzeResult {
	return newFailAnalyzeResult(title, &troubleshoot.Outcome{Fail: &troubleshoot.SingleOutcome{Message: message}})
}
