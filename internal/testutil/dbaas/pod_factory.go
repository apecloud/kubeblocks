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
	"context"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockPodFactory struct {
	Pod *corev1.Pod
}

func NewPodFactory(namespace, name string) *MockPodFactory {
	return &MockPodFactory{
		Pod: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{},
			},
		},
	}
}

func (factory *MockPodFactory) AddLabels(keysAndValues ...string) *MockPodFactory {
	for k, v := range withMap(keysAndValues...) {
		factory.Pod.Labels[k] = v
	}
	return factory
}

func (factory *MockPodFactory) AddContainer(container corev1.Container) *MockPodFactory {
	containers := &factory.Pod.Spec.Containers
	*containers = append(*containers, container)
	return factory
}

func (factory *MockPodFactory) Create(testCtx *testutil.TestContext) *MockPodFactory {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.Pod)).Should(gomega.Succeed())
	return factory
}

func (factory *MockPodFactory) CreateCli(ctx context.Context, cli client.Client) *MockPodFactory {
	gomega.Expect(cli.Create(ctx, factory.Pod)).Should(gomega.Succeed())
	return factory
}

func (factory *MockPodFactory) GetStatefulSet() *corev1.Pod {
	return factory.Pod
}
