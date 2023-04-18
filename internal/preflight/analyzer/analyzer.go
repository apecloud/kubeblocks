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
	"context"
	"fmt"

	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

type KBAnalyzer interface {
	Title() string
	IsExcluded() (bool, error)
	Analyze(getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) ([]*analyze.AnalyzeResult, error)
}

type GetCollectedFileContents func(string) ([]byte, error)
type GetChildCollectedFileContents func(string, []string) (map[string][]byte, error)

func GetAnalyzer(analyzer *preflightv1beta2.ExtendAnalyze) (KBAnalyzer, bool) {
	switch {
	case analyzer.ClusterAccess != nil:
		return &AnalyzeClusterAccess{analyzer: analyzer.ClusterAccess}, true
	case analyzer.StorageClass != nil:
		return &AnalyzeStorageClassByKb{analyzer: analyzer.StorageClass}, true
	default:
		return nil, false
	}
}

func KBAnalyze(ctx context.Context, kbAnalyzer *preflightv1beta2.ExtendAnalyze, getFile func(string) ([]byte, error), findFiles func(string, []string) (map[string][]byte, error)) []*analyze.AnalyzeResult {
	analyzer, ok := GetAnalyzer(kbAnalyzer)
	if !ok {
		return NewAnalyzeResultError(analyzer, errors.New("invalid analyzer"))
	}
	isExcluded, _ := analyzer.IsExcluded()
	if isExcluded {
		// logger.Printf("Excluding %q analyzer", analyzer.Title())
		return nil
	}
	results, err := analyzer.Analyze(getFile, findFiles)
	if err != nil {
		return NewAnalyzeResultError(analyzer, errors.Wrap(err, "analyze"))
	}
	return results
}

func HostKBAnalyze(ctx context.Context, kbHostAnalyzer *preflightv1beta2.ExtendHostAnalyze, getFile func(string) ([]byte, error), findFiles func(string, []string) (map[string][]byte, error)) []*analyze.AnalyzeResult {
	hostAnalyzer, ok := GetHostAnalyzer(kbHostAnalyzer)
	if !ok {
		return analyze.NewAnalyzeResultError(hostAnalyzer, errors.New("invalid host analyzer"))
	}
	isExcluded, _ := hostAnalyzer.IsExcluded()
	if isExcluded {
		// logger.Printf("Excluding %q analyzer", hostAnalyzer.Title())
		return nil
	}
	results, err := hostAnalyzer.Analyze(getFile)
	if err != nil {
		return analyze.NewAnalyzeResultError(hostAnalyzer, errors.Wrap(err, "analyze"))
	}
	return results
}

func NewAnalyzeResultError(analyzer KBAnalyzer, err error) []*analyze.AnalyzeResult {
	if analyzer != nil {
		return []*analyze.AnalyzeResult{{
			IsFail:  true,
			Title:   analyzer.Title(),
			Message: fmt.Sprintf("Analyzer Failed: %v", err),
		}}
	}
	return []*analyze.AnalyzeResult{{
		IsFail:  true,
		Title:   "nil analyzer",
		Message: fmt.Sprintf("Analyzer Failed: %v", err),
	}}
}
