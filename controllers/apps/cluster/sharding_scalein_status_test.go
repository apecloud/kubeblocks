/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("sharding scale-in status reducer", func() {
	const shardingName = "redis/shards"

	newCluster := func(status *appsv1.ShardingScaleInStatus) *appsv1.Cluster {
		return &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-cluster",
				Namespace:       testCtx.DefaultNamespace,
				UID:             types.UID("cluster-uid"),
				ResourceVersion: "17",
			},
			Status: appsv1.ClusterStatus{
				Shardings: map[string]appsv1.ClusterShardingStatus{
					shardingName: {
						Phase:   appsv1.RunningComponentPhase,
						ScaleIn: status,
					},
				},
			},
		}
	}

	newStatus := func(planID string, phase appsv1.ShardingScaleInPhase) *appsv1.ShardingScaleInStatus {
		return &appsv1.ShardingScaleInStatus{
			ProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
			PlanID:          planID,
			Phase:           phase,
		}
	}

	decodePatch := func(data []byte) []map[string]any {
		var operations []map[string]any
		Expect(json.Unmarshal(data, &operations)).Should(Succeed())
		return operations
	}

	It("builds an absent-to-Planned CAS without invoking an action", func() {
		actionCalls := 0
		cluster := newCluster(nil)
		next := newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned)

		reduced, patch, err := buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{Next: next})

		Expect(err).Should(BeNil())
		Expect(reduced).Should(Equal(next))
		Expect(reduced).ShouldNot(BeIdenticalTo(next))
		Expect(actionCalls).Should(Equal(0))

		operations := decodePatch(patch)
		Expect(operations).Should(HaveLen(3))
		Expect(operations[0]).Should(Equal(map[string]any{
			"op": "test", "path": "/metadata/uid", "value": "cluster-uid",
		}))
		Expect(operations[1]).Should(Equal(map[string]any{
			"op": "test", "path": "/metadata/resourceVersion", "value": "17",
		}))
		Expect(operations[2]["op"]).Should(Equal("add"))
		Expect(operations[2]["path"]).Should(Equal("/status/shardings/redis~1shards/scaleIn"))
	})

	It("tests the exact current plan and phase before advancing", func() {
		current := newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned)
		cluster := newCluster(current)
		next := newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)

		reduced, patch, err := buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    next,
			})

		Expect(err).Should(BeNil())
		Expect(reduced.Phase).Should(Equal(appsv1.ShardingScaleInPhaseDraining))
		operations := decodePatch(patch)
		Expect(operations).Should(HaveLen(6))
		Expect(operations[2]).Should(Equal(map[string]any{
			"op": "test", "path": "/status/shardings/redis~1shards/scaleIn/protocolVersion",
			"value": string(appsv1.ShardingScaleInResultProtocolV2),
		}))
		Expect(operations[3]).Should(Equal(map[string]any{
			"op": "test", "path": "/status/shardings/redis~1shards/scaleIn/planID", "value": "plan-1",
		}))
		Expect(operations[4]).Should(Equal(map[string]any{
			"op": "test", "path": "/status/shardings/redis~1shards/scaleIn/phase",
			"value": string(appsv1.ShardingScaleInPhasePlanned),
		}))
		Expect(operations[5]["op"]).Should(Equal("replace"))
	})

	It("rejects stale expectations and immutable identity drift", func() {
		current := newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned)
		cluster := newCluster(current)

		_, _, err := buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "stale-plan",
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining),
			})
		Expect(errors.Is(err, errShardingScaleInStatusCASConflict)).Should(BeTrue())

		_, _, err = buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    newStatus("plan-2", appsv1.ShardingScaleInPhaseDraining),
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		malformed := newStatus("", appsv1.ShardingScaleInPhasePlanned)
		cluster = newCluster(malformed)
		_, _, err = buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "",
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    newStatus("", appsv1.ShardingScaleInPhaseDraining),
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())
	})

	It("rejects a phase jump outside the transition matrix", func() {
		current := newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned)
		cluster := newCluster(current)

		_, _, err := buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    newStatus("plan-1", appsv1.ShardingScaleInPhaseVerified),
			})

		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())
	})

	It("binds Blocked to its exact source phase and recovery class", func() {
		current := newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)
		cluster := newCluster(current)
		blocked := newStatus("plan-1", appsv1.ShardingScaleInPhaseBlocked)
		blocked.BlockedFrom = appsv1.ShardingScaleInPhaseResetting
		blocked.BlockClass = appsv1.ShardingScaleInBlockClassRecoverable

		_, _, err := buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhaseDraining,
				Next:                    blocked,
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		blocked.BlockedFrom = appsv1.ShardingScaleInPhaseDraining
		_, _, err = buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhaseDraining,
				Next:                    blocked,
			})
		Expect(err).Should(BeNil())

		blockedCluster := newCluster(blocked)
		recovered := newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)
		_, _, err = buildShardingScaleInStatusPatch(blockedCluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhaseBlocked,
				Next:                    recovered,
			})
		Expect(err).Should(BeNil())

		blocked.BlockClass = appsv1.ShardingScaleInBlockClassTerminal
		blockedCluster = newCluster(blocked)
		_, _, err = buildShardingScaleInStatusPatch(blockedCluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhaseBlocked,
				Next:                    recovered,
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())
	})

	It("commits scale-in status from a fresh CAS snapshot", func() {
		cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, "scalein-cas-success", "").
			AddSharding("redis", "", "redis").
			GetObject()
		Expect(testCtx.Cli.Create(testCtx.Ctx, cluster)).Should(Succeed())
		DeferCleanup(func() {
			Expect(testCtx.Cli.Delete(testCtx.Ctx, cluster)).Should(Succeed())
		})

		next := newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned)
		Eventually(func() error {
			fresh := &appsv1.Cluster{}
			if err := testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), fresh); err != nil {
				return err
			}
			return patchShardingScaleInStatus(testCtx.Ctx, testCtx.Cli, fresh, "redis",
				shardingScaleInStatusTransition{Next: next})
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			readback := &appsv1.Cluster{}
			g.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), readback)).Should(Succeed())
			g.Expect(readback.Status.Shardings["redis"].ScaleIn).Should(Equal(next))
		}).Should(Succeed())
	})

	It("rejects a stale API object without overwriting another status writer", func() {
		cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, "scalein-cas", "").
			AddSharding("redis", "", "redis").
			GetObject()
		Expect(testCtx.Cli.Create(testCtx.Ctx, cluster)).Should(Succeed())
		DeferCleanup(func() {
			Expect(testCtx.Cli.Delete(testCtx.Ctx, cluster)).Should(Succeed())
		})

		var stale *appsv1.Cluster
		Eventually(func() error {
			live := &appsv1.Cluster{}
			if err := testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), live); err != nil {
				return err
			}
			stale = live.DeepCopy()
			live.Status.Message = "written-by-another-controller"
			return testCtx.Cli.Status().Update(testCtx.Ctx, live)
		}).Should(Succeed())

		err := patchShardingScaleInStatus(testCtx.Ctx, testCtx.Cli, stale, "redis",
			shardingScaleInStatusTransition{
				Next: newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned),
			})
		Expect(err).Should(HaveOccurred())

		readback := &appsv1.Cluster{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), readback)).Should(Succeed())
		Expect(readback.Status.Shardings["redis"].ScaleIn).Should(BeNil())
	})
})
