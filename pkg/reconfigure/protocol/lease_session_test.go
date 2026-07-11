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
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	types "k8s.io/apimachinery/pkg/types"
)

func TestLeaseAcquireBindsStrictNextEpochAndRealSessionKey(t *testing.T) {
	publicKey, _ := testEd25519KeyPair(t)
	request := LeaseAcquireRequest{
		HolderIdentity:        "controller-pod-uid-1/process-1",
		NextEpoch:             1,
		SessionKeyAlgorithm:   SessionKeyAlgorithmEd25519,
		SessionPublicKey:      append([]byte(nil), publicKey...),
		SessionKeyFingerprint: sessionKeyFingerprint(publicKey),
	}
	lease := LeaseSession{LeaseUID: types.UID("lease-uid-1")}

	acquired, err := AcquireLeaseSession(lease, request)
	require.NoError(t, err)
	require.Equal(t, int64(1), acquired.Epoch)
	require.Equal(t, request.HolderIdentity, acquired.HolderIdentity)
	require.Equal(t, request.SessionKeyFingerprint, acquired.SessionKeyFingerprint)

	request.SessionPublicKey[0] ^= 0xff
	require.Equal(t, publicKey, ed25519.PublicKey(acquired.SessionPublicKey), "state must not alias caller key bytes")

	for name, mutate := range map[string]func(*LeaseAcquireRequest){
		"same epoch":           func(v *LeaseAcquireRequest) { v.NextEpoch = 1 },
		"skipped epoch":        func(v *LeaseAcquireRequest) { v.NextEpoch = 3 },
		"empty holder":         func(v *LeaseAcquireRequest) { v.HolderIdentity = "" },
		"empty algorithm":      func(v *LeaseAcquireRequest) { v.SessionKeyAlgorithm = "" },
		"unknown algorithm":    func(v *LeaseAcquireRequest) { v.SessionKeyAlgorithm = "RSA" },
		"empty key":            func(v *LeaseAcquireRequest) { v.SessionPublicKey = nil },
		"malformed key":        func(v *LeaseAcquireRequest) { v.SessionPublicKey = []byte("short") },
		"empty fingerprint":    func(v *LeaseAcquireRequest) { v.SessionKeyFingerprint = "" },
		"fingerprint mismatch": func(v *LeaseAcquireRequest) { v.SessionKeyFingerprint = testDigest("other-key") },
		"noncanonical digest": func(v *LeaseAcquireRequest) {
			v.SessionKeyFingerprint = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		},
	} {
		t.Run(name, func(t *testing.T) {
			nextPublicKey, _ := testEd25519KeyPair(t)
			next := LeaseAcquireRequest{
				HolderIdentity:        "controller-pod-uid-1/process-2",
				NextEpoch:             2,
				SessionKeyAlgorithm:   SessionKeyAlgorithmEd25519,
				SessionPublicKey:      nextPublicKey,
				SessionKeyFingerprint: sessionKeyFingerprint(nextPublicKey),
			}
			mutate(&next)
			_, err := AcquireLeaseSession(acquired, next)
			require.Error(t, err)
		})
	}
}

func TestLeaseRenewRequiresByteExactSessionIdentity(t *testing.T) {
	current := testLeaseSession(t, 7, "controller-pod-uid-1/process-1")
	exact := LeaseRenewRequest{
		HolderIdentity:        current.HolderIdentity,
		Epoch:                 current.Epoch,
		SessionKeyAlgorithm:   current.SessionKeyAlgorithm,
		SessionPublicKey:      append([]byte(nil), current.SessionPublicKey...),
		SessionKeyFingerprint: current.SessionKeyFingerprint,
	}

	renewed, err := RenewLeaseSession(current, exact)
	require.NoError(t, err)
	require.Equal(t, current, renewed)

	for name, mutate := range map[string]func(*LeaseRenewRequest){
		"holder":      func(v *LeaseRenewRequest) { v.HolderIdentity = "controller-pod-uid-2/process-1" },
		"epoch":       func(v *LeaseRenewRequest) { v.Epoch++ },
		"algorithm":   func(v *LeaseRenewRequest) { v.SessionKeyAlgorithm = "RSA" },
		"public key":  func(v *LeaseRenewRequest) { v.SessionPublicKey[0] ^= 0xff },
		"fingerprint": func(v *LeaseRenewRequest) { v.SessionKeyFingerprint = testDigest("other-key") },
	} {
		t.Run(name, func(t *testing.T) {
			changed := exact
			changed.SessionPublicKey = append([]byte(nil), exact.SessionPublicKey...)
			mutate(&changed)
			_, err := RenewLeaseSession(current, changed)
			require.ErrorIs(t, err, ErrLeaseSessionKeyChanged)
		})
	}
}

