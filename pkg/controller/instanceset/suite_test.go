/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package instanceset

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.
const (
	namespace = "foo"
	name      = "bar"

	minReadySeconds = 10
)

var (
	its         *workloads.InstanceSet
	priorityMap map[string]int
	reconciler  kubebuilderx.Reconciler

	uid = types.UID("its-mock-uid")

	selectors = map[string]string{
		constant.AppInstanceLabelKey:   name,
		rsm.WorkloadsManagedByLabelKey: rsm.KindReplicatedStateMachine,
	}
	roles = []workloads.ReplicaRole{
		{
			Name:       "leader",
			IsLeader:   true,
			CanVote:    true,
			AccessMode: workloads.ReadWriteMode,
		},
		{
			Name:       "follower",
			IsLeader:   false,
			CanVote:    true,
			AccessMode: workloads.ReadonlyMode,
		},
		{
			Name:       "logger",
			IsLeader:   false,
			CanVote:    true,
			AccessMode: workloads.NoneMode,
		},
		{
			Name:       "learner",
			IsLeader:   false,
			CanVote:    false,
			AccessMode: workloads.ReadonlyMode,
		},
	}
	pod = builder.NewPodBuilder("", "").
		AddContainer(corev1.Container{
			Name:  "foo",
			Image: "bar",
			Ports: []corev1.ContainerPort{
				{
					Name:          "my-svc",
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: 12345,
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
		}).GetObject()
	template = corev1.PodTemplateSpec{
		ObjectMeta: pod.ObjectMeta,
		Spec:       pod.Spec,
	}

	volumeClaimTemplates = []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "data",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse("2G"),
					},
				},
			},
		},
	}
)

func init() {
}

func mockCompressedInstanceTemplates(ns, name string) (*corev1.ConfigMap, string, error) {
	instances := []workloads.InstanceTemplate{
		{
			Name:     "foo",
			Replicas: func() *int32 { r := int32(2); return &r }(),
		},
		{
			Name:     "bar0",
			Replicas: func() *int32 { r := int32(1); return &r }(),
			Image:    func() *string { i := "busybox"; return &i }(),
		},
	}
	templateByte, err := json.Marshal(instances)
	if err != nil {
		return nil, "", err
	}
	templateData := writer.EncodeAll(templateByte, nil)
	templateName := fmt.Sprintf("template-ref-%s", name)
	templateObj := builder.NewConfigMapBuilder(ns, templateName).
		SetBinaryData(map[string][]byte{
			templateRefDataKey: templateData,
		}).GetObject()
	templateRef := map[string]string{name: templateName}
	templateRefByte, err := json.Marshal(templateRef)
	if err != nil {
		return nil, "", err
	}
	return templateObj, string(templateRefByte), nil
}

func buildRandomPod() *corev1.Pod {
	randStr := rand.String(8)
	deadline := rand.Int63nRange(0, 1024*1024)
	randInt1 := rand.Int()
	randInt2 := rand.Int()
	return builder.NewPodBuilder(namespace, name).
		AddLabels(randStr, randStr).
		AddAnnotations(randStr, randStr).
		SetActiveDeadlineSeconds(&deadline).
		AddTolerations(corev1.Toleration{
			Key:      randStr,
			Operator: corev1.TolerationOpEqual,
			Value:    randStr,
		}).
		AddInitContainer(corev1.Container{
			Name:  "init-container",
			Image: randStr,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", randInt1)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dm", randInt2)),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", randInt1)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dm", randInt2)),
				},
			}}).
		AddContainer(corev1.Container{
			Name:  "container",
			Image: randStr,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", randInt1)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dm", randInt2)),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", randInt1)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dm", randInt2)),
				},
			}}).
		GetObject()
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "InstanceSet Suite")
}

var _ = BeforeSuite(func() {
	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
})
