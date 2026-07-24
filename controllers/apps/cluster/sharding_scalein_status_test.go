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
	"time"

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
			ProtocolVersion:    appsv1.ShardingScaleInResultProtocolV2,
			PlanID:             planID,
			Phase:              phase,
			TopologyFenceToken: "fence-1",
		}
	}

	newLock := func(planID string) *appsv1.TopologyMutationLockStatus {
		return &appsv1.TopologyMutationLockStatus{
			Version:               appsv1.TopologyMutationLockVersionV1,
			FenceToken:            "fence-1",
			ClusterUID:            types.UID("cluster-uid"),
			OwnerKind:             appsv1.TopologyMutationLockOwnerShardingScaleIn,
			OwnerPlanID:           planID,
			State:                 appsv1.TopologyMutationLockStateInstallingAuthority,
			AcquiredAt:            &metav1.Time{Time: time.Unix(1, 0).UTC()},
			AffectedComponentUIDs: []types.UID{"component-1", "component-2"},
		}
	}

	decodePatch := func(data []byte) []map[string]any {
		var operations []map[string]any
		Expect(json.Unmarshal(data, &operations)).Should(Succeed())
		return operations
	}

	It("builds one absent-to-Planned CAS with the topology lock", func() {
		cluster := newCluster(nil)
		next := newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned)
		lock := newLock("plan-1")

		reduced, reducedLock, patch, err := buildInitialShardingScaleInPlanPatch(
			cluster, shardingName, next, lock)

		Expect(err).Should(BeNil())
		Expect(reduced).Should(Equal(next))
		Expect(reduced).ShouldNot(BeIdenticalTo(next))
		Expect(reducedLock).Should(Equal(lock))
		Expect(reducedLock).ShouldNot(BeIdenticalTo(lock))

		operations := decodePatch(patch)
		Expect(operations).Should(HaveLen(4))
		Expect(operations[0]).Should(Equal(map[string]any{
			"op": "test", "path": "/metadata/uid", "value": "cluster-uid",
		}))
		Expect(operations[1]).Should(Equal(map[string]any{
			"op": "test", "path": "/metadata/resourceVersion", "value": "17",
		}))
		Expect(operations[2]["op"]).Should(Equal("add"))
		Expect(operations[2]["path"]).Should(Equal("/status/shardings/redis~1shards/scaleIn"))
		Expect(operations[3]["op"]).Should(Equal("add"))
		Expect(operations[3]["path"]).Should(Equal("/status/topologyMutationLock"))
	})

	It("rejects initial plan creation when any topology lock already exists", func() {
		cluster := newCluster(nil)
		cluster.Status.TopologyMutationLock = newLock("other-plan")

		_, _, _, err := buildInitialShardingScaleInPlanPatch(
			cluster, shardingName, newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned), newLock("plan-1"))

		Expect(errors.Is(err, errShardingScaleInStatusCASConflict)).Should(BeTrue())
	})

	It("rejects standalone initial status creation without the topology lock", func() {
		_, _, err := buildShardingScaleInStatusPatch(newCluster(nil), shardingName,
			shardingScaleInStatusTransition{
				Next: newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned),
			})

		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())
	})

	It("rejects malformed or mismatched initial topology locks", func() {
		cases := []func(*appsv1.TopologyMutationLockStatus){
			func(lock *appsv1.TopologyMutationLockStatus) { lock.Version = "" },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.FenceToken = "other-fence" },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.ClusterUID = "other-cluster" },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.OwnerKind = "" },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.OwnerPlanID = "other-plan" },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.State = appsv1.TopologyMutationLockStateHeld },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.AcquiredAt = nil },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.AffectedComponentUIDs = nil },
			func(lock *appsv1.TopologyMutationLockStatus) {
				lock.AffectedComponentUIDs = []types.UID{"component-2", "component-1"}
			},
			func(lock *appsv1.TopologyMutationLockStatus) {
				lock.AffectedComponentUIDs = []types.UID{"component-1", "component-1"}
			},
		}

		for _, mutate := range cases {
			lock := newLock("plan-1")
			mutate(lock)
			_, _, _, err := buildInitialShardingScaleInPlanPatch(
				newCluster(nil), shardingName,
				newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned), lock)
			Expect(errors.Is(err, errInvalidTopologyMutationLock)).Should(BeTrue())
		}
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

		next := newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)
		next.TopologyFenceToken = "other-fence"
		_, _, err = buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    next,
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		next = newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)
		next.ExternalWriteAuthorized = true
		_, _, err = buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    next,
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		current.ExternalWriteAuthorized = true
		next = newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)
		_, _, err = buildShardingScaleInStatusPatch(newCluster(current), shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          "plan-1",
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    next,
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
		lock := newLock("plan-1")
		Eventually(func() error {
			fresh := &appsv1.Cluster{}
			if err := testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), fresh); err != nil {
				return err
			}
			lock.ClusterUID = fresh.UID
			return patchInitialShardingScaleInPlan(testCtx.Ctx, testCtx.Cli, fresh, "redis", next, lock)
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			readback := &appsv1.Cluster{}
			g.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), readback)).Should(Succeed())
			g.Expect(readback.Status.Shardings["redis"].ScaleIn).Should(Equal(next))
			readLock := readback.Status.TopologyMutationLock.DeepCopy()
			expectedLock := lock.DeepCopy()
			g.Expect(readLock.AcquiredAt.Time.Equal(expectedLock.AcquiredAt.Time)).Should(BeTrue())
			readLock.AcquiredAt = nil
			expectedLock.AcquiredAt = nil
			g.Expect(readLock).Should(Equal(expectedLock))
		}).Should(Succeed())
	})

	It("rejects a stale initial plan without overwriting another plan and lock", func() {
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
			otherLock := newLock("other-plan")
			otherLock.ClusterUID = live.UID
			return patchInitialShardingScaleInPlan(testCtx.Ctx, testCtx.Cli, live, "redis",
				newStatus("other-plan", appsv1.ShardingScaleInPhasePlanned), otherLock)
		}).Should(Succeed())

		lock := newLock("plan-1")
		lock.ClusterUID = stale.UID
		err := patchInitialShardingScaleInPlan(testCtx.Ctx, testCtx.Cli, stale, "redis",
			newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned), lock)
		Expect(err).Should(HaveOccurred())

		readback := &appsv1.Cluster{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), readback)).Should(Succeed())
		Expect(readback.Status.Shardings["redis"].ScaleIn.PlanID).Should(Equal("other-plan"))
		Expect(readback.Status.TopologyMutationLock.OwnerPlanID).Should(Equal("other-plan"))
	})
})