func TestLeaseRecoveryRequiresExactLocalKeyAndRestartUsesNextEpoch(t *testing.T) {
	publicKey, privateKey := testEd25519KeyPair(t)
	current := LeaseSession{
		LeaseUID:              types.UID("lease-uid-1"),
		HolderIdentity:        "controller-pod-uid-1/process-1",
		Epoch:                 7,
		SessionKeyAlgorithm:   SessionKeyAlgorithmEd25519,
		SessionPublicKey:      publicKey,
		SessionKeyFingerprint: sessionKeyFingerprint(publicKey),
	}
	exactKeyPair := SessionKeyPair{
		Algorithm:  SessionKeyAlgorithmEd25519,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}

	recovered, err := RecoverLeaseSession(current, LeaseRecoveryRequest{
		HolderIdentity: current.HolderIdentity,
		KeyPair:        exactKeyPair,
	})
	require.NoError(t, err)
	require.Equal(t, current, recovered)

	_, wrongPrivateKey := testEd25519KeyPair(t)
	wrongPublicKey, wrongKeyPairPrivate := testEd25519KeyPair(t)
	for name, keyPair := range map[string]SessionKeyPair{
		"public fingerprint only": {
			Algorithm: SessionKeyAlgorithmEd25519,
			PublicKey: publicKey,
		},
		"wrong private key": {
			Algorithm:  SessionKeyAlgorithmEd25519,
			PublicKey:  publicKey,
			PrivateKey: wrongPrivateKey,
		},
		"wrong keypair": {
			Algorithm:  SessionKeyAlgorithmEd25519,
			PublicKey:  wrongPublicKey,
			PrivateKey: wrongKeyPairPrivate,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := RecoverLeaseSession(current, LeaseRecoveryRequest{
				HolderIdentity: current.HolderIdentity,
				KeyPair:        keyPair,
			})
			require.ErrorIs(t, err, ErrLeaseSessionKeyUnavailable)
		})
	}

	nextPublicKey, _ := testEd25519KeyPair(t)
	next, err := AcquireLeaseSession(current, LeaseAcquireRequest{
		HolderIdentity:        "controller-pod-uid-1/process-2",
		NextEpoch:             current.Epoch + 1,
		SessionKeyAlgorithm:   SessionKeyAlgorithmEd25519,
		SessionPublicKey:      nextPublicKey,
		SessionKeyFingerprint: sessionKeyFingerprint(nextPublicKey),
	})
	require.NoError(t, err)
	require.Equal(t, current.Epoch+1, next.Epoch)
	require.NotEqual(t, current.HolderIdentity, next.HolderIdentity)
	require.NotEqual(t, current.SessionKeyFingerprint, next.SessionKeyFingerprint)
}

