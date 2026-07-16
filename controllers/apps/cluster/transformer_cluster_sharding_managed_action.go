/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project.

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
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/operations"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
)

const (
	managedShardAddMarkerVersion = "v1"

	managedShardAddTokenParameter       = "shardAddToken"
	managedShardAddShardingParameter    = "shardingName"
	managedShardAddMembersParameter     = "newShardComponentNames"
	managedShardAddTargetCountParameter = "targetShardCount"
)

type managedShardAddPlanIdentity struct {
	ClusterUID        string   `json:"clusterUID"`
	ClusterGeneration int64    `json:"clusterGeneration"`
	ShardingName      string   `json:"shardingName"`
	TargetShardCount  int32    `json:"targetShardCount"`
	Members           []string `json:"members"`
}

type managedShardAddMarker struct {
	Version           string   `json:"version"`
	ClusterUID        string   `json:"clusterUID"`
	ClusterGeneration int64    `json:"clusterGeneration"`
	ShardingName      string   `json:"shardingName"`
	Token             string   `json:"token"`
	PlanHash          string   `json:"planHash"`
	TargetShardCount  int32    `json:"targetShardCount"`
	Members           []string `json:"members"`
}

type managedShardAddContractError struct {
	message string
}

func (e *managedShardAddContractError) Error() string {
	return e.message
}

func managedShardAddContractErrorf(format string, args ...any) error {
	return &managedShardAddContractError{message: fmt.Sprintf(format, args...)}
}

func isManagedShardAddContractError(err error) bool {
	var target *managedShardAddContractError
	return errors.As(err, &target)
}

func (c *clusterTransformContext) directReader() client.Reader {
	if c.APIReader != nil {
		return c.APIReader
	}
	return c.Client
}

func (h *clusterShardingHandler) managedShardAddAction(transCtx *clusterTransformContext,
	shardingName string) *appsv1.ShardingAction {
	shardingDef := h.shardingDef(transCtx, shardingName)
	if shardingDef == nil || shardingDef.Spec.LifecycleActions == nil ||
		shardingDef.Spec.LifecycleActions.ShardAdd == nil ||
		shardingDef.Spec.LifecycleActions.ShardAdd.OpsDefinitionName == "" {
		return nil
	}
	return shardingDef.Spec.LifecycleActions.ShardAdd
}

func (h *clusterShardingHandler) getManagedShardAddStatus(transCtx *clusterTransformContext,
	shardingName string) *appsv1.ShardingActionStatus {
	return transCtx.Cluster.Status.Shardings[shardingName].ShardAdd
}

func (h *clusterShardingHandler) setManagedShardAddStatus(transCtx *clusterTransformContext,
	shardingName string, status *appsv1.ShardingActionStatus) {
	if transCtx.Cluster.Status.Shardings == nil {
		transCtx.Cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{}
	}
	shardingStatus := transCtx.Cluster.Status.Shardings[shardingName]
	shardingStatus.ShardAdd = status
	transCtx.Cluster.Status.Shardings[shardingName] = shardingStatus
}

func (h *clusterShardingHandler) targetShardCount(transCtx *clusterTransformContext, shardingName string) (int32, error) {
	for _, shardingSpec := range transCtx.shardings {
		if shardingSpec != nil && shardingSpec.Name == shardingName {
			return shardingSpec.Shards, nil
		}
	}
	return 0, fmt.Errorf("sharding %q is not present in the normalized Cluster spec", shardingName)
}

func buildManagedShardAddPlan(cluster *appsv1.Cluster, shardingName string, targetShardCount int32,
	members []string) (*appsv1.ShardingActionStatus, error) {
	if cluster.UID == "" {
		return nil, fmt.Errorf("cluster %s/%s has no UID", cluster.Namespace, cluster.Name)
	}
	members = slices.Clone(members)
	slices.Sort(members)
	identity := managedShardAddPlanIdentity{
		ClusterUID:        string(cluster.UID),
		ClusterGeneration: cluster.Generation,
		ShardingName:      shardingName,
		TargetShardCount:  targetShardCount,
		Members:           members,
	}
	planHash, err := opsutil.CanonicalObjectHash("kubeblocks.io/managed-shard-add-plan/v1", identity)
	if err != nil {
		return nil, err
	}
	token, err := opsutil.CanonicalObjectHash("kubeblocks.io/managed-shard-add-token/v1", identity)
	if err != nil {
		return nil, err
	}
	memberStatus := make([]appsv1.ShardingActionMemberStatus, len(members))
	for i := range members {
		memberStatus[i].Name = members[i]
	}
	return &appsv1.ShardingActionStatus{
		LifecycleActionStatus: appsv1.LifecycleActionStatus{
			Phase:     appsv1.LifecycleActionPending,
			Reason:    "PlanPersisted",
			Message:   fmt.Sprintf("waiting to create %d planned shard member(s)", len(members)),
			StartTime: ptr.To(metav1.Now()),
		},
		ClusterGeneration: cluster.Generation,
		Token:             token,
		TargetShardCount:  targetShardCount,
		Members:           memberStatus,
		PlanHash:          planHash,
	}, nil
}

func markerForManagedShardAdd(cluster *appsv1.Cluster, shardingName string,
	status *appsv1.ShardingActionStatus) managedShardAddMarker {
	members := make([]string, len(status.Members))
	for i := range status.Members {
		members[i] = status.Members[i].Name
	}
	return managedShardAddMarker{
		Version:           managedShardAddMarkerVersion,
		ClusterUID:        string(cluster.UID),
		ClusterGeneration: status.ClusterGeneration,
		ShardingName:      shardingName,
		Token:             status.Token,
		PlanHash:          status.PlanHash,
		TargetShardCount:  status.TargetShardCount,
		Members:           members,
	}
}

