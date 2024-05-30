/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import corev1 "k8s.io/api/core/v1"

type MockConfigTemplateFactory struct {
	BaseFactory[corev1.ConfigMap, *corev1.ConfigMap, MockConfigTemplateFactory]
}

func NewConfigTemplateFactory(name, ns string) *MockConfigTemplateFactory {
	f := &MockConfigTemplateFactory{}
	f.Init(ns, name, &corev1.ConfigMap{}, f)
	return f
}

func (f *MockConfigTemplateFactory) Data(data map[string]string) *MockConfigTemplateFactory {
	f.Get().Data = data
	return f
}