func TestWindowPermitBindsEveryLeaseGateAndEffectField(t *testing.T) {
	publicKey, privateKey := testEd25519KeyPair(t)
	permit := testWindowCreatePermit(publicKey)
	canonical, err := CanonicalWindowCreatePermitBytes(permit)
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(canonical, []byte("kubeblocks.io/reconfigure-window-create-permit/v1\x00")))
	permit.Signature = ed25519.Sign(privateKey, WindowCreatePermitDigest(permit))
	context := testWindowPermitContext(permit, publicKey)

	require.NoError(t, ValidateWindowCreatePermit(context, permit))

	permitMutations := map[string]func(*WindowCreatePermit){
		"protocol version":   func(v *WindowCreatePermit) { v.ProtocolVersion = "v2" },
		"gate kind":          func(v *WindowCreatePermit) { v.GateKind = "Credential" },
		"window spec digest": func(v *WindowCreatePermit) { v.WindowSpecDigest = testDigest("other-window-spec") },
		"lease UID":          func(v *WindowCreatePermit) { v.LeaseUID = types.UID("lease-uid-2") },
		"lease epoch":        func(v *WindowCreatePermit) { v.LeaseEpoch++ },
		"lease algorithm":    func(v *WindowCreatePermit) { v.LeaseKeyAlgorithm = "RSA" },
		"lease fingerprint":  func(v *WindowCreatePermit) { v.LeaseKeyFingerprint = testDigest("other-key") },
		"protocol fence UID": func(v *WindowCreatePermit) { v.ProtocolFenceUID = types.UID("protocol-fence-uid-2") },
		"protocol fence RV":  func(v *WindowCreatePermit) { v.ProtocolFenceResourceVersion = "202" },
		"installation epoch": func(v *WindowCreatePermit) { v.InstallationEpoch = "install-8" },
		"authority UID":      func(v *WindowCreatePermit) { v.AuthorityUID = types.UID("authority-uid-2") },
		"authority RV":       func(v *WindowCreatePermit) { v.AuthorityResourceVersion = "302" },
		"prior gate UID":     func(v *WindowCreatePermit) { v.PriorGateUID = types.UID("prior-gate-uid-2") },
		"prior gate RV":      func(v *WindowCreatePermit) { v.PriorGateResourceVersion = "402" },
		"registration key":   func(v *WindowCreatePermit) { v.RegistrationKey = testDigest("registration-2") },
		"effect key":         func(v *WindowCreatePermit) { v.EffectKey = testDigest("effect-key-2") },
		"effect digest":      func(v *WindowCreatePermit) { v.EffectIdentityDigest = testDigest("effect-2") },
		"effect state":       func(v *WindowCreatePermit) { v.EffectState = EffectStateObjectBound },
	}
	for name, mutate := range permitMutations {
		t.Run("signed payload tamper "+name, func(t *testing.T) {
			changed := permit
			changed.Signature = append([]byte(nil), permit.Signature...)
			mutate(&changed)
			err := ValidateWindowCreatePermit(context, changed)
			require.ErrorIs(t, err, ErrWindowCreatePermitInvalid)
		})
	}

	contextMutations := map[string]func(*WindowPermitContext){
		"protocol version":   func(v *WindowPermitContext) { v.ProtocolVersion = "v2" },
		"gate kind":          func(v *WindowPermitContext) { v.ExpectedGateKind = "Credential" },
		"lease UID":          func(v *WindowPermitContext) { v.CurrentLeaseUID = types.UID("lease-uid-2") },
		"lease epoch":        func(v *WindowPermitContext) { v.CurrentLeaseEpoch++ },
		"lease algorithm":    func(v *WindowPermitContext) { v.CurrentLeaseKeyAlgorithm = "RSA" },
		"lease fingerprint":  func(v *WindowPermitContext) { v.CurrentLeaseKeyFingerprint = testDigest("other-key") },
		"protocol fence UID": func(v *WindowPermitContext) { v.ProtocolFenceUID = types.UID("protocol-fence-uid-2") },
		"protocol fence RV":  func(v *WindowPermitContext) { v.ProtocolFenceResourceVersion = "202" },
		"installation epoch": func(v *WindowPermitContext) { v.InstallationEpoch = "install-8" },
		"authority UID":      func(v *WindowPermitContext) { v.AuthorityUID = types.UID("authority-uid-2") },
		"authority RV":       func(v *WindowPermitContext) { v.AuthorityResourceVersion = "302" },
		"prior gate UID":     func(v *WindowPermitContext) { v.PriorGateUID = types.UID("prior-gate-uid-2") },
		"prior gate RV":      func(v *WindowPermitContext) { v.PriorGateResourceVersion = "402" },
		"registration key":   func(v *WindowPermitContext) { v.RegistrationKey = testDigest("registration-2") },
		"effect key":         func(v *WindowPermitContext) { v.EffectKey = testDigest("effect-key-2") },
		"effect digest":      func(v *WindowPermitContext) { v.EffectIdentityDigest = testDigest("effect-2") },
	}
	for name, mutate := range contextMutations {
		t.Run("fresh context mismatch "+name, func(t *testing.T) {
			changed := context
			changed.CurrentLeasePublicKey = append([]byte(nil), context.CurrentLeasePublicKey...)
			mutate(&changed)
			err := ValidateWindowCreatePermit(changed, permit)
			require.ErrorIs(t, err, ErrWindowCreatePermitInvalid)
		})
	}

	for name, mutate := range map[string]func(*WindowImmutableSpec){
		"name":                    func(v *WindowImmutableSpec) { v.Name = "other-window" },
		"physical API":            func(v *WindowImmutableSpec) { v.PhysicalAPIID = "physical-api-2" },
		"workload namespace name": func(v *WindowImmutableSpec) { v.WorkloadNamespaceName = "workload-2" },
		"workload namespace UID":  func(v *WindowImmutableSpec) { v.WorkloadNamespaceUID = types.UID("namespace-uid-2") },
		"execution UID":           func(v *WindowImmutableSpec) { v.ExecutionUID = types.UID("execution-uid-2") },
		"attempt ID":              func(v *WindowImmutableSpec) { v.AttemptID = "attempt-4" },
		"protocol version":        func(v *WindowImmutableSpec) { v.ProtocolVersion = "v2" },
		"gate kind":               func(v *WindowImmutableSpec) { v.GateKind = GateKindCredential },
		"installation epoch":      func(v *WindowImmutableSpec) { v.InstallationEpoch = "install-8" },
		"duration":                func(v *WindowImmutableSpec) { v.DurationSeconds++ },
		"protocol fence UID":      func(v *WindowImmutableSpec) { v.ProtocolFenceUID = types.UID("protocol-fence-uid-2") },
		"target API version":      func(v *WindowImmutableSpec) { v.Target.APIVersion = "wrong.example.io/v1" },
		"target kind":             func(v *WindowImmutableSpec) { v.Target.Kind = "WrongKind" },
		"target namespace":        func(v *WindowImmutableSpec) { v.Target.Namespace = "other-namespace" },
		"target name":             func(v *WindowImmutableSpec) { v.Target.Name = "other-target" },
		"key secret name":         func(v *WindowImmutableSpec) { v.KeySecretName = "key-2" },
		"key secret UID":          func(v *WindowImmutableSpec) { v.KeySecretUID = types.UID("key-secret-uid-2") },
		"key version":             func(v *WindowImmutableSpec) { v.KeyVersion++ },
		"key algorithm":           func(v *WindowImmutableSpec) { v.KeyAlgorithm = "RSA" },
		"target structural key":   func(v *WindowImmutableSpec) { v.TargetFence.TargetStructuralKey = "target-2" },
		"expected template hash":  func(v *WindowImmutableSpec) { v.TargetFence.ExpectedTemplateHash = testDigest("template-2") },
	} {
		t.Run("window spec "+name, func(t *testing.T) {
			changed := context
			changed.ExpectedWindowSpec = cloneWindowImmutableSpec(context.ExpectedWindowSpec)
			mutate(&changed.ExpectedWindowSpec)
			err := ValidateWindowCreatePermit(changed, permit)
			require.ErrorIs(t, err, ErrWindowCreatePermitInvalid)
		})
	}
}

