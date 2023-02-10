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

package dbaas

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewConfigMap(namespace, name string, options ...any) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{},
	}
	for _, option := range options {
		switch f := option.(type) {
		case func(*corev1.ConfigMap):
			f(cm)
		case func(object client.Object):
			f(cm)
		}
	}
	return cm
}

func SetConfigMapData(key string, value string) func(*corev1.ConfigMap) {
	return func(configMap *corev1.ConfigMap) {
		configMap.Data[key] = value
	}
}
