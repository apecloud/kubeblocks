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
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var errInvalidShardingScaleInPreparedRequestMaterial = errors.New(
	"invalid sharding scale-in prepared request material")

const (
	shardingScaleInHolderTargetVersionV1    = "kb.sharding.scalein.holder-target/v1"
	shardingScaleInHolderValueVersionV1     = "kb.sharding.scalein.holder-target-value/v1"
	shardingScaleInDurableEnvelopeVersionV1 = "kb.sharding.scalein.durable-envelope/v1"
	shardingScaleInRequestPayloadVersionV2  = "kb.sharding.scalein.request-payload/v2"
	shardingScaleInExecutionIDVersionV2     = "kb.sharding.scalein.execution-id/v2"

	shardingScaleInRequestTimeoutSeconds int32 = 30
	shardingScaleInRequestRetryPolicy    int32 = 0
)

type shardingScaleInClaimClass string

const (
	shardingScaleInClaimClassReadOnly             shardingScaleInClaimClass = "ReadOnly"
	shardingScaleInClaimClassMayMutate            shardingScaleInClaimClass = "MayMutate"
	shardingScaleInClaimClassRecoveryReadOnly     shardingScaleInClaimClass = "RecoveryReadOnly"
	shardingScaleInClaimClassProofReadOnly        shardingScaleInClaimClass = "ProofReadOnly"
	shardingScaleInClaimClassForgetEntryMayMutate shardingScaleInClaimClass = "ForgetEntryMayMutate"
)

type shardingScaleInEffectClass string

const (
	shardingScaleInEffectClassReadOnly  shardingScaleInEffectClass = "ReadOnly"
	shardingScaleInEffectClassMayMutate shardingScaleInEffectClass = "MayMutate"
)

type shardingScaleInPreparedRequestInput struct {
	HolderIndex              int32
	TopologyFenceToken       string
	Phase                    appsv1.ShardingScaleInPhase
	ResultRevision           int64
	ReceiptID                *string
	StepKey                  string
	ClaimClass               shardingScaleInClaimClass
	ExecutorPodUID           types.UID
	ExecutorDispatchSequence int64
}

type shardingScaleInHolderTarget struct {
	ParameterName      string    `json:"parameterName"`
	HolderIndex        int32     `json:"holderIndex"`
	ComponentName      string    `json:"componentName"`
	ComponentUID       types.UID `json:"componentUID"`
	ComponentShortName string    `json:"componentShortName"`
	SourceDigest       string    `json:"sourceDigest"`
	ValueB64           string    `json:"valueB64"`
	ValueDigest        string    `json:"valueDigest"`
}

type shardingScaleInHolderTargetSource struct {
	Version            string    `json:"version"`
	PlanID             string    `json:"planID"`
	HolderIndex        int32     `json:"holderIndex"`
	ComponentName      string    `json:"componentName"`
	ComponentUID       types.UID `json:"componentUID"`
	ComponentShortName string    `json:"componentShortName"`
}

type shardingScaleInHolderTargetValue struct {
	Version  string `json:"version"`
	ValueB64 string `json:"valueB64"`
}

type shardingScaleInDurableEnvelope struct {
	Version            string                             `json:"version"`
	PlanID             string                             `json:"planID"`
	TopologyFenceToken string                             `json:"topologyFenceToken"`
	HolderTarget       shardingScaleInHolderTarget        `json:"holderTarget"`
	Phase              appsv1.ShardingScaleInPhase        `json:"phase"`
	ResultRevision     int64                              `json:"resultRevision"`
	ReceiptID          string                             `json:"receiptID"`
	StepKey            string                             `json:"stepKey"`
	ClaimClass         shardingScaleInClaimClass          `json:"claimClass"`
	EffectClass        shardingScaleInEffectClass         `json:"effectClass"`
	Leaving            []appsv1.ShardingScaleInPlanMember `json:"leaving"`
	Staying            []appsv1.ShardingScaleInPlanMember `json:"staying"`
}

type shardingScaleInRequestExecutorIdentity struct {
	PodName               string    `json:"podName"`
	PodUID                types.UID `json:"podUID"`
	ComponentUID          types.UID `json:"componentUID"`
	AgentImageID          string    `json:"agentImageID"`
	AgentProcessUID       string    `json:"agentProcessUID"`
	AgentCapabilityDigest string    `json:"agentCapabilityDigest"`
}