func encodeManagedShardAddMarker(marker managedShardAddMarker) (string, error) {
	data, err := json.Marshal(marker)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeManagedShardAddMarker(value string) (*managedShardAddMarker, error) {
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	marker := &managedShardAddMarker{}
	if err := decoder.Decode(marker); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("managed shard-add marker contains trailing data")
	}
	if marker.Version != managedShardAddMarkerVersion || marker.ClusterUID == "" ||
		marker.ShardingName == "" || marker.Token == "" || marker.PlanHash == "" ||
		marker.TargetShardCount < 0 || len(marker.Members) == 0 {
		return nil, fmt.Errorf("managed shard-add marker is incomplete")
	}
	if !slices.IsSorted(marker.Members) || slices.ContainsFunc(marker.Members, func(name string) bool { return name == "" }) {
		return nil, fmt.Errorf("managed shard-add marker members must be non-empty and sorted")
	}
	for i := 1; i < len(marker.Members); i++ {
		if marker.Members[i] == marker.Members[i-1] {
			return nil, fmt.Errorf("managed shard-add marker contains duplicate members")
		}
	}
	return marker, nil
}

func (h *clusterShardingHandler) failManagedShardAdd(status *appsv1.ShardingActionStatus,
	reason, message string) {
	status.Phase = appsv1.LifecycleActionFailed
	status.Reason = reason
	status.Message = message
	if status.CompletionTime == nil {
		status.CompletionTime = ptr.To(metav1.Now())
	}
}

func (h *clusterShardingHandler) recoverManagedShardAddStatus(transCtx *clusterTransformContext,
	shardingName string, runningComps map[string]*appsv1.Component) (*appsv1.ShardingActionStatus, bool, error) {
	var recovered *managedShardAddMarker
	marked := sets.New[string]()
	for name, comp := range runningComps {
		value := comp.Annotations[shardingAddShardKey]
		if value == "" {
			continue
		}
		marker, err := decodeManagedShardAddMarker(value)
		if err != nil {
			return nil, true, fmt.Errorf("component %q has an unsupported shard-add marker: %w", name, err)
		}
		if recovered == nil {
			recovered = marker
		} else if !reflect.DeepEqual(recovered, marker) {
			return nil, true, fmt.Errorf("managed shard-add markers do not describe one identical plan")
		}
		marked.Insert(name)
	}
	if recovered == nil {
		return nil, false, nil
	}
	if recovered.ClusterUID != string(transCtx.Cluster.UID) || recovered.ShardingName != shardingName {
		return nil, true, fmt.Errorf("managed shard-add marker does not belong to this Cluster and sharding")
	}
	if !marked.Equal(sets.New[string](recovered.Members...)) {
		return nil, true, fmt.Errorf("managed shard-add marker set does not match its frozen member set")
	}
	identity := managedShardAddPlanIdentity{
		ClusterUID:        recovered.ClusterUID,
		ClusterGeneration: recovered.ClusterGeneration,
		ShardingName:      recovered.ShardingName,
		TargetShardCount:  recovered.TargetShardCount,
		Members:           recovered.Members,
	}
	planHash, err := opsutil.CanonicalObjectHash("kubeblocks.io/managed-shard-add-plan/v1", identity)
	if err != nil {
		return nil, true, err
	}
	token, err := opsutil.CanonicalObjectHash("kubeblocks.io/managed-shard-add-token/v1", identity)
	if err != nil {
		return nil, true, err
	}
	if recovered.PlanHash != planHash || recovered.Token != token {
		return nil, true, fmt.Errorf("managed shard-add marker hashes do not match the encoded plan")
	}
	status := &appsv1.ShardingActionStatus{
		LifecycleActionStatus: appsv1.LifecycleActionStatus{
			Phase:     appsv1.LifecycleActionPending,
			Reason:    "PlanRecovered",
			Message:   "recovered only the managed shard-add plan from exact member markers",
			StartTime: ptr.To(metav1.Now()),
		},
		ClusterGeneration: recovered.ClusterGeneration,
		Token:             recovered.Token,
		TargetShardCount:  recovered.TargetShardCount,
		PlanHash:          recovered.PlanHash,
		Members:           make([]appsv1.ShardingActionMemberStatus, len(recovered.Members)),
	}
	for i := range recovered.Members {
		status.Members[i].Name = recovered.Members[i]
	}
	return status, true, nil
}

func (h *clusterShardingHandler) dispatchManagedShardAddMembers(transCtx *clusterTransformContext,
	dag *graph.DAG, shardingName string, status *appsv1.ShardingActionStatus,
	_ map[string]*appsv1.Component, protoComps map[string]*appsv1.Component) error {
	markerValue, err := encodeManagedShardAddMarker(markerForManagedShardAdd(transCtx.Cluster, shardingName, status))
	if err != nil {
		return err
	}
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for _, member := range status.Members {
		live := &appsv1.Component{}
		key := types.NamespacedName{Namespace: transCtx.Cluster.Namespace, Name: member.Name}
		err := transCtx.directReader().Get(transCtx.Context, key, live)
		if err == nil {
			if err := validateManagedShardAddMember(transCtx.Cluster, live, member, markerValue); err != nil {
				return managedShardAddContractErrorf("partially dispatched component %q is not an exact plan match: %v", member.Name, err)
			}
			continue
		}
		if !apierrors.IsNotFound(err) {
			return err
		}
		proto := protoComps[member.Name]
		if proto == nil {
			return fmt.Errorf("planned member %q is no longer present in the desired topology", member.Name)
		}
		if proto.Annotations == nil {
			proto.Annotations = map[string]string{}
		}
		proto.Annotations[shardingAddShardKey] = markerValue
		graphCli.Create(dag, proto)
	}
	status.MembersDispatched = true
	status.Phase = appsv1.LifecycleActionRunning
	status.Reason = "MembersCreating"
	status.Message = fmt.Sprintf("waiting for %d planned shard member(s) to become ready", len(status.Members))
	return nil
}

func validateManagedShardAddMember(cluster *appsv1.Cluster, comp *appsv1.Component,
	member appsv1.ShardingActionMemberStatus, markerValue string) error {
	if !comp.DeletionTimestamp.IsZero() {
		return fmt.Errorf("component %q is terminating", comp.Name)
	}
	owner := metav1.GetControllerOf(comp)
	if owner == nil || owner.APIVersion != appsv1.GroupVersion.String() || owner.Kind != appsv1.ClusterKind ||
		owner.Name != cluster.Name || owner.UID != cluster.UID {
		return fmt.Errorf("component %q is not controlled by the exact Cluster UID", comp.Name)
	}
	if comp.Annotations[shardingAddShardKey] != markerValue {
		return fmt.Errorf("component %q does not carry the exact managed shard-add marker", comp.Name)
	}
	if member.UID != "" && string(comp.UID) != member.UID {
		return fmt.Errorf("component %q UID changed from %s to %s", comp.Name, member.UID, comp.UID)
	}
	if comp.UID == "" {
		return fmt.Errorf("component %q has no UID", comp.Name)
	}
	return nil
}

