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

package preflight

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	kbanalyzer "github.com/apecloud/kubeblocks/internal/preflight/analyzer"
)

type KBClusterCollectResult struct {
	preflight.ClusterCollectResult
	AnalyzerSpecs   []*troubleshoot.Analyze
	KbAnalyzerSpecs []*preflightv1beta2.ExtendAnalyze
}

type KBHostCollectResult struct {
	preflight.HostCollectResult
	AnalyzerSpecs   []*troubleshoot.HostAnalyze
	KbAnalyzerSpecs []*preflightv1beta2.ExtendHostAnalyze
}

func (c KBClusterCollectResult) Analyze() []*analyze.AnalyzeResult {
	return doAnalyze(c.Context, c.AllCollectedData, c.AnalyzerSpecs, c.KbAnalyzerSpecs, nil, nil)
}

func (c KBHostCollectResult) Analyze() []*analyze.AnalyzeResult {
	return doAnalyze(c.Context, c.AllCollectedData, nil, nil, c.AnalyzerSpecs, c.KbAnalyzerSpecs)
}

func doAnalyze(ctx context.Context,
	allCollectedData map[string][]byte,
	analyzers []*troubleshoot.Analyze,
	kbAnalyzers []*preflightv1beta2.ExtendAnalyze,
	hostAnalyzers []*troubleshoot.HostAnalyze,
	kbhHostAnalyzers []*preflightv1beta2.ExtendHostAnalyze,
) []*analyze.AnalyzeResult {
	getCollectedFileContents := func(fileName string) ([]byte, error) {
		contents, ok := allCollectedData[fileName]
		if !ok {
			return nil, fmt.Errorf("file %s was not collected", fileName)
		}

		return contents, nil
	}
	getChildCollectedFileContents := func(prefix string, excludeFiles []string) (map[string][]byte, error) {
		matching := make(map[string][]byte)
		for k, v := range allCollectedData {
			if strings.HasPrefix(k, prefix) {
				matching[k] = v
			}
		}
		for k, v := range allCollectedData {
			if ok, _ := filepath.Match(prefix, k); ok {
				matching[k] = v
			}
		}
		if len(excludeFiles) > 0 {
			for k := range matching {
				for _, ex := range excludeFiles {
					if ok, _ := filepath.Match(ex, k); ok {
						delete(matching, k)
					}
				}
			}
		}
		if len(matching) == 0 {
			return nil, fmt.Errorf("file not found: %s", prefix)
		}
		return matching, nil
	}
	var analyzeResults []*analyze.AnalyzeResult
	for _, analyzer := range analyzers {
		analyzeResult, _ := analyze.Analyze(ctx, analyzer, getCollectedFileContents, getChildCollectedFileContents)
		if analyzeResult != nil {
			analyzeResults = append(analyzeResults, analyzeResult...)
		}
	}
	for _, kbAnalyzer := range kbAnalyzers {
		analyzeResult := kbanalyzer.KBAnalyze(ctx, kbAnalyzer, getCollectedFileContents, getChildCollectedFileContents)
		analyzeResults = append(analyzeResults, analyzeResult...)
	}
	for _, hostAnalyzer := range hostAnalyzers {
		analyzeResult := analyze.HostAnalyze(ctx, hostAnalyzer, getCollectedFileContents, getChildCollectedFileContents)
		analyzeResults = append(analyzeResults, analyzeResult...)
	}
	for _, kbHostAnalyzer := range kbhHostAnalyzers {
		analyzeResult := kbanalyzer.HostKBAnalyze(ctx, kbHostAnalyzer, getCollectedFileContents, getChildCollectedFileContents)
		analyzeResults = append(analyzeResults, analyzeResult...)
	}
	return analyzeResults
}