func TestWindowPermitRejectsJointlySignedInvalidBindings(t *testing.T) {
	publicKey, privateKey := testEd25519KeyPair(t)
	basePermit := testWindowCreatePermit(publicKey)
	baseContext := testWindowPermitContext(basePermit, publicKey)

	for name, mutate := range map[string]func(*WindowPermitContext, *WindowCreatePermit){
		"spec protocol Fence UID mismatch": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.ProtocolFenceUID = types.UID("protocol-fence-uid-2")
			permit.ProtocolFenceUID = context.ProtocolFenceUID
		},
		"spec installation epoch mismatch": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.InstallationEpoch = "install-8"
			permit.InstallationEpoch = context.InstallationEpoch
		},
	} {
		t.Run(name, func(t *testing.T) {
			context := baseContext
			permit := basePermit
			mutate(&context, &permit)
			permit.Signature = ed25519.Sign(privateKey, WindowCreatePermitDigest(permit))
			require.ErrorIs(t, ValidateWindowCreatePermit(context, permit), ErrWindowCreatePermitInvalid)
		})
	}

	credentialSpec := testWindowImmutableSpec()
	credentialSpec.GateKind = GateKindCredential
	credentialSpec.TargetFence = nil
	credentialSpec.Credential = &CredentialWindowIdentity{PriorGateUID: types.UID("prior-gate-uid-1"), TargetFingerprintDomain: "kubeblocks.io/credential-target/v1"}
	credentialPermit := testWindowCreatePermitForSpec(publicKey, credentialSpec)
	credentialContext := testWindowPermitContext(credentialPermit, publicKey)
	credentialContext.ExpectedGateKind = GateKindCredential
	credentialContext.ExpectedWindowSpec = credentialSpec
	credentialPermit.PriorGateUID = types.UID("prior-gate-uid-2")
	credentialContext.PriorGateUID = credentialPermit.PriorGateUID
	credentialPermit.Signature = ed25519.Sign(privateKey, WindowCreatePermitDigest(credentialPermit))
	require.ErrorIs(t, ValidateWindowCreatePermit(credentialContext, credentialPermit), ErrWindowCreatePermitInvalid)

	for name, clear := range map[string]func(*WindowPermitContext, *WindowCreatePermit){
		"Lease UID": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.CurrentLeaseUID = ""
			permit.LeaseUID = ""
		},
		"Lease epoch": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.CurrentLeaseEpoch = 0
			permit.LeaseEpoch = 0
		},
		"Fence UID": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.ProtocolFenceUID = ""
			permit.ProtocolFenceUID = ""
		},
		"Fence RV": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.ProtocolFenceResourceVersion = ""
			permit.ProtocolFenceResourceVersion = ""
		},
		"installation epoch": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.InstallationEpoch = ""
			permit.InstallationEpoch = ""
		},
		"authority UID": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.AuthorityUID = ""
			permit.AuthorityUID = ""
		},
		"authority RV": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.AuthorityResourceVersion = ""
			permit.AuthorityResourceVersion = ""
		},
		"prior gate UID": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.PriorGateUID = ""
			permit.PriorGateUID = ""
		},
		"prior gate RV": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.PriorGateResourceVersion = ""
			permit.PriorGateResourceVersion = ""
		},
		"registration key": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.RegistrationKey = ""
			permit.RegistrationKey = ""
		},
		"effect key": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.EffectKey = ""
			permit.EffectKey = ""
		},
		"effect identity digest": func(context *WindowPermitContext, permit *WindowCreatePermit) {
			context.EffectIdentityDigest = ""
			permit.EffectIdentityDigest = ""
		},
	} {
		t.Run("joint empty "+name, func(t *testing.T) {
			context := baseContext
			permit := basePermit
			clear(&context, &permit)
			permit.Signature = ed25519.Sign(privateKey, WindowCreatePermitDigest(permit))
			require.ErrorIs(t, ValidateWindowCreatePermit(context, permit), ErrWindowCreatePermitInvalid)
		})
	}
}

