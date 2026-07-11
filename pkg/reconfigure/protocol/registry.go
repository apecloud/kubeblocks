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

package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

const (
	protocolFenceStatusV1               = "kubeblocks.io/reconfigure-protocol-fence-status/v1"
	ReservedProtocolFenceStatusBytes    = 4 * 1024
	MaxTerminalReasonCodeBytes          = 128
	maxFenceResourceVersionBytes        = 32
	maxEffectObjectUIDBytes             = 64
	maxEffectObjectResourceVersionBytes = 32
)

var (
	ErrIdentityIncomplete            = errors.New("identity incomplete")
	ErrIdentityDigestInvalid         = errors.New("identity digest invalid")
	ErrRegistrationIdentityConflict  = errors.New("registration identity conflict")
	ErrEffectIdentityConflict        = errors.New("effect identity conflict")
	ErrEffectAlreadyConsumed         = errors.New("effect already consumed")
	ErrRegistrationAlreadyConsumed   = errors.New("registration already consumed")
	ErrEffectObservationIncomplete   = errors.New("effect observation incomplete")
	ErrEffectObservationMismatch     = errors.New("effect observation mismatch")
	ErrEffectObservationInconclusive = errors.New("effect observation inconclusive")
	ErrEffectObjectIdentityConflict  = errors.New("effect object identity conflict")
	ErrEffectStillCreatable          = errors.New("effect still creatable")
	ErrEffectObservationStale        = errors.New("effect observation stale")
	ErrEffectObjectNotBound          = errors.New("effect object not bound")
	ErrDispatchOutcomeUnknown        = errors.New("dispatch outcome unknown")
	ErrEffectDispatchRequired        = errors.New("effect dispatch required")
	ErrEffectCloseoutVariantMismatch = errors.New("effect closeout variant mismatch")
	ErrEffectStateRegression         = errors.New("effect state regression")
	ErrTerminalEvidenceTooLarge      = errors.New("terminal evidence too large")
	ErrFenceReadbackMismatch         = errors.New("fence readback mismatch")
	ErrDrainReadbackAlreadyBound     = errors.New("drain readback already bound")
	ErrRegistryCapacity              = errors.New("registry capacity exceeded")
	ErrStoredStatusIntegrity         = errors.New("stored status integrity mismatch")
	ErrStoredStatusInvalid           = errors.New("stored status invalid")
	ErrEffectNotPlanned              = errors.New("effect not planned")
	ErrRegistryDraining              = errors.New("registry is draining")
	ErrDrainNotCommitted             = errors.New("drain not committed")
)

type EffectKind string

const (
	EffectKindUnitExecution     EffectKind = "UnitExecution"
	EffectKindTargetFenceWindow EffectKind = "TargetFenceWindow"
	EffectKindCredentialWindow  EffectKind = "CredentialWindow"
)

type EffectState string

const (
	EffectStateUnknown            EffectState = "Unknown"
	EffectStatePlanned            EffectState = "Planned"
	EffectStateObjectBound        EffectState = "ObjectBound"
	EffectStateDispatchAuthorized EffectState = "DispatchAuthorized"
	EffectStateTerminal           EffectState = "Terminal"
	EffectStateManual             EffectState = "Manual"
	EffectStateConsumed           EffectState = "Consumed"
)

type RegistrationPhase string

const (
	RegistrationPhaseActive   RegistrationPhase = "Active"
	RegistrationPhaseConsumed RegistrationPhase = "Consumed"
)

type EffectCloseoutVariant string

const (
	CloseoutVariantNotFound         EffectCloseoutVariant = "NotFound"
	CloseoutVariantUnitExecuted     EffectCloseoutVariant = "UnitExecuted"
	CloseoutVariantObservedTerminal EffectCloseoutVariant = "ObservedTerminal"
)

type EffectTerminalOutcome string

const (
	EffectTerminalSucceeded EffectTerminalOutcome = "Succeeded"
	EffectTerminalFailed    EffectTerminalOutcome = "Failed"
	EffectTerminalManual    EffectTerminalOutcome = "Manual"
)

type EffectObservationResult string

const (
	EffectObservationPresent     EffectObservationResult = "Present"
	EffectObservationNotFound    EffectObservationResult = "NotFound"
	EffectObservationTerminating EffectObservationResult = "Terminating"
	EffectObservationAPIError    EffectObservationResult = "APIError"
)

type RegistrationIdentity struct {
	PhysicalAPIID     string
	InstallationEpoch string
	ExecutionUID      types.UID
	AttemptID         string
	NamespaceUID      types.UID
	KeySecretUID      types.UID
	AuthorityUID      types.UID
	QueueUID          types.UID
}

type EffectIdentity struct {
	Kind               EffectKind
	DeterministicName  string
	FullIdentityDigest string
	Namespace          string
	PodUID             types.UID
	ContainerName      string
	FenceUID           types.UID
}

type EffectObjectTarget struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
}

type EffectObjectObservation struct {
	Result                EffectObservationResult
	Target                EffectObjectTarget
	ObjectUID             types.UID
	ObjectResourceVersion string
	LookupResourceVersion string
	ObservedAt            metav1.Time
	DrainIdentity         DrainIdentity
	ErrorClass            string
}

type EffectTerminalEvidence struct {
	CloseoutVariant  EffectCloseoutVariant
	Outcome          EffectTerminalOutcome
	ReasonCode       string
	EvidenceDigest   string
	RecoveryRequired bool
}

type FenceObjectIdentity struct {
	UID types.UID
}

type FenceObjectReadback struct {
	UID                types.UID
	ResourceVersion    string
	StoredStatusDigest string
}

type DrainIdentity struct {
	Token                      string
	ManifestDigest             string
	FenceUIDDigest             string
	FenceResourceVersionDigest string
}

type DrainScope struct {
	namespaceUID types.UID
	installation bool
}

func NamespaceDrainScope(namespaceUID types.UID) DrainScope {
	return DrainScope{namespaceUID: namespaceUID}
}
func InstallationDrainScope() DrainScope { return DrainScope{installation: true} }

type RegistryLimits struct {
	MaxNamespaces    int
	MaxRegistrations int
	MaxEffects       int
	MaxStatusBytes   int
}

type FenceState struct {
	status         storedStatus
	fenceIdentity  FenceObjectIdentity
	activeReadback *FenceObjectReadback
}

type storedStatus struct {
	ProtocolVersion      string               `json:"protocolVersion"`
	IntegrityDigest      string               `json:"integrityDigest"`
	FenceUIDDigest       string               `json:"fenceUIDDigest"`
	InstallationEpoch    string               `json:"installationEpoch"`
	InstallationManifest *storedManifest      `json:"installationManifest,omitempty"`
	Namespaces           []storedNamespace    `json:"namespaces"`
	Registrations        []storedRegistration `json:"registrations,omitempty"`
}

