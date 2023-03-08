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
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

// ConcatPreflightSpec splices multiple PreflightSpec into one Preflight object
func ConcatPreflightSpec(target *preflightv1beta2.Preflight, source *preflightv1beta2.Preflight) *preflightv1beta2.Preflight {
	if source == nil {
		return target
	}
	var newSpec *preflightv1beta2.Preflight
	if target == nil {
		newSpec = source
	} else {
		newSpec = target.DeepCopy()
		newSpec.Spec.Collectors = append(newSpec.Spec.Collectors, source.Spec.Collectors...)
		newSpec.Spec.RemoteCollectors = append(newSpec.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
		newSpec.Spec.Analyzers = append(newSpec.Spec.Analyzers, source.Spec.Analyzers...)
		newSpec.Spec.ExtendCollectors = append(newSpec.Spec.ExtendCollectors, source.Spec.ExtendCollectors...)
		newSpec.Spec.ExtendAnalyzers = append(newSpec.Spec.ExtendAnalyzers, source.Spec.ExtendAnalyzers...)
	}
	return newSpec
}

// ConcatHostPreflightSpec splices multiple HostPreflightSpec into one HostPreflight object
func ConcatHostPreflightSpec(target *preflightv1beta2.HostPreflight, source *preflightv1beta2.HostPreflight) *preflightv1beta2.HostPreflight {
	if source == nil {
		return target
	}
	var newSpec *preflightv1beta2.HostPreflight
	if target == nil {
		newSpec = source
	} else {
		newSpec = target.DeepCopy()
		newSpec.Spec.Collectors = append(newSpec.Spec.Collectors, source.Spec.Collectors...)
		newSpec.Spec.RemoteCollectors = append(newSpec.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
		newSpec.Spec.Analyzers = append(newSpec.Spec.Analyzers, source.Spec.Analyzers...)
		newSpec.Spec.ExtendCollectors = append(newSpec.Spec.ExtendCollectors, source.Spec.ExtendCollectors...)
		newSpec.Spec.ExtendAnalyzers = append(newSpec.Spec.ExtendAnalyzers, source.Spec.ExtendAnalyzers...)
	}
	return newSpec
}

// ExtractHostPreflightSpec extracts spec of troubleshootv1beta2.HostPreflight from preflightv1beta2.HostPreflight
func ExtractHostPreflightSpec(kb *preflightv1beta2.HostPreflight) *troubleshoot.HostPreflight {
	if kb != nil {
		return &troubleshoot.HostPreflight{
			TypeMeta:   kb.TypeMeta,
			ObjectMeta: kb.ObjectMeta,
			Spec: troubleshoot.HostPreflightSpec{
				Collectors:       kb.Spec.Collectors,
				RemoteCollectors: kb.Spec.RemoteCollectors,
				Analyzers:        kb.Spec.Analyzers,
			},
		}
	}
	return nil
}
