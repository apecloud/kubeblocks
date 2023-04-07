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

package builder

import corev1 "k8s.io/api/core/v1"

type PodBuilder struct {
	BaseBuilder[corev1.Pod, *corev1.Pod, PodBuilder]
}

func NewPodBuilder(namespace, name string) *PodBuilder {
	builder := &PodBuilder{}
	builder.init(namespace, name, &corev1.Pod{}, builder)
	return builder
}

func (builder *PodBuilder) SetSpec(spec corev1.PodSpec) *PodBuilder {
	builder.get().Spec = spec
	return builder
}
