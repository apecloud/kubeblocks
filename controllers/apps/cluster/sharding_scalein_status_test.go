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
	"bytes"
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

	planIDForSharding := func(planKey, statusShardingName string) string {
		material := newShardingScaleInPlanMaterialFixture()
		material.ShardingName = statusShardingName
		if planKey != "" {
			material.Source.OptionalOpsRequestName = "ops-" + planKey
			material.Source.OptionalOpsRequestUID = types.UID("ops-uid-" + planKey)
		}
		_, id, err := buildShardingScaleInPlanMaterial(material)
		Expect(err).ShouldNot(HaveOccurred())
		return id
	}
	planID := func(planKey string) string {
		return planIDForSharding(planKey, shardingName)
	}

	newStatusForSharding := func(planKey, statusShardingName string,
		phase appsv1.ShardingScaleInPhase,
	) *appsv1.ShardingScaleInStatus {
		material := newShardingScaleInPlanMaterialFixture()
		material.ShardingName = statusShardingName
		if planKey != "" {
			material.Source.OptionalOpsRequestName = "ops-" + planKey
			material.Source.OptionalOpsRequestUID = types.UID("ops-uid-" + planKey)
		}
		canonical, id, err := buildShardingScaleInPlanMaterial(material)
		Expect(err).ShouldNot(HaveOccurred())
		return &appsv1.ShardingScaleInStatus{
			ProtocolVersion:    appsv1.ShardingScaleInResultProtocolV2,
			PlanID:             id,
			Phase:              phase,
			TopologyFenceToken: shardingScaleInTestDigestC,
			PlanMaterial:       canonical,
			Holder: &appsv1.ShardingScaleInHolder{
				Name: canonical.Leaving[0].ComponentName,
				UID:  string(canonical.Leaving[0].ComponentUID),
			},
		}
	}
	newStatus := func(planKey string, phase appsv1.ShardingScaleInPhase) *appsv1.ShardingScaleInStatus {
		return newStatusForSharding(planKey, shardingName, phase)
	}

	newLock := func(planKey string) *appsv1.TopologyMutationLockStatus {
		return &appsv1.TopologyMutationLockStatus{
			Version:               appsv1.TopologyMutationLockVersionV1,
			FenceToken:            shardingScaleInTestDigestC,
			ClusterUID:            types.UID("cluster-uid"),
			OwnerKind:             appsv1.TopologyMutationLockOwnerShardingScaleIn,
			OwnerPlanID:           planID(planKey),
			State:                 appsv1.TopologyMutationLockStateInstallingAuthority,
			AcquiredAt:            &metav1.Time{Time: time.Unix(1, 0).UTC()},
			AffectedComponentUIDs: []types.UID{"component-0", "component-1", "component-2"},
		}
	}

	decodePatch := func(data []byte) []map[string]any {
		var operations []map[string]any
		Expect(json.Unmarshal(data, &operations)).Should(Succeed())
		return operations
	}

	It("derives the initial status, holder, and lock from one PlanMaterial", func() {
		material := newShardingScaleInPlanMaterialFixture()
		acquiredAt := &metav1.Time{Time: time.Unix(11, 0).UTC()}
		nonce := bytes.Repeat([]byte{0x42}, 32)

		status, lock, err := buildInitialShardingScaleInState(material, nonce, acquiredAt)

		Expect(err).ShouldNot(HaveOccurred())
		canonical, expectedPlanID, err := buildShardingScaleInPlanMaterial(material)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(status).Should(Equal(&appsv1.ShardingScaleInStatus{
			ProtocolVersion:    appsv1.ShardingScaleInResultProtocolV2,
			PlanID:             expectedPlanID,
			Phase:              appsv1.ShardingScaleInPhasePlanned,
			TopologyFenceToken: status.TopologyFenceToken,
			PlanMaterial:       canonical,
			Holder: &appsv1.ShardingScaleInHolder{
				Name: "demo-redis-2",
				UID:  "component-2",
			},
		}))
		Expect(isShardingScaleInSHA256(status.TopologyFenceToken)).Should(BeTrue())
		Expect(lock).Should(Equal(&appsv1.TopologyMutationLockStatus{
			Version:               appsv1.TopologyMutationLockVersionV1,
			FenceToken:            status.TopologyFenceToken,
			ClusterUID:            "cluster-uid",
			OwnerKind:             appsv1.TopologyMutationLockOwnerShardingScaleIn,
			OwnerPlanID:           expectedPlanID,
			State:                 appsv1.TopologyMutationLockStateInstallingAuthority,
			AcquiredAt:            acquiredAt,
			AffectedComponentUIDs: []types.UID{"component-0", "component-1", "component-2"},
		}))
		Expect(status.PlanMaterial).ShouldNot(BeIdenticalTo(material))
		Expect(lock.AcquiredAt).ShouldNot(BeIdenticalTo(acquiredAt))

		statusAgain, lockAgain, err := buildInitialShardingScaleInState(material, nonce, acquiredAt)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(statusAgain).Should(Equal(status))
		Expect(lockAgain).Should(Equal(lock))

		changedNonce := bytes.Repeat([]byte{0x24}, 32)
		changedStatus, _, err := buildInitialShardingScaleInState(material, changedNonce, acquiredAt)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(changedStatus.TopologyFenceToken).ShouldNot(Equal(status.TopologyFenceToken))
	})

	It("rejects invalid initial-state inputs before constructing a lock", func() {
		material := newShardingScaleInPlanMaterialFixture()
		acquiredAt := &metav1.Time{Time: time.Unix(11, 0).UTC()}

		_, _, err := buildInitialShardingScaleInState(material, []byte("short"), acquiredAt)
		Expect(errors.Is(err, errInvalidTopologyMutationLock)).Should(BeTrue())

		_, _, err = buildInitialShardingScaleInState(material, bytes.Repeat([]byte{0x42}, 32), nil)
		Expect(errors.Is(err, errInvalidTopologyMutationLock)).Should(BeTrue())
	})

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
		Expect(validateClusterStatusCASPatch(cluster, patch)).Should(Succeed())

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

	It("binds builder PlanMaterial shardingName to the status map key", func() {
		cluster := newCluster(nil)
		acquiredAt := &metav1.Time{Time: time.Unix(11, 0).UTC()}
		nonce := bytes.Repeat([]byte{0x42}, 32)

		mismatchedMaterial := newShardingScaleInPlanMaterialFixture()
		mismatchedStatus, mismatchedLock, err := buildInitialShardingScaleInState(
			mismatchedMaterial, nonce, acquiredAt)
		Expect(err).ShouldNot(HaveOccurred())
		reduced, reducedLock, patch, err := buildInitialShardingScaleInPlanPatch(
			cluster, "different-sharding", mismatchedStatus, mismatchedLock)
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())
		Expect(err.Error()).Should(ContainSubstring(
			`plan material sharding name "redis" must match status sharding key "different-sharding"`))
		Expect(reduced).Should(BeNil())
		Expect(reducedLock).Should(BeNil())
		Expect(patch).Should(BeNil())

		matchedMaterial := newShardingScaleInPlanMaterialFixture()
		matchedMaterial.ShardingName = shardingName
		matchedStatus, matchedLock, err := buildInitialShardingScaleInState(
			matchedMaterial, nonce, acquiredAt)
		Expect(err).ShouldNot(HaveOccurred())
		_, _, patch, err = buildInitialShardingScaleInPlanPatch(
			cluster, shardingName, matchedStatus, matchedLock)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(patch).ShouldNot(BeEmpty())
		Expect(decodePatch(patch)).Should(ContainElement(
			HaveKeyWithValue("path", "/status/shardings/redis~1shards/scaleIn")))
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

	It("rejects an initial holder that was not derived from canonical leaving", func() {
		cluster := newCluster(nil)
		next := newStatus("plan-1", appsv1.ShardingScaleInPhasePlanned)
		next.Holder = &appsv1.ShardingScaleInHolder{
			Name: next.PlanMaterial.Staying[0].ComponentName,
			UID:  string(next.PlanMaterial.Staying[0].ComponentUID),
		}

		_, _, _, err := buildInitialShardingScaleInPlanPatch(
			cluster, shardingName, next, newLock("plan-1"))

		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())
	})

	It("rejects malformed or mismatched initial topology locks", func() {
		cases := []func(*appsv1.TopologyMutationLockStatus){
			func(lock *appsv1.TopologyMutationLockStatus) { lock.Version = "" },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.FenceToken = "other-fence" },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.ClusterUID = "other-cluster" },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.OwnerKind = "" },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.OwnerPlanID = planID("other-plan") },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.State = appsv1.TopologyMutationLockStateHeld },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.AcquiredAt = nil },
			func(lock *appsv1.TopologyMutationLockStatus) { lock.AffectedComponentUIDs = nil },
			func(lock *appsv1.TopologyMutationLockStatus) {
				lock.AffectedComponentUIDs = []types.UID{"component-2", "component-1"}
			},
			func(lock *appsv1.TopologyMutationLockStatus) {
				lock.AffectedComponentUIDs = []types.UID{"component-1", "component-1"}
			},
			func(lock *appsv1.TopologyMutationLockStatus) {
				lock.AffectedComponentUIDs = []types.UID{"component-0", "component-1"}
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
				ExpectedPlanID:          planID("plan-1"),
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
			"op": "test", "path": "/status/shardings/redis~1shards/scaleIn/planID",
			"value": planID("plan-1"),
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
				ExpectedPlanID:          planID("plan-1"),
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    newStatus("plan-2", appsv1.ShardingScaleInPhaseDraining),
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		next := newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)
		next.TopologyFenceToken = "other-fence"
		_, _, err = buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          planID("plan-1"),
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    next,
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		next = newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)
		next.ExternalWriteAuthorized = true
		_, _, err = buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          planID("plan-1"),
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    next,
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		current.ExternalWriteAuthorized = true
		next = newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)
		_, _, err = buildShardingScaleInStatusPatch(newCluster(current), shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          planID("plan-1"),
				ExpectedPhase:           appsv1.ShardingScaleInPhasePlanned,
				Next:                    next,
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		malformed := newStatus("", appsv1.ShardingScaleInPhasePlanned)
		malformed.PlanID = ""
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
				ExpectedPlanID:          planID("plan-1"),
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
				ExpectedPlanID:          planID("plan-1"),
				ExpectedPhase:           appsv1.ShardingScaleInPhaseDraining,
				Next:                    blocked,
			})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		blocked.BlockedFrom = appsv1.ShardingScaleInPhaseDraining
		_, _, err = buildShardingScaleInStatusPatch(cluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          planID("plan-1"),
				ExpectedPhase:           appsv1.ShardingScaleInPhaseDraining,
				Next:                    blocked,
			})
		Expect(err).Should(BeNil())

		blockedCluster := newCluster(blocked)
		recovered := newStatus("plan-1", appsv1.ShardingScaleInPhaseDraining)
		_, _, err = buildShardingScaleInStatusPatch(blockedCluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          planID("plan-1"),
				ExpectedPhase:           appsv1.ShardingScaleInPhaseBlocked,
				Next:                    recovered,
			})
		Expect(err).Should(BeNil())

		blocked.BlockClass = appsv1.ShardingScaleInBlockClassTerminal
		blockedCluster = newCluster(blocked)
		_, _, err = buildShardingScaleInStatusPatch(blockedCluster, shardingName,
			shardingScaleInStatusTransition{
				ExpectedProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
				ExpectedPlanID:          planID("plan-1"),
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

		next := newStatusForSharding("plan-1", "redis", appsv1.ShardingScaleInPhasePlanned)
		lock := newLock("plan-1")
		lock.OwnerPlanID = next.PlanID
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
			otherStatus := newStatusForSharding(
				"other-plan", "redis", appsv1.ShardingScaleInPhasePlanned)
			otherLock := newLock("other-plan")
			otherLock.OwnerPlanID = otherStatus.PlanID
			otherLock.ClusterUID = live.UID
			return patchInitialShardingScaleInPlan(testCtx.Ctx, testCtx.Cli, live, "redis",
				otherStatus, otherLock)
		}).Should(Succeed())

		next := newStatusForSharding("plan-1", "redis", appsv1.ShardingScaleInPhasePlanned)
		lock := newLock("plan-1")
		lock.OwnerPlanID = next.PlanID
		lock.ClusterUID = stale.UID
		err := patchInitialShardingScaleInPlan(testCtx.Ctx, testCtx.Cli, stale, "redis",
			next, lock)
		Expect(err).Should(HaveOccurred())

		readback := &appsv1.Cluster{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), readback)).Should(Succeed())
		Expect(readback.Status.Shardings["redis"].ScaleIn.PlanID).
			Should(Equal(planIDForSharding("other-plan", "redis")))
		Expect(readback.Status.TopologyMutationLock.OwnerPlanID).
			Should(Equal(planIDForSharding("other-plan", "redis")))
	})
})
