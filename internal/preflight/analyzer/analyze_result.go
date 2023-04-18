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
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
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
