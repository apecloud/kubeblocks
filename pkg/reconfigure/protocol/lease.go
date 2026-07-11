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
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"sync"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	coordinationclient "k8s.io/client-go/kubernetes/typed/coordination/v1"
)

const (
	SessionKeyAlgorithmEd25519 = "Ed25519"

	LeaseSessionEpochAnnotation          = "reconfigure.kubeblocks.io/session-epoch"
	LeaseSessionKeyAlgorithmAnnotation   = "reconfigure.kubeblocks.io/session-key-algorithm"
	LeaseSessionPublicKeyAnnotation      = "reconfigure.kubeblocks.io/session-public-key"
	LeaseSessionKeyFingerprintAnnotation = "reconfigure.kubeblocks.io/session-key-fingerprint"

	WindowCreatePermitProtocolV1 = "kubeblocks.io/reconfigure-window-create-permit/v1"
)

var (
	ErrLeaseSessionKeyChanged       = errors.New("lease session key changed")
	ErrLeaseSessionKeyUnavailable   = errors.New("lease session key unavailable")
	ErrLeaseUIDChanged              = errors.New("lease UID changed")
	ErrLeaseSessionHeld             = errors.New("lease session held")
	ErrSessionKeyBindingUnsupported = errors.New("session key binding unsupported")
	ErrWindowCreatePermitInvalid    = errors.New("window create permit invalid")
	ErrWindowImmutableSpecInvalid   = errors.New("window immutable spec invalid")
)

type LeaseSession struct {
	LeaseUID              types.UID
	HolderIdentity        string
	Epoch                 int64
	SessionKeyAlgorithm   string
	SessionPublicKey      []byte
	SessionKeyFingerprint string
}

type LeaseAcquireRequest struct {
	HolderIdentity        string
	NextEpoch             int64
	SessionKeyAlgorithm   string
	SessionPublicKey      []byte
	SessionKeyFingerprint string
}

type LeaseRenewRequest struct {
	HolderIdentity        string
	Epoch                 int64
	SessionKeyAlgorithm   string
	SessionPublicKey      []byte
	SessionKeyFingerprint string
}