func (h *clusterShardingHandler) observeManagedShardAddMembers(transCtx *clusterTransformContext,
	shardingName string, status *appsv1.ShardingActionStatus) ([]*appsv1.Component, []string, error) {
	markerValue, err := encodeManagedShardAddMarker(markerForManagedShardAdd(transCtx.Cluster, shardingName, status))
	if err != nil {
		return nil, nil, err
	}
	members := make([]*appsv1.Component, 0, len(status.Members))
	pending := make([]string, 0)
	for _, member := range status.Members {
		comp := &appsv1.Component{}
		key := types.NamespacedName{Namespace: transCtx.Cluster.Namespace, Name: member.Name}
		if err := transCtx.directReader().Get(transCtx.Context, key, comp); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil, managedShardAddContractErrorf("planned component %q is missing after dispatch", member.Name)
			}
			return nil, nil, err
		}
		if err := validateManagedShardAddMember(transCtx.Cluster, comp, member, markerValue); err != nil {
			return nil, nil, managedShardAddContractErrorf("%v", err)
		}
		members = append(members, comp)
		switch {
		case comp.Status.Phase != appsv1.RunningComponentPhase:
			pending = append(pending, fmt.Sprintf("%s phase=%s", comp.Name, comp.Status.Phase))
		case comp.Status.ObservedGeneration != comp.Generation:
			pending = append(pending, fmt.Sprintf("%s observedGeneration=%d generation=%d",
				comp.Name, comp.Status.ObservedGeneration, comp.Generation))
		}
	}
	return members, pending, nil
}

func bindManagedShardAddMembers(status *appsv1.ShardingActionStatus, members []*appsv1.Component) (string, error) {
	byName := make(map[string]*appsv1.Component, len(members))
	for _, comp := range members {
		byName[comp.Name] = comp
	}
	for i := range status.Members {
		comp := byName[status.Members[i].Name]
		if comp == nil {
			return "", fmt.Errorf("planned component %q is absent from the ready snapshot", status.Members[i].Name)
		}
		status.Members[i].UID = string(comp.UID)
		status.Members[i].Generation = comp.Generation
		status.Members[i].ObservedGeneration = comp.Status.ObservedGeneration
	}
	return opsutil.CanonicalObjectHash("kubeblocks.io/managed-shard-add-members/v1", status.Members)
}

func validateManagedShardAddSnapshot(status *appsv1.ShardingActionStatus, members []*appsv1.Component) error {
	if len(members) != len(status.Members) {
		return fmt.Errorf("managed shard-add member count changed")
	}
	observed := make([]appsv1.ShardingActionMemberStatus, len(status.Members))
	byName := make(map[string]*appsv1.Component, len(members))
	for _, comp := range members {
		byName[comp.Name] = comp
	}
	for i, frozen := range status.Members {
		comp := byName[frozen.Name]
		if comp == nil || string(comp.UID) != frozen.UID || comp.Generation != frozen.Generation ||
			comp.Status.ObservedGeneration != frozen.ObservedGeneration {
			return fmt.Errorf("component %q no longer matches its frozen identity", frozen.Name)
		}
		observed[i] = frozen
	}
	hash, err := opsutil.CanonicalObjectHash("kubeblocks.io/managed-shard-add-members/v1", observed)
	if err != nil {
		return err
	}
	if hash != status.MemberSnapshotHash {
		return fmt.Errorf("managed shard-add member snapshot hash changed")
	}
	return nil
}

func readManagedShardAddOpsDefinition(transCtx *clusterTransformContext,
	name string) (*opsv1alpha1.OpsDefinition, string, error) {
	opsDef := &opsv1alpha1.OpsDefinition{}
	if err := transCtx.directReader().Get(transCtx.Context, types.NamespacedName{Name: name}, opsDef); err != nil {
		return nil, "", err
	}
	if !opsDef.DeletionTimestamp.IsZero() {
		return nil, "", managedShardAddContractErrorf("OpsDefinition %q is terminating", opsDef.Name)
	}
	if opsDef.Status.Phase != opsv1alpha1.AvailablePhase || opsDef.Status.ObservedGeneration != opsDef.Generation {
		return nil, "", managedShardAddContractErrorf("OpsDefinition %q is not available at generation %d", opsDef.Name, opsDef.Generation)
	}
	if err := operations.ValidateManagedJobOpsDefinitionSpec(opsDef); err != nil {
		return nil, "", managedShardAddContractErrorf("%v", err)
	}
	hash, err := opsutil.OpsDefinitionSpecHash(opsDef.Spec)
	if err != nil {
		return nil, "", err
	}
	return opsDef, hash, nil
}

func bindOrValidateManagedShardAddOpsDefinition(status *appsv1.ShardingActionStatus,
	opsDef *opsv1alpha1.OpsDefinition, hash string) (bool, error) {
	if status.OpsDefinitionUID == "" {
		status.OpsDefinitionName = opsDef.Name
		status.OpsDefinitionUID = string(opsDef.UID)
		status.OpsDefinitionGeneration = opsDef.Generation
		status.OpsDefinitionSpecHash = hash
		return true, nil
	}
	if status.OpsDefinitionName != opsDef.Name || status.OpsDefinitionUID != string(opsDef.UID) ||
		status.OpsDefinitionGeneration != opsDef.Generation || status.OpsDefinitionSpecHash != hash {
		return false, fmt.Errorf("OpsDefinition %q no longer matches the frozen managed shard-add identity", opsDef.Name)
	}
	return false, nil
}