func TestWindowImmutableSpecSeparatesNamespaceAttemptAndGateVariants(t *testing.T) {
	base := testWindowImmutableSpec()
	require.NoError(t, ValidateWindowImmutableSpec(base))
	baseDigest := CanonicalWindowImmutableSpecDigest(base)
	for name, algorithm := range map[string]EvidenceFingerprintAlgorithm{
		"empty evidence algorithm":   "",
		"Lease Ed25519 cross-domain": EvidenceFingerprintAlgorithm(SessionKeyAlgorithmEd25519),
		"unknown evidence algorithm": "SHA512",
	} {
		t.Run(name, func(t *testing.T) {
			invalid := cloneWindowImmutableSpec(base)
			invalid.KeyAlgorithm = algorithm
			require.ErrorIs(t, ValidateWindowImmutableSpec(invalid), ErrWindowImmutableSpecInvalid)
		})
	}

	newNamespace := cloneWindowImmutableSpec(base)
	newNamespace.WorkloadNamespaceUID = types.UID("namespace-uid-2")
	require.NotEqual(t, baseDigest, CanonicalWindowImmutableSpecDigest(newNamespace))

	newAttempt := cloneWindowImmutableSpec(base)
	newAttempt.AttemptID = "attempt-4"
	require.NotEqual(t, baseDigest, CanonicalWindowImmutableSpecDigest(newAttempt))

	credential := cloneWindowImmutableSpec(base)
	credential.GateKind = GateKindCredential
	credential.TargetFence = nil
	credential.Credential = &CredentialWindowIdentity{
		PriorGateUID:            types.UID("prior-gate-uid-1"),
		TargetFingerprintDomain: "kubeblocks.io/credential-target/v1",
	}
	require.NoError(t, ValidateWindowImmutableSpec(credential))
	require.NotEqual(t, baseDigest, CanonicalWindowImmutableSpecDigest(credential))

	for name, invalid := range map[string]WindowImmutableSpec{
		"Credential with TargetFence only": func() WindowImmutableSpec {
			value := cloneWindowImmutableSpec(base)
			value.GateKind = GateKindCredential
			return value
		}(),
		"TargetFence with Credential only": func() WindowImmutableSpec {
			value := cloneWindowImmutableSpec(credential)
			value.GateKind = GateKindTargetFence
			return value
		}(),
		"unknown GateKind": func() WindowImmutableSpec {
			value := cloneWindowImmutableSpec(base)
			value.GateKind = "UnknownGate"
			return value
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, ValidateWindowImmutableSpec(invalid), ErrWindowImmutableSpecInvalid)
		})
	}

	publicKey, privateKey := testEd25519KeyPair(t)
	credentialPermit := testWindowCreatePermitForSpec(publicKey, credential)
	credentialPermit.Signature = ed25519.Sign(privateKey, WindowCreatePermitDigest(credentialPermit))
	credentialContext := testWindowPermitContext(credentialPermit, publicKey)
	credentialContext.ExpectedGateKind = GateKindCredential
	credentialContext.ExpectedWindowSpec = credential
	require.NoError(t, ValidateWindowCreatePermit(credentialContext, credentialPermit))

	for name, mutate := range map[string]func(*WindowImmutableSpec){
		"prior gate UID": func(v *WindowImmutableSpec) {
			v.Credential.PriorGateUID = types.UID("prior-gate-uid-2")
		},
		"fingerprint domain": func(v *WindowImmutableSpec) {
			v.Credential.TargetFingerprintDomain = "kubeblocks.io/credential-target/v2"
		},
	} {
		t.Run("Credential "+name, func(t *testing.T) {
			changedSpec := cloneWindowImmutableSpec(credential)
			mutate(&changedSpec)
			require.NotEqual(t, CanonicalWindowImmutableSpecDigest(credential), CanonicalWindowImmutableSpecDigest(changedSpec))
			changedContext := credentialContext
			changedContext.ExpectedWindowSpec = changedSpec
			require.ErrorIs(t, ValidateWindowCreatePermit(changedContext, credentialPermit), ErrWindowCreatePermitInvalid)
		})
	}

	for name, domain := range map[string]string{
		"empty fingerprint domain":   "",
		"unknown fingerprint domain": "unversioned-domain",
	} {
		t.Run(name, func(t *testing.T) {
			invalid := cloneWindowImmutableSpec(credential)
			invalid.Credential.TargetFingerprintDomain = domain
			require.ErrorIs(t, ValidateWindowImmutableSpec(invalid), ErrWindowImmutableSpecInvalid)
		})
	}
	emptyPriorGate := cloneWindowImmutableSpec(credential)
	emptyPriorGate.Credential.PriorGateUID = ""
	require.ErrorIs(t, ValidateWindowImmutableSpec(emptyPriorGate), ErrWindowImmutableSpecInvalid)

	illegal := cloneWindowImmutableSpec(base)
	illegal.Credential = credential.Credential
	require.ErrorIs(t, ValidateWindowImmutableSpec(illegal), ErrWindowImmutableSpecInvalid)

	missingVariant := cloneWindowImmutableSpec(base)
	missingVariant.TargetFence = nil
	require.ErrorIs(t, ValidateWindowImmutableSpec(missingVariant), ErrWindowImmutableSpecInvalid)
}

