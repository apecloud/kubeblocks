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
	"context"
	"fmt"

	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"helm.sh/helm/v3/pkg/cli/values"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

type KBAnalyzer interface {
	Title() string
	IsExcluded() (bool, error)
	Analyze(getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) ([]*analyze.AnalyzeResult, error)
}

type GetCollectedFileContents func(string) ([]byte, error)
type GetChildCollectedFileContents func(string, []string) (map[string][]byte, error)

func GetAnalyzer(analyzer *preflightv1beta2.ExtendAnalyze, options *values.Options) (KBAnalyzer, bool) {
	switch {
	case analyzer.ClusterAccess != nil:
		return &AnalyzeClusterAccess{analyzer: analyzer.ClusterAccess}, true
	case analyzer.StorageClass != nil:
		return &AnalyzeStorageClassByKb{analyzer: analyzer.StorageClass}, true
	case analyzer.Taint != nil:
		return &AnalyzeTaintClassByKb{analyzer: analyzer.Taint, HelmOpts: options}, true
	default:
		return nil, false
	}
}

func KBAnalyze(ctx context.Context, kbAnalyzer *preflightv1beta2.ExtendAnalyze, getFile func(string) ([]byte, error), findFiles func(string, []string) (map[string][]byte, error), options *values.Options) []*analyze.AnalyzeResult {
	analyzer, ok := GetAnalyzer(kbAnalyzer, options)
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