type SessionKeyPair struct {
	Algorithm  string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

type LeaseRecoveryRequest struct {
	HolderIdentity string
	KeyPair        SessionKeyPair
}

func AcquireLeaseSession(current LeaseSession, request LeaseAcquireRequest) (LeaseSession, error) {
	if request.HolderIdentity == "" || request.NextEpoch != current.Epoch+1 || !validSessionPublicKey(request.SessionKeyAlgorithm, request.SessionPublicKey, request.SessionKeyFingerprint) {
		return current, ErrLeaseSessionKeyChanged
	}
	return LeaseSession{LeaseUID: current.LeaseUID, HolderIdentity: request.HolderIdentity, Epoch: request.NextEpoch, SessionKeyAlgorithm: request.SessionKeyAlgorithm, SessionPublicKey: append([]byte(nil), request.SessionPublicKey...), SessionKeyFingerprint: request.SessionKeyFingerprint}, nil
}

func RenewLeaseSession(current LeaseSession, request LeaseRenewRequest) (LeaseSession, error) {
	if request.HolderIdentity != current.HolderIdentity || request.Epoch != current.Epoch || request.SessionKeyAlgorithm != current.SessionKeyAlgorithm || request.SessionKeyFingerprint != current.SessionKeyFingerprint || !bytes.Equal(request.SessionPublicKey, current.SessionPublicKey) {
		return current, ErrLeaseSessionKeyChanged
	}
	return current, nil
}

func RecoverLeaseSession(current LeaseSession, request LeaseRecoveryRequest) (LeaseSession, error) {
	if request.HolderIdentity != current.HolderIdentity || request.KeyPair.Algorithm != current.SessionKeyAlgorithm || !bytes.Equal(request.KeyPair.PublicKey, current.SessionPublicKey) || len(request.KeyPair.PrivateKey) != ed25519.PrivateKeySize || !bytes.Equal(request.KeyPair.PrivateKey.Public().(ed25519.PublicKey), current.SessionPublicKey) {
		return current, ErrLeaseSessionKeyUnavailable
	}
	return current, nil
}

func validSessionPublicKey(algorithm string, publicKey []byte, fingerprint string) bool {
	return algorithm == SessionKeyAlgorithmEd25519 && len(publicKey) == ed25519.PublicKeySize && ValidateIdentityDigest(fingerprint) == nil && fingerprint == fingerprintSessionKey(publicKey)
}

func fingerprintSessionKey(publicKey []byte) string { return digest(string(publicKey)) }
func encodeSessionKey(publicKey []byte) string {
	return base64.RawStdEncoding.EncodeToString(publicKey)
}
func decodeSessionKey(value string) ([]byte, error)  { return base64.RawStdEncoding.DecodeString(value) }
func encodeSessionPublicKey(publicKey []byte) string { return encodeSessionKey(publicKey) }

type EvidenceFingerprintAlgorithm string

const EvidenceFingerprintAlgorithmHMACSHA256 EvidenceFingerprintAlgorithm = "HMAC-SHA256"

type GateKind string

const (
	GateKindTargetFence GateKind = "TargetFence"
	GateKindCredential  GateKind = "Credential"
)

type TargetFenceWindowIdentity struct {
	TargetStructuralKey  string `json:"targetStructuralKey"`
	ExpectedTemplateHash string `json:"expectedTemplateHash"`
}

type CredentialWindowIdentity struct {
	PriorGateUID            types.UID `json:"priorGateUID"`
	TargetFingerprintDomain string    `json:"targetFingerprintDomain"`
}

type WindowImmutableSpec struct {
	Name                  string                       `json:"name"`
	PhysicalAPIID         string                       `json:"physicalAPIID"`
	WorkloadNamespaceName string                       `json:"workloadNamespaceName"`
	WorkloadNamespaceUID  types.UID                    `json:"workloadNamespaceUID"`
	ExecutionUID          types.UID                    `json:"executionUID"`
	AttemptID             string                       `json:"attemptID"`
	ProtocolVersion       string                       `json:"protocolVersion"`
	GateKind              GateKind                     `json:"gateKind"`
	DurationSeconds       int64                        `json:"durationSeconds"`
	ProtocolFenceUID      types.UID                    `json:"protocolFenceUID"`
	InstallationEpoch     string                       `json:"installationEpoch"`
	Target                EffectObjectTarget           `json:"target"`
	KeySecretName         string                       `json:"keySecretName"`
	KeySecretUID          types.UID                    `json:"keySecretUID"`
	KeyVersion            int64                        `json:"keyVersion"`
	KeyAlgorithm          EvidenceFingerprintAlgorithm `json:"keyAlgorithm"`
	TargetFence           *TargetFenceWindowIdentity   `json:"targetFence,omitempty"`
	Credential            *CredentialWindowIdentity    `json:"credential,omitempty"`
}

func ValidateWindowImmutableSpec(spec WindowImmutableSpec) error {
	if spec.Name == "" || spec.PhysicalAPIID == "" || spec.WorkloadNamespaceName == "" || spec.WorkloadNamespaceUID == "" || spec.ExecutionUID == "" || spec.AttemptID == "" || spec.ProtocolVersion == "" || spec.DurationSeconds <= 0 || spec.ProtocolFenceUID == "" || spec.InstallationEpoch == "" || spec.Target.APIVersion == "" || spec.Target.Kind == "" || spec.Target.Namespace == "" || spec.Target.Name == "" || spec.KeySecretName == "" || spec.KeySecretUID == "" || spec.KeyVersion <= 0 || spec.KeyAlgorithm != EvidenceFingerprintAlgorithmHMACSHA256 {
		return ErrWindowImmutableSpecInvalid
	}
	switch spec.GateKind {
	case GateKindTargetFence:
		if spec.TargetFence == nil || spec.Credential != nil || spec.TargetFence.TargetStructuralKey == "" || ValidateIdentityDigest(spec.TargetFence.ExpectedTemplateHash) != nil {
			return ErrWindowImmutableSpecInvalid
		}
	case GateKindCredential:
		if spec.Credential == nil || spec.TargetFence != nil || spec.Credential.PriorGateUID == "" || spec.Credential.TargetFingerprintDomain != "kubeblocks.io/credential-target/v1" {
			return ErrWindowImmutableSpecInvalid
		}
	default:
		return ErrWindowImmutableSpecInvalid
	}
	return nil
}

func CanonicalWindowImmutableSpecDigest(spec WindowImmutableSpec) string {
	data, _ := json.Marshal(spec)
	return digest("kubeblocks.io/reconfigure-window-immutable-spec/v1\x00" + string(data))
}

type WindowCreatePermit struct {
	ProtocolVersion              string
	GateKind                     GateKind
	WindowSpecDigest             string
	LeaseUID                     types.UID
	LeaseEpoch                   int64
	LeaseKeyAlgorithm            string
	LeaseKeyFingerprint          string
	ProtocolFenceUID             types.UID
	ProtocolFenceResourceVersion string
	InstallationEpoch            string
	AuthorityUID                 types.UID
	AuthorityResourceVersion     string
	PriorGateUID                 types.UID
	PriorGateResourceVersion     string
	RegistrationKey              string
	EffectKey                    string
	EffectIdentityDigest         string
	EffectState                  EffectState
	Signature                    []byte
}

type WindowPermitContext struct {
	ProtocolVersion              string
	ExpectedGateKind             GateKind
	ExpectedWindowSpec           WindowImmutableSpec
	CurrentLeaseUID              types.UID
	CurrentLeaseEpoch            int64
	CurrentLeaseKeyAlgorithm     string
	CurrentLeaseKeyFingerprint   string
	CurrentLeasePublicKey        ed25519.PublicKey
	ProtocolFenceUID             types.UID
	ProtocolFenceResourceVersion string
	InstallationEpoch            string
	AuthorityUID                 types.UID
	AuthorityResourceVersion     string
	PriorGateUID                 types.UID
	PriorGateResourceVersion     string
	RegistrationKey              string
	EffectKey                    string
	EffectIdentityDigest         string
	EffectState                  EffectState
}

type windowPermitBinding struct {
	ProtocolVersion              string
	GateKind                     GateKind
	WindowSpecDigest             string
	LeaseUID                     types.UID
	LeaseEpoch                   int64
	LeaseKeyAlgorithm            string
	LeaseKeyFingerprint          string
	ProtocolFenceUID             types.UID
	ProtocolFenceResourceVersion string
	InstallationEpoch            string
	AuthorityUID                 types.UID
	AuthorityResourceVersion     string
	PriorGateUID                 types.UID
	PriorGateResourceVersion     string
	RegistrationKey              string
	EffectKey                    string
	EffectIdentityDigest         string
	EffectState                  EffectState
}

func CanonicalWindowCreatePermitBytes(permit WindowCreatePermit) ([]byte, error) {
	unsigned := permit
	unsigned.Signature = nil
	data, err := json.Marshal(unsigned)
	if err != nil {
		return nil, err
	}
	return append([]byte(WindowCreatePermitProtocolV1+"\x00"), data...), nil
}

func CanonicalProtocolDigest(domain string, data []byte) []byte {
	sum := sha256Bytes(append(append([]byte(domain), 0), data...))
	return sum
}

func sha256Bytes(data []byte) []byte {
	sum := sha256Sum(data)
	return sum[:]
}

func WindowCreatePermitDigest(permit WindowCreatePermit) []byte {
	canonical, _ := CanonicalWindowCreatePermitBytes(permit)
	return CanonicalProtocolDigest(WindowCreatePermitProtocolV1, canonical)
}

func ValidateWindowCreatePermit(context WindowPermitContext, permit WindowCreatePermit) error {
	if context.EffectState != EffectStatePlanned {
		return ErrEffectNotPlanned
	}
	if ValidateWindowImmutableSpec(context.ExpectedWindowSpec) != nil || CanonicalWindowImmutableSpecDigest(context.ExpectedWindowSpec) != permit.WindowSpecDigest {
		return ErrWindowCreatePermitInvalid
	}
	if context.ProtocolVersion != WindowCreatePermitProtocolV1 || context.ExpectedGateKind != context.ExpectedWindowSpec.GateKind {
		return ErrWindowCreatePermitInvalid
	}
	permitBinding := windowBindingFromPermit(permit)
	contextBinding := windowBindingFromContext(context)
	if permitBinding != contextBinding || !validWindowPermitBinding(permitBinding) {
		return ErrWindowCreatePermitInvalid
	}
	if context.ProtocolFenceUID != context.ExpectedWindowSpec.ProtocolFenceUID || context.InstallationEpoch != context.ExpectedWindowSpec.InstallationEpoch {
		return ErrWindowCreatePermitInvalid
	}
	if context.ExpectedWindowSpec.GateKind == GateKindCredential && context.PriorGateUID != context.ExpectedWindowSpec.Credential.PriorGateUID {
		return ErrWindowCreatePermitInvalid
	}
	if !validSessionPublicKey(permit.LeaseKeyAlgorithm, context.CurrentLeasePublicKey, permit.LeaseKeyFingerprint) || len(permit.Signature) != ed25519.SignatureSize || !ed25519.Verify(context.CurrentLeasePublicKey, WindowCreatePermitDigest(permit), permit.Signature) {
		return ErrWindowCreatePermitInvalid
	}
	return nil
}

func validWindowPermitBinding(binding windowPermitBinding) bool {
	if binding.ProtocolVersion == "" || binding.GateKind == "" || binding.LeaseUID == "" || binding.LeaseEpoch <= 0 || binding.LeaseKeyAlgorithm == "" || binding.ProtocolFenceUID == "" || binding.ProtocolFenceResourceVersion == "" || binding.InstallationEpoch == "" || binding.AuthorityUID == "" || binding.AuthorityResourceVersion == "" || binding.PriorGateUID == "" || binding.PriorGateResourceVersion == "" {
		return false
	}
	return ValidateIdentityDigest(binding.WindowSpecDigest) == nil && ValidateIdentityDigest(binding.LeaseKeyFingerprint) == nil && ValidateIdentityDigest(binding.RegistrationKey) == nil && ValidateIdentityDigest(binding.EffectKey) == nil && ValidateIdentityDigest(binding.EffectIdentityDigest) == nil
}

func windowBindingFromPermit(permit WindowCreatePermit) windowPermitBinding {
	return windowPermitBinding{
		ProtocolVersion:              permit.ProtocolVersion,
		GateKind:                     permit.GateKind,
		WindowSpecDigest:             permit.WindowSpecDigest,
		LeaseUID:                     permit.LeaseUID,
		LeaseEpoch:                   permit.LeaseEpoch,
		LeaseKeyAlgorithm:            permit.LeaseKeyAlgorithm,
		LeaseKeyFingerprint:          permit.LeaseKeyFingerprint,
		ProtocolFenceUID:             permit.ProtocolFenceUID,
		ProtocolFenceResourceVersion: permit.ProtocolFenceResourceVersion,
		InstallationEpoch:            permit.InstallationEpoch,
		AuthorityUID:                 permit.AuthorityUID,
		AuthorityResourceVersion:     permit.AuthorityResourceVersion,
		PriorGateUID:                 permit.PriorGateUID,
		PriorGateResourceVersion:     permit.PriorGateResourceVersion,
		RegistrationKey:              permit.RegistrationKey,
		EffectKey:                    permit.EffectKey,
		EffectIdentityDigest:         permit.EffectIdentityDigest,
		EffectState:                  permit.EffectState,
	}
}

func windowBindingFromContext(context WindowPermitContext) windowPermitBinding {
	return windowPermitBinding{
		ProtocolVersion:              context.ProtocolVersion,
		GateKind:                     context.ExpectedGateKind,
		WindowSpecDigest:             CanonicalWindowImmutableSpecDigest(context.ExpectedWindowSpec),
		LeaseUID:                     context.CurrentLeaseUID,
		LeaseEpoch:                   context.CurrentLeaseEpoch,
		LeaseKeyAlgorithm:            context.CurrentLeaseKeyAlgorithm,
		LeaseKeyFingerprint:          context.CurrentLeaseKeyFingerprint,
		ProtocolFenceUID:             context.ProtocolFenceUID,
		ProtocolFenceResourceVersion: context.ProtocolFenceResourceVersion,
		InstallationEpoch:            context.InstallationEpoch,
		AuthorityUID:                 context.AuthorityUID,
		AuthorityResourceVersion:     context.AuthorityResourceVersion,
		PriorGateUID:                 context.PriorGateUID,
		PriorGateResourceVersion:     context.PriorGateResourceVersion,
		RegistrationKey:              context.RegistrationKey,
		EffectKey:                    context.EffectKey,
		EffectIdentityDigest:         context.EffectIdentityDigest,
		EffectState:                  context.EffectState,
	}
}

type SessionLeaseLockConfig struct {
	Client         coordinationclient.CoordinationV1Interface
	Namespace      string
	Name           string
	HolderIdentity string
	KeyPair        SessionKeyPair
	LeaseDuration  time.Duration
}

type SessionLeaseLock struct {
	config SessionLeaseLockConfig
	mu     sync.RWMutex

	leaseUID       types.UID
	committedEpoch int64
}

func NewSessionLeaseLock(config SessionLeaseLockConfig) (*SessionLeaseLock, error) {
	duration := config.LeaseDuration / time.Second
	if config.Client == nil || config.Namespace == "" || config.Name == "" || config.HolderIdentity == "" ||
		config.LeaseDuration < time.Second || config.LeaseDuration%time.Second != 0 || duration > math.MaxInt32 {
		return nil, ErrLeaseSessionKeyUnavailable
	}
	if !validSessionPublicKey(config.KeyPair.Algorithm, config.KeyPair.PublicKey, fingerprintSessionKey(config.KeyPair.PublicKey)) {
		return nil, ErrLeaseSessionKeyUnavailable
	}
	if len(config.KeyPair.PrivateKey) != ed25519.PrivateKeySize || !bytes.Equal(config.KeyPair.PrivateKey.Public().(ed25519.PublicKey), config.KeyPair.PublicKey) {
		return nil, ErrLeaseSessionKeyUnavailable
	}
	config.KeyPair.PublicKey = append(ed25519.PublicKey(nil), config.KeyPair.PublicKey...)
	config.KeyPair.PrivateKey = nil
	return &SessionLeaseLock{config: config}, nil
}

func (lock *SessionLeaseLock) Acquire(ctx context.Context) (LeaseSession, error) {
	leases := lock.config.Client.Leases(lock.config.Namespace)
	lease, err := leases.Get(ctx, lock.config.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		now := metav1.NewMicroTime(time.Now())
		holder := lock.config.HolderIdentity
		duration := int32(lock.config.LeaseDuration / time.Second)
		created := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: lock.config.Name, Namespace: lock.config.Namespace, Annotations: lock.annotations(1)}, Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder, LeaseDurationSeconds: &duration, AcquireTime: &now, RenewTime: &now}}
		lease, err = leases.Create(ctx, created, metav1.CreateOptions{})
		if err != nil {
			return LeaseSession{}, err
		}
	case err != nil:
		return LeaseSession{}, err
	default:
		current, parseErr := leaseSessionFromLease(lease)
		if parseErr != nil {
			if !leaseExpired(lease, time.Now()) || lease.UID == "" || lease.ResourceVersion == "" {
				return LeaseSession{}, parseErr
			}
			uid := lease.UID
			resourceVersion := lease.ResourceVersion
			if err := leases.Delete(ctx, lock.config.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &resourceVersion}}); err != nil {
				return LeaseSession{}, err
			}
			now := metav1.NewMicroTime(time.Now())
			holder := lock.config.HolderIdentity
			duration := int32(lock.config.LeaseDuration / time.Second)
			replacement := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: lock.config.Name, Namespace: lock.config.Namespace, Annotations: lock.annotations(1)}, Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder, LeaseDurationSeconds: &duration, AcquireTime: &now, RenewTime: &now}}
			lease, err = leases.Create(ctx, replacement, metav1.CreateOptions{})
			if err != nil {
				return LeaseSession{}, err
			}
			break
		}
		if !leaseExpired(lease, time.Now()) && current.HolderIdentity != lock.config.HolderIdentity {
			return LeaseSession{}, ErrLeaseSessionHeld
		}
		next, acquireErr := AcquireLeaseSession(current, LeaseAcquireRequest{HolderIdentity: lock.config.HolderIdentity, NextEpoch: current.Epoch + 1, SessionKeyAlgorithm: lock.config.KeyPair.Algorithm, SessionPublicKey: lock.config.KeyPair.PublicKey, SessionKeyFingerprint: fingerprintSessionKey(lock.config.KeyPair.PublicKey)})
		if acquireErr != nil {
			return LeaseSession{}, acquireErr
		}
		updated := lease.DeepCopy()
		updated.Annotations = lock.annotations(next.Epoch)
		updated.Spec.HolderIdentity = &next.HolderIdentity
		duration := int32(lock.config.LeaseDuration / time.Second)
		updated.Spec.LeaseDurationSeconds = &duration
		now := metav1.NewMicroTime(time.Now())
		updated.Spec.AcquireTime = &now
		updated.Spec.RenewTime = &now
		lease, err = leases.Update(ctx, updated, metav1.UpdateOptions{})
		if err != nil {
			return LeaseSession{}, err
		}
	}
	session, err := leaseSessionFromLease(lease)
	if err != nil {
		return LeaseSession{}, err
	}
	lock.commitSession(session)
	return session, nil
}