type storedNamespace struct {
	UID      types.UID       `json:"uid"`
	Manifest *storedManifest `json:"manifest,omitempty"`
}

type storedManifest struct {
	Token            string          `json:"token"`
	Digest           string          `json:"digest"`
	RegistrationKeys []string        `json:"registrationKeys"`
	EffectKeys       []string        `json:"effectKeys"`
	CommitReadback   *storedReadback `json:"commitReadback,omitempty"`
}

type storedReadback struct {
	UIDDigest             string `json:"uidDigest"`
	ResourceVersionDigest string `json:"resourceVersionDigest"`
	StoredStatusDigest    string `json:"storedStatusDigest"`
}

type storedRegistration struct {
	Key      string                     `json:"key"`
	Identity storedRegistrationIdentity `json:"identity"`
	Phase    string                     `json:"phase"`
	Effects  []storedEffect             `json:"effects,omitempty"`
}

type storedRegistrationIdentity struct {
	PhysicalAPIID     string    `json:"physicalAPIID"`
	InstallationEpoch string    `json:"installationEpoch"`
	ExecutionUID      types.UID `json:"executionUID"`
	AttemptID         string    `json:"attemptID"`
	NamespaceUID      types.UID `json:"namespaceUID"`
	KeySecretUID      types.UID `json:"keySecretUID"`
	AuthorityUID      types.UID `json:"authorityUID"`
	QueueUID          types.UID `json:"queueUID"`
}

type storedEffect struct {
	Key             string                   `json:"key"`
	Identity        storedEffectIdentity     `json:"identity"`
	State           string                   `json:"state"`
	CloseoutVariant string                   `json:"closeoutVariant,omitempty"`
	ObjectBinding   *storedObjectBinding     `json:"objectBinding,omitempty"`
	Dispatch        *storedDispatch          `json:"dispatch,omitempty"`
	Terminal        *storedTerminal          `json:"terminal,omitempty"`
	NotFound        *storedNotFound          `json:"notFound,omitempty"`
	Tombstone       *storedConsumedTombstone `json:"tombstone,omitempty"`
}

type storedEffectIdentity struct {
	Kind               string    `json:"kind"`
	DeterministicName  string    `json:"deterministicName"`
	FullIdentityDigest string    `json:"fullIdentityDigest"`
	Namespace          string    `json:"namespace"`
	PodUID             types.UID `json:"podUID,omitempty"`
	ContainerName      string    `json:"containerName,omitempty"`
	FenceUID           types.UID `json:"fenceUID,omitempty"`
}

type storedObjectBinding struct {
	Target                EffectObjectTarget `json:"target"`
	UIDDigest             string             `json:"uidDigest"`
	ResourceVersionDigest string             `json:"resourceVersionDigest"`
}

type storedDispatch struct {
	Token string `json:"token"`
}

type storedTerminal struct {
	Outcome          string `json:"outcome"`
	ReasonCode       string `json:"reasonCode"`
	EvidenceDigest   string `json:"evidenceDigest"`
	RecoveryRequired bool   `json:"recoveryRequired"`
}

type storedConsumedTombstone struct {
	IdentityDigest string `json:"identityDigest"`
}

type storedNotFound struct {
	Target                      EffectObjectTarget `json:"target"`
	DrainToken                  string             `json:"drainToken"`
	ManifestDigest              string             `json:"manifestDigest"`
	FenceUIDDigest              string             `json:"fenceUIDDigest"`
	FenceResourceVersionDigest  string             `json:"fenceResourceVersionDigest"`
	LookupResourceVersionDigest string             `json:"lookupResourceVersionDigest"`
	EvidenceDigest              string             `json:"evidenceDigest"`
}

func digest(input string) string {
	sum := sha256.Sum256([]byte(input))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ValidateIdentityDigest(value string) error {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return ErrIdentityDigestInvalid
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:")); err != nil || value != strings.ToLower(value) {
		return ErrIdentityDigestInvalid
	}
	return nil
}

func canonicalFields(domain string, fields ...string) []byte {
	var b strings.Builder
	b.WriteString(domain)
	b.WriteByte(0)
	for _, field := range fields {
		fmt.Fprintf(&b, "%d:", len(field))
		b.WriteString(field)
		b.WriteByte(0)
	}
	return []byte(b.String())
}

func CanonicalRegistrationIdentityBytes(identity RegistrationIdentity) ([]byte, error) {
	fields := []string{identity.PhysicalAPIID, identity.InstallationEpoch, string(identity.ExecutionUID), identity.AttemptID, string(identity.NamespaceUID), string(identity.KeySecretUID), string(identity.AuthorityUID), string(identity.QueueUID)}
	for _, value := range fields {
		if value == "" {
			return nil, ErrIdentityIncomplete
		}
	}
	return canonicalFields("kubeblocks.io/reconfigure-registration/v1", fields...), nil
}

func RegistrationKey(identity RegistrationIdentity) (string, error) {
	data, err := CanonicalRegistrationIdentityBytes(identity)
	if err != nil {
		return "", err
	}
	return digest(string(data)), nil
}

func CanonicalEffectIdentityBytes(registrationKey string, identity EffectIdentity) ([]byte, error) {
	if err := ValidateIdentityDigest(registrationKey); err != nil {
		return nil, err
	}
	if identity.Kind != EffectKindUnitExecution && !isWindowKind(identity.Kind) {
		return nil, ErrIdentityIncomplete
	}
	if identity.DeterministicName == "" || identity.Namespace == "" || ValidateIdentityDigest(identity.FullIdentityDigest) != nil {
		return nil, ErrIdentityIncomplete
	}
	if identity.Kind == EffectKindUnitExecution && (identity.PodUID == "" || identity.ContainerName == "" || identity.FenceUID == "") {
		return nil, ErrIdentityIncomplete
	}
	return canonicalFields("kubeblocks.io/reconfigure-effect/v1", registrationKey, string(identity.Kind), identity.DeterministicName, identity.FullIdentityDigest, identity.Namespace, string(identity.PodUID), identity.ContainerName, string(identity.FenceUID)), nil
}

func EffectKey(registrationKey string, identity EffectIdentity) (string, error) {
	data, err := CanonicalEffectIdentityBytes(registrationKey, identity)
	if err != nil {
		return "", err
	}
	return digest(string(data)), nil
}

func NewFenceState(installationEpoch string, namespaceUID types.UID, identity FenceObjectIdentity) (FenceState, error) {
	if installationEpoch == "" || namespaceUID == "" || identity.UID == "" {
		return FenceState{}, ErrIdentityIncomplete
	}
	return FenceState{status: storedStatus{ProtocolVersion: protocolFenceStatusV1, FenceUIDDigest: digestExternalUID(identity.UID), InstallationEpoch: installationEpoch, Namespaces: []storedNamespace{{UID: namespaceUID}}}, fenceIdentity: identity}, nil
}

