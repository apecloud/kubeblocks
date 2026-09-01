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

package cluster

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("syncClusterConditions", func() {
	const (
		clusterName = "test-cluster"
		namespace   = "default"
	)

	var (
		transformer clusterStatusTransformer
		cluster     *appsv1.Cluster
		reader      *appsutil.MockReader
	)

	newComponent := func(name string, available *metav1.ConditionStatus, shardingName string) *appsv1.Component {
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      clusterName + "-" + name,
				Labels: map[string]string{
					constant.AppManagedByLabelKey: constant.AppName,
					constant.AppInstanceLabelKey:  clusterName,
				},
			},
			Status: appsv1.ComponentStatus{
				Conditions: []metav1.Condition{
					{
						Type:   appsv1.ComponentConditionHealthy,
						Status: metav1.ConditionTrue,
						Reason: "test",
					},
					{
						Type:   appsv1.ComponentConditionProgressing,
						Status: metav1.ConditionFalse,
						Reason: "test",
					},
				},
			},
		}
		if shardingName != "" {
			comp.Labels[constant.KBAppShardingNameLabelKey] = shardingName
		}
		if available != nil {
			meta.SetStatusCondition(&comp.Status.Conditions, metav1.Condition{
				Type:   appsv1.ConditionTypeAvailable,
				Status: *available,
				Reason: "test",
			})
		}
		return comp
	}

	BeforeEach(func() {
		cluster = &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: namespace,
			},
			Status: appsv1.ClusterStatus{
				Components: map[string]appsv1.ClusterComponentStatus{
					"comp1": {Phase: appsv1.RunningComponentPhase},
				},
			},
		}
		reader = &appsutil.MockReader{Objects: []client.Object{}}
	})

	It("should set Ready condition when phase is Running", func() {
		cluster.Status.Phase = appsv1.RunningClusterPhase
		available := metav1.ConditionTrue
		reader.Objects = []client.Object{newComponent("comp1", &available, "")}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		readyCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeReady)
		Expect(readyCond).ShouldNot(BeNil())
		Expect(readyCond.Status).Should(Equal(metav1.ConditionTrue))
		Expect(readyCond.Reason).Should(Equal(ReasonClusterReady))
	})

	It("should set NotReady condition when components are unhealthy", func() {
		cluster.Status.Components["comp1"] = appsv1.ClusterComponentStatus{Phase: appsv1.FailedComponentPhase}
		cluster.Status.Phase = appsv1.FailedClusterPhase
		available := metav1.ConditionTrue
		comp := newComponent("comp1", &available, "")
		meta.SetStatusCondition(&comp.Status.Conditions, metav1.Condition{
			Type:   appsv1.ComponentConditionHealthy,
			Status: metav1.ConditionFalse,
			Reason: "test",
		})
		reader.Objects = []client.Object{comp}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		readyCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeReady)
		Expect(readyCond).ShouldNot(BeNil())
		Expect(readyCond.Status).Should(Equal(metav1.ConditionFalse))
		Expect(readyCond.Reason).Should(Equal(ReasonComponentsNotReady))
	})

	It("should set NotReady condition when sharding components are unhealthy", func() {
		cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
			"shard1": {Phase: appsv1.FailedComponentPhase},
		}
		cluster.Status.Phase = appsv1.FailedClusterPhase
		available := metav1.ConditionTrue
		shardingComp := newComponent("shard1-0", &available, "shard1")
		meta.SetStatusCondition(&shardingComp.Status.Conditions, metav1.Condition{
			Type:   appsv1.ComponentConditionHealthy,
			Status: metav1.ConditionFalse,
			Reason: "test",
		})
		reader.Objects = []client.Object{
			newComponent("comp1", &available, ""),
			shardingComp,
		}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		readyCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeReady)
		Expect(readyCond).ShouldNot(BeNil())
		Expect(readyCond.Status).Should(Equal(metav1.ConditionFalse))
	})

	It("should remain Ready when restore changes phase but workloads are ready", func() {
		cluster.Status.Components["comp1"] = appsv1.ClusterComponentStatus{Phase: appsv1.FailedComponentPhase}
		cluster.Status.Phase = appsv1.FailedClusterPhase
		available := metav1.ConditionTrue
		reader.Objects = []client.Object{newComponent("comp1", &available, "")}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		readyCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeReady)
		Expect(readyCond).ShouldNot(BeNil())
		Expect(readyCond.Status).Should(Equal(metav1.ConditionTrue))
		availableCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeAvailable)
		Expect(availableCond).ShouldNot(BeNil())
		Expect(availableCond.Status).Should(Equal(metav1.ConditionTrue))
	})

	It("should set Available=True when all components are available", func() {
		cluster.Status.Phase = appsv1.RunningClusterPhase
		available := metav1.ConditionTrue
		reader.Objects = []client.Object{
			newComponent("comp1", &available, ""),
		}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		availCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeAvailable)
		Expect(availCond).ShouldNot(BeNil())
		Expect(availCond.Status).Should(Equal(metav1.ConditionTrue))
		Expect(availCond.Reason).Should(Equal("Available"))
	})

	It("should set Available=False when a component is not available", func() {
		cluster.Status.Phase = appsv1.RunningClusterPhase
		unavailable := metav1.ConditionFalse
		reader.Objects = []client.Object{
			newComponent("comp1", &unavailable, ""),
		}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		availCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeAvailable)
		Expect(availCond).ShouldNot(BeNil())
		Expect(availCond.Status).Should(Equal(metav1.ConditionFalse))
		Expect(availCond.Reason).Should(Equal("Unavailable"))
		Expect(availCond.Message).Should(ContainSubstring("comp1"))
	})

	It("should set Available=False when a component has no available condition", func() {
		cluster.Status.Phase = appsv1.RunningClusterPhase
		reader.Objects = []client.Object{
			newComponent("comp1", nil, ""),
		}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		availCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeAvailable)
		Expect(availCond).ShouldNot(BeNil())
		Expect(availCond.Status).Should(Equal(metav1.ConditionFalse))
		Expect(availCond.Message).Should(ContainSubstring("has no available condition"))
	})

	It("should set Available=False when a sharding component is not available", func() {
		cluster.Status.Phase = appsv1.RunningClusterPhase
		unavailable := metav1.ConditionFalse
		available := metav1.ConditionTrue
		reader.Objects = []client.Object{
			newComponent("comp1", &available, ""),
			newComponent("shard1-0", &unavailable, "shard1"),
		}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		availCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeAvailable)
		Expect(availCond).ShouldNot(BeNil())
		Expect(availCond.Status).Should(Equal(metav1.ConditionFalse))
		Expect(availCond.Message).Should(ContainSubstring("shard1"))
	})

	It("should set Available=True with mixed regular and sharding components all available", func() {
		cluster.Status.Phase = appsv1.RunningClusterPhase
		available := metav1.ConditionTrue
		reader.Objects = []client.Object{
			newComponent("comp1", &available, ""),
			newComponent("shard1-0", &available, "shard1"),
			newComponent("shard1-1", &available, "shard1"),
		}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		availCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeAvailable)
		Expect(availCond).ShouldNot(BeNil())
		Expect(availCond.Status).Should(Equal(metav1.ConditionTrue))
	})

	It("should set Available=False when no components exist", func() {
		cluster.Status.Phase = appsv1.RunningClusterPhase
		reader.Objects = []client.Object{}

		err := transformer.syncClusterConditions(context.Background(), reader, cluster)
		Expect(err).Should(BeNil())

		availCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeAvailable)
		Expect(availCond).ShouldNot(BeNil())
		Expect(availCond.Status).Should(Equal(metav1.ConditionFalse))
	})
})

var _ = DescribeTable("compose cluster phase from restore-projected component phases",
	func(componentPhases []appsv1.ComponentPhase, expected appsv1.ClusterPhase) {
		statuses := make([]appsv1.ClusterComponentStatus, 0, len(componentPhases))
		for _, phase := range componentPhases {
			statuses = append(statuses, appsv1.ClusterComponentStatus{Phase: phase})
		}
		Expect(composeClusterPhase(statuses)).Should(Equal(expected))
	},
	Entry("all components are restoring",
		[]appsv1.ComponentPhase{appsv1.CreatingComponentPhase, appsv1.CreatingComponentPhase},
		appsv1.CreatingClusterPhase),
	Entry("some components are restoring",
		[]appsv1.ComponentPhase{appsv1.CreatingComponentPhase, appsv1.RunningComponentPhase},
		appsv1.UpdatingClusterPhase),
	Entry("all components failed to restore",
		[]appsv1.ComponentPhase{appsv1.FailedComponentPhase, appsv1.FailedComponentPhase},
		appsv1.FailedClusterPhase),
	Entry("some components failed to restore",
		[]appsv1.ComponentPhase{appsv1.FailedComponentPhase, appsv1.RunningComponentPhase},
		appsv1.AbnormalClusterPhase),
)
