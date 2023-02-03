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

package troubleshoot

import troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

// ConcatPreflightSpec splices multiple PreflightSpec into one Preflight object
func ConcatPreflightSpec(target *troubleshootv1beta2.Preflight, source *troubleshootv1beta2.Preflight) *troubleshootv1beta2.Preflight {
	if source == nil {
		return target
	}
	var newSpec *troubleshootv1beta2.Preflight
	if target == nil {
		newSpec = source
	} else {
		newSpec = target.DeepCopy()
		newSpec.Spec.Collectors = append(newSpec.Spec.Collectors, source.Spec.Collectors...)
		newSpec.Spec.RemoteCollectors = append(newSpec.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
		newSpec.Spec.Analyzers = append(newSpec.Spec.Analyzers, source.Spec.Analyzers...)
	}
	return newSpec
}

// ConcatHostPreflightSpec splices multiple HostPreflightSpec into one HostPreflight object
func ConcatHostPreflightSpec(target *troubleshootv1beta2.HostPreflight, source *troubleshootv1beta2.HostPreflight) *troubleshootv1beta2.HostPreflight {
	if source == nil {
		return target
	}
	var newSpec *troubleshootv1beta2.HostPreflight
	if target == nil {
		newSpec = source
	} else {
		newSpec = target.DeepCopy()
		newSpec.Spec.Collectors = append(newSpec.Spec.Collectors, source.Spec.Collectors...)
		newSpec.Spec.RemoteCollectors = append(newSpec.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
		newSpec.Spec.Analyzers = append(newSpec.Spec.Analyzers, source.Spec.Analyzers...)
	}
	return newSpec
}