func TestWindowPermitCanonicalEncodingSeparatesFieldsAndMessageDomains(t *testing.T) {
	publicKey, privateKey := testEd25519KeyPair(t)
	left := testWindowCreatePermit(publicKey)
	left.AuthorityResourceVersion = "ab"
	left.PriorGateResourceVersion = "c"
	right := testWindowCreatePermit(publicKey)
	right.AuthorityResourceVersion = "a"
	right.PriorGateResourceVersion = "bc"
	require.NotEqual(t, WindowCreatePermitDigest(left), WindowCreatePermitDigest(right))

	signature := ed25519.Sign(privateKey, WindowCreatePermitDigest(left))
	canonical, err := CanonicalWindowCreatePermitBytes(left)
	require.NoError(t, err)
	otherDomainDigest := CanonicalProtocolDigest("kubeblocks.io/reconfigure-lease-proof/v1", canonical)
	require.False(t, ed25519.Verify(publicKey, otherDomainDigest, signature), "a Window CREATE signature must not authorize another protocol message")
}

func TestWindowPermitRejectsInvalidSignatureAndEveryNonPlannedState(t *testing.T) {
	publicKey, privateKey := testEd25519KeyPair(t)
	permit := testWindowCreatePermit(publicKey)
	permit.Signature = ed25519.Sign(privateKey, WindowCreatePermitDigest(permit))
	context := testWindowPermitContext(permit, publicKey)

	wrongPublicKey, _ := testEd25519KeyPair(t)
	wrongKeyContext := context
	wrongKeyContext.CurrentLeasePublicKey = wrongPublicKey
	require.ErrorIs(t, ValidateWindowCreatePermit(wrongKeyContext, permit), ErrWindowCreatePermitInvalid)

	changedSignature := permit
	changedSignature.Signature = append([]byte(nil), permit.Signature...)
	changedSignature.Signature[0] ^= 0xff
	require.ErrorIs(t, ValidateWindowCreatePermit(context, changedSignature), ErrWindowCreatePermitInvalid)

	emptySignature := permit
	emptySignature.Signature = nil
	require.ErrorIs(t, ValidateWindowCreatePermit(context, emptySignature), ErrWindowCreatePermitInvalid)

	unknownAlgorithm := permit
	unknownAlgorithm.LeaseKeyAlgorithm = "RSA"
	require.ErrorIs(t, ValidateWindowCreatePermit(context, unknownAlgorithm), ErrWindowCreatePermitInvalid)

	for _, state := range []EffectState{
		EffectStateUnknown,
		EffectStateObjectBound,
		EffectStateDispatchAuthorized,
		EffectStateTerminal,
		EffectStateManual,
		EffectStateConsumed,
	} {
		t.Run(string(state), func(t *testing.T) {
			changedContext := context
			changedContext.EffectState = state
			err := ValidateWindowCreatePermit(changedContext, permit)
			require.ErrorIs(t, err, ErrEffectNotPlanned)
		})
	}
}

