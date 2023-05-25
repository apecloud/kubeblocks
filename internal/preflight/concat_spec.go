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