func IsZeroFenceState(state FenceState) bool { return state.status.ProtocolVersion == "" }

func cloneState(state FenceState) FenceState {
	data, _ := json.Marshal(state.status)
	var status storedStatus
	_ = json.Unmarshal(data, &status)
	cloned := FenceState{status: status, fenceIdentity: state.fenceIdentity}
	if state.activeReadback != nil {
		value := *state.activeReadback
		cloned.activeReadback = &value
	}
	return cloned
}

func StoredProtocolFenceStatusBytes(state FenceState) ([]byte, error) {
	return marshalStoredStatus(state.status)
}
func CanonicalFenceStateBytes(state FenceState) ([]byte, error) {
	return marshalStoredStatus(state.status)
}

func marshalStoredStatus(status storedStatus) ([]byte, error) {
	status.IntegrityDigest = ""
	unsigned, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}
	status.IntegrityDigest = digest("kubeblocks.io/reconfigure-stored-status-integrity/v1\x00" + string(unsigned))
	return json.Marshal(status)
}

func digestExternalUID(uid types.UID) string {
	return digest("kubeblocks.io/reconfigure-external-uid/v1\x00" + string(uid))
}
func digestExternalRV(rv string) string {
	return digest("kubeblocks.io/reconfigure-external-rv/v1\x00" + rv)
}

func registrationIdentityStored(v RegistrationIdentity) storedRegistrationIdentity {
	return storedRegistrationIdentity(v)
}

func effectIdentityStored(v EffectIdentity) storedEffectIdentity {
	return storedEffectIdentity{v.KindString(), v.DeterministicName, v.FullIdentityDigest, v.Namespace, v.PodUID, v.ContainerName, v.FenceUID}
}

func (v EffectIdentity) KindString() string { return string(v.Kind) }

func registrationIdentityRuntime(v storedRegistrationIdentity) RegistrationIdentity {
	return RegistrationIdentity(v)
}

func effectIdentityRuntime(v storedEffectIdentity) EffectIdentity {
	return EffectIdentity{EffectKind(v.Kind), v.DeterministicName, v.FullIdentityDigest, v.Namespace, v.PodUID, v.ContainerName, v.FenceUID}
}

func RegisterExecution(state FenceState, identity RegistrationIdentity, limits RegistryLimits) (FenceState, string, error) {
	key, err := RegistrationKey(identity)
	if err != nil {
		return state, "", err
	}
	for _, registration := range state.status.Registrations {
		if registration.Key == key {
			return state, key, nil
		}
	}
	if state.status.InstallationManifest != nil || anyNamespaceDraining(state.status) {
		return state, "", ErrRegistryDraining
	}
	for _, registration := range state.status.Registrations {
		current := registrationIdentityRuntime(registration.Identity)
		if current.PhysicalAPIID == identity.PhysicalAPIID && current.InstallationEpoch == identity.InstallationEpoch && current.ExecutionUID == identity.ExecutionUID && current.AttemptID == identity.AttemptID {
			return state, "", ErrRegistrationIdentityConflict
		}
	}
	if len(state.status.Registrations) >= limits.MaxRegistrations {
		return state, "", ErrRegistryCapacity
	}
	next := cloneState(state)
	if !hasNamespace(next.status, identity.NamespaceUID) {
		if len(next.status.Namespaces) >= limits.MaxNamespaces {
			return state, "", ErrRegistryCapacity
		}
		next.status.Namespaces = append(next.status.Namespaces, storedNamespace{UID: identity.NamespaceUID})
	}
	next.status.Registrations = append(next.status.Registrations, storedRegistration{Key: key, Identity: registrationIdentityStored(identity), Phase: string(RegistrationPhaseActive)})
	if !withinCapacity(next, limits) {
		return state, "", ErrRegistryCapacity
	}
	return next, key, nil
}

func PlanEffect(state FenceState, registrationKey string, identity EffectIdentity, limits RegistryLimits) (FenceState, string, error) {
	key, err := EffectKey(registrationKey, identity)
	if err != nil {
		return state, "", err
	}
	registration, _, ok := findRegistration(&state.status, registrationKey)
	if !ok {
		return state, "", ErrRegistrationIdentityConflict
	}
	for _, effect := range registration.Effects {
		if effect.Key == key {
			if RegistrationPhase(registration.Phase) == RegistrationPhaseConsumed && len(registration.Effects) > 1 {
				return state, "", ErrRegistrationAlreadyConsumed
			}
			if EffectState(effect.State) == EffectStateConsumed {
				return state, "", ErrEffectAlreadyConsumed
			}
			return state, key, nil
		}
		if effect.Identity.Kind == string(identity.Kind) && effect.Identity.Namespace == identity.Namespace && effect.Identity.DeterministicName == identity.DeterministicName {
			if EffectState(effect.State) == EffectStateConsumed {
				return state, "", ErrEffectAlreadyConsumed
			}
			return state, "", ErrEffectIdentityConflict
		}
	}
	if RegistrationPhase(registration.Phase) == RegistrationPhaseConsumed {
		return state, "", ErrRegistrationAlreadyConsumed
	}
	if state.status.InstallationManifest != nil || anyNamespaceDraining(state.status) {
		return state, "", ErrRegistryDraining
	}
	if TotalEffectCount(state) >= limits.MaxEffects {
		return state, "", ErrRegistryCapacity
	}
	next := cloneState(state)
	registration, _, _ = findRegistration(&next.status, registrationKey)
	registration.Effects = append(registration.Effects, storedEffect{Key: key, Identity: effectIdentityStored(identity), State: string(EffectStatePlanned)})
	if !withinCapacity(next, limits) {
		return state, "", ErrRegistryCapacity
	}
	return next, key, nil
}