func buildManagedShardAddOpsRequest(cluster *appsv1.Cluster, shardingName string,
	status *appsv1.ShardingActionStatus) (*opsv1alpha1.OpsRequest, string, error) {
	if status.MemberSnapshotHash == "" || status.OpsDefinitionUID == "" {
		return nil, "", fmt.Errorf("managed shard-add execution snapshot is incomplete")
	}
	if len(status.Token) < 12 {
		return nil, "", fmt.Errorf("managed shard-add token is incomplete")
	}
	members := make([]string, len(status.Members))
	for i := range status.Members {
		members[i] = status.Members[i].Name
	}
	zero := int32(0)
	name := constant.ShortenKubeName(fmt.Sprintf("%s-%s-shardadd-%s", cluster.Name, shardingName, status.Token[:12]),
		constant.KubeNameMaxLength)
	opsRequest := &opsv1alpha1.OpsRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: opsv1alpha1.GroupVersion.String(),
			Kind:       "OpsRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, appsv1.GroupVersion.WithKind(appsv1.ClusterKind)),
			},
		},
		Spec: opsv1alpha1.OpsRequestSpec{
			ClusterName:                 cluster.Name,
			Type:                        opsv1alpha1.CustomType,
			PreConditionDeadlineSeconds: &zero,
			TimeoutSeconds:              &zero,
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
				CustomOps: &opsv1alpha1.CustomOps{
					OpsDefinitionName: status.OpsDefinitionName,
					ExecutionSnapshot: &opsv1alpha1.CustomOpsExecutionSnapshot{
						OpsDefinitionUID:        status.OpsDefinitionUID,
						OpsDefinitionGeneration: status.OpsDefinitionGeneration,
						OpsDefinitionSpecHash:   status.OpsDefinitionSpecHash,
						TargetSnapshotHash:      status.MemberSnapshotHash,
					},
					CustomOpsComponents: []opsv1alpha1.CustomOpsComponent{{
						ComponentOps: opsv1alpha1.ComponentOps{ComponentName: shardingName},
						Parameters: []opsv1alpha1.Parameter{
							{Name: managedShardAddTokenParameter, Value: status.Token},
							{Name: managedShardAddShardingParameter, Value: shardingName},
							{Name: managedShardAddMembersParameter, Value: strings.Join(members, ",")},
							{Name: managedShardAddTargetCountParameter, Value: strconv.FormatInt(int64(status.TargetShardCount), 10)},
						},
					}},
				},
			},
		},
	}
	hash, err := opsutil.OpsRequestSpecHash(opsRequest.Spec)
	if err != nil {
		return nil, "", err
	}
	return opsRequest, hash, nil
}

func validateManagedShardAddOpsRequest(cluster *appsv1.Cluster, status *appsv1.ShardingActionStatus,
	expected *opsv1alpha1.OpsRequest, expectedHash string, live *opsv1alpha1.OpsRequest) error {
	if !live.DeletionTimestamp.IsZero() {
		return fmt.Errorf("OpsRequest %s/%s is terminating", live.Namespace, live.Name)
	}
	owner := metav1.GetControllerOf(live)
	if owner == nil || owner.APIVersion != appsv1.GroupVersion.String() || owner.Kind != appsv1.ClusterKind ||
		owner.Name != cluster.Name || owner.UID != cluster.UID {
		return fmt.Errorf("OpsRequest %s/%s is not controlled by the exact Cluster UID", live.Namespace, live.Name)
	}
	if live.Name != expected.Name || live.Namespace != expected.Namespace || live.UID == "" {
		return fmt.Errorf("OpsRequest identity does not match the managed shard-add plan")
	}
	hash, err := opsutil.OpsRequestSpecHash(live.Spec)
	if err != nil {
		return err
	}
	if hash != expectedHash || hash != status.OpsRequestSpecHash {
		return fmt.Errorf("OpsRequest %s/%s spec no longer matches the managed shard-add plan", live.Namespace, live.Name)
	}
	if status.OpsRequestUID != "" && status.OpsRequestUID != string(live.UID) {
		return fmt.Errorf("OpsRequest %s/%s UID changed from %s to %s", live.Namespace, live.Name,
			status.OpsRequestUID, live.UID)
	}
	return nil
}

func managedJobTask(opsRequest *opsv1alpha1.OpsRequest, shardingName string) (*opsv1alpha1.ActionTask, error) {
	componentStatus, ok := opsRequest.Status.Components[shardingName]
	if !ok {
		return nil, nil
	}
	var tasks []*opsv1alpha1.ActionTask
	for i := range componentStatus.ProgressDetails {
		for j := range componentStatus.ProgressDetails[i].ActionTasks {
			tasks = append(tasks, &componentStatus.ProgressDetails[i].ActionTasks[j])
		}
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	if len(tasks) != 1 {
		return nil, fmt.Errorf("managed shard-add OpsRequest reported %d action tasks, expected exactly one", len(tasks))
	}
	return tasks[0], nil
}

func managedJobName(task *opsv1alpha1.ActionTask) (string, error) {
	kind, name, ok := strings.Cut(task.ObjectKey, "/")
	if !ok || kind != constant.JobKind || name == "" {
		return "", fmt.Errorf("managed shard-add action task has invalid objectKey %q", task.ObjectKey)
	}
	return name, nil
}

func validateManagedShardAddJob(transCtx *clusterTransformContext, status *appsv1.ShardingActionStatus,
	opsRequest *opsv1alpha1.OpsRequest, task *opsv1alpha1.ActionTask) (*batchv1.Job, error) {
	if task.DispatchState != opsv1alpha1.CreatedActionTaskDispatchState || task.WorkloadUID == "" ||
		task.WorkloadSpecHash == "" {
		return nil, managedShardAddContractErrorf("managed shard-add action task is not bound to an exact Job")
	}
	if task.TaskIndex == nil || *task.TaskIndex != 0 || task.Namespace != opsRequest.Namespace {
		return nil, managedShardAddContractErrorf("managed shard-add action task has an invalid index or namespace")
	}
	name, err := managedJobName(task)
	if err != nil {
		return nil, managedShardAddContractErrorf("%v", err)
	}
	job := &batchv1.Job{}
	key := types.NamespacedName{Namespace: task.Namespace, Name: name}
	if err := transCtx.directReader().Get(transCtx.Context, key, job); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, managedShardAddContractErrorf("managed Job %s/%s disappeared", task.Namespace, name)
		}
		return nil, err
	}
	if !job.DeletionTimestamp.IsZero() {
		return nil, managedShardAddContractErrorf("managed Job %s/%s is terminating", job.Namespace, job.Name)
	}
	owner := metav1.GetControllerOf(job)
	if owner == nil || owner.APIVersion != opsv1alpha1.GroupVersion.String() || owner.Kind != "OpsRequest" ||
		owner.Name != opsRequest.Name || owner.UID != opsRequest.UID {
		return nil, managedShardAddContractErrorf("managed Job %s/%s is not controlled by the exact OpsRequest UID", job.Namespace, job.Name)
	}
	if string(job.UID) != task.WorkloadUID {
		return nil, managedShardAddContractErrorf("managed Job %s/%s UID does not match the Operations task", job.Namespace, job.Name)
	}
	hash, err := opsutil.ManagedJobSpecHash(job.Spec)
	if err != nil {
		return nil, err
	}
	if hash != task.WorkloadSpecHash {
		return nil, managedShardAddContractErrorf("managed Job %s/%s spec does not match the Operations task", job.Namespace, job.Name)
	}
	if status.JobUID != "" && (status.JobName != job.Name || status.JobUID != string(job.UID) || status.JobSpecHash != hash) {
		return nil, managedShardAddContractErrorf("managed Job identity no longer matches the frozen Cluster status")
	}
	return job, nil
}

