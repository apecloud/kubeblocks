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

package rsm2

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.
const (
	namespace = "foo"
	name      = "bar"

	minReadySeconds = 10
)

var (
	rsm         *workloads.ReplicatedStateMachine
	priorityMap map[string]int
	reconciler  kubebuilderx.Reconciler

	uid = types.UID("rsm-mock-uid")

	selectors = map[string]string{
		constant.AppInstanceLabelKey:    name,
		rsm1.WorkloadsManagedByLabelKey: rsm1.KindReplicatedStateMachine,
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

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "RSM2 Suite")
}

var _ = BeforeSuite(func() {
	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
})