type shardingScaleInRequestPayloadMaterial struct {
	Version                  string                                 `json:"version"`
	PlanID                   string                                 `json:"planID"`
	HolderIndex              int32                                  `json:"holderIndex"`
	Phase                    appsv1.ShardingScaleInPhase            `json:"phase"`
	ResultRevision           int64                                  `json:"resultRevision"`
	ReceiptID                string                                 `json:"receiptID"`
	StepKey                  string                                 `json:"stepKey"`
	ClaimClass               shardingScaleInClaimClass              `json:"claimClass"`
	EffectClass              shardingScaleInEffectClass             `json:"effectClass"`
	Executor                 shardingScaleInRequestExecutorIdentity `json:"executor"`
	ExecutorDispatchSequence int64                                  `json:"executorDispatchSequence"`
	RequestAuthorityDigest   string                                 `json:"requestAuthorityDigest"`
	BaseParameterRecordB64   string                                 `json:"baseParameterRecordB64"`
	BaseParameterDigest      string                                 `json:"baseParameterDigest"`
	Envelope                 shardingScaleInDurableEnvelope         `json:"envelope"`
	EnvelopeDigest           string                                 `json:"envelopeDigest"`
	TimeoutSeconds           int32                                  `json:"timeoutSeconds"`
	RetryPolicy              int32                                  `json:"retryPolicy"`
}

type shardingScaleInExecutionIDMaterial struct {
	Version                  string                      `json:"version"`
	PlanID                   string                      `json:"planID"`
	HolderIndex              int32                       `json:"holderIndex"`
	Phase                    appsv1.ShardingScaleInPhase `json:"phase"`
	ResultRevision           int64                       `json:"resultRevision"`
	ReceiptID                string                      `json:"receiptID"`
	StepKey                  string                      `json:"stepKey"`
	ExecutorPodUID           types.UID                   `json:"executorPodUID"`
	ExecutorAgentProcessUID  string                      `json:"executorAgentProcessUID"`
	ExecutorDispatchSequence int64                       `json:"executorDispatchSequence"`
	RequestPayloadDigest     string                      `json:"requestPayloadDigest"`
}

type shardingScaleInPreparedRequestMaterial struct {
	HolderTarget         shardingScaleInHolderTarget           `json:"holderTarget"`
	Envelope             shardingScaleInDurableEnvelope        `json:"envelope"`
	EnvelopeDigest       string                                `json:"envelopeDigest"`
	RequestPayload       shardingScaleInRequestPayloadMaterial `json:"requestPayload"`
	RequestPayloadDigest string                                `json:"requestPayloadDigest"`
	ExecutionID          string                                `json:"executionID"`
}