func managedShardAddJobSucceeded(job *batchv1.Job) bool {
	complete := false
	for _, condition := range job.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		switch condition.Type {
		case batchv1.JobFailed:
			return false
		case batchv1.JobComplete:
			complete = true
		}
	}
	return complete
}

func (h *clusterShardingHandler) cleanupManagedShardAddMarkers(transCtx *clusterTransformContext,
	dag *graph.DAG, shardingName string, status *appsv1.ShardingActionStatus) (bool, error) {
	markerValue, err := encodeManagedShardAddMarker(markerForManagedShardAdd(transCtx.Cluster, shardingName, status))
	if err != nil {
		return false, err
	}
	graphCli, _ := transCtx.Client.(model.GraphClient)
	changed := false
	for _, member := range status.Members {
		comp := &appsv1.Component{}
		key := types.NamespacedName{Namespace: transCtx.Cluster.Namespace, Name: member.Name}
		if err := transCtx.directReader().Get(transCtx.Context, key, comp); err != nil {
			if apierrors.IsNotFound(err) {
				return false, managedShardAddContractErrorf("planned component %q disappeared before marker cleanup", member.Name)
			}
			return false, err
		}
		if !comp.DeletionTimestamp.IsZero() {
			return false, managedShardAddContractErrorf("planned component %q is terminating during marker cleanup", member.Name)
		}
		if string(comp.UID) != member.UID {
			return false, managedShardAddContractErrorf("planned component %q was replaced before marker cleanup", member.Name)
		}
		value := comp.Annotations[shardingAddShardKey]
		if value == "" {
			continue
		}
		if value != markerValue {
			return false, managedShardAddContractErrorf("planned component %q marker changed before cleanup", member.Name)
		}
		updated := comp.DeepCopy()
		delete(updated.Annotations, shardingAddShardKey)
		graphCli.Patch(dag, comp, updated)
		changed = true
	}
	return changed, nil
}

func (h *clusterShardingHandler) continueManagedShardAddMarkerCleanup(transCtx *clusterTransformContext,
	dag *graph.DAG, shardingName string, status *appsv1.ShardingActionStatus) (bool, error) {
	if !status.CleanupStarted || status.MarkersCleaned {
		h.failManagedShardAdd(status, "CleanupStateInvalid", "managed shard-add cleanup state is inconsistent")
		return true, nil
	}
	changed, err := h.cleanupManagedShardAddMarkers(transCtx, dag, shardingName, status)
	if err != nil {
		if !isManagedShardAddContractError(err) {
			return true, err
		}
		h.failManagedShardAdd(status, "MarkerCleanupFailed", err.Error())
		return true, nil
	}
	if changed {
		status.Reason = "CleaningMarkers"
		status.Message = "removing managed shard-add markers from the frozen member set"
		return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after cleaning managed shard-add markers")
	}
	if !status.MarkersCleaned {
		status.MarkersCleaned = true
		status.Phase = appsv1.LifecycleActionSucceeded
		status.Reason = "Completed"
		status.Message = "managed shard-add operation and marker cleanup completed"
		status.CompletionTime = ptr.To(metav1.Now())
		return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after closing the managed shard-add action")
	}
	return true, nil
}

func validateManagedShardAddRetryOpsRequest(cluster *appsv1.Cluster, status *appsv1.ShardingActionStatus,
	live *opsv1alpha1.OpsRequest) error {
	owner := metav1.GetControllerOf(live)
	if owner == nil || owner.APIVersion != appsv1.GroupVersion.String() || owner.Kind != appsv1.ClusterKind ||
		owner.Name != cluster.Name || owner.UID != cluster.UID {
		return fmt.Errorf("OpsRequest %s/%s is not controlled by the exact Cluster UID", live.Namespace, live.Name)
	}
	if live.Namespace != cluster.Namespace || live.Name != status.OpsRequestName ||
		string(live.UID) != status.OpsRequestUID {
		return fmt.Errorf("OpsRequest %s/%s no longer matches the failed attempt identity", live.Namespace, live.Name)
	}
	hash, err := opsutil.OpsRequestSpecHash(live.Spec)
	if err != nil {
		return err
	}
	if hash != status.OpsRequestSpecHash {
		return fmt.Errorf("OpsRequest %s/%s spec no longer matches the failed attempt", live.Namespace, live.Name)
	}
	switch live.Status.Phase {
	case opsv1alpha1.OpsFailedPhase, opsv1alpha1.OpsCancelledPhase, opsv1alpha1.OpsAbortedPhase:
		return nil
	default:
		return fmt.Errorf("OpsRequest %s/%s left its unsuccessful terminal phase", live.Namespace, live.Name)
	}
}

