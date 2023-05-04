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