func buildShardingScaleInPreparedRequest(
	persisted *appsv1.ShardingScaleInPlanMaterial,
	planID string,
	input shardingScaleInPreparedRequestInput,
) (*shardingScaleInPreparedRequestMaterial, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInPreparedRequestMaterial,
			fmt.Sprintf(format, args...))
	}
	if persisted == nil {
		return nil, invalid("plan material must not be nil")
	}
	canonical, expectedPlanID, err := buildShardingScaleInPlanMaterial(persisted)
	if err != nil {
		return nil, invalid("plan material is invalid: %v", err)
	}
	if !reflect.DeepEqual(canonical, persisted) {
		return nil, invalid("plan material is not canonical")
	}
	if planID != expectedPlanID {
		return nil, invalid("plan ID does not match canonical plan material")
	}
	if input.HolderIndex < 0 || int(input.HolderIndex) >= len(canonical.Leaving) {
		return nil, invalid("holder index is outside leaving")
	}
	if !isShardingScaleInSHA256(input.TopologyFenceToken) {
		return nil, invalid("topology fence token must be a SHA256 digest")
	}
	if !isShardingScaleInDispatchablePhase(input.Phase) {
		return nil, invalid("phase is not dispatchable")
	}
	if input.ResultRevision <= 0 {
		return nil, invalid("result revision must be positive")
	}
	if input.StepKey == "" {
		return nil, invalid("step key must not be empty")
	}
	effectClass, err := shardingScaleInEffectClassForClaimClass(input.ClaimClass)
	if err != nil {
		return nil, invalid("claim class is not supported")
	}
	receiptID, err := validateShardingScaleInPreparedRequestContract(input)
	if err != nil {
		return nil, invalid("phase, step, receipt, and claim combination is not supported: %v", err)
	}
	if input.ExecutorDispatchSequence <= 0 {
		return nil, invalid("executor dispatch sequence must be positive")
	}

	executor, template, err := findShardingScaleInRequestExecutor(canonical, input.ExecutorPodUID)
	if err != nil {
		return nil, invalid("executor must bind one exact plan Pod: %v", err)
	}
	if input.ClaimClass == shardingScaleInClaimClassProofReadOnly &&
		(executor.PodName != canonical.ProofExecutor.PodName ||
			executor.PodUID != canonical.ProofExecutor.PodUID) {
		return nil, invalid("proof claim executor must match the frozen proof executor")
	}
	encodedBase, baseDigest, _, err := canonicalizeShardingScaleInBaseParameterRecord(
		template.BaseParameterRecordB64, template.BaseParameterDigest)
	if err != nil {
		return nil, invalid("executor base parameter record is invalid: %v", err)
	}
	if encodedBase != template.BaseParameterRecordB64 || baseDigest != template.BaseParameterDigest {
		return nil, invalid("executor base parameter record is not canonical")
	}

	holder := canonical.Leaving[input.HolderIndex]
	target, err := buildShardingScaleInHolderTarget(planID, input.HolderIndex, holder)
	if err != nil {
		return nil, err
	}
	envelope := shardingScaleInDurableEnvelope{
		Version:            shardingScaleInDurableEnvelopeVersionV1,
		PlanID:             planID,
		TopologyFenceToken: input.TopologyFenceToken,
		HolderTarget:       target,
		Phase:              input.Phase,
		ResultRevision:     input.ResultRevision,
		ReceiptID:          receiptID,
		StepKey:            input.StepKey,
		ClaimClass:         input.ClaimClass,
		EffectClass:        effectClass,
		Leaving:            cloneShardingScaleInPlanMembers(canonical.Leaving),
		Staying:            cloneShardingScaleInPlanMembers(canonical.Staying),
	}
	envelopeDigest, err := digestShardingScaleInCanonicalJSON(envelope)
	if err != nil {
		return nil, invalid("durable envelope digest failed: %v", err)
	}

	payload := shardingScaleInRequestPayloadMaterial{
		Version:                  shardingScaleInRequestPayloadVersionV2,
		PlanID:                   planID,
		HolderIndex:              input.HolderIndex,
		Phase:                    input.Phase,
		ResultRevision:           input.ResultRevision,
		ReceiptID:                receiptID,
		StepKey:                  input.StepKey,
		ClaimClass:               input.ClaimClass,
		EffectClass:              effectClass,
		Executor:                 executor,
		ExecutorDispatchSequence: input.ExecutorDispatchSequence,
		RequestAuthorityDigest:   canonical.RequestAuthority.RequestAuthorityDigest,
		BaseParameterRecordB64:   encodedBase,
		BaseParameterDigest:      baseDigest,
		Envelope:                 cloneShardingScaleInDurableEnvelope(envelope),
		EnvelopeDigest:           envelopeDigest,
		TimeoutSeconds:           shardingScaleInRequestTimeoutSeconds,
		RetryPolicy:              shardingScaleInRequestRetryPolicy,
	}
	requestPayloadDigest, err := digestShardingScaleInCanonicalJSON(payload)
	if err != nil {
		return nil, invalid("request payload digest failed: %v", err)
	}
	executionID, err := digestShardingScaleInCanonicalJSON(shardingScaleInExecutionIDMaterial{
		Version:                  shardingScaleInExecutionIDVersionV2,
		PlanID:                   planID,
		HolderIndex:              input.HolderIndex,
		Phase:                    input.Phase,
		ResultRevision:           input.ResultRevision,
		ReceiptID:                receiptID,
		StepKey:                  input.StepKey,
		ExecutorPodUID:           executor.PodUID,
		ExecutorAgentProcessUID:  executor.AgentProcessUID,
		ExecutorDispatchSequence: input.ExecutorDispatchSequence,
		RequestPayloadDigest:     requestPayloadDigest,
	})
	if err != nil {
		return nil, invalid("execution ID digest failed: %v", err)
	}

	return &shardingScaleInPreparedRequestMaterial{
		HolderTarget:         target,
		Envelope:             cloneShardingScaleInDurableEnvelope(envelope),
		EnvelopeDigest:       envelopeDigest,
		RequestPayload:       payload,
		RequestPayloadDigest: requestPayloadDigest,
		ExecutionID:          executionID,
	}, nil
}

func cloneShardingScaleInDurableEnvelope(
	envelope shardingScaleInDurableEnvelope,
) shardingScaleInDurableEnvelope {
	envelope.Leaving = cloneShardingScaleInPlanMembers(envelope.Leaving)
	envelope.Staying = cloneShardingScaleInPlanMembers(envelope.Staying)
	return envelope
}

func cloneShardingScaleInPlanMembers(
	members []appsv1.ShardingScaleInPlanMember,
) []appsv1.ShardingScaleInPlanMember {
	if members == nil {
		return nil
	}
	cloned := make([]appsv1.ShardingScaleInPlanMember, len(members))
	for i := range members {
		members[i].DeepCopyInto(&cloned[i])
	}
	return cloned
}