func validateManagedShardAddRetryJob(status *appsv1.ShardingActionStatus, job *batchv1.Job) error {
	owner := metav1.GetControllerOf(job)
	if owner == nil || owner.APIVersion != opsv1alpha1.GroupVersion.String() || owner.Kind != "OpsRequest" ||
		owner.Name != status.OpsRequestName || string(owner.UID) != status.OpsRequestUID {
		return fmt.Errorf("managed Job %s/%s is not controlled by the failed OpsRequest UID", job.Namespace, job.Name)
	}
	if job.Name != status.JobName || string(job.UID) != status.JobUID {
		return fmt.Errorf("managed Job %s/%s no longer matches the failed attempt identity", job.Namespace, job.Name)
	}
	hash, err := opsutil.ManagedJobSpecHash(job.Spec)
	if err != nil {
		return err
	}
	if hash != status.JobSpecHash {
		return fmt.Errorf("managed Job %s/%s spec no longer matches the failed attempt", job.Namespace, job.Name)
	}
	return nil
}

func validateManagedShardAddRetryPod(status *appsv1.ShardingActionStatus, pod *corev1.Pod) error {
	if status.JobUID == "" {
		return nil
	}
	owner := metav1.GetControllerOf(pod)
	if owner == nil || owner.APIVersion != batchv1.SchemeGroupVersion.String() || owner.Kind != constant.JobKind ||
		owner.Name != status.JobName || string(owner.UID) != status.JobUID {
		return fmt.Errorf("managed worker Pod %s/%s is not controlled by the failed Job UID", pod.Namespace, pod.Name)
	}
	return nil
}

func (h *clusterShardingHandler) handleFailedManagedShardAdd(transCtx *clusterTransformContext,
	status *appsv1.ShardingActionStatus) (bool, error) {
	if status.Reason != "OpsRequestUnsuccessful" {
		return true, nil
	}
	if status.OpsRequestName == "" || status.OpsRequestUID == "" || status.OpsRequestSpecHash == "" {
		h.failManagedShardAdd(status, "RetryStateInvalid", "failed managed shard-add attempt has no exact OpsRequest identity")
		return true, nil
	}

	opsRequest := &opsv1alpha1.OpsRequest{}
	key := types.NamespacedName{Namespace: transCtx.Cluster.Namespace, Name: status.OpsRequestName}
	err := transCtx.directReader().Get(transCtx.Context, key, opsRequest)
	if err == nil {
		if err := validateManagedShardAddRetryOpsRequest(transCtx.Cluster, status, opsRequest); err != nil {
			h.failManagedShardAdd(status, "RetryIdentityMismatch", err.Error())
			return true, nil
		}
		if opsRequest.DeletionTimestamp.IsZero() {
			status.Message = "managed shard-add failed; preserve evidence, then delete the exact failed OpsRequest to request a retry"
			return true, nil
		}
		status.Message = "waiting for the exact failed OpsRequest to become absent before retry"
		return true, intctrlutil.NewDelayedRequeueError(time.Second, "waiting for failed managed OpsRequest deletion")
	}
	if !apierrors.IsNotFound(err) {
		return true, err
	}

	if status.JobName != "" {
		job := &batchv1.Job{}
		err = transCtx.directReader().Get(transCtx.Context,
			types.NamespacedName{Namespace: transCtx.Cluster.Namespace, Name: status.JobName}, job)
		if err == nil {
			if err := validateManagedShardAddRetryJob(status, job); err != nil {
				h.failManagedShardAdd(status, "RetryIdentityMismatch", err.Error())
				return true, nil
			}
			status.Message = "waiting for the exact failed managed Job to become absent before retry"
			return true, intctrlutil.NewDelayedRequeueError(time.Second, "waiting for failed managed Job deletion")
		}
		if !apierrors.IsNotFound(err) {
			return true, err
		}
	}

	pods := &corev1.PodList{}
	selector := opsutil.ManagedJobSelectorValue(status.OpsRequestUID, 0)
	if err := transCtx.directReader().List(transCtx.Context, pods,
		client.InNamespace(transCtx.Cluster.Namespace),
		client.MatchingLabels{constant.ManagedJobSelectorLabelKey: selector}); err != nil {
		return true, err
	}
	for i := range pods.Items {
		if err := validateManagedShardAddRetryPod(status, &pods.Items[i]); err != nil {
			h.failManagedShardAdd(status, "RetryIdentityMismatch", err.Error())
			return true, nil
		}
	}
	if len(pods.Items) > 0 {
		status.Message = "waiting for every Pod from the failed managed Job to become absent before retry"
		return true, intctrlutil.NewDelayedRequeueError(time.Second, "waiting for failed managed worker Pods")
	}

	status.Phase = appsv1.LifecycleActionRunning
	status.Reason = "RetryPrepared"
	status.Message = "the failed OpsRequest, Job, and worker Pods are absent; the same persisted plan may start a new attempt"
	status.CompletionTime = nil
	status.OpsRequestUID = ""
	status.OpsRequestPhase = ""
	status.JobName = ""
	status.JobUID = ""
	status.JobSpecHash = ""
	return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after preparing an explicit managed shard-add retry")
}

