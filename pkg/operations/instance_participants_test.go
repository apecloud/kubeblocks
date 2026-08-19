/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

package operations

import (
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func participantStatus(name, template string, desired workloads.InstanceDesiredState) workloads.InstanceStatus {
	return workloads.InstanceStatus{
		PodName:      name,
		TemplateName: ptr.To(template),
		DesiredState: desired,
		CurrentState: workloads.InstanceCurrentStatePresent,
	}
}

func TestRetainedParticipantsPreserveOfflineIdentity(t *testing.T) {
	participants, err := retainedParticipants([]workloads.InstanceStatus{
		participantStatus("demo-a", "a", workloads.InstanceDesiredStateActive),
		participantStatus("demo-offline", "a", workloads.InstanceDesiredStateOffline),
		participantStatus("demo-released", "a", workloads.InstanceDesiredStateReleased),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(participants) != 2 {
		t.Fatalf("expected two retained participants, got %#v", participants)
	}
	if !participants[0].Active || participants[1].Active {
		t.Fatalf("active/offline source states were not preserved: %#v", participants)
	}
}

func TestFreezeTargetParticipantsWaitsForCompleteAuthoritativeAllocation(t *testing.T) {
	component := &appsv1.ClusterComponentSpec{
		Replicas:  2,
		Instances: []appsv1.InstanceTemplate{{Name: "a", Replicas: ptr.To[int32](1)}},
	}
	compStatus := &opsv1alpha1.OpsRequestComponentStatus{
		InstanceParticipants: []opsv1alpha1.InstanceParticipantSnapshot{{
			WorkloadName:     "cluster-comp",
			WorkloadUID:      types.UID("uid-1"),
			SourceGeneration: 3,
			Source: []opsv1alpha1.InstanceParticipant{
				{PodName: "cluster-comp-a-0", TemplateName: ptr.To("a"), Active: true},
			},
		}},
	}
	workload := &defaultWorkload{
		name:              "cluster-comp",
		uid:               types.UID("uid-1"),
		generation:        3,
		hasInstanceStatus: true,
		instanceStatuses: []workloads.InstanceStatus{
			participantStatus("cluster-comp-a-0", "a", workloads.InstanceDesiredStateActive),
			participantStatus("cluster-comp-0", "", workloads.InstanceDesiredStateActive),
		},
	}
	if _, frozen, err := freezeTargetParticipants(compStatus, workload, component, nil, false); err != nil || frozen {
		t.Fatalf("same-generation status must not freeze: frozen=%v err=%v", frozen, err)
	}

	workload.generation = 4
	workload.instanceStatuses[1].TemplateName = ptr.To("a")
	if _, frozen, err := freezeTargetParticipants(compStatus, workload, component, nil, false); err != nil || frozen {
		t.Fatalf("wrong template allocation must not freeze: frozen=%v err=%v", frozen, err)
	}

	workload.instanceStatuses[1].TemplateName = ptr.To("")
	snapshot, frozen, err := freezeTargetParticipants(compStatus, workload, component, nil, false)
	if err != nil || !frozen {
		t.Fatalf("complete target allocation did not freeze: frozen=%v err=%v", frozen, err)
	}
	if len(snapshot.Created) != 1 || snapshot.Created[0].PodName != "cluster-comp-0" {
		t.Fatalf("unexpected created participants: %#v", snapshot.Created)
	}

	workload.generation = 5
	workload.instanceStatuses = nil
	sameSnapshot, frozen, err := freezeTargetParticipants(compStatus, workload, component, sets.New[string]("a"), true)
	if err != nil || !frozen || len(sameSnapshot.Created) != 1 || len(sameSnapshot.Updated) != 0 {
		t.Fatalf("a frozen snapshot was recomputed: %#v frozen=%v err=%v", sameSnapshot, frozen, err)
	}
}

func TestFreezeDeletedParticipantsExcludesOfflineSource(t *testing.T) {
	opsRes := &OpsResource{OpsRequest: &opsv1alpha1.OpsRequest{}}
	opsRes.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{
		"comp": {
			InstanceParticipants: []opsv1alpha1.InstanceParticipantSnapshot{{
				SourceGeneration: 7,
				Source: []opsv1alpha1.InstanceParticipant{
					{PodName: "active", Active: true},
					{PodName: "offline"},
				},
			}},
		},
	}
	freezeDeletedSourceParticipants(opsRes, newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "comp"}}))
	snapshot := opsRes.OpsRequest.Status.Components["comp"].InstanceParticipants[0]
	if !snapshot.Frozen || len(snapshot.Deleted) != 1 || snapshot.Deleted[0].PodName != "active" {
		t.Fatalf("unexpected stop participant snapshot: %#v", snapshot)
	}
}

func TestFreezeCreatedParticipantsUsesAllocatedActiveIdentities(t *testing.T) {
	opsRes := &OpsResource{OpsRequest: &opsv1alpha1.OpsRequest{}}
	opsRes.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{
		"comp": {
			InstanceParticipants: []opsv1alpha1.InstanceParticipantSnapshot{{
				SourceGeneration: 7,
				Source: []opsv1alpha1.InstanceParticipant{
					{PodName: "active-absent", Active: true},
					{PodName: "offline"},
				},
			}},
		},
	}
	freezeCreatedSourceParticipants(opsRes, newComponentOpsHelper([]opsv1alpha1.ComponentOps{{ComponentName: "comp"}}))
	snapshot := opsRes.OpsRequest.Status.Components["comp"].InstanceParticipants[0]
	if !snapshot.Frozen || len(snapshot.Created) != 1 || snapshot.Created[0].PodName != "active-absent" {
		t.Fatalf("unexpected start participant snapshot: %#v", snapshot)
	}
}