func buildShardingScaleInHolderTarget(planID string, holderIndex int32,
	holder appsv1.ShardingScaleInPlanMember,
) (shardingScaleInHolderTarget, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInPreparedRequestMaterial,
			fmt.Sprintf(format, args...))
	}
	if !isShardingScaleInSHA256(planID) || holderIndex < 0 ||
		holder.ComponentName == "" || holder.ComponentUID == "" || holder.ComponentShortName == "" {
		return shardingScaleInHolderTarget{}, invalid("holder source identity must be complete")
	}
	sourceDigest, err := digestShardingScaleInCanonicalJSON(shardingScaleInHolderTargetSource{
		Version:            shardingScaleInHolderTargetVersionV1,
		PlanID:             planID,
		HolderIndex:        holderIndex,
		ComponentName:      holder.ComponentName,
		ComponentUID:       holder.ComponentUID,
		ComponentShortName: holder.ComponentShortName,
	})
	if err != nil {
		return shardingScaleInHolderTarget{}, invalid("holder source digest failed: %v", err)
	}
	valueB64 := base64.StdEncoding.EncodeToString([]byte(holder.ComponentName))
	valueDigest, err := digestShardingScaleInCanonicalJSON(shardingScaleInHolderTargetValue{
		Version:  shardingScaleInHolderValueVersionV1,
		ValueB64: valueB64,
	})
	if err != nil {
		return shardingScaleInHolderTarget{}, invalid("holder value digest failed: %v", err)
	}
	return shardingScaleInHolderTarget{
		ParameterName:      shardingRemoveShardNameVar,
		HolderIndex:        holderIndex,
		ComponentName:      holder.ComponentName,
		ComponentUID:       holder.ComponentUID,
		ComponentShortName: holder.ComponentShortName,
		SourceDigest:       sourceDigest,
		ValueB64:           valueB64,
		ValueDigest:        valueDigest,
	}, nil
}

func findShardingScaleInRequestExecutor(material *appsv1.ShardingScaleInPlanMaterial,
	podUID types.UID,
) (shardingScaleInRequestExecutorIdentity, appsv1.ShardingScaleInExecutorTemplate, error) {
	if podUID == "" {
		return shardingScaleInRequestExecutorIdentity{},
			appsv1.ShardingScaleInExecutorTemplate{}, errors.New("executor Pod UID must not be empty")
	}
	var identity shardingScaleInRequestExecutorIdentity
	foundPod := false
	findPod := func(members []appsv1.ShardingScaleInPlanMember) error {
		for _, member := range members {
			for _, pod := range member.Pods {
				if pod.UID != podUID {
					continue
				}
				if foundPod {
					return errors.New("executor Pod UID is duplicated")
				}
				foundPod = true
				identity = shardingScaleInRequestExecutorIdentity{
					PodName:               pod.Name,
					PodUID:                pod.UID,
					ComponentUID:          member.ComponentUID,
					AgentImageID:          pod.AgentImageID,
					AgentProcessUID:       pod.AgentProcessUID,
					AgentCapabilityDigest: pod.AgentCapabilityDigest,
				}
			}
		}
		return nil
	}
	if err := findPod(material.Leaving); err != nil {
		return shardingScaleInRequestExecutorIdentity{},
			appsv1.ShardingScaleInExecutorTemplate{}, err
	}
	if err := findPod(material.Staying); err != nil {
		return shardingScaleInRequestExecutorIdentity{},
			appsv1.ShardingScaleInExecutorTemplate{}, err
	}
	if !foundPod {
		return shardingScaleInRequestExecutorIdentity{},
			appsv1.ShardingScaleInExecutorTemplate{}, errors.New("executor Pod is absent")
	}
	var template appsv1.ShardingScaleInExecutorTemplate
	foundTemplate := false
	for _, candidate := range material.RequestAuthority.ExecutorTemplates {
		if candidate.ExecutorPodUID != podUID {
			continue
		}
		if foundTemplate {
			return shardingScaleInRequestExecutorIdentity{},
				appsv1.ShardingScaleInExecutorTemplate{}, errors.New("executor template is duplicated")
		}
		foundTemplate = true
		template = candidate
	}
	if !foundTemplate || template.ExecutorComponentUID != identity.ComponentUID ||
		template.ServerRuntimeBinding.AgentProcessUID != identity.AgentProcessUID ||
		template.ServerRuntimeBinding.AgentImageID != identity.AgentImageID {
		return shardingScaleInRequestExecutorIdentity{},
			appsv1.ShardingScaleInExecutorTemplate{}, errors.New(
				"executor template does not match the plan Pod")
	}
	return identity, template, nil
}

