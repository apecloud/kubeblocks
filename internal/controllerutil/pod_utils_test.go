/*
Copyright 2022.

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
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("tpl template", func() {

	var (
		statefulSet     *appsv1.StatefulSet
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
		test_containers = `
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
          "name": "$(OPENDBAAS_MY_SECRET_NAME)",
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
		statefulSet.ObjectMeta.Name = "pod_test"
		statefulSet.ObjectMeta.Namespace = "pod_test_ns"

		container := v1.Container{}
		if err := json.Unmarshal([]byte(test_containers), &container); err != nil {
			Fail("convert container failed!")
		}

		container2 := container.DeepCopy()
		container2.Name = "mysql2"
		container2.VolumeMounts[1].Name = container2.VolumeMounts[1].Name + "123"
		container3 := container.DeepCopy()
		container3.Name = "mysql3"
		container3.VolumeMounts[0].Name = container3.VolumeMounts[0].Name + "123"

		statefulSet.Spec.Template.Spec.Containers = []v1.Container{
			*container2, *container3, container}

		// init container
		initContainer := container.DeepCopy()
		initContainer.Name = "init_mysql"
		initContainer2 := container.DeepCopy()
		initContainer2.Name = "init_mysql_2"
		initContainer3 := container.DeepCopy()
		initContainer3.Name = "init_mysql_3"
		initContainer.VolumeMounts[0].Name = initContainer.VolumeMounts[0].Name + "_init_container"
		initContainer.VolumeMounts[1].Name = initContainer.VolumeMounts[1].Name + "_init_container"
		statefulSet.Spec.Template.Spec.InitContainers = []v1.Container{
			*initContainer, *initContainer2, *initContainer3}

	})

	// for test GetContainerUsingConfig
	Context("GetContainerUsingConfig test", func() {
		// found name: mysql3
		It("Should success with no error", func() {
			Expect(GetContainerUsingConfig(statefulSet, configTemplates)).To(Equal(&statefulSet.Spec.Template.Spec.Containers[2]))
		})
		// found name: init_mysql
		It("Should success with no error", func() {
			Expect(GetContainerUsingConfig(statefulSet, foundInitContainerConfigTemplates)).To(Equal(&statefulSet.Spec.Template.Spec.InitContainers[0]))
		})
		// not found container
		It("Should failed", func() {
			Expect(GetContainerUsingConfig(statefulSet, notFoundConfigTemplates)).To(BeNil(), "get container is nil!")
		})
	})

	// for test GetVolumeMountName
	Context("GetVolumeMountName test", func() {
		It("Should success with no error", func() {
			// TODO add test
		})
		It("Should failed", func() {
			// TODO add test
		})
	})

	// for test GetContainerWithVolumeMount
	Context("GetContainerWithVolumeMount test", func() {
		It("Should success with no error", func() {
			// TODO add test
		})
		It("Should failed", func() {
			// TODO add test
		})
	})

})