func ObserveEffectObject(state FenceState, registrationKey, effectKey string, observation EffectObjectObservation) (FenceState, error) {
	effect, registration, ok := findEffect(&state.status, registrationKey, effectKey)
	if !ok {
		return state, ErrEffectIdentityConflict
	}
	identity := effectIdentityRuntime(effect.Identity)
	if observation.Target != expectedTarget(identity) {
		return state, ErrEffectObservationMismatch
	}
	if EffectState(effect.State) == EffectStateConsumed {
		return state, ErrEffectAlreadyConsumed
	}
	switch observation.Result {
	case EffectObservationAPIError, EffectObservationTerminating:
		return state, ErrEffectObservationInconclusive
	case EffectObservationPresent:
		if observation.ObjectUID == "" || observation.ObjectResourceVersion == "" {
			return state, ErrEffectObservationIncomplete
		}
		uidDigest := digestExternalUID(observation.ObjectUID)
		rvDigest := digestExternalRV(observation.ObjectResourceVersion)
		if effect.ObjectBinding != nil {
			if effect.ObjectBinding.UIDDigest == uidDigest && effect.ObjectBinding.ResourceVersionDigest == rvDigest && effect.ObjectBinding.Target == observation.Target {
				return state, nil
			}
			return state, ErrEffectObjectIdentityConflict
		}
		if EffectState(effect.State) != EffectStatePlanned {
			return state, ErrEffectObjectIdentityConflict
		}
		next := cloneState(state)
		effect, _, _ = findEffect(&next.status, registrationKey, effectKey)
		effect.State = string(EffectStateObjectBound)
		effect.ObjectBinding = &storedObjectBinding{Target: observation.Target, UIDDigest: uidDigest, ResourceVersionDigest: rvDigest}
		return next, nil
	case EffectObservationNotFound:
		if !isWindowKind(identity.Kind) {
			return state, ErrEffectCloseoutVariantMismatch
		}
		if observation.LookupResourceVersion == "" {
			return state, ErrEffectObservationIncomplete
		}
		if EffectState(effect.State) != EffectStatePlanned {
			return state, ErrEffectObservationStale
		}
		manifest := matchingCommittedManifest(state.status, registration.Identity.NamespaceUID, observation.DrainIdentity)
		if manifest == nil {
			if !anyNamespaceDraining(state.status) && state.status.InstallationManifest == nil {
				return state, ErrEffectStillCreatable
			}
			return state, ErrEffectObservationStale
		}
		next := cloneState(state)
		effect, _, _ = findEffect(&next.status, registrationKey, effectKey)
		effect.State = string(EffectStateConsumed)
		effect.CloseoutVariant = string(CloseoutVariantNotFound)
		effect.NotFound = &storedNotFound{Target: observation.Target, DrainToken: observation.DrainIdentity.Token, ManifestDigest: observation.DrainIdentity.ManifestDigest, FenceUIDDigest: observation.DrainIdentity.FenceUIDDigest, FenceResourceVersionDigest: observation.DrainIdentity.FenceResourceVersionDigest, LookupResourceVersionDigest: digestExternalRV(observation.LookupResourceVersion)}
		effect.NotFound.EvidenceDigest = notFoundEvidenceDigest(*effect.NotFound)
		effect.Tombstone = &storedConsumedTombstone{IdentityDigest: consumedTombstoneDigest(effectKey)}
		return next, nil
	default:
		return state, ErrEffectObservationInconclusive
	}
}

func AuthorizeDispatch(state FenceState, registrationKey, effectKey string) (FenceState, error) {
	effect, _, ok := findEffect(&state.status, registrationKey, effectKey)
	if !ok {
		return state, ErrEffectIdentityConflict
	}
	if EffectState(effect.State) == EffectStateConsumed {
		return state, ErrEffectAlreadyConsumed
	}
	if EffectState(effect.State) == EffectStateDispatchAuthorized {
		return state, ErrDispatchOutcomeUnknown
	}
	if EffectState(effect.State) != EffectStateObjectBound || effect.ObjectBinding == nil {
		return state, ErrEffectObjectNotBound
	}
	if hasUncommittedDrain(state.status) {
		return state, ErrRegistryDraining
	}
	if EffectKind(effect.Identity.Kind) != EffectKindUnitExecution {
		return state, ErrEffectCloseoutVariantMismatch
	}
	next := cloneState(state)
	effect, _, _ = findEffect(&next.status, registrationKey, effectKey)
	effect.State = string(EffectStateDispatchAuthorized)
	effect.Dispatch = &storedDispatch{Token: dispatchToken(registrationKey, effectKey, effect.ObjectBinding.UIDDigest)}
	return next, nil
}

func MarkEffectTerminal(state FenceState, registrationKey, effectKey string, evidence EffectTerminalEvidence) (FenceState, error) {
	effect, _, ok := findEffect(&state.status, registrationKey, effectKey)
	if !ok {
		return state, ErrEffectIdentityConflict
	}
	if len(evidence.ReasonCode) > MaxTerminalReasonCodeBytes {
		return state, ErrTerminalEvidenceTooLarge
	}
	if evidence.ReasonCode == "" || !validTerminalOutcome(evidence.Outcome) || ValidateIdentityDigest(evidence.EvidenceDigest) != nil {
		return state, ErrStoredStatusInvalid
	}
	kind := EffectKind(effect.Identity.Kind)
	if (kind == EffectKindUnitExecution && evidence.CloseoutVariant != CloseoutVariantUnitExecuted) || (isWindowKind(kind) && evidence.CloseoutVariant != CloseoutVariantObservedTerminal) {
		return state, ErrEffectCloseoutVariantMismatch
	}
	terminal := storedTerminal{Outcome: string(evidence.Outcome), ReasonCode: evidence.ReasonCode, EvidenceDigest: evidence.EvidenceDigest, RecoveryRequired: evidence.RecoveryRequired}
	if EffectState(effect.State) == EffectStateTerminal {
		if effect.CloseoutVariant == string(evidence.CloseoutVariant) && effect.Terminal != nil && *effect.Terminal == terminal {
			return state, nil
		}
		return state, ErrEffectStateRegression
	}
	if kind == EffectKindUnitExecution && (EffectState(effect.State) != EffectStateDispatchAuthorized || effect.Dispatch == nil) {
		return state, ErrEffectDispatchRequired
	}
	if isWindowKind(kind) && (EffectState(effect.State) != EffectStateObjectBound || effect.ObjectBinding == nil) {
		return state, ErrEffectObjectNotBound
	}
	next := cloneState(state)
	effect, _, _ = findEffect(&next.status, registrationKey, effectKey)
	effect.State = string(EffectStateTerminal)
	effect.CloseoutVariant = string(evidence.CloseoutVariant)
	effect.Terminal = &terminal
	return next, nil
}

func ConsumeEffect(state FenceState, registrationKey, effectKey string) (FenceState, error) {
	effect, _, ok := findEffect(&state.status, registrationKey, effectKey)
	if !ok {
		return state, ErrEffectIdentityConflict
	}
	if EffectState(effect.State) == EffectStateConsumed {
		return state, nil
	}
	if EffectState(effect.State) != EffectStateTerminal {
		return state, ErrEffectStateRegression
	}
	if EffectKind(effect.Identity.Kind) == EffectKindUnitExecution && (state.status.InstallationManifest == nil || state.status.InstallationManifest.CommitReadback == nil) {
		return state, ErrDrainNotCommitted
	}
	next := cloneState(state)
	effect, _, _ = findEffect(&next.status, registrationKey, effectKey)
	effect.State = string(EffectStateConsumed)
	effect.Tombstone = &storedConsumedTombstone{IdentityDigest: consumedTombstoneDigest(effectKey)}
	return next, nil
}

