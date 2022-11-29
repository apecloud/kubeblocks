/*
Copyright ApeCloud Inc.

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

package controllerutil

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type TestResourceUnit struct {
	container        corev1.Container
	expectMemorySize int64
	expectCPU        int
}

var _ = Describe("tpl template", func() {

	var (
		statefulSet     *appsv1.StatefulSet
		pod             *corev1.Pod
		configTemplates = []dbaasv1alpha1.ConfigTemplate{
			{
				Name:       "xxxxx",
				VolumeName: "config1",
			},
			{
				Name:       "xxxxx2",
				VolumeName: "config2",
			},
		}

		foundInitContainerConfigTemplates = []dbaasv1alpha1.ConfigTemplate{
			{
				Name:       "xxxxx",
				VolumeName: "config1_init_container",
			},
			{
				Name:       "xxxxx2",
				VolumeName: "config2_init_container",
			},
		}

		notFoundConfigTemplates = []dbaasv1alpha1.ConfigTemplate{
			{
				Name:       "xxxxx",
				VolumeName: "config1_not_fount",
			},
			{
				Name:       "xxxxx2",
				VolumeName: "config2_not_fount",
			},
		}
	)

	const (
		testContainers = `
{
  "name": "mysql",
  "imagePullPolicy": "IfNotPresent",
  "ports": [
    {
      "containerPort": 3306,
      "protocol": "TCP",
      "name": "mysql"
    },
    {
      "containerPort": 13306,
      "protocol": "TCP",
      "name": "paxos"
    }
  ],
  "volumeMounts": [
    {
      "mountPath": "/data/config",
      "name": "config1"
    },
    {
      "mountPath": "/data/config",
      "name": "config2"
    },
    {
      "mountPath": "/data",
      "name": "data"
    },
    {
      "mountPath": "/log",
      "name": "log"
    }
  ],
  "env": [
    {
      "name": "MYSQL_ROOT_PASSWORD",
      "valueFrom": {
        "secretKeyRef": {
          "name": "$(KB_SECRET_NAME)",
          "key": "password"
        }
      }
    }
  ]
}
`
	)

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		statefulSet = &appsv1.StatefulSet{}
		statefulSet.ObjectMeta.Name = "stateful_test"
		statefulSet.ObjectMeta.Namespace = "stateful_test_ns"

		container := corev1.Container{}
		if err := json.Unmarshal([]byte(testContainers), &container); err != nil {
			Fail("convert container failed!")
		}

		container2 := container.DeepCopy()
		container2.Name = "mysql2"
		container2.VolumeMounts[1].Name += "_not_found"
		container3 := container.DeepCopy()
		container3.Name = "mysql3"
		container3.VolumeMounts[0].Name += "_not_found"

		statefulSet.Spec.Template.Spec.Containers = []corev1.Container{
			*container2, *container3, container}

		// init container
		initContainer := container.DeepCopy()
		initContainer.Name = "init_mysql"
		initContainer2 := container.DeepCopy()
		initContainer2.Name = "init_mysql_2"
		initContainer3 := container.DeepCopy()
		initContainer3.Name = "init_mysql_3"
		initContainer.VolumeMounts[0].Name += "_init_container"
		initContainer.VolumeMounts[1].Name += "_init_container"
		statefulSet.Spec.Template.Spec.InitContainers = []corev1.Container{
			*initContainer, *initContainer2, *initContainer3}

		// init pod
		pod = &corev1.Pod{}
		pod.ObjectMeta.Name = "pod_test"
		pod.ObjectMeta.Namespace = "pod_test_ns"
		pod.Spec.Containers = []corev1.Container{container, *container2, *container3}
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "config1",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "stateful_test-config1"},
					},
				},
			},
			{
				Name: "config2",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "stateful_test-config2"},
					},
				},
			},
		}

	})

	// for test GetContainerUsingConfig
	Context("GetContainerUsingConfig test", func() {
		// found name: mysql3
		It("Should success with no error", func() {
			podSpec := &statefulSet.Spec.Template.Spec
			Expect(GetContainerUsingConfig(podSpec, configTemplates)).To(Equal(&podSpec.Containers[2]))
		})
		// found name: init_mysql
		It("Should success with no error", func() {
			podSpec := &statefulSet.Spec.Template.Spec
			Expect(GetContainerUsingConfig(podSpec, foundInitContainerConfigTemplates)).To(Equal(&podSpec.InitContainers[0]))
		})
		// not found container
		It("Should failed", func() {
			podSpec := &statefulSet.Spec.Template.Spec
			Expect(GetContainerUsingConfig(podSpec, notFoundConfigTemplates)).To(BeNil(), "get container is nil!")
		})
	})

	// for test GetVolumeMountName
	Context("GetPodContainerWithVolumeMount test", func() {
		It("Should success with no error", func() {
			mountedContainers := GetPodContainerWithVolumeMount(pod, "config1")
			Expect(len(mountedContainers)).To(Equal(2))
			Expect(mountedContainers[0].Name).To(Equal("mysql"))
			Expect(mountedContainers[1].Name).To(Equal("mysql2"))

			//
			mountedContainers = GetPodContainerWithVolumeMount(pod, "config2")
			Expect(len(mountedContainers)).To(Equal(2))
			Expect(mountedContainers[0].Name).To(Equal("mysql"))
			Expect(mountedContainers[1].Name).To(Equal("mysql3"))
		})
		It("Should failed", func() {
			Expect(len(GetPodContainerWithVolumeMount(pod, "not_exist_cm"))).To(Equal(0))

			emptyPod := corev1.Pod{}
			emptyPod.ObjectMeta.Name = "empty_test"
			emptyPod.ObjectMeta.Namespace = "empty_test_ns"
			Expect(GetPodContainerWithVolumeMount(&emptyPod, "not_exist_cm")).To(BeNil())

		})
	})

	// for test GetContainerWithVolumeMount
	Context("GetVolumeMountName test", func() {
		It("Should success with no error", func() {
			volume := GetVolumeMountName(pod.Spec.Volumes, "stateful_test-config1")
			Expect(volume).NotTo(BeNil())
			Expect(volume.Name).To(Equal("config1"))

			Expect(GetVolumeMountName(pod.Spec.Volumes, "stateful_test-config1")).To(Equal(&pod.Spec.Volumes[0]))
		})
		It("Should failed", func() {
			Expect(GetVolumeMountName(pod.Spec.Volumes, "not_exist_resource")).To(BeNil())
		})
	})

	// for test MemroySize or CoreNum
	Context("Get Resource test", func() {
		It("Resource exists limit", func() {
			testResources := []TestResourceUnit{
				// memory unit: Gi
				{
					container: corev1.Container{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory:  resource.MustParse("10Gi"),
								corev1.ResourceCPU:     resource.MustParse("6"),
								corev1.ResourceStorage: resource.MustParse("100G"),
							},
						},
					},
					expectMemorySize: 10 * 1024 * 1024 * 1024,
					expectCPU:        6,
				},
				// memory unit: G
				{
					container: corev1.Container{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory:  resource.MustParse("10G"),
								corev1.ResourceCPU:     resource.MustParse("16"),
								corev1.ResourceStorage: resource.MustParse("100G"),
							},
						},
					},
					expectMemorySize: 10 * 1000 * 1000 * 1000,
					expectCPU:        16,
				},
				// memory unit: no
				{
					container: corev1.Container{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory:  resource.MustParse("1024000"),
								corev1.ResourceCPU:     resource.MustParse("26"),
								corev1.ResourceStorage: resource.MustParse("100G"),
							},
						},
					},
					expectMemorySize: 1024000,
					expectCPU:        26,
				},
			}

			for i := range testResources {
				Expect(GetMemorySize(testResources[i].container)).To(BeEquivalentTo(testResources[i].expectMemorySize))
				Expect(GetCoreNum(testResources[i].container)).To(BeEquivalentTo(testResources[i].expectCPU))
			}
		})
		It("Resource not limit", func() {
			container := corev1.Container{}
			Expect(GetMemorySize(container)).To(BeEquivalentTo(0))
			Expect(GetCoreNum(container)).To(BeEquivalentTo(0))
		})
	})

})