func testLeaseSession(t *testing.T, epoch int64, holder string) LeaseSession {
	t.Helper()
	publicKey, _ := testEd25519KeyPair(t)
	return LeaseSession{
		LeaseUID:              types.UID("lease-uid-1"),
		HolderIdentity:        holder,
		Epoch:                 epoch,
		SessionKeyAlgorithm:   SessionKeyAlgorithmEd25519,
		SessionPublicKey:      publicKey,
		SessionKeyFingerprint: sessionKeyFingerprint(publicKey),
	}
}

func testWindowCreatePermit(publicKey ed25519.PublicKey) WindowCreatePermit {
	return testWindowCreatePermitForSpec(publicKey, testWindowImmutableSpec())
}

func testWindowCreatePermitForSpec(publicKey ed25519.PublicKey, windowSpec WindowImmutableSpec) WindowCreatePermit {
	return WindowCreatePermit{
		ProtocolVersion:              WindowCreatePermitProtocolV1,
		GateKind:                     windowSpec.GateKind,
		WindowSpecDigest:             CanonicalWindowImmutableSpecDigest(windowSpec),
		LeaseUID:                     types.UID("lease-uid-1"),
		LeaseEpoch:                   3,
		LeaseKeyAlgorithm:            SessionKeyAlgorithmEd25519,
		LeaseKeyFingerprint:          sessionKeyFingerprint(publicKey),
		ProtocolFenceUID:             types.UID("protocol-fence-uid-1"),
		ProtocolFenceResourceVersion: "201",
		InstallationEpoch:            "install-7",
		AuthorityUID:                 types.UID("authority-uid-1"),
		AuthorityResourceVersion:     "301",
		PriorGateUID:                 types.UID("prior-gate-uid-1"),
		PriorGateResourceVersion:     "401",
		RegistrationKey:              testDigest("registration-1"),
		EffectKey:                    testDigest("effect-key-1"),
		EffectIdentityDigest:         testDigest("effect-identity-1"),
		EffectState:                  EffectStatePlanned,
	}
}