func ConsumeRegistration(state FenceState, registrationKey string) (FenceState, error) {
	registration, _, ok := findRegistration(&state.status, registrationKey)
	if !ok {
		return state, ErrRegistrationIdentityConflict
	}
	for _, effect := range registration.Effects {
		if EffectState(effect.State) != EffectStateConsumed {
			return state, ErrEffectStateRegression
		}
	}
	next := cloneState(state)
	registration, _, _ = findRegistration(&next.status, registrationKey)
	registration.Phase = string(RegistrationPhaseConsumed)
	return next, nil
}

func DrainNamespace(state FenceState, namespaceUID types.UID) (FenceState, error) {
	next := cloneState(state)
	for index := range next.status.Namespaces {
		if next.status.Namespaces[index].UID == namespaceUID {
			if next.status.Namespaces[index].Manifest != nil {
				return state, ErrRegistryDraining
			}
			registrationKeys := registrationKeysForNamespace(next.status, namespaceUID)
			effectKeys := effectKeysForNamespace(next.status, namespaceUID)
			next.status.Namespaces[index].Manifest = newManifest("namespace:"+string(namespaceUID), registrationKeys, effectKeys)
			return next, nil
		}
	}
	return state, ErrIdentityIncomplete
}

func DrainInstallation(state FenceState) (FenceState, error) {
	if state.status.InstallationManifest != nil {
		return state, ErrRegistryDraining
	}
	next := cloneState(state)
	next.status.InstallationManifest = newManifest("installation:"+next.status.InstallationEpoch, allRegistrationKeys(next.status), allEffectKeys(next.status))
	return next, nil
}

func BindActiveFenceReadback(state FenceState, readback FenceObjectReadback) (FenceState, error) {
	if err := validateReadback(state, readback); err != nil {
		return state, err
	}
	next := cloneState(state)
	next.activeReadback = &readback
	return next, nil
}

func BindDrainCommitReadback(state FenceState, scope DrainScope, readback FenceObjectReadback) (FenceState, error) {
	if err := validateReadback(state, readback); err != nil {
		return state, err
	}
	next := cloneState(state)
	manifest := manifestForScope(&next.status, scope)
	if manifest == nil {
		return state, ErrFenceReadbackMismatch
	}
	if manifest.CommitReadback != nil {
		return state, ErrDrainReadbackAlreadyBound
	}
	manifest.CommitReadback = &storedReadback{UIDDigest: digestExternalUID(readback.UID), ResourceVersionDigest: digestExternalRV(readback.ResourceVersion), StoredStatusDigest: readback.StoredStatusDigest}
	return next, nil
}

func RebindDrainCommitReadback(state FenceState, scope DrainScope, readback FenceObjectReadback) (FenceState, error) {
	manifest := manifestForScope(&state.status, scope)
	if manifest != nil && manifest.CommitReadback != nil {
		return state, ErrDrainReadbackAlreadyBound
	}
	return BindDrainCommitReadback(state, scope, readback)
}

func validateReadback(state FenceState, readback FenceObjectReadback) error {
	if readback.UID == "" || readback.ResourceVersion == "" || readback.StoredStatusDigest == "" || readback.UID != state.fenceIdentity.UID {
		return ErrFenceReadbackMismatch
	}
	bytes, err := StoredProtocolFenceStatusBytes(state)
	if err != nil || readback.StoredStatusDigest != digest(string(bytes)) {
		return ErrFenceReadbackMismatch
	}
	return nil
}

func ActiveDrainIdentity(state FenceState) DrainIdentity {
	if state.activeReadback == nil {
		return DrainIdentity{}
	}
	return DrainIdentity{FenceUIDDigest: digestExternalUID(state.activeReadback.UID), FenceResourceVersionDigest: digestExternalRV(state.activeReadback.ResourceVersion)}
}

func NamespaceDrainIdentity(state FenceState, namespaceUID types.UID) DrainIdentity {
	return drainIdentity(manifestForScope(&state.status, NamespaceDrainScope(namespaceUID)))
}
func InstallationDrainIdentity(state FenceState) DrainIdentity {
	return drainIdentity(state.status.InstallationManifest)
}

func drainIdentity(manifest *storedManifest) DrainIdentity {
	if manifest == nil {
		return DrainIdentity{}
	}
	identity := DrainIdentity{Token: manifest.Token, ManifestDigest: manifest.Digest}
	if manifest.CommitReadback != nil {
		identity.FenceUIDDigest = manifest.CommitReadback.UIDDigest
		identity.FenceResourceVersionDigest = manifest.CommitReadback.ResourceVersionDigest
	}
	return identity
}

func NamespaceManifestDigest(state FenceState, namespaceUID types.UID) string {
	manifest := manifestForScope(&state.status, NamespaceDrainScope(namespaceUID))
	if manifest == nil {
		return ""
	}
	return manifest.Digest
}
func InstallationManifestDigest(state FenceState) string {
	if state.status.InstallationManifest == nil {
		return ""
	}
	return state.status.InstallationManifest.Digest
}
func NamespaceManifestContainsEffect(state FenceState, registrationKey, effectKey string) bool {
	registration, _, ok := findRegistration(&state.status, registrationKey)
	if !ok {
		return false
	}
	manifest := manifestForScope(&state.status, NamespaceDrainScope(registration.Identity.NamespaceUID))
	return manifestContains(manifest, effectKey)
}
func InstallationManifestContainsEffect(state FenceState, _, effectKey string) bool {
	return manifestContains(state.status.InstallationManifest, effectKey)
}

func manifestContains(manifest *storedManifest, effectKey string) bool {
	if manifest == nil {
		return false
	}
	for _, key := range manifest.EffectKeys {
		if key == effectKey {
			return true
		}
	}
	return false
}

func RegistrationCount(state FenceState) int { return len(state.status.Registrations) }
func NamespaceCount(state FenceState) int    { return len(state.status.Namespaces) }
func TotalEffectCount(state FenceState) int {
	total := 0
	for _, registration := range state.status.Registrations {
		total += len(registration.Effects)
	}
	return total
}
func EffectCount(state FenceState, registrationKey string) int {
	registration, _, ok := findRegistration(&state.status, registrationKey)
	if !ok {
		return 0
	}
	return len(registration.Effects)
}
func EffectStateOf(state FenceState, registrationKey, effectKey string) EffectState {
	effect, _, ok := findEffect(&state.status, registrationKey, effectKey)
	if !ok {
		return EffectStateUnknown
	}
	return EffectState(effect.State)
}
func CanonicalManifestDigest(state FenceState) string {
	data, _ := json.Marshal([]any{state.status.Namespaces, state.status.InstallationManifest})
	return digest(string(data))
}

