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

package testing

import (
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	kubefakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	metricsfakeclient "k8s.io/metrics/pkg/client/clientset/versioned/fake"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
)

func FakeClientSet(objects ...runtime.Object) *kubefakeclient.Clientset {
	return kubefakeclient.NewSimpleClientset(objects...)
}

func FakeDynamicClient(objects ...runtime.Object) *dynamicfakeclient.FakeDynamicClient {
	_ = appsv1alpha1.AddToScheme(scheme.Scheme)
	_ = extensionsv1alpha1.AddToScheme(scheme.Scheme)
	_ = dpv1alpha1.AddToScheme(scheme.Scheme)
	return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme, objects...)
}

func FakeMetricsClientSet(objects ...runtime.Object) *metricsfakeclient.Clientset {
	return metricsfakeclient.NewSimpleClientset(objects...)
}
