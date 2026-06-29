/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package component

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("workload PVC templates", func() {
	It("preserves PVC dataSourceRef and template annotations", func() {
		apiGroup := "example.kubeblocks.io"
		ref := &corev1.TypedObjectReference{
			APIGroup: &apiGroup,
			Kind:     "ExampleSource",
			Name:     "example-source",
		}
		templateAnnotationKey := "example.kubeblocks.io/template"
		pvcs := toPersistentVolumeClaims(&SynthesizedComponent{
			StaticAnnotations:  map[string]string{"static": "true"},
			DynamicAnnotations: map[string]string{"dynamic": "true"},
		}, []corev1.PersistentVolumeClaimTemplate{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "data",
					Annotations: map[string]string{
						templateAnnotationKey: "data",
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					DataSourceRef: ref,
				},
			},
		})

		Expect(pvcs).Should(HaveLen(1))
		Expect(pvcs[0].Spec.DataSourceRef).Should(Equal(ref))
		Expect(pvcs[0].Annotations[templateAnnotationKey]).Should(Equal("data"))
		Expect(pvcs[0].Annotations["static"]).Should(Equal("true"))
		Expect(pvcs[0].Annotations["dynamic"]).Should(Equal("true"))
	})

	It("propagates dataSourceRef cleanup", func() {
		pvcs := toPersistentVolumeClaims(&SynthesizedComponent{}, []corev1.PersistentVolumeClaimTemplate{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec:       corev1.PersistentVolumeClaimSpec{},
			},
		})

		Expect(pvcs).Should(HaveLen(1))
		Expect(pvcs[0].Spec.DataSourceRef).Should(BeNil())
	})
})

var _ = Describe("workload InstanceSet", func() {
	It("propagates sharding labels to InstanceSet metadata", func() {
		clusterName := "test-cluster"
		compName := "shard-a"
		shardingName := "shard"
		compLabels := constant.GetClusterLabels(clusterName, map[string]string{
			constant.KBAppShardingNameLabelKey: shardingName,
		})
		synthesizedComp := &SynthesizedComponent{
			Namespace:     "default",
			ClusterName:   clusterName,
			Name:          compName,
			CompDefName:   "mongodb",
			Generation:    "1",
			Labels:        compLabels,
			DynamicLabels: map[string]string{"dynamic-label": "true"},
			Replicas:      1,
			PodSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mongodb"}},
			},
		}

		its, err := BuildInstanceSet(synthesizedComp, nil)

		Expect(err).Should(Succeed())
		Expect(its.Labels).Should(HaveKeyWithValue(constant.KBAppShardingNameLabelKey, shardingName))
		Expect(its.Labels).Should(HaveKeyWithValue("dynamic-label", "true"))
		Expect(its.Spec.Selector.MatchLabels).Should(HaveKeyWithValue(constant.KBAppShardingNameLabelKey, shardingName))
		Expect(its.Spec.Template.Labels).Should(HaveKeyWithValue(constant.KBAppShardingNameLabelKey, shardingName))
	})
})