func DecodeStoredProtocolFenceStatus(data []byte, identity FenceObjectIdentity) (FenceState, error) {
	var status storedStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return FenceState{}, ErrStoredStatusInvalid
	}
	wantIntegrity := status.IntegrityDigest
	status.IntegrityDigest = ""
	unsigned, _ := json.Marshal(status)
	if wantIntegrity != digest("kubeblocks.io/reconfigure-stored-status-integrity/v1\x00"+string(unsigned)) {
		return FenceState{}, ErrStoredStatusIntegrity
	}
	status.IntegrityDigest = wantIntegrity
	if status.FenceUIDDigest == "" {
		return FenceState{}, ErrStoredStatusInvalid
	}
	if status.FenceUIDDigest != digestExternalUID(identity.UID) {
		return FenceState{}, ErrFenceReadbackMismatch
	}
	if err := validateStoredStatus(status); err != nil {
		return FenceState{}, err
	}
	return FenceState{status: status, fenceIdentity: identity}, nil
}

func validateStoredStatus(status storedStatus) error {
	if status.ProtocolVersion != protocolFenceStatusV1 || status.InstallationEpoch == "" || ValidateIdentityDigest(status.FenceUIDDigest) != nil {
		return ErrStoredStatusInvalid
	}
	namespaceSet := map[types.UID]bool{}
	for _, namespace := range status.Namespaces {
		if namespace.UID == "" || namespaceSet[namespace.UID] {
			return ErrStoredStatusInvalid
		}
		namespaceSet[namespace.UID] = true
	}
	registrationKeys := map[string]bool{}
	effectKeys := map[string]bool{}
	for registrationIndex := range status.Registrations {
		registration := &status.Registrations[registrationIndex]
		key, err := RegistrationKey(registrationIdentityRuntime(registration.Identity))
		if err != nil || key != registration.Key || registrationKeys[key] || !namespaceSet[registration.Identity.NamespaceUID] {
			return ErrStoredStatusInvalid
		}
		registrationKeys[key] = true
		phase := RegistrationPhase(registration.Phase)
		if phase != RegistrationPhaseActive && phase != RegistrationPhaseConsumed {
			return ErrStoredStatusInvalid
		}
		allConsumed := true
		for effectIndex := range registration.Effects {
			effect := &registration.Effects[effectIndex]
			key, err := EffectKey(registration.Key, effectIdentityRuntime(effect.Identity))
			if err != nil || key != effect.Key || effectKeys[key] {
				return ErrStoredStatusInvalid
			}
			effectKeys[key] = true
			if err := validateStoredEffect(status, *registration, *effect); err != nil {
				return err
			}
			allConsumed = allConsumed && EffectState(effect.State) == EffectStateConsumed
		}
		if phase == RegistrationPhaseConsumed && !allConsumed {
			return ErrStoredStatusInvalid
		}
	}
	if err := validateManifest(status, status.InstallationManifest, "installation:"+status.InstallationEpoch, allRegistrationKeys(status), allEffectKeys(status)); err != nil {
		return err
	}
	for _, namespace := range status.Namespaces {
		if err := validateManifest(status, namespace.Manifest, "namespace:"+string(namespace.UID), registrationKeysForNamespace(status, namespace.UID), effectKeysForNamespace(status, namespace.UID)); err != nil {
			return err
		}
	}
	return nil
}

func validateStoredEffect(status storedStatus, registration storedRegistration, effect storedEffect) error {
	state := EffectState(effect.State)
	kind := EffectKind(effect.Identity.Kind)
	if kind != EffectKindUnitExecution && !isWindowKind(kind) {
		return ErrStoredStatusInvalid
	}
	validateObject := func() bool {
		return effect.ObjectBinding != nil && effect.ObjectBinding.Target == expectedTarget(effectIdentityRuntime(effect.Identity)) && ValidateIdentityDigest(effect.ObjectBinding.UIDDigest) == nil && ValidateIdentityDigest(effect.ObjectBinding.ResourceVersionDigest) == nil
	}
	validateDispatch := func() bool {
		return effect.Dispatch != nil && validateObject() && effect.Dispatch.Token == dispatchToken(registration.Key, effect.Key, effect.ObjectBinding.UIDDigest)
	}
	validateTerminal := func() bool {
		return effect.Terminal != nil && effect.Terminal.ReasonCode != "" && len(effect.Terminal.ReasonCode) <= MaxTerminalReasonCodeBytes && validTerminalOutcome(EffectTerminalOutcome(effect.Terminal.Outcome)) && ValidateIdentityDigest(effect.Terminal.EvidenceDigest) == nil
	}
	validateTombstone := func() bool {
		return effect.Tombstone != nil && effect.Tombstone.IdentityDigest == consumedTombstoneDigest(effect.Key)
	}
	switch state {
	case EffectStatePlanned:
		if effect.CloseoutVariant != "" || effect.ObjectBinding != nil || effect.Dispatch != nil || effect.Terminal != nil || effect.NotFound != nil || effect.Tombstone != nil {
			return ErrStoredStatusInvalid
		}
	case EffectStateObjectBound:
		if effect.CloseoutVariant != "" || !validateObject() || effect.Dispatch != nil || effect.Terminal != nil || effect.NotFound != nil || effect.Tombstone != nil {
			return ErrStoredStatusInvalid
		}
	case EffectStateDispatchAuthorized:
		if kind != EffectKindUnitExecution || effect.CloseoutVariant != "" || !validateDispatch() || effect.Terminal != nil || effect.NotFound != nil || effect.Tombstone != nil {
			return ErrStoredStatusInvalid
		}
	case EffectStateTerminal:
		if effect.Tombstone != nil || effect.NotFound != nil || !validateTerminal() {
			return ErrStoredStatusInvalid
		}
		switch EffectCloseoutVariant(effect.CloseoutVariant) {
		case CloseoutVariantUnitExecuted:
			if kind != EffectKindUnitExecution || !validateDispatch() {
				return ErrStoredStatusInvalid
			}
		case CloseoutVariantObservedTerminal:
			if !isWindowKind(kind) || !validateObject() || effect.Dispatch != nil {
				return ErrStoredStatusInvalid
			}
		default:
			return ErrStoredStatusInvalid
		}
	case EffectStateConsumed:
		if !validateTombstone() {
			return ErrStoredStatusInvalid
		}
		switch EffectCloseoutVariant(effect.CloseoutVariant) {
		case CloseoutVariantUnitExecuted:
			if kind != EffectKindUnitExecution || status.InstallationManifest == nil || status.InstallationManifest.CommitReadback == nil || !validateDispatch() || !validateTerminal() || effect.NotFound != nil {
				return ErrStoredStatusInvalid
			}
		case CloseoutVariantObservedTerminal:
			if !isWindowKind(kind) || !validateObject() || effect.Dispatch != nil || !validateTerminal() || effect.NotFound != nil {
				return ErrStoredStatusInvalid
			}
		case CloseoutVariantNotFound:
			if !isWindowKind(kind) || effect.ObjectBinding != nil || effect.Dispatch != nil || effect.Terminal != nil || !validNotFound(status, registration, effect) {
				return ErrStoredStatusInvalid
			}
		default:
			return ErrStoredStatusInvalid
		}
	default:
		return ErrStoredStatusInvalid
	}
	return nil
}

