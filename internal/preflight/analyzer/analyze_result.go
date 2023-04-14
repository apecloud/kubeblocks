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

func NewAnalyzeResult(title string, resultType string, outcomes []*troubleshoot.Outcome) *analyze.AnalyzeResult {
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
			return NewFailedResultWithMessage(title, IncorrectOutcomeType)
		}
	}
	return NewFailedResultWithMessage(title, MissingOutcomeMessage)
}

func newFailAnalyzeResult(titile string, outcome *troubleshoot.Outcome) *analyze.AnalyzeResult {
	return &analyze.AnalyzeResult{
		Title:   titile,
		IsFail:  true,
		Message: outcome.Fail.Message,
		URI:     outcome.Fail.URI,
	}
}

func newWarnAnalyzeResult(titile string, outcome *troubleshoot.Outcome) *analyze.AnalyzeResult {
	return &analyze.AnalyzeResult{
		Title:   titile,
		IsPass:  true,
		Message: outcome.Fail.Message,
		URI:     outcome.Fail.URI,
	}
}

func newPassAnalyzeResult(titile string, outcome *troubleshoot.Outcome) *analyze.AnalyzeResult {
	return &analyze.AnalyzeResult{
		Title:   titile,
		IsPass:  true,
		Message: outcome.Fail.Message,
		URI:     outcome.Fail.URI,
	}
}

func NewFailedResultWithMessage(title, message string) *analyze.AnalyzeResult {
	return newFailAnalyzeResult(title, &troubleshoot.Outcome{Fail: &troubleshoot.SingleOutcome{Message: message}})
}