var _ = Describe("workload instance listing", func() {
	const (
		namespace   = "default"
		clusterName = "test-cluster"
		compName    = "mysql"
		placement   = "member-a"
	)

	newScheme := func() *runtime.Scheme {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).Should(Succeed())
		Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
		Expect(workloads.AddToScheme(scheme)).Should(Succeed())
		return scheme
	}

	newComponent := func(annotations map[string]string) *appsv1.Component {
		return &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:        FullName(clusterName, compName),
				Namespace:   namespace,
				Labels:      constant.GetCompLabels(clusterName, compName),
				Annotations: annotations,
			},
		}
	}

	newPod := func(name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    constant.GetCompLabels(clusterName, compName),
			},
		}
	}

	newMultiClusterClient := func(scheme *runtime.Scheme, controlObjects []client.Object, workerObjects []client.Object) client.Client {
		control := fake.NewClientBuilder().WithScheme(scheme).WithObjects(controlObjects...).Build()
		worker := fake.NewClientBuilder().WithScheme(scheme).WithObjects(workerObjects...).Build()
		return multicluster.NewClient(control, map[string]client.Client{placement: worker})
	}

	It("uses Component placement when listing instance pods from member clusters", func() {
		scheme := newScheme()
		comp := newComponent(map[string]string{constant.KBAppMultiClusterPlacementKey: placement})
		pod := newPod("test-cluster-mysql-0")
		cli := newMultiClusterClient(scheme, []client.Object{comp}, []client.Object{pod})

		pods, err := ListOwnedPods(context.Background(), cli, namespace, clusterName, compName)
		Expect(err).Should(Succeed())
		Expect(pods).Should(BeEmpty())

		pods, err = ListOwnedInstances(context.Background(), cli, comp)
		Expect(err).Should(Succeed())
		Expect(pods).Should(HaveLen(1))
		Expect(pods[0].Name).Should(Equal(pod.Name))
	})

	It("falls back to running InstanceSet placement when Component has no placement", func() {
		scheme := newScheme()
		comp := newComponent(nil)
		runningITS := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:        FullName(clusterName, compName),
				Namespace:   namespace,
				Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: placement},
			},
		}
		pod := newPod("test-cluster-mysql-0")
		cli := newMultiClusterClient(scheme, []client.Object{comp, runningITS}, []client.Object{pod})

		pods, err := ListOwnedPods(context.Background(), cli, namespace, clusterName, compName)
		Expect(err).Should(Succeed())
		Expect(pods).Should(BeEmpty())

		pods, err = ListOwnedInstances(context.Background(), cli, comp, runningITS)
		Expect(err).Should(Succeed())
		Expect(pods).Should(HaveLen(1))
		Expect(pods[0].Name).Should(Equal(pod.Name))
	})
})

var _ = Describe("workload resource defaults", func() {
	AfterEach(func() {
		viper.Set(constant.CfgKeyClusterDefaultResources, "")
	})

	newInstanceSet := func() *workloads.InstanceSet {
		return &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "main"},
							{Name: "sidecar"},
						},
						InitContainers: []corev1.Container{
							{Name: "init"},
						},
					},
				},
			},
		}
	}

	It("should not inject zero resources when cluster default resources are not configured", func() {
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		Expect(its.Spec.Template.Spec.Containers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Requests).Should(BeNil())
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Limits).Should(BeNil())
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Requests).Should(BeNil())
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Limits).Should(BeNil())
	})

	It("should keep zero resource limit behavior when zero is true", func() {
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{},"limits":{}}`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		Expect(its.Spec.Template.Spec.Containers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
	})

	It("should leave init and sidecar resources empty when zero is false and no defaults are configured", func() {
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":false,"requests":{},"limits":{}}`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		Expect(its.Spec.Template.Spec.Containers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Requests).Should(BeNil())
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Limits).Should(BeNil())
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Requests).Should(BeNil())
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Limits).Should(BeNil())
	})

	It("should apply configured resources to init and sidecar containers", func() {
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{"cpu":"10m","memory":"16Mi"},"limits":{"cpu":"100m","memory":"64Mi"}}`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		main := its.Spec.Template.Spec.Containers[0]
		sidecar := its.Spec.Template.Spec.Containers[1]
		initContainer := its.Spec.Template.Spec.InitContainers[0]
		Expect(main.Resources.Requests).Should(BeNil())
		Expect(main.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("16Mi")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("64Mi")))
		Expect(initContainer.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(initContainer.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("16Mi")))
		Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
		Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("64Mi")))
	})

	It("should let configured resource names override zero by resource name", func() {
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{"cpu":"10m"},"limits":{}}`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		sidecar := its.Spec.Template.Spec.Containers[1]
		initContainer := its.Spec.Template.Spec.InitContainers[0]
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(sidecar.Resources.Requests).ShouldNot(HaveKey(corev1.ResourceMemory))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
		Expect(initContainer.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
	})

	It("should not override sidecar resource values already set by definitions", func() {
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{"cpu":"10m","memory":"16Mi"},"limits":{"cpu":"100m","memory":"64Mi"}}`)
		its := newInstanceSet()
		its.Spec.Template.Spec.Containers[1].Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("250m"),
		}

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		sidecar := its.Spec.Template.Spec.Containers[1]
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")))
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("16Mi")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("64Mi")))
	})

	It("should return an error when cluster default resources are invalid", func() {
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).ShouldNot(Succeed())
	})
})