func testWindowPermitContext(permit WindowCreatePermit, publicKey ed25519.PublicKey) WindowPermitContext {
	return WindowPermitContext{
		ProtocolVersion:              WindowCreatePermitProtocolV1,
		ExpectedGateKind:             GateKindTargetFence,
		ExpectedWindowSpec:           testWindowImmutableSpec(),
		CurrentLeaseUID:              permit.LeaseUID,
		CurrentLeaseEpoch:            permit.LeaseEpoch,
		CurrentLeaseKeyAlgorithm:     permit.LeaseKeyAlgorithm,
		CurrentLeaseKeyFingerprint:   permit.LeaseKeyFingerprint,
		CurrentLeasePublicKey:        publicKey,
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

func testWindowImmutableSpec() WindowImmutableSpec {
	return WindowImmutableSpec{
		Name:                  "target-window-1",
		PhysicalAPIID:         "physical-api-1",
		WorkloadNamespaceName: "workload-1",
		WorkloadNamespaceUID:  types.UID("namespace-uid-1"),
		ExecutionUID:          types.UID("execution-uid-1"),
		AttemptID:             "attempt-3",
		ProtocolVersion:       "kubeblocks.io/reconfigure-window/v1",
		GateKind:              GateKindTargetFence,
		DurationSeconds:       600,
		ProtocolFenceUID:      types.UID("protocol-fence-uid-1"),
		InstallationEpoch:     "install-7",
		Target: EffectObjectTarget{
			APIVersion: "protocol.kubeblocks.io/v1alpha1",
			Kind:       "UnitExecution",
			Namespace:  "kb-system",
			Name:       "unit-1",
		},
		KeySecretName: "evidence-key-v7",
		KeySecretUID:  types.UID("key-secret-uid-1"),
		KeyVersion:    7,
		KeyAlgorithm:  EvidenceFingerprintAlgorithmHMACSHA256,
		TargetFence: &TargetFenceWindowIdentity{
			TargetStructuralKey:  "mysql/component-0/pod-0/mysql",
			ExpectedTemplateHash: testDigest("template-1"),
		},
	}
}

func cloneWindowImmutableSpec(spec WindowImmutableSpec) WindowImmutableSpec {
	cloned := spec
	if spec.TargetFence != nil {
		targetFence := *spec.TargetFence
		cloned.TargetFence = &targetFence
	}
	if spec.Credential != nil {
		credential := *spec.Credential
		cloned.Credential = &credential
	}
	return cloned
}

func testEd25519KeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return publicKey, privateKey
}

func sessionKeyFingerprint(publicKey ed25519.PublicKey) string {
	return testDigest(string(publicKey))
}
