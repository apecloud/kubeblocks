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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

var _ = Describe("component status transformer", func() {
	DescribeTable("role probe timeout requires a current Ready condition", func(readyGeneration int64, readyFor time.Duration, wantFailure bool) {
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 2},
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](1), Roles: []workloads.ReplicaRole{{Name: "leader"}},
			},
			Status: workloads.InstanceSetStatus{
				ObservedGeneration: 2, Replicas: 1, ReadyReplicas: 1, UpdatedReplicas: 1,
				Conditions: []metav1.Condition{{
					Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue,
					ObservedGeneration: readyGeneration, LastTransitionTime: metav1.NewTime(time.Now().Add(-readyFor)), Reason: workloads.ReasonReady,
				}},
			},
		}
		transformer := &componentStatusTransformer{runningITS: its}
		failed, messages := transformer.hasFailedPod()
		Expect(failed).Should(Equal(wantFailure))
		if wantFailure {
			Expect(messages).Should(HaveKeyWithValue("InstanceSet/test", "Role probe timeout, check whether the application is available"))
		} else {
			Expect(messages).Should(BeEmpty())
		}
	},
		Entry("ignores a timeout from an older generation", int64(1), 2*time.Minute, false),
		Entry("reports the current generation's timeout", int64(2), 2*time.Minute, true),
		Entry("waits for the current generation's deadline", int64(2), time.Duration(0), false),
	)
})