func (h *clusterShardingHandler) handleManagedShardAdd(transCtx *clusterTransformContext, dag *graph.DAG,
	shardingName string, runningComps, protoComps map[string]*appsv1.Component,
	toCreate sets.Set[string]) (bool, error) {
	status := h.getManagedShardAddStatus(transCtx, shardingName)
	action := h.managedShardAddAction(transCtx, shardingName)
	if status == nil {
		if action == nil {
			return false, nil
		}
		recovered, found, err := h.recoverManagedShardAddStatus(transCtx, shardingName, runningComps)
		if err != nil {
			targetCount, _ := h.targetShardCount(transCtx, shardingName)
			status = &appsv1.ShardingActionStatus{
				LifecycleActionStatus: appsv1.LifecycleActionStatus{StartTime: ptr.To(metav1.Now())},
				ClusterGeneration:     transCtx.Cluster.Generation,
				TargetShardCount:      targetCount,
				OpsDefinitionName:     action.OpsDefinitionName,
			}
			h.failManagedShardAdd(status, "MarkerRecoveryFailed", err.Error())
			h.setManagedShardAddStatus(transCtx, shardingName, status)
			return true, nil
		}
		if found {
			status = recovered
			status.OpsDefinitionName = action.OpsDefinitionName
			// Member markers can reconstruct only the plan. They do not carry a write-ahead
			// OpsRequest/Job attempt identity or stage, so continuing could replay an effect
			// whose status was lost.
			h.failManagedShardAdd(status, "AttemptStateLost",
				"member markers recovered the plan, but durable attempt/stage identity is missing; automatic replay is forbidden")
			h.setManagedShardAddStatus(transCtx, shardingName, status)
			return true, nil
		}
		if len(toCreate) == 0 {
			return false, nil
		}
		targetCount, err := h.targetShardCount(transCtx, shardingName)
		if err != nil {
			return true, err
		}
		status, err = buildManagedShardAddPlan(transCtx.Cluster, shardingName, targetCount, sets.List(toCreate))
		if err != nil {
			return true, err
		}
		status.OpsDefinitionName = action.OpsDefinitionName
		h.setManagedShardAddStatus(transCtx, shardingName, status)
		return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after persisting the managed shard-add plan")
	}

	if status.Phase == appsv1.LifecycleActionFailed {
		return h.handleFailedManagedShardAdd(transCtx, status)
	}
	if status.CleanupStarted && !status.MarkersCleaned {
		return h.continueManagedShardAddMarkerCleanup(transCtx, dag, shardingName, status)
	}
	if status.Phase == appsv1.LifecycleActionSucceeded && status.MarkersCleaned {
		if len(toCreate) == 0 {
			return false, nil
		}
		if action == nil {
			targetCount, err := h.targetShardCount(transCtx, shardingName)
			if err != nil {
				return true, err
			}
			nextStatus, err := buildManagedShardAddPlan(transCtx.Cluster, shardingName, targetCount, sets.List(toCreate))
			if err != nil {
				return true, err
			}
			nextStatus.OpsDefinitionName = status.OpsDefinitionName
			h.failManagedShardAdd(nextStatus, "OpsDefinitionChanged",
				"configured managed shard-add action is unavailable for the next shard group")
			h.setManagedShardAddStatus(transCtx, shardingName, nextStatus)
			return true, nil
		}
		targetCount, err := h.targetShardCount(transCtx, shardingName)
		if err != nil {
			return true, err
		}
		status, err = buildManagedShardAddPlan(transCtx.Cluster, shardingName, targetCount, sets.List(toCreate))
		if err != nil {
			return true, err
		}
		status.OpsDefinitionName = action.OpsDefinitionName
		h.setManagedShardAddStatus(transCtx, shardingName, status)
		return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after persisting the next managed shard-add plan")
	}
	if action == nil {
		h.failManagedShardAdd(status, "OpsDefinitionChanged", "configured managed shard-add action was removed before completion")
		return true, nil
	}
	if status.OpsDefinitionName != action.OpsDefinitionName {
		h.failManagedShardAdd(status, "OpsDefinitionChanged",
			fmt.Sprintf("configured OpsDefinition changed from %q to %q", status.OpsDefinitionName, action.OpsDefinitionName))
		return true, nil
	}
	if status.Phase == appsv1.LifecycleActionSucceeded {
		h.failManagedShardAdd(status, "CleanupStateInvalid", "managed shard-add reached Succeeded before durable cleanup completed")
		return true, nil
	}

	if !status.MembersDispatched {
		if err := h.dispatchManagedShardAddMembers(transCtx, dag, shardingName, status, runningComps, protoComps); err != nil {
			h.failManagedShardAdd(status, "MemberDispatchFailed", err.Error())
			return true, nil
		}
		return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after creating the managed shard-add members")
	}

	members, pending, err := h.observeManagedShardAddMembers(transCtx, shardingName, status)
	if err != nil {
		if !isManagedShardAddContractError(err) {
			return true, err
		}
		h.failManagedShardAdd(status, "MemberIdentityMismatch", err.Error())
		return true, nil
	}
	if len(pending) > 0 {
		status.Phase = appsv1.LifecycleActionRunning
		status.Reason = "MembersNotReady"
		status.Message = strings.Join(pending, "; ")
		return true, intctrlutil.NewDelayedRequeueError(3*time.Second, "waiting for managed shard-add members")
	}

	opsDef, opsDefHash, err := readManagedShardAddOpsDefinition(transCtx, action.OpsDefinitionName)
	if err != nil {
		if !apierrors.IsNotFound(err) && !isManagedShardAddContractError(err) {
			return true, err
		}
		h.failManagedShardAdd(status, "OpsDefinitionInvalid", err.Error())
		return true, nil
	}
	snapshotChanged := false
	if status.MemberSnapshotHash == "" {
		status.MemberSnapshotHash, err = bindManagedShardAddMembers(status, members)
		if err != nil {
			h.failManagedShardAdd(status, "MemberSnapshotFailed", err.Error())
			return true, nil
		}
		snapshotChanged = true
	} else if err := validateManagedShardAddSnapshot(status, members); err != nil {
		h.failManagedShardAdd(status, "MemberSnapshotChanged", err.Error())
		return true, nil
	}
	definitionChanged, err := bindOrValidateManagedShardAddOpsDefinition(status, opsDef, opsDefHash)
	if err != nil {
		h.failManagedShardAdd(status, "OpsDefinitionChanged", err.Error())
		return true, nil
	}
	if snapshotChanged || definitionChanged {
		status.Phase = appsv1.LifecycleActionRunning
		status.Reason = "ExecutionSnapshotPersisted"
		status.Message = "persisted exact member and OpsDefinition identities"
		return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after persisting the managed shard-add execution snapshot")
	}

	expectedOpsRequest, expectedOpsRequestHash, err := buildManagedShardAddOpsRequest(transCtx.Cluster, shardingName, status)
	if err != nil {
		h.failManagedShardAdd(status, "OpsRequestPlanFailed", err.Error())
		return true, nil
	}
	if status.OpsRequestName == "" {
		status.OpsRequestName = expectedOpsRequest.Name
		status.OpsRequestSpecHash = expectedOpsRequestHash
		status.Reason = "OpsRequestPlanned"
		status.Message = "persisted the deterministic managed Custom OpsRequest identity"
		return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after persisting the managed OpsRequest plan")
	}
	if status.OpsRequestName != expectedOpsRequest.Name || status.OpsRequestSpecHash != expectedOpsRequestHash {
		h.failManagedShardAdd(status, "OpsRequestPlanChanged", "the deterministic managed OpsRequest inputs changed")
		return true, nil
	}

	liveOpsRequest := &opsv1alpha1.OpsRequest{}
	opsKey := types.NamespacedName{Namespace: expectedOpsRequest.Namespace, Name: expectedOpsRequest.Name}
	err = transCtx.directReader().Get(transCtx.Context, opsKey, liveOpsRequest)
	if status.OpsRequestUID == "" {
		switch {
		case err == nil:
			if err := validateManagedShardAddOpsRequest(transCtx.Cluster, status, expectedOpsRequest,
				expectedOpsRequestHash, liveOpsRequest); err != nil {
				h.failManagedShardAdd(status, "OpsRequestIdentityMismatch", err.Error())
				return true, nil
			}
			status.OpsRequestUID = string(liveOpsRequest.UID)
			status.OpsRequestPhase = string(liveOpsRequest.Status.Phase)
			status.Reason = "OpsRequestBound"
			status.Message = "bound the exact live managed OpsRequest UID"
			return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after binding the managed OpsRequest UID")
		case apierrors.IsNotFound(err):
			graphCli, _ := transCtx.Client.(model.GraphClient)
			graphCli.Create(dag, expectedOpsRequest)
			status.Reason = "OpsRequestCreating"
			status.Message = "submitted the persisted managed OpsRequest plan"
			return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after creating the managed OpsRequest")
		default:
			return true, err
		}
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			h.failManagedShardAdd(status, "OpsRequestMissing", "the exact managed OpsRequest disappeared; automatic recreation is forbidden")
			return true, nil
		}
		return true, err
	}
	if err := validateManagedShardAddOpsRequest(transCtx.Cluster, status, expectedOpsRequest,
		expectedOpsRequestHash, liveOpsRequest); err != nil {
		h.failManagedShardAdd(status, "OpsRequestIdentityMismatch", err.Error())
		return true, nil
	}
	status.OpsRequestPhase = string(liveOpsRequest.Status.Phase)

	task, err := managedJobTask(liveOpsRequest, shardingName)
	if err != nil {
		h.failManagedShardAdd(status, "ManagedJobStatusInvalid", err.Error())
		return true, nil
	}
	var managedJob *batchv1.Job
	if task != nil && task.DispatchState == opsv1alpha1.CreatedActionTaskDispatchState {
		managedJob, err = validateManagedShardAddJob(transCtx, status, liveOpsRequest, task)
		if err != nil {
			if !isManagedShardAddContractError(err) {
				return true, err
			}
			h.failManagedShardAdd(status, "ManagedJobIdentityMismatch", err.Error())
			return true, nil
		}
		if status.JobUID == "" {
			status.JobName = managedJob.Name
			status.JobUID = string(managedJob.UID)
			status.JobSpecHash = task.WorkloadSpecHash
			status.Reason = "ManagedJobBound"
			status.Message = "bound the exact managed Job UID and defaulted spec hash"
			return true, intctrlutil.NewDelayedRequeueError(time.Second, "requeue after binding the managed Job identity")
		}
	}
	if status.JobUID != "" && (task == nil || task.DispatchState != opsv1alpha1.CreatedActionTaskDispatchState || managedJob == nil) {
		h.failManagedShardAdd(status, "ManagedJobStatusInvalid", "bound managed Job identity disappeared from the Operations task")
		return true, nil
	}

	switch liveOpsRequest.Status.Phase {
	case opsv1alpha1.OpsFailedPhase, opsv1alpha1.OpsCancelledPhase, opsv1alpha1.OpsAbortedPhase:
		h.failManagedShardAdd(status, "OpsRequestUnsuccessful",
			fmt.Sprintf("managed OpsRequest reached terminal phase %s; preserve evidence, then delete it to request an explicit retry",
				liveOpsRequest.Status.Phase))
		return true, nil
	}

	if liveOpsRequest.Status.Phase != opsv1alpha1.OpsSucceedPhase {
		status.Phase = appsv1.LifecycleActionRunning
		switch {
		case task == nil:
			status.Reason = "WaitingForManagedJobPlan"
			status.Message = "waiting for the Operations controller to persist the managed Job plan"
		case task.DispatchState == opsv1alpha1.PlannedActionTaskDispatchState:
			status.Reason = "WaitingForManagedJob"
			status.Message = "waiting for the Operations controller to bind the managed Job"
		case status.JobUID != "":
			status.Reason = "ManagedJobRunning"
			status.Message = "waiting for the exact managed Job to complete"
		default:
			h.failManagedShardAdd(status, "ManagedJobStatusInvalid", "managed Job status is missing a durable dispatch state")
			return true, nil
		}
		return true, intctrlutil.NewDelayedRequeueError(3*time.Second, "waiting for the managed shard-add OpsRequest")
	}
	if task == nil || status.JobUID == "" || managedJob == nil {
		h.failManagedShardAdd(status, "ManagedJobStatusInvalid", "successful managed OpsRequest has no exact managed Job identity")
		return true, nil
	}
	if task.Status != opsv1alpha1.SucceedActionTaskStatus || !managedShardAddJobSucceeded(managedJob) {
		h.failManagedShardAdd(status, "ManagedJobStatusInvalid", "successful managed OpsRequest does not have a successful exact Job observation")
		return true, nil
	}

	status.CleanupStarted = true
	status.Phase = appsv1.LifecycleActionRunning
	status.Reason = "CleanupPending"
	status.Message = "managed shard-add operation succeeded; marker cleanup will start after this state is persisted"
	return true, intctrlutil.NewDelayedRequeueError(time.Second,
		"requeue after persisting the managed shard-add cleanup state")
}
