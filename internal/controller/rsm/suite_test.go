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

package rsm

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
	"github.com/apecloud/kubeblocks/internal/testutil/k8s/mocks"
)

var (
	controller *gomock.Controller
	k8sMock    *mocks.MockClient
)

func kindPriority(o client.Object) int {
	switch o.(type) {
	case nil:
		return 0
	case *workloads.ReplicatedStateMachine:
		return 1
	case *apps.StatefulSet:
		return 2
	case *corev1.Service:
		return 3
	case *corev1.ConfigMap:
		return 4
	default:
		return 5
	}
}

func less(v1, v2 graph.Vertex) bool {
	o1, _ := v1.(*model.ObjectVertex)
	o2, _ := v2.(*model.ObjectVertex)
	p1 := kindPriority(o1.Obj)
	p2 := kindPriority(o2.Obj)
	if p1 == p2 {
		// TODO(free6om): compare each field of same kind
		return o1.Obj.GetName() < o2.Obj.GetName()
	}
	return p1 < p2
}

func makePodUpdateReady(newRevision string, pods ...*corev1.Pod) {
	readyCondition := corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	}
	for _, pod := range pods {
		pod.Labels[apps.StatefulSetRevisionLabel] = newRevision
		if pod.Labels[roleLabelKey] == "" {
			pod.Labels[roleLabelKey] = "learner"
		}
		pod.Status.Conditions = append(pod.Status.Conditions, readyCondition)
	}
}

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "ReplicatedStateMachine Suite")
}

var _ = BeforeSuite(func() {
	controller, k8sMock = testutil.SetupK8sMock()
	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
	controller.Finish()
})