func validNotFound(status storedStatus, registration storedRegistration, effect storedEffect) bool {
	value := effect.NotFound
	if value == nil || value.Target != expectedTarget(effectIdentityRuntime(effect.Identity)) || ValidateIdentityDigest(value.LookupResourceVersionDigest) != nil || ValidateIdentityDigest(value.EvidenceDigest) != nil || value.EvidenceDigest != notFoundEvidenceDigest(*value) {
		return false
	}
	identity := DrainIdentity{Token: value.DrainToken, ManifestDigest: value.ManifestDigest, FenceUIDDigest: value.FenceUIDDigest, FenceResourceVersionDigest: value.FenceResourceVersionDigest}
	return matchingCommittedManifest(status, registration.Identity.NamespaceUID, identity) != nil
}

func validateManifest(_ storedStatus, manifest *storedManifest, scope string, expectedRegistrationKeys, expectedEffectKeys []string) error {
	if manifest == nil {
		return nil
	}
	if manifest.Token != drainToken(scope, expectedRegistrationKeys, expectedEffectKeys) || manifest.Digest != manifestDigest(expectedRegistrationKeys, expectedEffectKeys) || !stringSlicesEqual(manifest.RegistrationKeys, expectedRegistrationKeys) || !stringSlicesEqual(manifest.EffectKeys, expectedEffectKeys) {
		return ErrStoredStatusInvalid
	}
	if manifest.CommitReadback != nil && (ValidateIdentityDigest(manifest.CommitReadback.UIDDigest) != nil || ValidateIdentityDigest(manifest.CommitReadback.ResourceVersionDigest) != nil || ValidateIdentityDigest(manifest.CommitReadback.StoredStatusDigest) != nil) {
		return ErrStoredStatusInvalid
	}
	return nil
}

func RequiredCloseoutReserve(state FenceState) int {
	current, _ := marshalStoredStatus(state.status)
	closeout := maxLegalCloseoutStatus(state.status)
	closeoutBytes, _ := marshalStoredStatus(closeout)
	return len(closeoutBytes) - len(current)
}

func maxLegalCloseoutStatus(status storedStatus) storedStatus {
	closeout := cloneStatus(status)
	var allKeys []string
	var allRegistrationKeys []string
	for index := range closeout.Namespaces {
		registrationKeys := registrationKeysForNamespace(closeout, closeout.Namespaces[index].UID)
		effectKeys := effectKeysForNamespace(closeout, closeout.Namespaces[index].UID)
		allRegistrationKeys = append(allRegistrationKeys, registrationKeys...)
		allKeys = append(allKeys, effectKeys...)
		closeout.Namespaces[index].Manifest = newManifest("namespace:"+string(closeout.Namespaces[index].UID), registrationKeys, effectKeys)
		closeout.Namespaces[index].Manifest.CommitReadback = maxReadback()
	}
	closeout.InstallationManifest = newManifest("installation:"+closeout.InstallationEpoch, allRegistrationKeys, allKeys)
	closeout.InstallationManifest.CommitReadback = maxReadback()
	for r := range closeout.Registrations {
		closeout.Registrations[r].Phase = string(RegistrationPhaseConsumed)
		for e := range closeout.Registrations[r].Effects {
			registration := &closeout.Registrations[r]
			manifest := manifestForScope(&closeout, NamespaceDrainScope(registration.Identity.NamespaceUID))
			options := legalStoredCloseoutEffects(registration.Key, registration.Effects[e], manifest)
			selected := options[0]
			selectedBytes, _ := json.Marshal(selected)
			for _, option := range options[1:] {
				optionBytes, _ := json.Marshal(option)
				if len(optionBytes) > len(selectedBytes) {
					selected = option
					selectedBytes = optionBytes
				}
			}
			registration.Effects[e] = selected
		}
	}
	return closeout
}

func legalStoredCloseoutEffects(registrationKey string, planned storedEffect, manifest *storedManifest) []storedEffect {
	base := planned
	base.State = string(EffectStateConsumed)
	base.Tombstone = &storedConsumedTombstone{IdentityDigest: consumedTombstoneDigest(base.Key)}
	object := maxObjectBinding(base.Identity)
	terminal := &storedTerminal{Outcome: string(EffectTerminalManual), ReasonCode: strings.Repeat("R", MaxTerminalReasonCodeBytes), EvidenceDigest: digest("max-terminal-evidence"), RecoveryRequired: true}
	kind := EffectKind(base.Identity.Kind)
	if kind == EffectKindUnitExecution {
		base.CloseoutVariant = string(CloseoutVariantUnitExecuted)
		base.ObjectBinding = object
		base.Dispatch = &storedDispatch{Token: dispatchToken(registrationKey, base.Key, object.UIDDigest)}
		base.Terminal = terminal
		return []storedEffect{base}
	}
	if !isWindowKind(kind) {
		return nil
	}
	notFound := base
	notFound.CloseoutVariant = string(CloseoutVariantNotFound)
	notFound.NotFound = &storedNotFound{Target: object.Target, DrainToken: manifest.Token, ManifestDigest: manifest.Digest, FenceUIDDigest: manifest.CommitReadback.UIDDigest, FenceResourceVersionDigest: manifest.CommitReadback.ResourceVersionDigest, LookupResourceVersionDigest: digestExternalRV(strings.Repeat("9", maxFenceResourceVersionBytes))}
	notFound.NotFound.EvidenceDigest = notFoundEvidenceDigest(*notFound.NotFound)
	observed := base
	observed.CloseoutVariant = string(CloseoutVariantObservedTerminal)
	observed.ObjectBinding = object
	observed.Terminal = terminal
	return []storedEffect{notFound, observed}
}

func withinCapacity(state FenceState, limits RegistryLimits) bool {
	bytes, err := marshalStoredStatus(state.status)
	return err == nil && len(bytes)+RequiredCloseoutReserve(state)+ReservedProtocolFenceStatusBytes <= limits.MaxStatusBytes
}