func (lock *SessionLeaseLock) Renew(ctx context.Context) error {
	lease, err := lock.config.Client.Leases(lock.config.Namespace).Get(ctx, lock.config.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	leaseUID, committedEpoch := lock.committedSession()
	if leaseUID != "" && lease.UID != leaseUID {
		return ErrLeaseUIDChanged
	}
	current, err := leaseSessionFromLease(lease)
	if err != nil {
		return ErrLeaseSessionKeyChanged
	}
	_, err = RenewLeaseSession(current, LeaseRenewRequest{HolderIdentity: lock.config.HolderIdentity, Epoch: committedEpoch, SessionKeyAlgorithm: lock.config.KeyPair.Algorithm, SessionPublicKey: lock.config.KeyPair.PublicKey, SessionKeyFingerprint: fingerprintSessionKey(lock.config.KeyPair.PublicKey)})
	if err != nil {
		return err
	}
	updated := lease.DeepCopy()
	now := metav1.NewMicroTime(time.Now())
	updated.Spec.RenewTime = &now
	_, err = lock.config.Client.Leases(lock.config.Namespace).Update(ctx, updated, metav1.UpdateOptions{})
	return err
}

func (lock *SessionLeaseLock) RecoverCommittedSession(ctx context.Context, keyPair SessionKeyPair) (LeaseSession, error) {
	lease, err := lock.config.Client.Leases(lock.config.Namespace).Get(ctx, lock.config.Name, metav1.GetOptions{})
	if err != nil {
		return LeaseSession{}, err
	}
	leaseUID, _ := lock.committedSession()
	if leaseUID != "" && lease.UID != leaseUID {
		return LeaseSession{}, ErrLeaseUIDChanged
	}
	current, err := leaseSessionFromLease(lease)
	if err != nil {
		return LeaseSession{}, err
	}
	recovered, err := RecoverLeaseSession(current, LeaseRecoveryRequest{HolderIdentity: lock.config.HolderIdentity, KeyPair: keyPair})
	if err != nil {
		return LeaseSession{}, err
	}
	lock.commitSession(recovered)
	return recovered, nil
}

func (lock *SessionLeaseLock) commitSession(session LeaseSession) {
	lock.mu.Lock()
	defer lock.mu.Unlock()
	lock.leaseUID = session.LeaseUID
	lock.committedEpoch = session.Epoch
}

func (lock *SessionLeaseLock) committedSession() (types.UID, int64) {
	lock.mu.RLock()
	defer lock.mu.RUnlock()
	return lock.leaseUID, lock.committedEpoch
}

func (lock *SessionLeaseLock) annotations(epoch int64) map[string]string {
	return map[string]string{LeaseSessionEpochAnnotation: strconv.FormatInt(epoch, 10), LeaseSessionKeyAlgorithmAnnotation: lock.config.KeyPair.Algorithm, LeaseSessionPublicKeyAnnotation: encodeSessionKey(lock.config.KeyPair.PublicKey), LeaseSessionKeyFingerprintAnnotation: fingerprintSessionKey(lock.config.KeyPair.PublicKey)}
}

func leaseSessionFromLease(lease *coordinationv1.Lease) (LeaseSession, error) {
	if lease == nil || lease.Spec.HolderIdentity == nil {
		return LeaseSession{}, ErrLeaseSessionKeyUnavailable
	}
	epoch, err := strconv.ParseInt(lease.Annotations[LeaseSessionEpochAnnotation], 10, 64)
	if err != nil {
		return LeaseSession{}, ErrLeaseSessionKeyUnavailable
	}
	publicKey, err := decodeSessionKey(lease.Annotations[LeaseSessionPublicKeyAnnotation])
	if err != nil || !validSessionPublicKey(lease.Annotations[LeaseSessionKeyAlgorithmAnnotation], publicKey, lease.Annotations[LeaseSessionKeyFingerprintAnnotation]) {
		return LeaseSession{}, ErrLeaseSessionKeyUnavailable
	}
	return LeaseSession{LeaseUID: lease.UID, HolderIdentity: *lease.Spec.HolderIdentity, Epoch: epoch, SessionKeyAlgorithm: lease.Annotations[LeaseSessionKeyAlgorithmAnnotation], SessionPublicKey: publicKey, SessionKeyFingerprint: lease.Annotations[LeaseSessionKeyFingerprintAnnotation]}, nil
}

func leaseExpired(lease *coordinationv1.Lease, now time.Time) bool {
	if lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil || *lease.Spec.LeaseDurationSeconds <= 0 {
		return false
	}
	return lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second).Before(now)
}

func ValidateSessionKeyBindingLock(lock any) error {
	if _, ok := lock.(*SessionLeaseLock); !ok {
		return ErrSessionKeyBindingUnsupported
	}
	return nil
}

// Local wrappers keep crypto implementation details out of protocol call sites.
func sha256Sum(data []byte) [32]byte { return sha256.Sum256(data) }