func isShardingScaleInDispatchablePhase(phase appsv1.ShardingScaleInPhase) bool {
	switch phase {
	case appsv1.ShardingScaleInPhaseDraining,
		appsv1.ShardingScaleInPhasePurgePrepared,
		appsv1.ShardingScaleInPhaseResetting,
		appsv1.ShardingScaleInPhaseForgetting,
		appsv1.ShardingScaleInPhaseVerified:
		return true
	default:
		return false
	}
}

func validateShardingScaleInPreparedRequestContract(
	input shardingScaleInPreparedRequestInput,
) (string, error) {
	receiptID := ""
	if input.ReceiptID != nil {
		if !isShardingScaleInSHA256(*input.ReceiptID) {
			return "", errors.New("bound receipt ID must be a SHA256 digest")
		}
		receiptID = *input.ReceiptID
	}

	switch {
	case input.Phase == appsv1.ShardingScaleInPhaseDraining &&
		input.ClaimClass == shardingScaleInClaimClassMayMutate &&
		input.ReceiptID == nil &&
		input.StepKey == fmt.Sprintf("drain/%d", input.ResultRevision):
		return receiptID, nil
	case input.Phase == appsv1.ShardingScaleInPhasePurgePrepared &&
		input.ClaimClass == shardingScaleInClaimClassProofReadOnly &&
		input.ReceiptID != nil &&
		input.StepKey == "purge-proof/"+receiptID:
		return receiptID, nil
	case input.Phase == appsv1.ShardingScaleInPhaseResetting &&
		input.ClaimClass == shardingScaleInClaimClassMayMutate &&
		input.ReceiptID != nil &&
		isShardingScaleInResetStepKey(input.StepKey, input.ExecutorPodUID):
		return receiptID, nil
	case input.Phase == appsv1.ShardingScaleInPhaseForgetting &&
		input.ClaimClass == shardingScaleInClaimClassForgetEntryMayMutate &&
		input.ReceiptID != nil &&
		isShardingScaleInForgetStepKey(input.StepKey, input.ExecutorPodUID):
		return receiptID, nil
	case input.Phase == appsv1.ShardingScaleInPhaseForgetting &&
		input.ClaimClass == shardingScaleInClaimClassProofReadOnly &&
		input.ReceiptID != nil &&
		input.StepKey == "final-proof/"+receiptID:
		return receiptID, nil
	case input.Phase == appsv1.ShardingScaleInPhaseVerified &&
		input.ClaimClass == shardingScaleInClaimClassProofReadOnly &&
		input.ReceiptID != nil &&
		isShardingScaleInDeleteProofStepKey(input.StepKey):
		return receiptID, nil
	default:
		return "", errors.New("typed request tuple is not registered")
	}
}

func isShardingScaleInResetStepKey(stepKey string, podUID types.UID) bool {
	parts := strings.Split(stepKey, "/")
	if len(parts) != 4 || parts[0] != "reset" || parts[2] != string(podUID) {
		return false
	}
	cursor, err := strconv.ParseUint(parts[1], 10, 32)
	return err == nil && strconv.FormatUint(cursor, 10) == parts[1] &&
		isShardingScaleInLowerHex(parts[3], 40)
}

func isShardingScaleInForgetStepKey(stepKey string, podUID types.UID) bool {
	parts := strings.Split(stepKey, "/")
	return len(parts) == 3 && parts[0] == "forget-epoch" &&
		isShardingScaleInSHA256(parts[1]) && parts[2] == string(podUID)
}

func isShardingScaleInDeleteProofStepKey(stepKey string) bool {
	parts := strings.Split(stepKey, "/")
	return len(parts) == 2 && parts[0] == "delete-proof" &&
		isShardingScaleInSHA256(parts[1])
}

func isShardingScaleInLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func shardingScaleInEffectClassForClaimClass(
	class shardingScaleInClaimClass,
) (shardingScaleInEffectClass, error) {
	switch class {
	case shardingScaleInClaimClassReadOnly,
		shardingScaleInClaimClassRecoveryReadOnly,
		shardingScaleInClaimClassProofReadOnly:
		return shardingScaleInEffectClassReadOnly, nil
	case shardingScaleInClaimClassMayMutate,
		shardingScaleInClaimClassForgetEntryMayMutate:
		return shardingScaleInEffectClassMayMutate, nil
	default:
		return "", errors.New("unsupported claim class")
	}
}