func expectedTarget(identity EffectIdentity) EffectObjectTarget {
	kind := "GateWindow"
	if identity.Kind == EffectKindUnitExecution {
		kind = "UnitExecution"
	}
	return EffectObjectTarget{"protocol.kubeblocks.io/v1alpha1", kind, identity.Namespace, identity.DeterministicName}
}

func isWindowKind(kind EffectKind) bool {
	return kind == EffectKindTargetFenceWindow || kind == EffectKindCredentialWindow
}

func validTerminalOutcome(outcome EffectTerminalOutcome) bool {
	return outcome == EffectTerminalSucceeded || outcome == EffectTerminalFailed || outcome == EffectTerminalManual
}
func dispatchToken(registrationKey, effectKey, objectUIDDigest string) string {
	return digest("kubeblocks.io/reconfigure-dispatch/v1\x00" + registrationKey + "\x00" + effectKey + "\x00" + objectUIDDigest)
}
func consumedTombstoneDigest(effectKey string) string {
	return digest("kubeblocks.io/reconfigure-consumed/v1\x00" + effectKey)
}
func notFoundEvidenceDigest(v storedNotFound) string {
	return digest(strings.Join([]string{"kubeblocks.io/reconfigure-notfound-evidence/v1", v.Target.APIVersion, v.Target.Kind, v.Target.Namespace, v.Target.Name, v.DrainToken, v.ManifestDigest, v.FenceUIDDigest, v.FenceResourceVersionDigest, v.LookupResourceVersionDigest}, "\x00"))
}
func drainToken(scope string, registrationKeys, effectKeys []string) string {
	return digest("kubeblocks.io/reconfigure-drain-token/v1\x00" + scope + "\x00" + strings.Join(registrationKeys, "\x00") + "\x00" + strings.Join(effectKeys, "\x00"))
}
func manifestDigest(registrationKeys, effectKeys []string) string {
	return digest("kubeblocks.io/reconfigure-manifest/v1\x00" + strings.Join(registrationKeys, "\x00") + "\x00" + strings.Join(effectKeys, "\x00"))
}
func newManifest(scope string, registrationKeys, effectKeys []string) *storedManifest {
	copiedRegistrations := append([]string(nil), registrationKeys...)
	copiedEffects := append([]string(nil), effectKeys...)
	return &storedManifest{Token: drainToken(scope, copiedRegistrations, copiedEffects), Digest: manifestDigest(copiedRegistrations, copiedEffects), RegistrationKeys: copiedRegistrations, EffectKeys: copiedEffects}
}
func maxReadback() *storedReadback {
	return &storedReadback{digestExternalUID(types.UID("protocol-fence-uid-1")), digestExternalRV(strings.Repeat("9", maxFenceResourceVersionBytes)), digest("max-stored-status")}
}
func maxObjectBinding(identity storedEffectIdentity) *storedObjectBinding {
	return &storedObjectBinding{Target: expectedTarget(effectIdentityRuntime(identity)), UIDDigest: digestExternalUID(types.UID(strings.Repeat("u", maxEffectObjectUIDBytes))), ResourceVersionDigest: digestExternalRV(strings.Repeat("9", maxEffectObjectResourceVersionBytes))}
}

func newManifestReadbackIdentity(manifest *storedManifest) DrainIdentity {
	return drainIdentity(manifest)
}
func matchingCommittedManifest(status storedStatus, namespaceUID types.UID, identity DrainIdentity) *storedManifest {
	candidates := []*storedManifest{manifestForScope(&status, NamespaceDrainScope(namespaceUID)), status.InstallationManifest}
	for _, manifest := range candidates {
		if manifest == nil || manifest.CommitReadback == nil {
			continue
		}
		current := newManifestReadbackIdentity(manifest)
		if current == identity {
			return manifest
		}
	}
	return nil
}
func manifestForScope(status *storedStatus, scope DrainScope) *storedManifest {
	if scope.installation {
		return status.InstallationManifest
	}
	for index := range status.Namespaces {
		if status.Namespaces[index].UID == scope.namespaceUID {
			return status.Namespaces[index].Manifest
		}
	}
	return nil
}
func hasNamespace(status storedStatus, uid types.UID) bool {
	for _, namespace := range status.Namespaces {
		if namespace.UID == uid {
			return true
		}
	}
	return false
}
func anyNamespaceDraining(status storedStatus) bool {
	for _, namespace := range status.Namespaces {
		if namespace.Manifest != nil {
			return true
		}
	}
	return false
}

func hasUncommittedDrain(status storedStatus) bool {
	if status.InstallationManifest != nil && status.InstallationManifest.CommitReadback == nil {
		return true
	}
	for _, namespace := range status.Namespaces {
		if namespace.Manifest != nil && namespace.Manifest.CommitReadback == nil {
			return true
		}
	}
	return false
}
func allEffectKeys(status storedStatus) []string {
	var keys []string
	for _, registration := range status.Registrations {
		for _, effect := range registration.Effects {
			keys = append(keys, effect.Key)
		}
	}
	return keys
}
func allRegistrationKeys(status storedStatus) []string {
	keys := make([]string, 0, len(status.Registrations))
	for _, registration := range status.Registrations {
		keys = append(keys, registration.Key)
	}
	return keys
}
func registrationKeysForNamespace(status storedStatus, uid types.UID) []string {
	var keys []string
	for _, registration := range status.Registrations {
		if registration.Identity.NamespaceUID == uid {
			keys = append(keys, registration.Key)
		}
	}
	return keys
}
func effectKeysForNamespace(status storedStatus, uid types.UID) []string {
	var keys []string
	for _, registration := range status.Registrations {
		if registration.Identity.NamespaceUID != uid {
			continue
		}
		for _, effect := range registration.Effects {
			keys = append(keys, effect.Key)
		}
	}
	return keys
}
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func cloneStatus(status storedStatus) storedStatus {
	data, _ := json.Marshal(status)
	var cloned storedStatus
	_ = json.Unmarshal(data, &cloned)
	return cloned
}
func findRegistration(status *storedStatus, key string) (*storedRegistration, int, bool) {
	for index := range status.Registrations {
		if status.Registrations[index].Key == key {
			return &status.Registrations[index], index, true
		}
	}
	return nil, -1, false
}
func findEffect(status *storedStatus, registrationKey, effectKey string) (*storedEffect, *storedRegistration, bool) {
	registration, _, ok := findRegistration(status, registrationKey)
	if !ok {
		return nil, nil, false
	}
	for index := range registration.Effects {
		if registration.Effects[index].Key == effectKey {
			return &registration.Effects[index], registration, true
		}
	}
	return nil, registration, false
}
