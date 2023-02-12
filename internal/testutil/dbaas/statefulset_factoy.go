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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

type MockStatefulSetFactory struct {
	Sts *appsv1.StatefulSet
}

func NewStatefulSetFactory(namespace, name string, clusterName string, componentName string) *MockStatefulSetFactory {
	return &MockStatefulSetFactory{
		Sts: &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						intctrlutil.AppInstanceLabelKey:  clusterName,
						intctrlutil.AppComponentLabelKey: componentName,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							intctrlutil.AppInstanceLabelKey:  clusterName,
							intctrlutil.AppComponentLabelKey: componentName,
						},
					},
				},
			},
		},
	}
}

func (factory *MockStatefulSetFactory) WithRandomName() *MockStatefulSetFactory {
	key := GetRandomizedKey(factory.Sts.Namespace, factory.Sts.Name)
	factory.Sts.Name = key.Name
	return factory
}

func (factory *MockStatefulSetFactory) AddLabels(keysAndValues ...string) *MockStatefulSetFactory {
	for k, v := range WithMap(keysAndValues...) {
		factory.Sts.Labels[k] = v
	}
	return factory
}

func (factory *MockStatefulSetFactory) AddVolume(volume corev1.Volume) *MockStatefulSetFactory {
	volumes := &factory.Sts.Spec.Template.Spec.Volumes
	*volumes = append(*volumes, volume)
	return factory
}

func (factory *MockStatefulSetFactory) AddConfigmapVolume(volumeName string, configmapName string) *MockStatefulSetFactory {
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configmapName},
			},
		},
	}
	factory.AddVolume(volume)
	return factory
}

func (factory *MockStatefulSetFactory) AddContainer(container corev1.Container) *MockStatefulSetFactory {
	containers := &factory.Sts.Spec.Template.Spec.Containers
	*containers = append(*containers, container)
	return factory
}

func (factory *MockStatefulSetFactory) Create(testCtx *testutil.TestContext) *MockStatefulSetFactory {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.Sts)).Should(gomega.Succeed())
	return factory
}

func (factory *MockStatefulSetFactory) CreateCli(ctx context.Context, cli client.Client) *MockStatefulSetFactory {
	gomega.Expect(cli.Create(ctx, factory.Sts)).Should(gomega.Succeed())
	return factory
}

func (factory *MockStatefulSetFactory) GetStatefulSet() *appsv1.StatefulSet {
	return factory.Sts
}
