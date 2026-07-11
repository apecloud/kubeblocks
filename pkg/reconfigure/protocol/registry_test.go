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
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

func TestRegistrationKeyUsesVersionedCanonicalIdentity(t *testing.T) {
	identity := testRegistrationIdentity()
	canonical, err := CanonicalRegistrationIdentityBytes(identity)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(canonical), "kubeblocks.io/reconfigure-registration/v1\x00"))

	first, err := RegistrationKey(identity)
	require.NoError(t, err)
	second, err := RegistrationKey(identity)
	require.NoError(t, err)
	require.Equal(t, first, second)

	mutations := map[string]func(*RegistrationIdentity){
		"physical API":       func(v *RegistrationIdentity) { v.PhysicalAPIID = "api-uid-2" },
		"installation epoch": func(v *RegistrationIdentity) { v.InstallationEpoch = "install-8" },
		"execution UID":      func(v *RegistrationIdentity) { v.ExecutionUID = types.UID("execution-uid-2") },
		"attempt ID":         func(v *RegistrationIdentity) { v.AttemptID = "attempt-4" },
		"namespace UID":      func(v *RegistrationIdentity) { v.NamespaceUID = types.UID("namespace-uid-2") },
		"key secret UID":     func(v *RegistrationIdentity) { v.KeySecretUID = types.UID("key-secret-uid-2") },
		"authority UID":      func(v *RegistrationIdentity) { v.AuthorityUID = types.UID("authority-uid-2") },
		"queue UID":          func(v *RegistrationIdentity) { v.QueueUID = types.UID("queue-uid-2") },
	}
	for name, mutate := range mutations {
		t.Run("binds "+name, func(t *testing.T) {
			changed := identity
			mutate(&changed)
			key, err := RegistrationKey(changed)
			require.NoError(t, err)
			require.NotEqual(t, first, key)
		})
	}

	emptyMutations := map[string]func(*RegistrationIdentity){
		"physical API":       func(v *RegistrationIdentity) { v.PhysicalAPIID = "" },
		"installation epoch": func(v *RegistrationIdentity) { v.InstallationEpoch = "" },
		"execution UID":      func(v *RegistrationIdentity) { v.ExecutionUID = "" },
		"attempt ID":         func(v *RegistrationIdentity) { v.AttemptID = "" },
		"namespace UID":      func(v *RegistrationIdentity) { v.NamespaceUID = "" },
		"key secret UID":     func(v *RegistrationIdentity) { v.KeySecretUID = "" },
		"authority UID":      func(v *RegistrationIdentity) { v.AuthorityUID = "" },
		"queue UID":          func(v *RegistrationIdentity) { v.QueueUID = "" },
	}
	for name, clear := range emptyMutations {
		t.Run("rejects empty "+name, func(t *testing.T) {
			changed := identity
			clear(&changed)
			_, err := RegistrationKey(changed)
			require.ErrorIs(t, err, ErrIdentityIncomplete)
		})
	}

	left := identity
	left.PhysicalAPIID = "ab"
	left.InstallationEpoch = "c"
	right := identity
	right.PhysicalAPIID = "a"
	right.InstallationEpoch = "bc"
	leftKey, err := RegistrationKey(left)
	require.NoError(t, err)
	rightKey, err := RegistrationKey(right)
	require.NoError(t, err)
	require.NotEqual(t, leftKey, rightKey, "field boundaries must not use raw concatenation")
}

func TestEffectKeyUsesVersionedCanonicalIdentity(t *testing.T) {
	effect := testUnitEffect("unit-1")
	registrationKey := testDigest("registration-1")
	canonical, err := CanonicalEffectIdentityBytes(registrationKey, effect)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(canonical), "kubeblocks.io/reconfigure-effect/v1\x00"))

	first, err := EffectKey(registrationKey, effect)
	require.NoError(t, err)
	replayed, err := EffectKey(registrationKey, effect)
	require.NoError(t, err)
	require.Equal(t, first, replayed)
	otherRegistration, err := EffectKey(testDigest("registration-2"), effect)
	require.NoError(t, err)
	require.NotEqual(t, first, otherRegistration)

	for name, invalidRegistrationKey := range map[string]string{
		"empty":        "",
		"unknown alg":  "sha512:0123",
		"short":        "sha256:0123",
		"non hex":      "sha256:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
		"uppercase":    "sha256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"extra prefix": "SHA256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	} {
		t.Run("registration key "+name, func(t *testing.T) {
			_, err := CanonicalEffectIdentityBytes(invalidRegistrationKey, effect)
			require.ErrorIs(t, err, ErrIdentityDigestInvalid)
			_, err = EffectKey(invalidRegistrationKey, effect)
			require.ErrorIs(t, err, ErrIdentityDigestInvalid)
		})
	}

	for name, mutate := range map[string]func(*EffectIdentity){
		"kind":   func(v *EffectIdentity) { v.Kind = EffectKindCredentialWindow },
		"name":   func(v *EffectIdentity) { v.DeterministicName = "unit-2" },
		"digest": func(v *EffectIdentity) { v.FullIdentityDigest = testDigest("unit-2") },
	} {
		t.Run(name, func(t *testing.T) {
			changed := effect
			mutate(&changed)
			key, err := EffectKey(registrationKey, changed)
			require.NoError(t, err)
			require.NotEqual(t, first, key)
		})
	}

	left := effect
	left.DeterministicName = "ab"
	left.FullIdentityDigest = testDigest("c")
	right := effect
	right.DeterministicName = "a"
	right.FullIdentityDigest = testDigest("bc")
	leftKey, err := EffectKey(registrationKey, left)
	require.NoError(t, err)
	rightKey, err := EffectKey(registrationKey, right)
	require.NoError(t, err)
	require.NotEqual(t, leftKey, rightKey)

	actualRegistrationKey, err := RegistrationKey(testRegistrationIdentity())
	require.NoError(t, err)
	require.NotEqual(t, actualRegistrationKey, first, "registration and effect keys need distinct domains")
}

func TestIdentityDigestMustBeCanonicalSHA256(t *testing.T) {
	require.NoError(t, ValidateIdentityDigest(testDigest("valid")))

	for name, digest := range map[string]string{
		"empty":        "",
		"unknown alg":  "sha512:0123",
		"short":        "sha256:0123",
		"non hex":      "sha256:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
		"uppercase":    "sha256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"extra prefix": "SHA256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, ValidateIdentityDigest(digest), ErrIdentityDigestInvalid)
		})
	}
}

func TestFenceStateRequiresServerObjectIdentityBeforeActive(t *testing.T) {
	_, err := NewFenceState("install-7", types.UID("namespace-uid-1"), FenceObjectIdentity{})
	require.ErrorIs(t, err, ErrIdentityIncomplete)

	_, err = NewFenceState("install-7", types.UID("namespace-uid-1"), testFenceObjectIdentity())
	require.NoError(t, err)
}

func TestRegistrationAndEffectReplayAreIdempotent(t *testing.T) {
	limits := generousRegistryLimits()
	identity := testRegistrationIdentity()
	state := newFenceState(t, identity.InstallationEpoch, identity.NamespaceUID, testFenceObjectIdentity())
	registered, registrationKey, err := RegisterExecution(state, identity, limits)
	require.NoError(t, err)
	registeredBytes := canonicalStateBytes(t, registered)

	for i := 0; i < 100; i++ {
		replayed, replayedKey, err := RegisterExecution(registered, identity, limits)
		require.NoError(t, err)
		require.Equal(t, registrationKey, replayedKey)
		require.Equal(t, 1, RegistrationCount(replayed))
		require.Equal(t, registeredBytes, canonicalStateBytes(t, replayed))
	}

	conflicting := identity
	conflicting.AuthorityUID = types.UID("authority-uid-2")
	unchanged, _, err := RegisterExecution(registered, conflicting, limits)
	require.ErrorIs(t, err, ErrRegistrationIdentityConflict)
	requireStateEqual(t, registered, unchanged)

	effect := testUnitEffect("unit-1")
	planned, effectKey, err := PlanEffect(registered, registrationKey, effect, limits)
	require.NoError(t, err)
	plannedBytes := canonicalStateBytes(t, planned)

	for i := 0; i < 100; i++ {
		replayed, replayedKey, err := PlanEffect(planned, registrationKey, effect, limits)
		require.NoError(t, err)
		require.Equal(t, effectKey, replayedKey)
		require.Equal(t, 1, EffectCount(replayed, registrationKey))
		require.Equal(t, plannedBytes, canonicalStateBytes(t, replayed))
	}

	for name, mutate := range map[string]func(*EffectIdentity){
		"digest":    func(v *EffectIdentity) { v.FullIdentityDigest = testDigest("other-unit") },
		"pod UID":   func(v *EffectIdentity) { v.PodUID = types.UID("pod-uid-2") },
		"container": func(v *EffectIdentity) { v.ContainerName = "sidecar" },
		"fence UID": func(v *EffectIdentity) { v.FenceUID = types.UID("fence-uid-2") },
	} {
		t.Run("rejects replay with changed "+name, func(t *testing.T) {
			changed := effect
			mutate(&changed)
			unchanged, returnedKey, err := PlanEffect(planned, registrationKey, changed, limits)
			require.ErrorIs(t, err, ErrEffectIdentityConflict)
			require.Empty(t, returnedKey)
			requireStateEqual(t, planned, unchanged)
		})
	}
}

func TestEffectNameCollisionIsScopedToNamespace(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	first := testUnitEffect("unit-1")
	first.Namespace = "namespace-a"
	second := first
	second.Namespace = "namespace-b"

	planned, firstKey, err := PlanEffect(state, registrationKey, first, limits)
	require.NoError(t, err)
	planned, secondKey, err := PlanEffect(planned, registrationKey, second, limits)
	require.NoError(t, err)
	require.NotEqual(t, firstKey, secondKey)
	require.Equal(t, 2, EffectCount(planned, registrationKey))

	bound, err := ObserveEffectObject(planned, registrationKey, firstKey, presentObservation(first, types.UID("unit-object-uid-a")))
	require.NoError(t, err)
	bound, err = ObserveEffectObject(bound, registrationKey, secondKey, presentObservation(second, types.UID("unit-object-uid-b")))
	require.NoError(t, err)
	require.Equal(t, EffectStateObjectBound, EffectStateOf(bound, registrationKey, firstKey))
	require.Equal(t, EffectStateObjectBound, EffectStateOf(bound, registrationKey, secondKey))

	conflicting := first
	conflicting.FullIdentityDigest = testDigest("different-unit")
	unchanged, returnedKey, err := PlanEffect(bound, registrationKey, conflicting, limits)
	require.ErrorIs(t, err, ErrEffectIdentityConflict)
	require.Empty(t, returnedKey)
	requireStateEqual(t, bound, unchanged)
}

func TestDrainFreezesRegistrationMembership(t *testing.T) {
	for _, drain := range []struct {
		name string
		run  func(FenceState) (FenceState, error)
	}{
		{name: "namespace", run: func(state FenceState) (FenceState, error) {
			return DrainNamespace(state, testRegistrationIdentity().NamespaceUID)
		}},
		{name: "installation", run: DrainInstallation},
	} {
		t.Run(drain.name, func(t *testing.T) {
			state, registrationKey, limits := registeredFence(t)
			draining, err := drain.run(state)
			require.NoError(t, err)

			replayed, replayedKey, err := RegisterExecution(draining, testRegistrationIdentity(), limits)
			require.NoError(t, err)
			require.Equal(t, registrationKey, replayedKey)
			requireStateEqual(t, draining, replayed)

			for name, mutate := range map[string]func(*RegistrationIdentity){
				"conflicting identity": func(v *RegistrationIdentity) {
					v.AuthorityUID = types.UID("authority-uid-2")
				},
				"same namespace": func(v *RegistrationIdentity) {
					v.ExecutionUID = types.UID("execution-uid-2")
					v.AttemptID = "attempt-4"
				},
				"new namespace": func(v *RegistrationIdentity) {
					v.ExecutionUID = types.UID("execution-uid-2")
					v.AttemptID = "attempt-4"
					v.NamespaceUID = types.UID("namespace-uid-2")
					v.AuthorityUID = types.UID("authority-uid-2")
					v.QueueUID = types.UID("queue-uid-2")
				},
			} {
				t.Run(name, func(t *testing.T) {
					identity := testRegistrationIdentity()
					mutate(&identity)
					unchanged, _, err := RegisterExecution(draining, identity, limits)
					require.ErrorIs(t, err, ErrRegistryDraining)
					requireStateEqual(t, draining, unchanged)
				})
			}
		})
	}
}

func TestObjectBindingIsExactAndIdempotent(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	effect := testWindowEffect("target-window-1")
	planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
	require.NoError(t, err)
	present := presentObservation(effect, types.UID("window-uid-1"))
	for name, mutate := range map[string]func(*EffectObjectObservation){
		"empty object UID": func(v *EffectObjectObservation) { v.ObjectUID = "" },
		"empty object RV":  func(v *EffectObjectObservation) { v.ObjectResourceVersion = "" },
	} {
		t.Run(name, func(t *testing.T) {
			incomplete := present
			mutate(&incomplete)
			unchanged, err := ObserveEffectObject(planned, registrationKey, effectKey, incomplete)
			require.ErrorIs(t, err, ErrEffectObservationIncomplete)
			requireStateEqual(t, planned, unchanged)
		})
	}

	bound, err := ObserveEffectObject(planned, registrationKey, effectKey, present)
	require.NoError(t, err)
	replayed, err := ObserveEffectObject(bound, registrationKey, effectKey, present)
	require.NoError(t, err)
	requireStateEqual(t, bound, replayed)

	wrongUID := present
	wrongUID.ObjectUID = types.UID("window-uid-2")
	unchanged, err := ObserveEffectObject(bound, registrationKey, effectKey, wrongUID)
	require.ErrorIs(t, err, ErrEffectObjectIdentityConflict)
	requireStateEqual(t, bound, unchanged)

	for name, observation := range map[string]EffectObjectObservation{
		"terminating": terminatingObservation(effect, present.ObjectUID),
		"API error":   apiErrorObservation(effect),
	} {
		t.Run(name, func(t *testing.T) {
			unchanged, err := ObserveEffectObject(bound, registrationKey, effectKey, observation)
			require.ErrorIs(t, err, ErrEffectObservationInconclusive)
			requireStateEqual(t, bound, unchanged)
		})
	}

	draining, err := DrainNamespace(bound, testRegistrationIdentity().NamespaceUID)
	require.NoError(t, err)
	terminal, err := MarkEffectTerminal(draining, registrationKey, effectKey, observedTerminalEvidence())
	require.NoError(t, err)
	consumed, err := ConsumeEffect(terminal, registrationKey, effectKey)
	require.NoError(t, err)
	var stored storedProtocolFenceStatusDTO
	require.NoError(t, json.Unmarshal(storedStateBytes(t, consumed), &stored))
	storedEffect := stored.Registrations[0].Effects[0]
	require.Equal(t, string(CloseoutVariantObservedTerminal), storedEffect.CloseoutVariant)
	require.NotNil(t, storedEffect.ObjectBinding)
	require.Nil(t, storedEffect.Dispatch)
	require.NotNil(t, storedEffect.Terminal)
	require.Nil(t, storedEffect.NotFound)
	require.NotNil(t, storedEffect.Tombstone)
	restarted, err := DecodeStoredProtocolFenceStatus(storedStateBytes(t, consumed), testFenceObjectIdentity())
	require.NoError(t, err)
	unchanged, err = ObserveEffectObject(consumed, registrationKey, effectKey, present)
	require.ErrorIs(t, err, ErrEffectAlreadyConsumed)
	requireStateEqual(t, consumed, unchanged)
	unchanged, err = ObserveEffectObject(restarted, registrationKey, effectKey, present)
	require.ErrorIs(t, err, ErrEffectAlreadyConsumed)
	requireStateEqual(t, restarted, unchanged)
}

func TestPlannedNotFoundRequiresDrainAndExactFreshObservation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		drain          func(FenceState) (FenceState, error)
		commit         func(FenceState) (FenceState, DrainIdentity)
		otherIdentity  func(FenceState) DrainIdentity
		manifestDigest func(FenceState) string
	}{
		{
			name: "namespace",
			drain: func(state FenceState) (FenceState, error) {
				return DrainNamespace(state, testRegistrationIdentity().NamespaceUID)
			},
			commit: func(state FenceState) (FenceState, DrainIdentity) {
				return commitNamespaceDrain(t, state, "502")
			},
			otherIdentity: func(state FenceState) DrainIdentity {
				other, err := DrainInstallation(state)
				require.NoError(t, err)
				_, identity := commitInstallationDrain(t, other, "502")
				return identity
			},
			manifestDigest: func(state FenceState) string {
				return NamespaceManifestDigest(state, testRegistrationIdentity().NamespaceUID)
			},
		},
		{
			name:  "installation",
			drain: DrainInstallation,
			commit: func(state FenceState) (FenceState, DrainIdentity) {
				return commitInstallationDrain(t, state, "502")
			},
			otherIdentity: func(state FenceState) DrainIdentity {
				other, err := DrainNamespace(state, testRegistrationIdentity().NamespaceUID)
				require.NoError(t, err)
				_, identity := commitNamespaceDrain(t, other, "502")
				return identity
			},
			manifestDigest: InstallationManifestDigest,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, registrationKey, limits := registeredFence(t)
			effect := testWindowEffect("target-window-1")
			planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
			require.NoError(t, err)
			preDrain := notFoundObservation(effect, activeDrainIdentity(t, planned))

			unchanged, err := ObserveEffectObject(planned, registrationKey, effectKey, preDrain)
			require.ErrorIs(t, err, ErrEffectStillCreatable)
			requireStateEqual(t, planned, unchanged)

			draining, err := tc.drain(planned)
			require.NoError(t, err)
			draining, drainIdentity := tc.commit(draining)
			require.NotEmpty(t, drainIdentity.Token)
			require.NotEmpty(t, drainIdentity.ManifestDigest)
			manifestBefore := tc.manifestDigest(draining)

			unchanged, err = ObserveEffectObject(draining, registrationKey, effectKey, preDrain)
			require.ErrorIs(t, err, ErrEffectObservationStale)
			requireStateEqual(t, draining, unchanged)

			for name, staleIdentity := range map[string]DrainIdentity{
				"wrong scope token": tc.otherIdentity(planned),
				"old token": func() DrainIdentity {
					changed := drainIdentity
					changed.Token = "old-drain-token"
					return changed
				}(),
				"wrong manifest": func() DrainIdentity {
					changed := drainIdentity
					changed.ManifestDigest = testDigest("wrong-manifest")
					return changed
				}(),
				"wrong fence UID": func() DrainIdentity {
					changed := drainIdentity
					changed.FenceUIDDigest = externalUIDDigest(types.UID("different-fence-uid"))
					return changed
				}(),
				"wrong fence RV": func() DrainIdentity {
					changed := drainIdentity
					changed.FenceResourceVersionDigest = externalResourceVersionDigest("different-fence-rv")
					return changed
				}(),
			} {
				t.Run(name, func(t *testing.T) {
					unchanged, err := ObserveEffectObject(draining, registrationKey, effectKey, notFoundObservation(effect, staleIdentity))
					require.ErrorIs(t, err, ErrEffectObservationStale)
					requireStateEqual(t, draining, unchanged)
				})
			}

			fresh := notFoundObservation(effect, drainIdentity)
			incomplete := fresh
			incomplete.LookupResourceVersion = ""
			unchanged, err = ObserveEffectObject(draining, registrationKey, effectKey, incomplete)
			require.ErrorIs(t, err, ErrEffectObservationIncomplete)
			requireStateEqual(t, draining, unchanged)

			consumed, err := ObserveEffectObject(draining, registrationKey, effectKey, fresh)
			require.NoError(t, err)
			require.Equal(t, EffectStateConsumed, EffectStateOf(consumed, registrationKey, effectKey))
			require.Equal(t, manifestBefore, tc.manifestDigest(consumed))
			var stored storedProtocolFenceStatusDTO
			require.NoError(t, json.Unmarshal(storedStateBytes(t, consumed), &stored))
			storedEffect := stored.Registrations[0].Effects[0]
			require.Equal(t, string(CloseoutVariantNotFound), storedEffect.CloseoutVariant)
			require.Nil(t, storedEffect.ObjectBinding)
			require.Nil(t, storedEffect.Dispatch)
			require.Nil(t, storedEffect.Terminal)
			require.Equal(t, storedNotFoundEvidenceDTOFrom(fresh), storedEffect.NotFound)
			require.NotNil(t, storedEffect.Tombstone)
			require.Equal(t, expectedConsumedTombstoneDigest(effectKey), storedEffect.Tombstone.IdentityDigest)
			restarted, err := DecodeStoredProtocolFenceStatus(storedStateBytes(t, consumed), testFenceObjectIdentity())
			require.NoError(t, err)

			unchanged, returnedKey, err := PlanEffect(consumed, registrationKey, effect, limits)
			require.ErrorIs(t, err, ErrEffectAlreadyConsumed)
			require.Empty(t, returnedKey)
			requireStateEqual(t, consumed, unchanged)
			changedDigest := effect
			changedDigest.FullIdentityDigest = testDigest("different-target-window")
			unchanged, _, err = PlanEffect(consumed, registrationKey, changedDigest, limits)
			require.ErrorIs(t, err, ErrEffectAlreadyConsumed)
			requireStateEqual(t, consumed, unchanged)
			unchanged, err = ObserveEffectObject(consumed, registrationKey, effectKey, presentObservation(effect, types.UID("window-uid-1")))
			require.ErrorIs(t, err, ErrEffectAlreadyConsumed)
			requireStateEqual(t, consumed, unchanged)
			unchanged, err = AuthorizeDispatch(consumed, registrationKey, effectKey)
			require.ErrorIs(t, err, ErrEffectAlreadyConsumed)
			requireStateEqual(t, consumed, unchanged)
			unchanged, returnedKey, err = PlanEffect(restarted, registrationKey, effect, limits)
			require.ErrorIs(t, err, ErrEffectAlreadyConsumed)
			require.Empty(t, returnedKey)
			requireStateEqual(t, restarted, unchanged)
		})
	}
}

func TestUnitNotFoundIsRejectedBeforeStoredStateMutation(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	effect := testUnitEffect("unit-1")
	planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
	require.NoError(t, err)
	draining, err := DrainNamespace(planned, testRegistrationIdentity().NamespaceUID)
	require.NoError(t, err)
	draining, drainIdentity := commitNamespaceDrain(t, draining, "502")

	unchanged, err := ObserveEffectObject(draining, registrationKey, effectKey, notFoundObservation(effect, drainIdentity))
	require.ErrorIs(t, err, ErrEffectCloseoutVariantMismatch)
	requireStateEqual(t, draining, unchanged)
}

func TestUnitConsumeRequiresCommittedInstallationDrain(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	effect := testUnitEffect("unit-1")
	planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
	require.NoError(t, err)
	bound, err := ObserveEffectObject(planned, registrationKey, effectKey, presentObservation(effect, types.UID("unit-object-uid-1")))
	require.NoError(t, err)
	authorized, err := AuthorizeDispatch(bound, registrationKey, effectKey)
	require.NoError(t, err)
	terminal, err := MarkEffectTerminal(authorized, registrationKey, effectKey, executedTerminalEvidence())
	require.NoError(t, err)

	unchanged, err := ConsumeEffect(terminal, registrationKey, effectKey)
	require.ErrorIs(t, err, ErrDrainNotCommitted)
	requireStateEqual(t, terminal, unchanged)

	draining, err := DrainInstallation(terminal)
	require.NoError(t, err)
	unchanged, err = ConsumeEffect(draining, registrationKey, effectKey)
	require.ErrorIs(t, err, ErrDrainNotCommitted)
	requireStateEqual(t, draining, unchanged)

	committed, _ := commitInstallationDrain(t, draining, "503")
	consumed, err := ConsumeEffect(committed, registrationKey, effectKey)
	require.NoError(t, err)
	require.Equal(t, EffectStateConsumed, EffectStateOf(consumed, registrationKey, effectKey))
}

func TestTerminalCloseoutVariantMatchesEffectKindAtWriteSite(t *testing.T) {
	t.Run("Unit rejects ObservedTerminal", func(t *testing.T) {
		state, registrationKey, limits := registeredFence(t)
		effect := testUnitEffect("unit-1")
		planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
		require.NoError(t, err)
		bound, err := ObserveEffectObject(planned, registrationKey, effectKey, presentObservation(effect, types.UID("unit-object-uid-1")))
		require.NoError(t, err)
		authorized, err := AuthorizeDispatch(bound, registrationKey, effectKey)
		require.NoError(t, err)

		unchanged, err := MarkEffectTerminal(authorized, registrationKey, effectKey, observedTerminalEvidence())
		require.ErrorIs(t, err, ErrEffectCloseoutVariantMismatch)
		requireStateEqual(t, authorized, unchanged)

		terminal, err := MarkEffectTerminal(authorized, registrationKey, effectKey, executedTerminalEvidence())
		require.NoError(t, err)
		require.Equal(t, EffectStateTerminal, EffectStateOf(terminal, registrationKey, effectKey))
	})

	t.Run("Window rejects UnitExecuted", func(t *testing.T) {
		state, registrationKey, limits := registeredFence(t)
		effect := testWindowEffect("target-window-1")
		planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
		require.NoError(t, err)
		bound, err := ObserveEffectObject(planned, registrationKey, effectKey, presentObservation(effect, types.UID("window-object-uid-1")))
		require.NoError(t, err)
		draining, err := DrainNamespace(bound, testRegistrationIdentity().NamespaceUID)
		require.NoError(t, err)

		unchanged, err := MarkEffectTerminal(draining, registrationKey, effectKey, executedTerminalEvidence())
		require.ErrorIs(t, err, ErrEffectCloseoutVariantMismatch)
		requireStateEqual(t, draining, unchanged)

		terminal, err := MarkEffectTerminal(draining, registrationKey, effectKey, observedTerminalEvidence())
		require.NoError(t, err)
		require.Equal(t, EffectStateTerminal, EffectStateOf(terminal, registrationKey, effectKey))
	})
}

func TestEffectObservationTargetMatchesExactEffectKindAndLocation(t *testing.T) {
	for _, effect := range []EffectIdentity{
		testWindowEffect("shared-name"),
		testUnitEffect("shared-name"),
	} {
		t.Run(string(effect.Kind), func(t *testing.T) {
			state, registrationKey, limits := registeredFence(t)
			planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
			require.NoError(t, err)
			draining, err := DrainNamespace(planned, testRegistrationIdentity().NamespaceUID)
			require.NoError(t, err)
			draining, drainIdentity := commitNamespaceDrain(t, draining, "502")

			for name, mutate := range map[string]func(*EffectObjectTarget){
				"API version": func(v *EffectObjectTarget) { v.APIVersion = "wrong.example.io/v1" },
				"kind":        func(v *EffectObjectTarget) { v.Kind = "WrongKind" },
				"namespace":   func(v *EffectObjectTarget) { v.Namespace = "wrong-namespace" },
				"name":        func(v *EffectObjectTarget) { v.Name = "wrong-name" },
			} {
				t.Run(name, func(t *testing.T) {
					wrongPresent := presentObservation(effect, types.UID("object-uid-1"))
					mutate(&wrongPresent.Target)
					unchanged, err := ObserveEffectObject(planned, registrationKey, effectKey, wrongPresent)
					require.ErrorIs(t, err, ErrEffectObservationMismatch)
					requireStateEqual(t, planned, unchanged)

					wrongNotFound := notFoundObservation(effect, drainIdentity)
					mutate(&wrongNotFound.Target)
					unchanged, err = ObserveEffectObject(draining, registrationKey, effectKey, wrongNotFound)
					require.ErrorIs(t, err, ErrEffectObservationMismatch)
					requireStateEqual(t, draining, unchanged)
				})
			}

			_, err = ObserveEffectObject(planned, registrationKey, effectKey, presentObservation(effect, types.UID("object-uid-1")))
			require.NoError(t, err)
			unchanged, err := ObserveEffectObject(draining, registrationKey, effectKey, notFoundObservation(effect, drainIdentity))
			if effect.Kind == EffectKindUnitExecution {
				require.ErrorIs(t, err, ErrEffectCloseoutVariantMismatch)
				requireStateEqual(t, draining, unchanged)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDrainIdentityRequiresFreshCommittedFenceReadback(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	planned, _, err := PlanEffect(state, registrationKey, testWindowEffect("target-window-1"), limits)
	require.NoError(t, err)
	preDrainStoredDigest := testDigest(string(storedStateBytes(t, planned)))
	draining, err := DrainNamespace(planned, testRegistrationIdentity().NamespaceUID)
	require.NoError(t, err)

	for name, readback := range map[string]FenceObjectReadback{
		"empty UID": {
			ResourceVersion:    "502",
			StoredStatusDigest: testDigest(string(storedStateBytes(t, draining))),
		},
		"empty RV": {
			UID:                testFenceObjectIdentity().UID,
			StoredStatusDigest: testDigest(string(storedStateBytes(t, draining))),
		},
		"same name new UID": {
			UID:                types.UID("protocol-fence-uid-2"),
			ResourceVersion:    "502",
			StoredStatusDigest: testDigest(string(storedStateBytes(t, draining))),
		},
		"pre-commit status": {
			UID:                testFenceObjectIdentity().UID,
			ResourceVersion:    "501",
			StoredStatusDigest: preDrainStoredDigest,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := BindDrainCommitReadback(draining, NamespaceDrainScope(testRegistrationIdentity().NamespaceUID), readback)
			require.ErrorIs(t, err, ErrFenceReadbackMismatch)
		})
	}

	readback := testFenceReadback(t, draining, "502")
	view, err := BindDrainCommitReadback(draining, NamespaceDrainScope(testRegistrationIdentity().NamespaceUID), readback)
	require.NoError(t, err)
	identity := NamespaceDrainIdentity(view, testRegistrationIdentity().NamespaceUID)
	require.Equal(t, externalUIDDigest(readback.UID), identity.FenceUIDDigest)
	require.Equal(t, externalResourceVersionDigest(readback.ResourceVersion), identity.FenceResourceVersionDigest)
	require.NotEmpty(t, identity.FenceUIDDigest)
	require.NotEmpty(t, identity.FenceResourceVersionDigest)

	_, err = RebindDrainCommitReadback(view, NamespaceDrainScope(testRegistrationIdentity().NamespaceUID), testFenceReadback(t, draining, "503"))
	require.ErrorIs(t, err, ErrDrainReadbackAlreadyBound)
}

func TestDrainFreezesManifestWhileLivePhasesAdvance(t *testing.T) {
	for _, tc := range []struct {
		name     string
		drain    func(FenceState) (FenceState, error)
		commit   func(FenceState) (FenceState, DrainIdentity)
		digest   func(FenceState) string
		contains func(FenceState, string, string) bool
	}{
		{
			name: "namespace",
			drain: func(state FenceState) (FenceState, error) {
				return DrainNamespace(state, testRegistrationIdentity().NamespaceUID)
			},
			digest: func(state FenceState) string {
				return NamespaceManifestDigest(state, testRegistrationIdentity().NamespaceUID)
			},
			commit: func(state FenceState) (FenceState, DrainIdentity) {
				return commitNamespaceDrain(t, state, "502")
			},
			contains: NamespaceManifestContainsEffect,
		},
		{
			name:  "installation",
			drain: DrainInstallation,
			commit: func(state FenceState) (FenceState, DrainIdentity) {
				return commitInstallationDrain(t, state, "502")
			},
			digest:   InstallationManifestDigest,
			contains: InstallationManifestContainsEffect,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, registrationKey, limits := registeredFence(t)
			windowEffect := testWindowEffect("target-window-1")
			unitEffect := testUnitEffect("unit-1")
			planned, plannedKey, err := PlanEffect(state, registrationKey, windowEffect, limits)
			require.NoError(t, err)
			planned, boundKey, err := PlanEffect(planned, registrationKey, unitEffect, limits)
			require.NoError(t, err)
			bound, err := ObserveEffectObject(planned, registrationKey, boundKey, presentObservation(unitEffect, types.UID("unit-object-uid-1")))
			require.NoError(t, err)

			draining, err := tc.drain(bound)
			require.NoError(t, err)
			draining, drainIdentity := tc.commit(draining)
			manifestDigest := tc.digest(draining)
			require.NotEmpty(t, manifestDigest)
			require.True(t, tc.contains(draining, registrationKey, plannedKey))
			require.True(t, tc.contains(draining, registrationKey, boundKey))

			consumedPlanned, err := ObserveEffectObject(draining, registrationKey, plannedKey, notFoundObservation(windowEffect, drainIdentity))
			require.NoError(t, err)
			unchanged, err := MarkEffectTerminal(consumedPlanned, registrationKey, boundKey, executedTerminalEvidence())
			require.ErrorIs(t, err, ErrEffectDispatchRequired)
			requireStateEqual(t, consumedPlanned, unchanged)
			authorized, err := AuthorizeDispatch(consumedPlanned, registrationKey, boundKey)
			require.NoError(t, err)
			terminal, err := MarkEffectTerminal(authorized, registrationKey, boundKey, executedTerminalEvidence())
			require.NoError(t, err)
			if tc.name == "namespace" {
				terminal, err = DrainInstallation(terminal)
				require.NoError(t, err)
				terminal, _ = commitInstallationDrain(t, terminal, "503")
			}
			consumed, err := ConsumeEffect(terminal, registrationKey, boundKey)
			require.NoError(t, err)
			require.Equal(t, manifestDigest, tc.digest(consumed))
			require.True(t, tc.contains(consumed, registrationKey, plannedKey))
			require.True(t, tc.contains(consumed, registrationKey, boundKey))

			unchanged, _, err = PlanEffect(consumed, registrationKey, testWindowEffect("late-window"), limits)
			require.Error(t, err)
			requireStateEqual(t, consumed, unchanged)
		})
	}
}

func TestDispatchAuthorizationIsBoundAndNotReplayable(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	effect := testUnitEffect("unit-1")
	planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
	require.NoError(t, err)

	unchanged, err := AuthorizeDispatch(planned, registrationKey, effectKey)
	require.ErrorIs(t, err, ErrEffectObjectNotBound)
	requireStateEqual(t, planned, unchanged)

	bound, err := ObserveEffectObject(planned, registrationKey, effectKey, presentObservation(effect, types.UID("unit-object-uid-1")))
	require.NoError(t, err)
	authorized, err := AuthorizeDispatch(bound, registrationKey, effectKey)
	require.NoError(t, err)
	require.Equal(t, EffectStateDispatchAuthorized, EffectStateOf(authorized, registrationKey, effectKey))

	unchanged, err = AuthorizeDispatch(authorized, registrationKey, effectKey)
	require.ErrorIs(t, err, ErrDispatchOutcomeUnknown)
	requireStateEqual(t, authorized, unchanged)

	for _, drain := range []func(FenceState) (FenceState, error){
		func(state FenceState) (FenceState, error) {
			return DrainNamespace(state, testRegistrationIdentity().NamespaceUID)
		},
		DrainInstallation,
	} {
		draining, err := drain(bound)
		require.NoError(t, err)
		unchanged, err := AuthorizeDispatch(draining, registrationKey, effectKey)
		require.Error(t, err)
		requireStateEqual(t, draining, unchanged)
	}
}

func TestMarkEffectTerminalExactReplayIsIdempotent(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	effect := testUnitEffect("unit-1")
	planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
	require.NoError(t, err)
	bound, err := ObserveEffectObject(planned, registrationKey, effectKey, presentObservation(effect, types.UID("unit-object-uid-1")))
	require.NoError(t, err)
	authorized, err := AuthorizeDispatch(bound, registrationKey, effectKey)
	require.NoError(t, err)
	evidence := executedTerminalEvidence()
	terminal, err := MarkEffectTerminal(authorized, registrationKey, effectKey, evidence)
	require.NoError(t, err)

	replayed, err := MarkEffectTerminal(terminal, registrationKey, effectKey, evidence)
	require.NoError(t, err)
	requireStateEqual(t, terminal, replayed)

	conflicting := evidence
	conflicting.ReasonCode = "different-reason"
	unchanged, err := MarkEffectTerminal(terminal, registrationKey, effectKey, conflicting)
	require.ErrorIs(t, err, ErrEffectStateRegression)
	requireStateEqual(t, terminal, unchanged)
}

func TestEffectObservationUsesConfiguredNamespace(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	effect := testUnitEffect("unit-1")
	effect.Namespace = "custom-system"
	planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
	require.NoError(t, err)

	bound, err := ObserveEffectObject(planned, registrationKey, effectKey, presentObservation(effect, types.UID("unit-object-uid-1")))
	require.NoError(t, err)
	require.Equal(t, EffectStateObjectBound, EffectStateOf(bound, registrationKey, effectKey))
}

func TestStoredProtocolFenceStatusMatchesIndependentJSONContract(t *testing.T) {
	identity := testRegistrationIdentity()
	limits := generousRegistryLimits()
	base := newFenceState(t, identity.InstallationEpoch, identity.NamespaceUID, testFenceObjectIdentity())
	baseExpected := expectedStoredStatusBytes(t, identity.InstallationEpoch, identity.NamespaceUID, nil)
	require.JSONEq(t, string(baseExpected), string(storedStateBytes(t, base)))
	requireWrongFenceUIDDecodeRejected(t, baseExpected)

	registered, registrationKey, err := RegisterExecution(base, identity, limits)
	require.NoError(t, err)
	registeredExpected := expectedStoredStatusBytes(t, identity.InstallationEpoch, identity.NamespaceUID, []storedRegistrationDTO{{
		Key:      registrationKey,
		Identity: storedRegistrationIdentityDTOFrom(identity),
		Phase:    string(RegistrationPhaseActive),
	}})
	require.JSONEq(t, string(registeredExpected), string(storedStateBytes(t, registered)))
	requireWrongFenceUIDDecodeRejected(t, registeredExpected)

	effect := testWindowEffect("target-window-1")
	planned, effectKey, err := PlanEffect(registered, registrationKey, effect, limits)
	require.NoError(t, err)
	plannedDTO := storedProtocolFenceStatusDTO{
		ProtocolVersion:   storedProtocolFenceStatusV1,
		FenceUIDDigest:    externalUIDDigest(testFenceObjectIdentity().UID),
		InstallationEpoch: identity.InstallationEpoch,
		Namespaces:        []storedNamespaceDTO{{UID: identity.NamespaceUID}},
		Registrations: []storedRegistrationDTO{{
			Key:      registrationKey,
			Identity: storedRegistrationIdentityDTOFrom(identity),
			Phase:    string(RegistrationPhaseActive),
			Effects: []storedEffectDTO{{
				Key:      effectKey,
				Identity: storedEffectIdentityDTOFrom(effect),
				State:    string(EffectStatePlanned),
			}},
		}},
	}
	plannedExpected := marshalExpectedStoredStatus(t, plannedDTO)
	require.JSONEq(t, string(plannedExpected), string(storedStateBytes(t, planned)))
	requireWrongFenceUIDDecodeRejected(t, plannedExpected)
	require.Less(t, len(baseExpected), len(registeredExpected))
	require.Less(t, len(registeredExpected), len(plannedExpected))

	namespaceManifest := expectedStoredManifest("namespace:"+string(identity.NamespaceUID), []string{registrationKey}, []string{effectKey})
	namespaceDraining, err := DrainNamespace(planned, identity.NamespaceUID)
	require.NoError(t, err)
	namespaceDTO := plannedDTO
	namespaceDTO.Namespaces = []storedNamespaceDTO{{UID: identity.NamespaceUID, Manifest: namespaceManifest}}
	namespaceUnboundExpected := marshalExpectedStoredStatus(t, namespaceDTO)
	require.JSONEq(t, string(namespaceUnboundExpected), string(storedStateBytes(t, namespaceDraining)))
	namespaceReadback := FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    "502",
		StoredStatusDigest: testDigest(string(namespaceUnboundExpected)),
	}
	namespaceCommitted, err := BindDrainCommitReadback(namespaceDraining, NamespaceDrainScope(identity.NamespaceUID), namespaceReadback)
	require.NoError(t, err)
	namespaceManifest.CommitReadback = storedFenceReadbackDTOFrom(namespaceReadback)
	namespaceExpected := marshalExpectedStoredStatus(t, namespaceDTO)
	require.JSONEq(t, string(namespaceExpected), string(storedStateBytes(t, namespaceCommitted)))
	namespaceIdentity := NamespaceDrainIdentity(namespaceCommitted, identity.NamespaceUID)
	require.Equal(t, namespaceManifest.Token, namespaceIdentity.Token)
	require.Equal(t, namespaceManifest.Digest, namespaceIdentity.ManifestDigest)
	require.Less(t, len(plannedExpected), len(namespaceExpected))

	installationManifest := expectedStoredManifest("installation:"+identity.InstallationEpoch, []string{registrationKey}, []string{effectKey})
	installationDraining, err := DrainInstallation(planned)
	require.NoError(t, err)
	installationDTO := plannedDTO
	installationDTO.InstallationManifest = installationManifest
	installationUnboundExpected := marshalExpectedStoredStatus(t, installationDTO)
	require.JSONEq(t, string(installationUnboundExpected), string(storedStateBytes(t, installationDraining)))
	installationReadback := FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    "503",
		StoredStatusDigest: testDigest(string(installationUnboundExpected)),
	}
	installationCommitted, err := BindDrainCommitReadback(installationDraining, InstallationDrainScope(), installationReadback)
	require.NoError(t, err)
	installationManifest.CommitReadback = storedFenceReadbackDTOFrom(installationReadback)
	installationExpected := marshalExpectedStoredStatus(t, installationDTO)
	require.JSONEq(t, string(installationExpected), string(storedStateBytes(t, installationCommitted)))
	installationIdentity := InstallationDrainIdentity(installationCommitted)
	require.Equal(t, installationManifest.Token, installationIdentity.Token)
	require.Equal(t, installationManifest.Digest, installationIdentity.ManifestDigest)
	require.Less(t, len(plannedExpected), len(installationExpected))
}

func TestStoredStatusRoundTripPreservesDurableEffectAndDrainEvidence(t *testing.T) {
	identity := testRegistrationIdentity()
	state, registrationKey, limits := registeredFence(t)
	effect := testUnitEffect("unit-1")
	planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
	require.NoError(t, err)
	dto := storedProtocolFenceStatusDTO{
		ProtocolVersion:   storedProtocolFenceStatusV1,
		FenceUIDDigest:    externalUIDDigest(testFenceObjectIdentity().UID),
		InstallationEpoch: identity.InstallationEpoch,
		Namespaces:        []storedNamespaceDTO{{UID: identity.NamespaceUID}},
		Registrations: []storedRegistrationDTO{{
			Key:      registrationKey,
			Identity: storedRegistrationIdentityDTOFrom(identity),
			Phase:    string(RegistrationPhaseActive),
			Effects: []storedEffectDTO{{
				Key:      effectKey,
				Identity: storedEffectIdentityDTOFrom(effect),
				State:    string(EffectStatePlanned),
			}},
		}},
	}
	require.JSONEq(t, string(marshalExpectedStoredStatus(t, dto)), string(storedStateBytes(t, planned)))

	present := presentObservation(effect, types.UID(strings.Repeat("u", testMaxEffectObjectUIDBytes)))
	present.ObjectResourceVersion = strings.Repeat("9", testMaxEffectObjectResourceVersionBytes)
	bound, err := ObserveEffectObject(planned, registrationKey, effectKey, present)
	require.NoError(t, err)
	dto.Registrations[0].Effects[0].State = string(EffectStateObjectBound)
	dto.Registrations[0].Effects[0].ObjectBinding = &storedObjectBindingDTO{
		Target:                present.Target,
		UIDDigest:             externalUIDDigest(present.ObjectUID),
		ResourceVersionDigest: externalResourceVersionDigest(present.ObjectResourceVersion),
	}
	boundExpected := marshalExpectedStoredStatus(t, dto)
	require.JSONEq(t, string(boundExpected), string(storedStateBytes(t, bound)))
	requireWrongFenceUIDDecodeRejected(t, boundExpected)
	restartedBound, err := DecodeStoredProtocolFenceStatus(boundExpected, testFenceObjectIdentity())
	require.NoError(t, err)
	wrongUID := present
	wrongUID.ObjectUID = types.UID("different-object-uid")
	unchanged, err := ObserveEffectObject(restartedBound, registrationKey, effectKey, wrongUID)
	require.ErrorIs(t, err, ErrEffectObjectIdentityConflict)
	requireStateEqual(t, restartedBound, unchanged)

	authorized, err := AuthorizeDispatch(bound, registrationKey, effectKey)
	require.NoError(t, err)
	dto.Registrations[0].Effects[0].State = string(EffectStateDispatchAuthorized)
	dto.Registrations[0].Effects[0].Dispatch = &storedDispatchAuthorizationDTO{Token: expectedDispatchToken(registrationKey, effectKey, externalUIDDigest(present.ObjectUID))}
	authorizedExpected := marshalExpectedStoredStatus(t, dto)
	require.JSONEq(t, string(authorizedExpected), string(storedStateBytes(t, authorized)))
	requireWrongFenceUIDDecodeRejected(t, authorizedExpected)
	restartedAuthorized, err := DecodeStoredProtocolFenceStatus(authorizedExpected, testFenceObjectIdentity())
	require.NoError(t, err)
	unchanged, err = AuthorizeDispatch(restartedAuthorized, registrationKey, effectKey)
	require.ErrorIs(t, err, ErrDispatchOutcomeUnknown)
	requireStateEqual(t, restartedAuthorized, unchanged)

	namespaceDraining, err := DrainNamespace(authorized, identity.NamespaceUID)
	require.NoError(t, err)
	namespaceManifest := expectedStoredManifest("namespace:"+string(identity.NamespaceUID), []string{registrationKey}, []string{effectKey})
	dto.Namespaces[0].Manifest = namespaceManifest
	namespaceUnboundExpected := marshalExpectedStoredStatus(t, dto)
	require.JSONEq(t, string(namespaceUnboundExpected), string(storedStateBytes(t, namespaceDraining)))
	namespaceReadback := FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    strings.Repeat("8", testMaxFenceResourceVersionBytes),
		StoredStatusDigest: testDigest(string(namespaceUnboundExpected)),
	}
	namespaceCommitted, err := BindDrainCommitReadback(namespaceDraining, NamespaceDrainScope(identity.NamespaceUID), namespaceReadback)
	require.NoError(t, err)
	namespaceManifest.CommitReadback = storedFenceReadbackDTOFrom(namespaceReadback)
	require.JSONEq(t, string(marshalExpectedStoredStatus(t, dto)), string(storedStateBytes(t, namespaceCommitted)))

	installationDraining, err := DrainInstallation(namespaceCommitted)
	require.NoError(t, err)
	installationManifest := expectedStoredManifest("installation:"+identity.InstallationEpoch, []string{registrationKey}, []string{effectKey})
	dto.InstallationManifest = installationManifest
	installationUnboundExpected := marshalExpectedStoredStatus(t, dto)
	require.JSONEq(t, string(installationUnboundExpected), string(storedStateBytes(t, installationDraining)))
	installationReadback := FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    strings.Repeat("9", testMaxFenceResourceVersionBytes),
		StoredStatusDigest: testDigest(string(installationUnboundExpected)),
	}
	installationCommitted, err := BindDrainCommitReadback(installationDraining, InstallationDrainScope(), installationReadback)
	require.NoError(t, err)
	installationManifest.CommitReadback = storedFenceReadbackDTOFrom(installationReadback)

	terminalEvidence := maxTerminalEvidence()
	terminal, err := MarkEffectTerminal(installationCommitted, registrationKey, effectKey, terminalEvidence)
	require.NoError(t, err)
	dto.Registrations[0].Effects[0].State = string(EffectStateTerminal)
	dto.Registrations[0].Effects[0].CloseoutVariant = string(CloseoutVariantUnitExecuted)
	dto.Registrations[0].Effects[0].Terminal = storedTerminalEvidenceDTOFrom(terminalEvidence)
	require.JSONEq(t, string(marshalExpectedStoredStatus(t, dto)), string(storedStateBytes(t, terminal)))

	consumed, err := ConsumeEffect(terminal, registrationKey, effectKey)
	require.NoError(t, err)
	dto.Registrations[0].Effects[0].State = string(EffectStateConsumed)
	dto.Registrations[0].Effects[0].Tombstone = &storedConsumedTombstoneDTO{IdentityDigest: expectedConsumedTombstoneDigest(effectKey)}
	consumed, err = ConsumeRegistration(consumed, registrationKey)
	require.NoError(t, err)
	dto.Registrations[0].Phase = string(RegistrationPhaseConsumed)
	finalExpected := marshalExpectedStoredStatus(t, dto)
	require.JSONEq(t, string(finalExpected), string(storedStateBytes(t, consumed)))
	_, err = DecodeStoredProtocolFenceStatus(finalExpected, FenceObjectIdentity{UID: types.UID("protocol-fence-uid-2")})
	require.ErrorIs(t, err, ErrFenceReadbackMismatch)

	restarted, err := DecodeStoredProtocolFenceStatus(finalExpected, testFenceObjectIdentity())
	require.NoError(t, err)
	unchanged, err = RebindDrainCommitReadback(restarted, InstallationDrainScope(), FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    "new-resource-version",
		StoredStatusDigest: testDigest("new-status"),
	})
	require.ErrorIs(t, err, ErrDrainReadbackAlreadyBound)
	requireStateEqual(t, restarted, unchanged)
	unchanged, _, err = PlanEffect(restarted, registrationKey, effect, limits)
	require.ErrorIs(t, err, ErrEffectAlreadyConsumed)
	requireStateEqual(t, restarted, unchanged)
}

func TestDecodeStoredStatusRejectsEveryInconsistentInvariant(t *testing.T) {
	valid := validFinalStoredStatusDTO(t)
	validBytes := marshalExpectedStoredStatus(t, valid)
	_, err := DecodeStoredProtocolFenceStatus(validBytes, testFenceObjectIdentity())
	require.NoError(t, err)

	for name, mutate := range map[string]func(*storedProtocolFenceStatusDTO){
		"unknown protocol version":     func(v *storedProtocolFenceStatusDTO) { v.ProtocolVersion = "v2" },
		"top Fence UID digest missing": func(v *storedProtocolFenceStatusDTO) { v.FenceUIDDigest = "" },
		"registration key mismatch":    func(v *storedProtocolFenceStatusDTO) { v.Registrations[0].Key = testDigest("wrong-registration") },
		"duplicate registration key":   func(v *storedProtocolFenceStatusDTO) { v.Registrations = append(v.Registrations, v.Registrations[0]) },
		"drained manifest misses appended zero-effect registration": func(v *storedProtocolFenceStatusDTO) {
			identity := testRegistrationIdentity()
			identity.ExecutionUID = types.UID("execution-uid-2")
			identity.AttemptID = "attempt-4"
			key, err := RegistrationKey(identity)
			require.NoError(t, err)
			v.Registrations = append(v.Registrations, storedRegistrationDTO{Key: key, Identity: storedRegistrationIdentityDTOFrom(identity), Phase: string(RegistrationPhaseActive)})
		},
		"drained manifest retains removed registration": func(v *storedProtocolFenceStatusDTO) { v.Registrations = nil },
		"unknown registration phase":                    func(v *storedProtocolFenceStatusDTO) { v.Registrations[0].Phase = "UnknownPhase" },
		"effect key mismatch":                           func(v *storedProtocolFenceStatusDTO) { v.Registrations[0].Effects[0].Key = testDigest("wrong-effect") },
		"duplicate effect key": func(v *storedProtocolFenceStatusDTO) {
			v.Registrations[0].Effects = append(v.Registrations[0].Effects, v.Registrations[0].Effects[0])
		},
		"unknown effect state":   func(v *storedProtocolFenceStatusDTO) { v.Registrations[0].Effects[0].State = "UnknownState" },
		"object binding missing": func(v *storedProtocolFenceStatusDTO) { v.Registrations[0].Effects[0].ObjectBinding = nil },
		"object target mismatch": func(v *storedProtocolFenceStatusDTO) {
			v.Registrations[0].Effects[0].ObjectBinding.Target.Kind = "GateWindow"
		},
		"object UID digest malformed": func(v *storedProtocolFenceStatusDTO) {
			v.Registrations[0].Effects[0].ObjectBinding.UIDDigest = "sha256:short"
		},
		"object RV digest missing": func(v *storedProtocolFenceStatusDTO) {
			v.Registrations[0].Effects[0].ObjectBinding.ResourceVersionDigest = ""
		},
		"dispatch token wrong": func(v *storedProtocolFenceStatusDTO) {
			v.Registrations[0].Effects[0].Dispatch.Token = testDigest("wrong-dispatch")
		},
		"Planned carries dispatch": func(v *storedProtocolFenceStatusDTO) {
			v.Registrations[0].Effects[0].State = string(EffectStatePlanned)
		},
		"terminal evidence missing":  func(v *storedProtocolFenceStatusDTO) { v.Registrations[0].Effects[0].Terminal = nil },
		"Consumed tombstone missing": func(v *storedProtocolFenceStatusDTO) { v.Registrations[0].Effects[0].Tombstone = nil },
		"Consumed tombstone wrong": func(v *storedProtocolFenceStatusDTO) {
			v.Registrations[0].Effects[0].Tombstone.IdentityDigest = testDigest("wrong-tombstone")
		},
		"namespace manifest token wrong":            func(v *storedProtocolFenceStatusDTO) { v.Namespaces[0].Manifest.Token = testDigest("wrong-token") },
		"namespace manifest digest wrong":           func(v *storedProtocolFenceStatusDTO) { v.Namespaces[0].Manifest.Digest = testDigest("wrong-manifest") },
		"namespace manifest registration set wrong": func(v *storedProtocolFenceStatusDTO) { v.Namespaces[0].Manifest.RegistrationKeys = nil },
		"namespace manifest effect set wrong":       func(v *storedProtocolFenceStatusDTO) { v.Namespaces[0].Manifest.EffectKeys = nil },
		"namespace readback UID missing":            func(v *storedProtocolFenceStatusDTO) { v.Namespaces[0].Manifest.CommitReadback.UIDDigest = "" },
		"namespace readback RV malformed": func(v *storedProtocolFenceStatusDTO) {
			v.Namespaces[0].Manifest.CommitReadback.ResourceVersionDigest = "sha256:short"
		},
		"installation manifest missing": func(v *storedProtocolFenceStatusDTO) { v.InstallationManifest = nil },
		"installation manifest digest wrong": func(v *storedProtocolFenceStatusDTO) {
			v.InstallationManifest.Digest = testDigest("wrong-installation-manifest")
		},
		"installation readback status digest missing": func(v *storedProtocolFenceStatusDTO) { v.InstallationManifest.CommitReadback.StoredStatusDigest = "" },
		"installation commit readback missing":        func(v *storedProtocolFenceStatusDTO) { v.InstallationManifest.CommitReadback = nil },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := cloneStoredStatusDTO(t, valid)
			mutate(&invalid)
			decoded, err := DecodeStoredProtocolFenceStatus(marshalExpectedStoredStatus(t, invalid), testFenceObjectIdentity())
			require.ErrorIs(t, err, ErrStoredStatusInvalid)
			require.True(t, IsZeroFenceState(decoded), "decoder must not return partially usable state")
		})
	}

	t.Run("integrity digest mismatch", func(t *testing.T) {
		invalid := cloneStoredStatusDTO(t, valid)
		invalid.Registrations[0].Phase = "tampered-without-reseal"
		data, err := json.Marshal(invalid)
		require.NoError(t, err)
		decoded, err := DecodeStoredProtocolFenceStatus(data, testFenceObjectIdentity())
		require.ErrorIs(t, err, ErrStoredStatusIntegrity)
		require.True(t, IsZeroFenceState(decoded))
	})
}

func TestDecodeStoredStatusEnforcesExactPhaseEvidenceMatrix(t *testing.T) {
	for _, phase := range []EffectState{
		EffectStatePlanned,
		EffectStateObjectBound,
		EffectStateDispatchAuthorized,
		EffectStateTerminal,
		EffectStateConsumed,
	} {
		t.Run("valid "+string(phase), func(t *testing.T) {
			valid := validStoredStatusForPhase(t, phase)
			_, err := DecodeStoredProtocolFenceStatus(marshalExpectedStoredStatus(t, valid), testFenceObjectIdentity())
			require.NoError(t, err)
		})
	}
	for name, status := range map[string]storedProtocolFenceStatusDTO{
		"valid Window namespace NotFound Consumed":    validWindowConsumedStatusDTO(t, CloseoutVariantNotFound),
		"valid Window installation NotFound Consumed": validWindowInstallationNotFoundConsumedStatusDTO(t),
		"valid Window ObservedTerminal Terminal":      validWindowTerminalStatusDTO(t),
		"valid Window ObservedTerminal Consumed":      validWindowConsumedStatusDTO(t, CloseoutVariantObservedTerminal),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeStoredProtocolFenceStatus(marshalExpectedStoredStatus(t, status), testFenceObjectIdentity())
			require.NoError(t, err)
		})
	}
	mixed := validMixedConsumedStatusDTO(t)
	restartedMixed, err := DecodeStoredProtocolFenceStatus(marshalExpectedStoredStatus(t, mixed), testFenceObjectIdentity())
	require.NoError(t, err)
	for _, storedEffect := range mixed.Registrations[0].Effects {
		unchanged, returnedKey, err := PlanEffect(restartedMixed, mixed.Registrations[0].Key, effectIdentityFromStored(storedEffect.Identity), generousRegistryLimits())
		require.ErrorIs(t, err, ErrRegistrationAlreadyConsumed)
		require.Empty(t, returnedKey)
		requireStateEqual(t, restartedMixed, unchanged)
	}

	for _, tc := range []struct {
		name   string
		phase  EffectState
		mutate func(*storedEffectDTO)
	}{
		{name: "Planned carries object", phase: EffectStatePlanned, mutate: func(v *storedEffectDTO) {
			v.ObjectBinding = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].ObjectBinding
		}},
		{name: "Planned carries dispatch", phase: EffectStatePlanned, mutate: func(v *storedEffectDTO) {
			v.Dispatch = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Dispatch
		}},
		{name: "Planned carries terminal", phase: EffectStatePlanned, mutate: func(v *storedEffectDTO) {
			v.Terminal = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Terminal
		}},
		{name: "Planned carries tombstone", phase: EffectStatePlanned, mutate: func(v *storedEffectDTO) {
			v.Tombstone = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Tombstone
		}},
		{name: "Planned carries closeout variant", phase: EffectStatePlanned, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = string(CloseoutVariantUnitExecuted) }},
		{name: "Planned carries NotFound", phase: EffectStatePlanned, mutate: func(v *storedEffectDTO) {
			v.NotFound = validWindowConsumedStatusDTO(t, CloseoutVariantNotFound).Registrations[0].Effects[0].NotFound
		}},
		{name: "ObjectBound missing object", phase: EffectStateObjectBound, mutate: func(v *storedEffectDTO) { v.ObjectBinding = nil }},
		{name: "ObjectBound carries dispatch", phase: EffectStateObjectBound, mutate: func(v *storedEffectDTO) {
			v.Dispatch = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Dispatch
		}},
		{name: "ObjectBound carries terminal", phase: EffectStateObjectBound, mutate: func(v *storedEffectDTO) {
			v.Terminal = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Terminal
		}},
		{name: "ObjectBound carries tombstone", phase: EffectStateObjectBound, mutate: func(v *storedEffectDTO) {
			v.Tombstone = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Tombstone
		}},
		{name: "ObjectBound carries closeout variant", phase: EffectStateObjectBound, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = string(CloseoutVariantUnitExecuted) }},
		{name: "ObjectBound carries NotFound", phase: EffectStateObjectBound, mutate: func(v *storedEffectDTO) {
			v.NotFound = validWindowConsumedStatusDTO(t, CloseoutVariantNotFound).Registrations[0].Effects[0].NotFound
		}},
		{name: "DispatchAuthorized missing object", phase: EffectStateDispatchAuthorized, mutate: func(v *storedEffectDTO) { v.ObjectBinding = nil }},
		{name: "DispatchAuthorized missing dispatch", phase: EffectStateDispatchAuthorized, mutate: func(v *storedEffectDTO) { v.Dispatch = nil }},
		{name: "DispatchAuthorized carries terminal", phase: EffectStateDispatchAuthorized, mutate: func(v *storedEffectDTO) {
			v.Terminal = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Terminal
		}},
		{name: "DispatchAuthorized carries tombstone", phase: EffectStateDispatchAuthorized, mutate: func(v *storedEffectDTO) {
			v.Tombstone = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Tombstone
		}},
		{name: "DispatchAuthorized carries closeout variant", phase: EffectStateDispatchAuthorized, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = string(CloseoutVariantUnitExecuted) }},
		{name: "DispatchAuthorized carries NotFound", phase: EffectStateDispatchAuthorized, mutate: func(v *storedEffectDTO) {
			v.NotFound = validWindowConsumedStatusDTO(t, CloseoutVariantNotFound).Registrations[0].Effects[0].NotFound
		}},
		{name: "Terminal missing object", phase: EffectStateTerminal, mutate: func(v *storedEffectDTO) { v.ObjectBinding = nil }},
		{name: "Terminal missing dispatch", phase: EffectStateTerminal, mutate: func(v *storedEffectDTO) { v.Dispatch = nil }},
		{name: "Terminal missing terminal evidence", phase: EffectStateTerminal, mutate: func(v *storedEffectDTO) { v.Terminal = nil }},
		{name: "Terminal carries tombstone", phase: EffectStateTerminal, mutate: func(v *storedEffectDTO) {
			v.Tombstone = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Tombstone
		}},
		{name: "Terminal missing closeout variant", phase: EffectStateTerminal, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = "" }},
		{name: "Unit Terminal uses Window variant", phase: EffectStateTerminal, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = string(CloseoutVariantObservedTerminal) }},
		{name: "Terminal carries NotFound", phase: EffectStateTerminal, mutate: func(v *storedEffectDTO) {
			v.NotFound = validWindowConsumedStatusDTO(t, CloseoutVariantNotFound).Registrations[0].Effects[0].NotFound
		}},
		{name: "Consumed missing object", phase: EffectStateConsumed, mutate: func(v *storedEffectDTO) { v.ObjectBinding = nil }},
		{name: "Consumed missing dispatch", phase: EffectStateConsumed, mutate: func(v *storedEffectDTO) { v.Dispatch = nil }},
		{name: "Consumed missing terminal", phase: EffectStateConsumed, mutate: func(v *storedEffectDTO) { v.Terminal = nil }},
		{name: "Consumed missing tombstone", phase: EffectStateConsumed, mutate: func(v *storedEffectDTO) { v.Tombstone = nil }},
		{name: "Consumed missing closeout variant", phase: EffectStateConsumed, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = "" }},
		{name: "Consumed unknown closeout variant", phase: EffectStateConsumed, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = "UnknownVariant" }},
		{name: "Unit Consumed uses Window variant", phase: EffectStateConsumed, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = string(CloseoutVariantObservedTerminal) }},
		{name: "Unit Consumed carries NotFound", phase: EffectStateConsumed, mutate: func(v *storedEffectDTO) {
			v.NotFound = validWindowConsumedStatusDTO(t, CloseoutVariantNotFound).Registrations[0].Effects[0].NotFound
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := validStoredStatusForPhase(t, tc.phase)
			tc.mutate(&invalid.Registrations[0].Effects[0])
			decoded, err := DecodeStoredProtocolFenceStatus(marshalExpectedStoredStatus(t, invalid), testFenceObjectIdentity())
			require.ErrorIs(t, err, ErrStoredStatusInvalid)
			require.True(t, IsZeroFenceState(decoded))
		})
	}

	for _, tc := range []struct {
		name    string
		variant EffectCloseoutVariant
		mutate  func(*storedEffectDTO)
	}{
		{name: "NotFound missing evidence", variant: CloseoutVariantNotFound, mutate: func(v *storedEffectDTO) { v.NotFound = nil }},
		{name: "NotFound carries object", variant: CloseoutVariantNotFound, mutate: func(v *storedEffectDTO) {
			v.ObjectBinding = validWindowConsumedStatusDTO(t, CloseoutVariantObservedTerminal).Registrations[0].Effects[0].ObjectBinding
		}},
		{name: "NotFound carries dispatch", variant: CloseoutVariantNotFound, mutate: func(v *storedEffectDTO) {
			v.Dispatch = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Dispatch
		}},
		{name: "NotFound carries terminal", variant: CloseoutVariantNotFound, mutate: func(v *storedEffectDTO) {
			v.Terminal = validWindowConsumedStatusDTO(t, CloseoutVariantObservedTerminal).Registrations[0].Effects[0].Terminal
		}},
		{name: "NotFound uses Observed variant", variant: CloseoutVariantNotFound, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = string(CloseoutVariantObservedTerminal) }},
		{name: "ObservedTerminal missing object", variant: CloseoutVariantObservedTerminal, mutate: func(v *storedEffectDTO) { v.ObjectBinding = nil }},
		{name: "ObservedTerminal missing terminal", variant: CloseoutVariantObservedTerminal, mutate: func(v *storedEffectDTO) { v.Terminal = nil }},
		{name: "ObservedTerminal carries dispatch", variant: CloseoutVariantObservedTerminal, mutate: func(v *storedEffectDTO) {
			v.Dispatch = validFinalStoredStatusDTO(t).Registrations[0].Effects[0].Dispatch
		}},
		{name: "ObservedTerminal carries NotFound", variant: CloseoutVariantObservedTerminal, mutate: func(v *storedEffectDTO) {
			v.NotFound = validWindowConsumedStatusDTO(t, CloseoutVariantNotFound).Registrations[0].Effects[0].NotFound
		}},
		{name: "ObservedTerminal uses Unit variant", variant: CloseoutVariantObservedTerminal, mutate: func(v *storedEffectDTO) { v.CloseoutVariant = string(CloseoutVariantUnitExecuted) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := validWindowConsumedStatusDTO(t, tc.variant)
			tc.mutate(&invalid.Registrations[0].Effects[0])
			decoded, err := DecodeStoredProtocolFenceStatus(marshalExpectedStoredStatus(t, invalid), testFenceObjectIdentity())
			require.ErrorIs(t, err, ErrStoredStatusInvalid)
			require.True(t, IsZeroFenceState(decoded))
		})
	}

	for _, tc := range []struct {
		name                   string
		resealNotFoundEvidence bool
		mutate                 func(*storedNotFoundEvidenceDTO)
	}{
		{name: "NotFound target API version mismatch", resealNotFoundEvidence: true, mutate: func(v *storedNotFoundEvidenceDTO) { v.Target.APIVersion = "wrong.example.io/v1" }},
		{name: "NotFound target kind mismatch", resealNotFoundEvidence: true, mutate: func(v *storedNotFoundEvidenceDTO) { v.Target.Kind = "WrongKind" }},
		{name: "NotFound target namespace mismatch", resealNotFoundEvidence: true, mutate: func(v *storedNotFoundEvidenceDTO) { v.Target.Namespace = "wrong-namespace" }},
		{name: "NotFound target name mismatch", resealNotFoundEvidence: true, mutate: func(v *storedNotFoundEvidenceDTO) { v.Target.Name = "wrong-name" }},
		{name: "NotFound drain token mismatch", resealNotFoundEvidence: true, mutate: func(v *storedNotFoundEvidenceDTO) { v.DrainToken = testDigest("wrong-drain-token") }},
		{name: "NotFound manifest digest mismatch", resealNotFoundEvidence: true, mutate: func(v *storedNotFoundEvidenceDTO) { v.ManifestDigest = testDigest("wrong-manifest") }},
		{name: "NotFound Fence UID digest mismatch", resealNotFoundEvidence: true, mutate: func(v *storedNotFoundEvidenceDTO) { v.FenceUIDDigest = externalUIDDigest(types.UID("wrong-fence-uid")) }},
		{name: "NotFound Fence RV digest mismatch", resealNotFoundEvidence: true, mutate: func(v *storedNotFoundEvidenceDTO) {
			v.FenceResourceVersionDigest = externalResourceVersionDigest("wrong-fence-rv")
		}},
		{name: "NotFound lookup RV digest empty", mutate: func(v *storedNotFoundEvidenceDTO) { v.LookupResourceVersionDigest = "" }},
		{name: "NotFound lookup RV digest malformed", mutate: func(v *storedNotFoundEvidenceDTO) { v.LookupResourceVersionDigest = "not-a-digest" }},
		{name: "NotFound lookup RV digest changed", mutate: func(v *storedNotFoundEvidenceDTO) {
			v.LookupResourceVersionDigest = externalResourceVersionDigest("different-lookup-rv")
		}},
		{name: "NotFound evidence digest missing", mutate: func(v *storedNotFoundEvidenceDTO) { v.EvidenceDigest = "" }},
		{name: "NotFound evidence digest mismatch", mutate: func(v *storedNotFoundEvidenceDTO) { v.EvidenceDigest = testDigest("wrong-notfound-evidence") }},
	} {
		for scope, valid := range map[string]storedProtocolFenceStatusDTO{
			"namespace":    validWindowConsumedStatusDTO(t, CloseoutVariantNotFound),
			"installation": validWindowInstallationNotFoundConsumedStatusDTO(t),
		} {
			t.Run(scope+"/"+tc.name, func(t *testing.T) {
				invalid := cloneStoredStatusDTO(t, valid)
				notFound := invalid.Registrations[0].Effects[0].NotFound
				tc.mutate(notFound)
				if tc.resealNotFoundEvidence {
					notFound.EvidenceDigest = expectedNotFoundEvidenceDigest(*notFound)
				}
				decoded, err := DecodeStoredProtocolFenceStatus(marshalExpectedStoredStatus(t, invalid), testFenceObjectIdentity())
				require.ErrorIs(t, err, ErrStoredStatusInvalid)
				require.True(t, IsZeroFenceState(decoded))
			})
		}
	}

	t.Run("Consumed registration contains non-Consumed effect", func(t *testing.T) {
		invalid := validStoredStatusForPhase(t, EffectStateTerminal)
		invalid.Registrations[0].Phase = string(RegistrationPhaseConsumed)
		decoded, err := DecodeStoredProtocolFenceStatus(marshalExpectedStoredStatus(t, invalid), testFenceObjectIdentity())
		require.ErrorIs(t, err, ErrStoredStatusInvalid)
		require.True(t, IsZeroFenceState(decoded))
	})
	t.Run("mixed Consumed registration contains one ObjectBound effect", func(t *testing.T) {
		invalid := validMixedConsumedStatusDTO(t)
		objectBoundUnit := validStoredStatusForPhase(t, EffectStateObjectBound).Registrations[0].Effects[0]
		invalid.Registrations[0].Effects[0] = objectBoundUnit
		decoded, err := DecodeStoredProtocolFenceStatus(marshalExpectedStoredStatus(t, invalid), testFenceObjectIdentity())
		require.ErrorIs(t, err, ErrStoredStatusInvalid)
		require.True(t, IsZeroFenceState(decoded))
	})
}

func TestExternalEvidenceUsesFixedDigestsAndControllerEvidenceIsBounded(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	effect := testUnitEffect("unit-1")
	planned, effectKey, err := PlanEffect(state, registrationKey, effect, limits)
	require.NoError(t, err)

	var storedLengths []int
	for _, size := range []int{testMaxEffectObjectUIDBytes, testMaxEffectObjectUIDBytes + 1} {
		observation := presentObservation(effect, types.UID(strings.Repeat("u", size)))
		observation.ObjectResourceVersion = strings.Repeat("9", testMaxEffectObjectResourceVersionBytes)
		bound, err := ObserveEffectObject(planned, registrationKey, effectKey, observation)
		require.NoError(t, err, "server-owned UID max+1 must be represented by a fixed digest, not rejected after child creation")
		storedLengths = append(storedLengths, len(storedStateBytes(t, bound)))
		replayed, err := ObserveEffectObject(bound, registrationKey, effectKey, observation)
		require.NoError(t, err)
		requireStateEqual(t, bound, replayed)
	}
	require.Equal(t, storedLengths[0], storedLengths[1])

	storedLengths = nil
	for _, size := range []int{testMaxEffectObjectResourceVersionBytes, testMaxEffectObjectResourceVersionBytes + 1} {
		observation := presentObservation(effect, types.UID("unit-object-uid-1"))
		observation.ObjectResourceVersion = strings.Repeat("9", size)
		bound, err := ObserveEffectObject(planned, registrationKey, effectKey, observation)
		require.NoError(t, err, "server-owned object RV max+1 must use a fixed digest")
		storedLengths = append(storedLengths, len(storedStateBytes(t, bound)))
	}
	require.Equal(t, storedLengths[0], storedLengths[1])

	storedLengths = nil
	for _, size := range []int{testMaxFenceResourceVersionBytes, testMaxFenceResourceVersionBytes + 1} {
		draining, err := DrainNamespace(planned, testRegistrationIdentity().NamespaceUID)
		require.NoError(t, err)
		readback := testFenceReadback(t, draining, strings.Repeat("9", size))
		committed, err := BindDrainCommitReadback(draining, NamespaceDrainScope(testRegistrationIdentity().NamespaceUID), readback)
		require.NoError(t, err, "server-owned Fence RV max+1 must use a fixed digest")
		storedLengths = append(storedLengths, len(storedStateBytes(t, committed)))
	}
	require.Equal(t, storedLengths[0], storedLengths[1])

	observation := presentObservation(effect, types.UID("unit-object-uid-1"))
	bound, err := ObserveEffectObject(planned, registrationKey, effectKey, observation)
	require.NoError(t, err)
	authorized, err := AuthorizeDispatch(bound, registrationKey, effectKey)
	require.NoError(t, err)
	exactMax := maxTerminalEvidence()
	terminal, err := MarkEffectTerminal(authorized, registrationKey, effectKey, exactMax)
	require.NoError(t, err)
	require.Equal(t, testMaxTerminalReasonCodeBytes, len(exactMax.ReasonCode))
	require.NotEmpty(t, storedStateBytes(t, terminal))

	tooLong := exactMax
	tooLong.ReasonCode += "X"
	unchanged, err := MarkEffectTerminal(authorized, registrationKey, effectKey, tooLong)
	require.ErrorIs(t, err, ErrTerminalEvidenceTooLarge)
	requireStateEqual(t, authorized, unchanged)
}

func TestRegistryCapacityBoundariesAreAtomic(t *testing.T) {
	require.Equal(t, 4*1024, ReservedProtocolFenceStatusBytes, "reserve is frozen headroom for conditions and future status metadata")
	require.Equal(t, testMaxTerminalReasonCodeBytes, MaxTerminalReasonCodeBytes)

	t.Run("max namespaces", func(t *testing.T) {
		limits := generousRegistryLimits()
		limits.MaxNamespaces = 1
		identity := testRegistrationIdentity()
		state := newFenceState(t, identity.InstallationEpoch, identity.NamespaceUID, testFenceObjectIdentity())
		registered, _, err := RegisterExecution(state, identity, limits)
		require.NoError(t, err)
		before := registrySnapshotOf(t, registered)

		second := identity
		second.ExecutionUID = types.UID("execution-uid-2")
		second.AttemptID = "attempt-4"
		second.NamespaceUID = types.UID("namespace-uid-2")
		second.AuthorityUID = types.UID("authority-uid-2")
		second.QueueUID = types.UID("queue-uid-2")
		next, _, err := RegisterExecution(registered, second, limits)
		require.ErrorIs(t, err, ErrRegistryCapacity)
		requireRegistrySnapshotEqual(t, before, next)
	})

	t.Run("max registrations", func(t *testing.T) {
		limits := generousRegistryLimits()
		limits.MaxRegistrations = 1
		state, _, _ := registeredFenceWithLimits(t, limits)
		before := registrySnapshotOf(t, state)
		second := testRegistrationIdentity()
		second.ExecutionUID = types.UID("execution-uid-2")
		second.AttemptID = "attempt-4"
		next, _, err := RegisterExecution(state, second, limits)
		require.ErrorIs(t, err, ErrRegistryCapacity)
		requireRegistrySnapshotEqual(t, before, next)
	})

	t.Run("max effects", func(t *testing.T) {
		limits := generousRegistryLimits()
		limits.MaxEffects = 1
		state, registrationKey, _ := registeredFenceWithLimits(t, limits)
		planned, _, err := PlanEffect(state, registrationKey, testWindowEffect("target-window-1"), limits)
		require.NoError(t, err)
		before := registrySnapshotOf(t, planned)
		next, _, err := PlanEffect(planned, registrationKey, testWindowEffect("target-window-2"), limits)
		require.ErrorIs(t, err, ErrRegistryCapacity)
		requireRegistrySnapshotEqual(t, before, next)
	})

	t.Run("registration status bytes boundary and plus one", func(t *testing.T) {
		identity := testRegistrationIdentity()
		identity.AttemptID = strings.Repeat("a", 512)
		base := newFenceState(t, identity.InstallationEpoch, identity.NamespaceUID, testFenceObjectIdentity())
		unlimited := generousRegistryLimits()
		candidate, _, err := RegisterExecution(base, identity, unlimited)
		require.NoError(t, err)
		registrationKey, err := RegistrationKey(identity)
		require.NoError(t, err)
		expectedDTO := storedProtocolFenceStatusDTO{
			ProtocolVersion:   storedProtocolFenceStatusV1,
			FenceUIDDigest:    externalUIDDigest(testFenceObjectIdentity().UID),
			InstallationEpoch: identity.InstallationEpoch,
			Namespaces:        []storedNamespaceDTO{{UID: identity.NamespaceUID}},
			Registrations: []storedRegistrationDTO{{
				Key:      registrationKey,
				Identity: storedRegistrationIdentityDTOFrom(identity),
				Phase:    string(RegistrationPhaseActive),
			}},
		}
		expected := marshalExpectedStoredStatus(t, expectedDTO)
		require.JSONEq(t, string(expected), string(storedStateBytes(t, candidate)))
		closeoutReserve := expectedCloseoutReserve(t, expectedDTO)
		require.Equal(t, closeoutReserve, RequiredCloseoutReserve(candidate))
		exact := len(expected) + closeoutReserve + ReservedProtocolFenceStatusBytes

		limits := unlimited
		limits.MaxStatusBytes = exact
		_, _, err = RegisterExecution(base, identity, limits)
		require.NoError(t, err)
		limits.MaxStatusBytes = exact - 1
		next, _, err := RegisterExecution(base, identity, limits)
		require.ErrorIs(t, err, ErrRegistryCapacity)
		requireRegistrySnapshotEqual(t, registrySnapshotOf(t, base), next)
	})

	t.Run("effect status bytes boundary and plus one", func(t *testing.T) {
		state, registrationKey, limits := registeredFence(t)
		effect := testWindowEffect(strings.Repeat("w", 512))
		candidate, _, err := PlanEffect(state, registrationKey, effect, limits)
		require.NoError(t, err)
		effectKey, err := EffectKey(registrationKey, effect)
		require.NoError(t, err)
		expectedDTO := storedProtocolFenceStatusDTO{
			ProtocolVersion:   storedProtocolFenceStatusV1,
			FenceUIDDigest:    externalUIDDigest(testFenceObjectIdentity().UID),
			InstallationEpoch: testRegistrationIdentity().InstallationEpoch,
			Namespaces:        []storedNamespaceDTO{{UID: testRegistrationIdentity().NamespaceUID}},
			Registrations: []storedRegistrationDTO{{
				Key:      registrationKey,
				Identity: storedRegistrationIdentityDTOFrom(testRegistrationIdentity()),
				Phase:    string(RegistrationPhaseActive),
				Effects: []storedEffectDTO{{
					Key:      effectKey,
					Identity: storedEffectIdentityDTOFrom(effect),
					State:    string(EffectStatePlanned),
				}},
			}},
		}
		expected := marshalExpectedStoredStatus(t, expectedDTO)
		require.JSONEq(t, string(expected), string(storedStateBytes(t, candidate)))
		closeoutReserve := expectedCloseoutReserve(t, expectedDTO)
		require.Equal(t, closeoutReserve, RequiredCloseoutReserve(candidate))
		exact := len(expected) + closeoutReserve + ReservedProtocolFenceStatusBytes

		limits.MaxStatusBytes = exact
		_, _, err = PlanEffect(state, registrationKey, effect, limits)
		require.NoError(t, err)
		limits.MaxStatusBytes = exact - 1
		next, _, err := PlanEffect(state, registrationKey, effect, limits)
		require.ErrorIs(t, err, ErrRegistryCapacity)
		requireRegistrySnapshotEqual(t, registrySnapshotOf(t, state), next)
	})
}

func TestPlanEffectReservesTwoLevelCloseoutCapacity(t *testing.T) {
	firstIdentity := testRegistrationIdentity()
	secondIdentity := testRegistrationIdentity()
	secondIdentity.ExecutionUID = types.UID("execution-uid-2")
	secondIdentity.AttemptID = "attempt-4"
	secondIdentity.NamespaceUID = types.UID("namespace-uid-2")
	secondIdentity.AuthorityUID = types.UID("authority-uid-2")
	secondIdentity.QueueUID = types.UID("queue-uid-2")
	effects := []struct {
		registration int
		identity     EffectIdentity
	}{
		{registration: 0, identity: testUnitEffect("unit-" + strings.Repeat("a", 128))},
		{registration: 1, identity: testUnitEffect("unit-" + strings.Repeat("b", 128))},
		{registration: 0, identity: testUnitEffect("unit-" + strings.Repeat("c", 128))},
		{registration: 1, identity: testWindowEffect("window-" + strings.Repeat("d", 128))},
	}

	probeLimits := generousRegistryLimits()
	probe, probeRegistrationKeys := registerTwoNamespaces(t, firstIdentity, secondIdentity, probeLimits)
	probeDTO := storedProtocolFenceStatusDTO{
		ProtocolVersion:   storedProtocolFenceStatusV1,
		FenceUIDDigest:    externalUIDDigest(testFenceObjectIdentity().UID),
		InstallationEpoch: firstIdentity.InstallationEpoch,
		Namespaces: []storedNamespaceDTO{
			{UID: firstIdentity.NamespaceUID},
			{UID: secondIdentity.NamespaceUID},
		},
		Registrations: []storedRegistrationDTO{
			{Key: probeRegistrationKeys[0], Identity: storedRegistrationIdentityDTOFrom(firstIdentity), Phase: string(RegistrationPhaseActive)},
			{Key: probeRegistrationKeys[1], Identity: storedRegistrationIdentityDTOFrom(secondIdentity), Phase: string(RegistrationPhaseActive)},
		},
	}
	for _, effect := range effects {
		var effectKey string
		var err error
		probe, effectKey, err = PlanEffect(probe, probeRegistrationKeys[effect.registration], effect.identity, probeLimits)
		require.NoError(t, err)
		probeDTO.Registrations[effect.registration].Effects = append(probeDTO.Registrations[effect.registration].Effects, storedEffectDTO{
			Key:      effectKey,
			Identity: storedEffectIdentityDTOFrom(effect.identity),
			State:    string(EffectStatePlanned),
		})
	}
	probePlannedBytes := marshalExpectedStoredStatus(t, probeDTO)
	require.JSONEq(t, string(probePlannedBytes), string(storedStateBytes(t, probe)))
	maxStatusBytes := len(probePlannedBytes) + expectedCloseoutReserve(t, probeDTO) + ReservedProtocolFenceStatusBytes

	limits := probeLimits
	limits.MaxEffects = 64
	limits.MaxStatusBytes = maxStatusBytes
	state, registrationKeys := registerTwoNamespaces(t, firstIdentity, secondIdentity, limits)
	acceptedDTO := cloneStoredStatusDTO(t, storedProtocolFenceStatusDTO{
		ProtocolVersion:   storedProtocolFenceStatusV1,
		FenceUIDDigest:    externalUIDDigest(testFenceObjectIdentity().UID),
		InstallationEpoch: firstIdentity.InstallationEpoch,
		Namespaces: []storedNamespaceDTO{
			{UID: firstIdentity.NamespaceUID},
			{UID: secondIdentity.NamespaceUID},
		},
		Registrations: []storedRegistrationDTO{
			{Key: registrationKeys[0], Identity: storedRegistrationIdentityDTOFrom(firstIdentity), Phase: string(RegistrationPhaseActive)},
			{Key: registrationKeys[1], Identity: storedRegistrationIdentityDTOFrom(secondIdentity), Phase: string(RegistrationPhaseActive)},
		},
	})

	for _, effect := range effects {
		var effectKey string
		var err error
		state, effectKey, err = PlanEffect(state, registrationKeys[effect.registration], effect.identity, limits)
		require.NoError(t, err)
		acceptedDTO.Registrations[effect.registration].Effects = append(acceptedDTO.Registrations[effect.registration].Effects, storedEffectDTO{
			Key:      effectKey,
			Identity: storedEffectIdentityDTOFrom(effect.identity),
			State:    string(EffectStatePlanned),
		})
		require.Equal(t, expectedCloseoutReserve(t, acceptedDTO), RequiredCloseoutReserve(state))
		maxCloseoutDTO := expectedMaxLegalCloseoutStatus(t, acceptedDTO)

		closeout := state
		for registrationIndex, registration := range acceptedDTO.Registrations {
			for _, storedEffect := range registration.Effects {
				plannedIdentity := effectIdentityFromStored(storedEffect.Identity)
				maxEffect := storedEffectByKey(t, maxCloseoutDTO, storedEffect.Key)
				if plannedIdentity.Kind == EffectKindTargetFenceWindow && maxEffect.CloseoutVariant == string(CloseoutVariantNotFound) {
					continue
				}
				observation := presentObservation(plannedIdentity, types.UID(strings.Repeat("u", testMaxEffectObjectUIDBytes)))
				observation.ObjectResourceVersion = strings.Repeat("9", testMaxEffectObjectResourceVersionBytes)
				closeout, err = ObserveEffectObject(closeout, registrationKeys[registrationIndex], storedEffect.Key, observation)
				require.NoError(t, err)
				requireStoredWithinLimit(t, closeout, limits)
				if plannedIdentity.Kind == EffectKindUnitExecution {
					closeout, err = AuthorizeDispatch(closeout, registrationKeys[registrationIndex], storedEffect.Key)
					require.NoError(t, err)
					requireStoredWithinLimit(t, closeout, limits)
				}
			}
		}

		closeout, err = DrainNamespace(closeout, firstIdentity.NamespaceUID)
		require.NoError(t, err)
		requireStoredWithinLimit(t, closeout, limits)
		closeout, err = BindDrainCommitReadback(closeout, NamespaceDrainScope(firstIdentity.NamespaceUID), testFenceReadback(t, closeout, strings.Repeat("7", testMaxFenceResourceVersionBytes)))
		require.NoError(t, err)
		requireStoredWithinLimit(t, closeout, limits)
		closeout, err = DrainNamespace(closeout, secondIdentity.NamespaceUID)
		require.NoError(t, err)
		requireStoredWithinLimit(t, closeout, limits)
		closeout, err = BindDrainCommitReadback(closeout, NamespaceDrainScope(secondIdentity.NamespaceUID), testFenceReadback(t, closeout, strings.Repeat("8", testMaxFenceResourceVersionBytes)))
		require.NoError(t, err)
		requireStoredWithinLimit(t, closeout, limits)
		closeout, err = DrainInstallation(closeout)
		require.NoError(t, err)
		requireStoredWithinLimit(t, closeout, limits)
		closeout, err = BindDrainCommitReadback(closeout, InstallationDrainScope(), testFenceReadback(t, closeout, strings.Repeat("9", testMaxFenceResourceVersionBytes)))
		require.NoError(t, err)
		requireStoredWithinLimit(t, closeout, limits)

		for registrationIndex, registration := range acceptedDTO.Registrations {
			for _, storedEffect := range registration.Effects {
				plannedIdentity := effectIdentityFromStored(storedEffect.Identity)
				maxEffect := storedEffectByKey(t, maxCloseoutDTO, storedEffect.Key)
				if plannedIdentity.Kind == EffectKindTargetFenceWindow && maxEffect.CloseoutVariant == string(CloseoutVariantNotFound) {
					drainIdentity := NamespaceDrainIdentity(closeout, registration.Identity.NamespaceUID)
					closeout, err = ObserveEffectObject(closeout, registrationKeys[registrationIndex], storedEffect.Key, notFoundObservation(plannedIdentity, drainIdentity))
					require.NoError(t, err)
					requireStoredWithinLimit(t, closeout, limits)
				} else {
					terminalEvidence := maxTerminalEvidence()
					if plannedIdentity.Kind == EffectKindTargetFenceWindow {
						terminalEvidence.CloseoutVariant = CloseoutVariantObservedTerminal
					}
					closeout, err = MarkEffectTerminal(closeout, registrationKeys[registrationIndex], storedEffect.Key, terminalEvidence)
					require.NoError(t, err)
					requireStoredWithinLimit(t, closeout, limits)
					closeout, err = ConsumeEffect(closeout, registrationKeys[registrationIndex], storedEffect.Key)
					require.NoError(t, err)
					requireStoredWithinLimit(t, closeout, limits)
				}
			}
			closeout, err = ConsumeRegistration(closeout, registrationKeys[registrationIndex])
			require.NoError(t, err)
			requireStoredWithinLimit(t, closeout, limits)
		}
	}

	before := registrySnapshotOf(t, state)
	tooLarge := testUnitEffect("unit-" + strings.Repeat("e", 128))
	next, _, err := PlanEffect(state, registrationKeys[0], tooLarge, limits)
	require.ErrorIs(t, err, ErrRegistryCapacity)
	requireRegistrySnapshotEqual(t, before, next)
}

func TestUnitEffectRequiresFrozenTargetIdentity(t *testing.T) {
	state, registrationKey, limits := registeredFence(t)
	base := testUnitEffect("unit-1")
	for name, clear := range map[string]func(*EffectIdentity){
		"pod UID":   func(v *EffectIdentity) { v.PodUID = "" },
		"container": func(v *EffectIdentity) { v.ContainerName = "" },
		"fence UID": func(v *EffectIdentity) { v.FenceUID = "" },
		"namespace": func(v *EffectIdentity) { v.Namespace = "" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := base
			clear(&changed)
			_, _, err := PlanEffect(state, registrationKey, changed, limits)
			require.ErrorIs(t, err, ErrIdentityIncomplete)
		})
	}
}

type storedProtocolFenceStatusDTO struct {
	ProtocolVersion      string                  `json:"protocolVersion"`
	IntegrityDigest      string                  `json:"integrityDigest"`
	FenceUIDDigest       string                  `json:"fenceUIDDigest"`
	InstallationEpoch    string                  `json:"installationEpoch"`
	InstallationManifest *storedManifestDTO      `json:"installationManifest,omitempty"`
	Namespaces           []storedNamespaceDTO    `json:"namespaces"`
	Registrations        []storedRegistrationDTO `json:"registrations,omitempty"`
}

const storedProtocolFenceStatusV1 = "kubeblocks.io/reconfigure-protocol-fence-status/v1"

const (
	testMaxTerminalReasonCodeBytes          = 128
	testMaxFenceResourceVersionBytes        = 32
	testMaxEffectObjectUIDBytes             = 64
	testMaxEffectObjectResourceVersionBytes = 32
)

type storedNamespaceDTO struct {
	UID      types.UID          `json:"uid"`
	Manifest *storedManifestDTO `json:"manifest,omitempty"`
}

type storedManifestDTO struct {
	Token            string                        `json:"token"`
	Digest           string                        `json:"digest"`
	RegistrationKeys []string                      `json:"registrationKeys"`
	EffectKeys       []string                      `json:"effectKeys"`
	CommitReadback   *storedFenceObjectReadbackDTO `json:"commitReadback,omitempty"`
}

type storedFenceObjectReadbackDTO struct {
	UIDDigest             string `json:"uidDigest"`
	ResourceVersionDigest string `json:"resourceVersionDigest"`
	StoredStatusDigest    string `json:"storedStatusDigest"`
}

type storedRegistrationDTO struct {
	Key      string                        `json:"key"`
	Identity storedRegistrationIdentityDTO `json:"identity"`
	Phase    string                        `json:"phase"`
	Effects  []storedEffectDTO             `json:"effects,omitempty"`
}

type storedRegistrationIdentityDTO struct {
	PhysicalAPIID     string    `json:"physicalAPIID"`
	InstallationEpoch string    `json:"installationEpoch"`
	ExecutionUID      types.UID `json:"executionUID"`
	AttemptID         string    `json:"attemptID"`
	NamespaceUID      types.UID `json:"namespaceUID"`
	KeySecretUID      types.UID `json:"keySecretUID"`
	AuthorityUID      types.UID `json:"authorityUID"`
	QueueUID          types.UID `json:"queueUID"`
}

type storedEffectDTO struct {
	Key             string                          `json:"key"`
	Identity        storedEffectIdentityDTO         `json:"identity"`
	State           string                          `json:"state"`
	CloseoutVariant string                          `json:"closeoutVariant,omitempty"`
	ObjectBinding   *storedObjectBindingDTO         `json:"objectBinding,omitempty"`
	Dispatch        *storedDispatchAuthorizationDTO `json:"dispatch,omitempty"`
	Terminal        *storedTerminalEvidenceDTO      `json:"terminal,omitempty"`
	NotFound        *storedNotFoundEvidenceDTO      `json:"notFound,omitempty"`
	Tombstone       *storedConsumedTombstoneDTO     `json:"tombstone,omitempty"`
}

type storedObjectBindingDTO struct {
	Target                EffectObjectTarget `json:"target"`
	UIDDigest             string             `json:"uidDigest"`
	ResourceVersionDigest string             `json:"resourceVersionDigest"`
}

type storedDispatchAuthorizationDTO struct {
	Token string `json:"token"`
}

type storedTerminalEvidenceDTO struct {
	Outcome          string `json:"outcome"`
	ReasonCode       string `json:"reasonCode"`
	EvidenceDigest   string `json:"evidenceDigest"`
	RecoveryRequired bool   `json:"recoveryRequired"`
}

type storedConsumedTombstoneDTO struct {
	IdentityDigest string `json:"identityDigest"`
}

type storedNotFoundEvidenceDTO struct {
	Target                      EffectObjectTarget `json:"target"`
	DrainToken                  string             `json:"drainToken"`
	ManifestDigest              string             `json:"manifestDigest"`
	FenceUIDDigest              string             `json:"fenceUIDDigest"`
	FenceResourceVersionDigest  string             `json:"fenceResourceVersionDigest"`
	LookupResourceVersionDigest string             `json:"lookupResourceVersionDigest"`
	EvidenceDigest              string             `json:"evidenceDigest"`
}

type storedEffectIdentityDTO struct {
	Kind               string    `json:"kind"`
	DeterministicName  string    `json:"deterministicName"`
	FullIdentityDigest string    `json:"fullIdentityDigest"`
	Namespace          string    `json:"namespace"`
	PodUID             types.UID `json:"podUID,omitempty"`
	ContainerName      string    `json:"containerName,omitempty"`
	FenceUID           types.UID `json:"fenceUID,omitempty"`
}

func expectedStoredStatusBytes(t *testing.T, installationEpoch string, namespaceUID types.UID, registrations []storedRegistrationDTO) []byte {
	t.Helper()
	return marshalExpectedStoredStatus(t, storedProtocolFenceStatusDTO{
		ProtocolVersion:   storedProtocolFenceStatusV1,
		FenceUIDDigest:    externalUIDDigest(testFenceObjectIdentity().UID),
		InstallationEpoch: installationEpoch,
		Namespaces:        []storedNamespaceDTO{{UID: namespaceUID}},
		Registrations:     registrations,
	})
}

func validFinalStoredStatusDTO(t *testing.T) storedProtocolFenceStatusDTO {
	t.Helper()
	identity := testRegistrationIdentity()
	registrationKey, err := RegistrationKey(identity)
	require.NoError(t, err)
	effect := testUnitEffect("unit-1")
	effectKey, err := EffectKey(registrationKey, effect)
	require.NoError(t, err)
	objectUID := types.UID("unit-object-uid-1")
	namespaceManifest := expectedStoredManifest("namespace:"+string(identity.NamespaceUID), []string{registrationKey}, []string{effectKey})
	namespaceManifest.CommitReadback = storedFenceReadbackDTOFrom(FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    "502",
		StoredStatusDigest: testDigest("namespace-drain-status"),
	})
	installationManifest := expectedStoredManifest("installation:"+identity.InstallationEpoch, []string{registrationKey}, []string{effectKey})
	installationManifest.CommitReadback = storedFenceReadbackDTOFrom(FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    "503",
		StoredStatusDigest: testDigest("installation-drain-status"),
	})
	terminal := maxTerminalEvidence()
	return storedProtocolFenceStatusDTO{
		ProtocolVersion:      storedProtocolFenceStatusV1,
		FenceUIDDigest:       externalUIDDigest(testFenceObjectIdentity().UID),
		InstallationEpoch:    identity.InstallationEpoch,
		InstallationManifest: installationManifest,
		Namespaces:           []storedNamespaceDTO{{UID: identity.NamespaceUID, Manifest: namespaceManifest}},
		Registrations: []storedRegistrationDTO{{
			Key:      registrationKey,
			Identity: storedRegistrationIdentityDTOFrom(identity),
			Phase:    string(RegistrationPhaseConsumed),
			Effects: []storedEffectDTO{{
				Key:             effectKey,
				Identity:        storedEffectIdentityDTOFrom(effect),
				State:           string(EffectStateConsumed),
				CloseoutVariant: string(CloseoutVariantUnitExecuted),
				ObjectBinding: &storedObjectBindingDTO{
					Target:                effectTarget(effect),
					UIDDigest:             externalUIDDigest(objectUID),
					ResourceVersionDigest: externalResourceVersionDigest("101"),
				},
				Dispatch:  &storedDispatchAuthorizationDTO{Token: expectedDispatchToken(registrationKey, effectKey, externalUIDDigest(objectUID))},
				Terminal:  storedTerminalEvidenceDTOFrom(terminal),
				Tombstone: &storedConsumedTombstoneDTO{IdentityDigest: expectedConsumedTombstoneDigest(effectKey)},
			}},
		}},
	}
}

func validWindowConsumedStatusDTO(t *testing.T, variant EffectCloseoutVariant) storedProtocolFenceStatusDTO {
	return validWindowConsumedStatusDTOForName(t, variant, "target-window-1")
}

func validWindowConsumedStatusDTOForName(t *testing.T, variant EffectCloseoutVariant, name string) storedProtocolFenceStatusDTO {
	t.Helper()
	identity := testRegistrationIdentity()
	registrationKey, err := RegistrationKey(identity)
	require.NoError(t, err)
	effect := testWindowEffect(name)
	effectKey, err := EffectKey(registrationKey, effect)
	require.NoError(t, err)
	namespaceManifest := expectedStoredManifest("namespace:"+string(identity.NamespaceUID), []string{registrationKey}, []string{effectKey})
	namespaceReadback := FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    "502",
		StoredStatusDigest: testDigest("namespace-window-drain-status"),
	}
	namespaceManifest.CommitReadback = storedFenceReadbackDTOFrom(namespaceReadback)
	storedEffect := storedEffectDTO{
		Key:             effectKey,
		Identity:        storedEffectIdentityDTOFrom(effect),
		State:           string(EffectStateConsumed),
		CloseoutVariant: string(variant),
		Tombstone:       &storedConsumedTombstoneDTO{IdentityDigest: expectedConsumedTombstoneDigest(effectKey)},
	}
	switch variant {
	case CloseoutVariantNotFound:
		storedEffect.NotFound = &storedNotFoundEvidenceDTO{
			Target:                      effectTarget(effect),
			DrainToken:                  namespaceManifest.Token,
			ManifestDigest:              namespaceManifest.Digest,
			FenceUIDDigest:              externalUIDDigest(namespaceReadback.UID),
			FenceResourceVersionDigest:  externalResourceVersionDigest(namespaceReadback.ResourceVersion),
			LookupResourceVersionDigest: externalResourceVersionDigest("5001"),
		}
		storedEffect.NotFound.EvidenceDigest = expectedNotFoundEvidenceDigest(*storedEffect.NotFound)
	case CloseoutVariantObservedTerminal:
		storedEffect.ObjectBinding = &storedObjectBindingDTO{
			Target:                effectTarget(effect),
			UIDDigest:             externalUIDDigest(types.UID("window-object-uid-1")),
			ResourceVersionDigest: externalResourceVersionDigest("101"),
		}
		storedEffect.Terminal = storedTerminalEvidenceDTOFrom(observedTerminalEvidence())
	default:
		require.FailNow(t, "unsupported Window closeout variant", string(variant))
	}
	return storedProtocolFenceStatusDTO{
		ProtocolVersion:   storedProtocolFenceStatusV1,
		FenceUIDDigest:    externalUIDDigest(testFenceObjectIdentity().UID),
		InstallationEpoch: identity.InstallationEpoch,
		Namespaces:        []storedNamespaceDTO{{UID: identity.NamespaceUID, Manifest: namespaceManifest}},
		Registrations: []storedRegistrationDTO{{
			Key:      registrationKey,
			Identity: storedRegistrationIdentityDTOFrom(identity),
			Phase:    string(RegistrationPhaseConsumed),
			Effects:  []storedEffectDTO{storedEffect},
		}},
	}
}

func validWindowTerminalStatusDTO(t *testing.T) storedProtocolFenceStatusDTO {
	t.Helper()
	status := validWindowConsumedStatusDTO(t, CloseoutVariantObservedTerminal)
	status.Registrations[0].Phase = string(RegistrationPhaseActive)
	effect := &status.Registrations[0].Effects[0]
	effect.State = string(EffectStateTerminal)
	effect.Tombstone = nil
	return status
}

func validWindowInstallationNotFoundConsumedStatusDTO(t *testing.T) storedProtocolFenceStatusDTO {
	t.Helper()
	status := validWindowConsumedStatusDTO(t, CloseoutVariantNotFound)
	identity := testRegistrationIdentity()
	effectKey := status.Registrations[0].Effects[0].Key
	installationManifest := expectedStoredManifest("installation:"+identity.InstallationEpoch, []string{status.Registrations[0].Key}, []string{effectKey})
	installationReadback := FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    "503",
		StoredStatusDigest: testDigest("installation-window-drain-status"),
	}
	installationManifest.CommitReadback = storedFenceReadbackDTOFrom(installationReadback)
	status.Namespaces[0].Manifest = nil
	status.InstallationManifest = installationManifest
	notFound := status.Registrations[0].Effects[0].NotFound
	notFound.DrainToken = installationManifest.Token
	notFound.ManifestDigest = installationManifest.Digest
	notFound.FenceUIDDigest = externalUIDDigest(installationReadback.UID)
	notFound.FenceResourceVersionDigest = externalResourceVersionDigest(installationReadback.ResourceVersion)
	notFound.EvidenceDigest = expectedNotFoundEvidenceDigest(*notFound)
	return status
}

func validMixedConsumedStatusDTO(t *testing.T) storedProtocolFenceStatusDTO {
	t.Helper()
	status := validFinalStoredStatusDTO(t)
	notFoundStatus := validWindowConsumedStatusDTOForName(t, CloseoutVariantNotFound, "window-notfound")
	observedStatus := validWindowConsumedStatusDTOForName(t, CloseoutVariantObservedTerminal, "window-observed")
	status.Registrations[0].Effects = append(status.Registrations[0].Effects,
		notFoundStatus.Registrations[0].Effects[0],
		observedStatus.Registrations[0].Effects[0],
	)
	var effectKeys []string
	for _, effect := range status.Registrations[0].Effects {
		effectKeys = append(effectKeys, effect.Key)
	}
	identity := testRegistrationIdentity()
	namespaceManifest := expectedStoredManifest("namespace:"+string(identity.NamespaceUID), []string{status.Registrations[0].Key}, effectKeys)
	namespaceReadback := FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    "502",
		StoredStatusDigest: testDigest("mixed-namespace-drain-status"),
	}
	namespaceManifest.CommitReadback = storedFenceReadbackDTOFrom(namespaceReadback)
	status.Namespaces[0].Manifest = namespaceManifest
	installationManifest := expectedStoredManifest("installation:"+identity.InstallationEpoch, []string{status.Registrations[0].Key}, effectKeys)
	installationManifest.CommitReadback = storedFenceReadbackDTOFrom(FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    "503",
		StoredStatusDigest: testDigest("mixed-installation-drain-status"),
	})
	status.InstallationManifest = installationManifest
	notFound := status.Registrations[0].Effects[1].NotFound
	notFound.DrainToken = namespaceManifest.Token
	notFound.ManifestDigest = namespaceManifest.Digest
	notFound.FenceUIDDigest = externalUIDDigest(namespaceReadback.UID)
	notFound.FenceResourceVersionDigest = externalResourceVersionDigest(namespaceReadback.ResourceVersion)
	notFound.EvidenceDigest = expectedNotFoundEvidenceDigest(*notFound)
	return status
}

func validStoredStatusForPhase(t *testing.T, phase EffectState) storedProtocolFenceStatusDTO {
	t.Helper()
	status := cloneStoredStatusDTO(t, validFinalStoredStatusDTO(t))
	status.Registrations[0].Phase = string(RegistrationPhaseActive)
	effect := &status.Registrations[0].Effects[0]
	effect.State = string(phase)
	switch phase {
	case EffectStatePlanned:
		effect.CloseoutVariant = ""
		effect.ObjectBinding = nil
		effect.Dispatch = nil
		effect.Terminal = nil
		effect.Tombstone = nil
	case EffectStateObjectBound:
		effect.CloseoutVariant = ""
		effect.Dispatch = nil
		effect.Terminal = nil
		effect.Tombstone = nil
	case EffectStateDispatchAuthorized:
		effect.CloseoutVariant = ""
		effect.Terminal = nil
		effect.Tombstone = nil
	case EffectStateTerminal:
		effect.CloseoutVariant = string(CloseoutVariantUnitExecuted)
		effect.Tombstone = nil
	case EffectStateConsumed:
		effect.CloseoutVariant = string(CloseoutVariantUnitExecuted)
	default:
		require.FailNow(t, "unsupported phase fixture", string(phase))
	}
	return status
}

func marshalExpectedStoredStatus(t *testing.T, status storedProtocolFenceStatusDTO) []byte {
	t.Helper()
	status.IntegrityDigest = ""
	unsigned, err := json.Marshal(status)
	require.NoError(t, err)
	status.IntegrityDigest = testDigest("kubeblocks.io/reconfigure-stored-status-integrity/v1\x00" + string(unsigned))
	data, err := json.Marshal(status)
	require.NoError(t, err)
	return data
}

func expectedStoredManifest(scope string, registrationKeys, effectKeys []string) *storedManifestDTO {
	joinedRegistrations := strings.Join(registrationKeys, "\x00")
	joinedKeys := strings.Join(effectKeys, "\x00")
	return &storedManifestDTO{
		Token:            testDigest("kubeblocks.io/reconfigure-drain-token/v1\x00" + scope + "\x00" + joinedRegistrations + "\x00" + joinedKeys),
		Digest:           testDigest("kubeblocks.io/reconfigure-manifest/v1\x00" + joinedRegistrations + "\x00" + joinedKeys),
		RegistrationKeys: registrationKeys,
		EffectKeys:       effectKeys,
	}
}

func storedFenceReadbackDTOFrom(readback FenceObjectReadback) *storedFenceObjectReadbackDTO {
	return &storedFenceObjectReadbackDTO{
		UIDDigest:             externalUIDDigest(readback.UID),
		ResourceVersionDigest: externalResourceVersionDigest(readback.ResourceVersion),
		StoredStatusDigest:    readback.StoredStatusDigest,
	}
}

func externalUIDDigest(uid types.UID) string {
	return testDigest("kubeblocks.io/reconfigure-external-uid/v1\x00" + string(uid))
}

func externalResourceVersionDigest(resourceVersion string) string {
	return testDigest("kubeblocks.io/reconfigure-external-rv/v1\x00" + resourceVersion)
}

func expectedDispatchToken(registrationKey, effectKey, objectUIDDigest string) string {
	return testDigest("kubeblocks.io/reconfigure-dispatch/v1\x00" + registrationKey + "\x00" + effectKey + "\x00" + objectUIDDigest)
}

func expectedConsumedTombstoneDigest(effectKey string) string {
	return testDigest("kubeblocks.io/reconfigure-consumed/v1\x00" + effectKey)
}

func observedTerminalEvidence() EffectTerminalEvidence {
	return EffectTerminalEvidence{
		CloseoutVariant: CloseoutVariantObservedTerminal,
		Outcome:         EffectTerminalSucceeded,
		ReasonCode:      "Completed",
		EvidenceDigest:  testDigest("terminal-success"),
	}
}

func executedTerminalEvidence() EffectTerminalEvidence {
	evidence := observedTerminalEvidence()
	evidence.CloseoutVariant = CloseoutVariantUnitExecuted
	return evidence
}

func maxTerminalEvidence() EffectTerminalEvidence {
	return EffectTerminalEvidence{
		CloseoutVariant:  CloseoutVariantUnitExecuted,
		Outcome:          EffectTerminalManual,
		ReasonCode:       strings.Repeat("R", testMaxTerminalReasonCodeBytes),
		EvidenceDigest:   testDigest("max-terminal-evidence"),
		RecoveryRequired: true,
	}
}

func storedTerminalEvidenceDTOFrom(evidence EffectTerminalEvidence) *storedTerminalEvidenceDTO {
	return &storedTerminalEvidenceDTO{
		Outcome:          string(evidence.Outcome),
		ReasonCode:       evidence.ReasonCode,
		EvidenceDigest:   evidence.EvidenceDigest,
		RecoveryRequired: evidence.RecoveryRequired,
	}
}

func storedNotFoundEvidenceDTOFrom(observation EffectObjectObservation) *storedNotFoundEvidenceDTO {
	evidence := &storedNotFoundEvidenceDTO{
		Target:                      observation.Target,
		DrainToken:                  observation.DrainIdentity.Token,
		ManifestDigest:              observation.DrainIdentity.ManifestDigest,
		FenceUIDDigest:              observation.DrainIdentity.FenceUIDDigest,
		FenceResourceVersionDigest:  observation.DrainIdentity.FenceResourceVersionDigest,
		LookupResourceVersionDigest: externalResourceVersionDigest(observation.LookupResourceVersion),
	}
	evidence.EvidenceDigest = expectedNotFoundEvidenceDigest(*evidence)
	return evidence
}

func expectedNotFoundEvidenceDigest(evidence storedNotFoundEvidenceDTO) string {
	return testDigest(strings.Join([]string{
		"kubeblocks.io/reconfigure-notfound-evidence/v1",
		evidence.Target.APIVersion,
		evidence.Target.Kind,
		evidence.Target.Namespace,
		evidence.Target.Name,
		evidence.DrainToken,
		evidence.ManifestDigest,
		evidence.FenceUIDDigest,
		evidence.FenceResourceVersionDigest,
		evidence.LookupResourceVersionDigest,
	}, "\x00"))
}

func expectedCloseoutReserve(t *testing.T, planned storedProtocolFenceStatusDTO) int {
	t.Helper()
	plannedBytes := marshalExpectedStoredStatus(t, planned)
	closeout := expectedMaxLegalCloseoutStatus(t, planned)
	return len(marshalExpectedStoredStatus(t, closeout)) - len(plannedBytes)
}

func expectedMaxLegalCloseoutStatus(t *testing.T, planned storedProtocolFenceStatusDTO) storedProtocolFenceStatusDTO {
	t.Helper()
	closeout := cloneStoredStatusDTO(t, planned)
	var allRegistrationKeys []string
	var allEffectKeys []string
	for namespaceIndex := range closeout.Namespaces {
		namespaceUID := closeout.Namespaces[namespaceIndex].UID
		var namespaceRegistrationKeys []string
		var namespaceEffectKeys []string
		for _, registration := range closeout.Registrations {
			if registration.Identity.NamespaceUID != namespaceUID {
				continue
			}
			namespaceRegistrationKeys = append(namespaceRegistrationKeys, registration.Key)
			allRegistrationKeys = append(allRegistrationKeys, registration.Key)
			for _, effect := range registration.Effects {
				namespaceEffectKeys = append(namespaceEffectKeys, effect.Key)
				allEffectKeys = append(allEffectKeys, effect.Key)
			}
		}
		closeout.Namespaces[namespaceIndex].Manifest = expectedStoredManifest("namespace:"+string(namespaceUID), namespaceRegistrationKeys, namespaceEffectKeys)
		closeout.Namespaces[namespaceIndex].Manifest.CommitReadback = maxStoredFenceReadbackDTO()
	}
	closeout.InstallationManifest = expectedStoredManifest("installation:"+closeout.InstallationEpoch, allRegistrationKeys, allEffectKeys)
	closeout.InstallationManifest.CommitReadback = maxStoredFenceReadbackDTO()
	type effectPosition struct {
		registration int
		effect       int
	}
	var positions []effectPosition
	for registrationIndex := range closeout.Registrations {
		closeout.Registrations[registrationIndex].Phase = string(RegistrationPhaseConsumed)
		for effectIndex := range closeout.Registrations[registrationIndex].Effects {
			positions = append(positions, effectPosition{registration: registrationIndex, effect: effectIndex})
		}
	}
	var candidates []storedProtocolFenceStatusDTO
	var build func(int)
	build = func(positionIndex int) {
		if positionIndex == len(positions) {
			candidates = append(candidates, cloneStoredStatusDTO(t, closeout))
			return
		}
		position := positions[positionIndex]
		registration := &closeout.Registrations[position.registration]
		plannedEffect := registration.Effects[position.effect]
		namespaceManifest := namespaceManifestForRegistration(t, closeout, registration.Identity.NamespaceUID)
		for _, legal := range legalCloseoutEffects(t, registration.Key, plannedEffect, namespaceManifest) {
			registration.Effects[position.effect] = legal
			build(positionIndex + 1)
		}
		registration.Effects[position.effect] = plannedEffect
	}
	build(0)
	require.NotEmpty(t, candidates)
	maxIndex := 0
	maxBytes := -1
	for index, candidate := range candidates {
		data := marshalExpectedStoredStatus(t, candidate)
		decoded, err := DecodeStoredProtocolFenceStatus(data, testFenceObjectIdentity())
		require.NoError(t, err, "every reserve candidate must be a legal restart state")
		require.False(t, IsZeroFenceState(decoded))
		if len(data) > maxBytes {
			maxIndex = index
			maxBytes = len(data)
		}
	}
	return candidates[maxIndex]
}

func legalCloseoutEffects(t *testing.T, registrationKey string, planned storedEffectDTO, namespaceManifest *storedManifestDTO) []storedEffectDTO {
	t.Helper()
	base := planned
	base.State = string(EffectStateConsumed)
	base.Tombstone = &storedConsumedTombstoneDTO{IdentityDigest: expectedConsumedTombstoneDigest(base.Key)}
	objectBinding := maxStoredObjectBindingDTO(base.Identity)
	terminal := &storedTerminalEvidenceDTO{
		Outcome:          string(EffectTerminalManual),
		ReasonCode:       strings.Repeat("R", testMaxTerminalReasonCodeBytes),
		EvidenceDigest:   testDigest("max-terminal-evidence"),
		RecoveryRequired: true,
	}
	switch base.Identity.Kind {
	case string(EffectKindUnitExecution):
		base.CloseoutVariant = string(CloseoutVariantUnitExecuted)
		base.ObjectBinding = objectBinding
		base.Dispatch = &storedDispatchAuthorizationDTO{Token: expectedDispatchToken(registrationKey, base.Key, objectBinding.UIDDigest)}
		base.Terminal = terminal
		return []storedEffectDTO{base}
	case string(EffectKindTargetFenceWindow):
		require.NotNil(t, namespaceManifest)
		require.NotNil(t, namespaceManifest.CommitReadback)
		notFound := base
		notFound.CloseoutVariant = string(CloseoutVariantNotFound)
		notFound.NotFound = &storedNotFoundEvidenceDTO{
			Target:                      objectBinding.Target,
			DrainToken:                  namespaceManifest.Token,
			ManifestDigest:              namespaceManifest.Digest,
			FenceUIDDigest:              namespaceManifest.CommitReadback.UIDDigest,
			FenceResourceVersionDigest:  namespaceManifest.CommitReadback.ResourceVersionDigest,
			LookupResourceVersionDigest: externalResourceVersionDigest(strings.Repeat("9", testMaxFenceResourceVersionBytes)),
		}
		notFound.NotFound.EvidenceDigest = expectedNotFoundEvidenceDigest(*notFound.NotFound)
		observed := base
		observed.CloseoutVariant = string(CloseoutVariantObservedTerminal)
		observed.ObjectBinding = objectBinding
		observed.Terminal = terminal
		return []storedEffectDTO{notFound, observed}
	default:
		require.FailNow(t, "unsupported effect kind in capacity oracle", base.Identity.Kind)
		return nil
	}
}

func namespaceManifestForRegistration(t *testing.T, status storedProtocolFenceStatusDTO, namespaceUID types.UID) *storedManifestDTO {
	t.Helper()
	for _, namespace := range status.Namespaces {
		if namespace.UID == namespaceUID {
			return namespace.Manifest
		}
	}
	require.FailNow(t, "registration namespace missing from stored status", string(namespaceUID))
	return nil
}

func storedEffectByKey(t *testing.T, status storedProtocolFenceStatusDTO, effectKey string) storedEffectDTO {
	t.Helper()
	for _, registration := range status.Registrations {
		for _, effect := range registration.Effects {
			if effect.Key == effectKey {
				return effect
			}
		}
	}
	require.FailNow(t, "effect missing from stored status", effectKey)
	return storedEffectDTO{}
}

func maxStoredFenceReadbackDTO() *storedFenceObjectReadbackDTO {
	return &storedFenceObjectReadbackDTO{
		UIDDigest:             externalUIDDigest(testFenceObjectIdentity().UID),
		ResourceVersionDigest: externalResourceVersionDigest(strings.Repeat("9", testMaxFenceResourceVersionBytes)),
		StoredStatusDigest:    testDigest("max-stored-status"),
	}
}

func maxStoredObjectBindingDTO(identity storedEffectIdentityDTO) *storedObjectBindingDTO {
	kind := "GateWindow"
	if identity.Kind == string(EffectKindUnitExecution) {
		kind = "UnitExecution"
	}
	return &storedObjectBindingDTO{
		Target: EffectObjectTarget{
			APIVersion: "protocol.kubeblocks.io/v1alpha1",
			Kind:       kind,
			Namespace:  "kb-system",
			Name:       identity.DeterministicName,
		},
		UIDDigest:             externalUIDDigest(types.UID(strings.Repeat("u", testMaxEffectObjectUIDBytes))),
		ResourceVersionDigest: externalResourceVersionDigest(strings.Repeat("9", testMaxEffectObjectResourceVersionBytes)),
	}
}

func cloneStoredStatusDTO(t *testing.T, status storedProtocolFenceStatusDTO) storedProtocolFenceStatusDTO {
	t.Helper()
	data := marshalExpectedStoredStatus(t, status)
	var cloned storedProtocolFenceStatusDTO
	require.NoError(t, json.Unmarshal(data, &cloned))
	return cloned
}

func storedRegistrationIdentityDTOFrom(identity RegistrationIdentity) storedRegistrationIdentityDTO {
	return storedRegistrationIdentityDTO(identity)
}

func storedEffectIdentityDTOFrom(identity EffectIdentity) storedEffectIdentityDTO {
	return storedEffectIdentityDTO{
		Kind:               string(identity.Kind),
		DeterministicName:  identity.DeterministicName,
		FullIdentityDigest: identity.FullIdentityDigest,
		Namespace:          identity.Namespace,
		PodUID:             identity.PodUID,
		ContainerName:      identity.ContainerName,
		FenceUID:           identity.FenceUID,
	}
}

func effectIdentityFromStored(identity storedEffectIdentityDTO) EffectIdentity {
	return EffectIdentity{
		Kind:               EffectKind(identity.Kind),
		DeterministicName:  identity.DeterministicName,
		FullIdentityDigest: identity.FullIdentityDigest,
		Namespace:          identity.Namespace,
		PodUID:             identity.PodUID,
		ContainerName:      identity.ContainerName,
		FenceUID:           identity.FenceUID,
	}
}

func requireStoredWithinLimit(t *testing.T, state FenceState, limits RegistryLimits) {
	t.Helper()
	require.LessOrEqual(t, len(storedStateBytes(t, state))+ReservedProtocolFenceStatusBytes, limits.MaxStatusBytes)
}

func requireWrongFenceUIDDecodeRejected(t *testing.T, data []byte) {
	t.Helper()
	decoded, err := DecodeStoredProtocolFenceStatus(data, FenceObjectIdentity{UID: types.UID("same-name-new-fence-uid")})
	require.ErrorIs(t, err, ErrFenceReadbackMismatch)
	require.True(t, IsZeroFenceState(decoded))
}

type registrySnapshot struct {
	bytes         []byte
	storedBytes   []byte
	namespaces    int
	registrations int
	effects       int
	manifest      string
}

func registrySnapshotOf(t *testing.T, state FenceState) registrySnapshot {
	t.Helper()
	return registrySnapshot{
		bytes:         canonicalStateBytes(t, state),
		storedBytes:   storedStateBytes(t, state),
		namespaces:    NamespaceCount(state),
		registrations: RegistrationCount(state),
		effects:       TotalEffectCount(state),
		manifest:      CanonicalManifestDigest(state),
	}
}

func storedStateBytes(t *testing.T, state FenceState) []byte {
	t.Helper()
	data, err := StoredProtocolFenceStatusBytes(state)
	require.NoError(t, err)
	return data
}

func requireRegistrySnapshotEqual(t *testing.T, expected registrySnapshot, actual FenceState) {
	t.Helper()
	require.Equal(t, expected, registrySnapshotOf(t, actual))
}

func registeredFence(t *testing.T) (FenceState, string, RegistryLimits) {
	t.Helper()
	return registeredFenceWithLimits(t, generousRegistryLimits())
}

func registeredFenceWithLimits(t *testing.T, limits RegistryLimits) (FenceState, string, RegistryLimits) {
	t.Helper()
	identity := testRegistrationIdentity()
	state := newFenceState(t, identity.InstallationEpoch, identity.NamespaceUID, testFenceObjectIdentity())
	registered, registrationKey, err := RegisterExecution(state, identity, limits)
	require.NoError(t, err)
	return registered, registrationKey, limits
}

func registerTwoNamespaces(t *testing.T, first, second RegistrationIdentity, limits RegistryLimits) (FenceState, [2]string) {
	t.Helper()
	state := newFenceState(t, first.InstallationEpoch, first.NamespaceUID, testFenceObjectIdentity())
	registered, firstKey, err := RegisterExecution(state, first, limits)
	require.NoError(t, err)
	registered, secondKey, err := RegisterExecution(registered, second, limits)
	require.NoError(t, err)
	return registered, [2]string{firstKey, secondKey}
}

func generousRegistryLimits() RegistryLimits {
	return RegistryLimits{
		MaxNamespaces:    8,
		MaxRegistrations: 16,
		MaxEffects:       64,
		MaxStatusBytes:   128 * 1024,
	}
}

func testRegistrationIdentity() RegistrationIdentity {
	return RegistrationIdentity{
		PhysicalAPIID:     "api-uid-1",
		InstallationEpoch: "install-7",
		ExecutionUID:      types.UID("execution-uid-1"),
		AttemptID:         "attempt-3",
		NamespaceUID:      types.UID("namespace-uid-1"),
		KeySecretUID:      types.UID("key-secret-uid-1"),
		AuthorityUID:      types.UID("authority-uid-1"),
		QueueUID:          types.UID("queue-uid-1"),
	}
}

func testFenceObjectIdentity() FenceObjectIdentity {
	return FenceObjectIdentity{UID: types.UID("protocol-fence-uid-1")}
}

func newFenceState(t *testing.T, installationEpoch string, namespaceUID types.UID, identity FenceObjectIdentity) FenceState {
	t.Helper()
	state, err := NewFenceState(installationEpoch, namespaceUID, identity)
	require.NoError(t, err)
	return state
}

func testFenceReadback(t *testing.T, state FenceState, resourceVersion string) FenceObjectReadback {
	t.Helper()
	return FenceObjectReadback{
		UID:                testFenceObjectIdentity().UID,
		ResourceVersion:    resourceVersion,
		StoredStatusDigest: testDigest(string(storedStateBytes(t, state))),
	}
}

func activeDrainIdentity(t *testing.T, state FenceState) DrainIdentity {
	t.Helper()
	view, err := BindActiveFenceReadback(state, testFenceReadback(t, state, "501"))
	require.NoError(t, err)
	identity := ActiveDrainIdentity(view)
	require.NotEmpty(t, identity.FenceUIDDigest)
	require.NotEmpty(t, identity.FenceResourceVersionDigest)
	return identity
}

func commitNamespaceDrain(t *testing.T, state FenceState, resourceVersion string) (FenceState, DrainIdentity) {
	t.Helper()
	committed, err := BindDrainCommitReadback(state, NamespaceDrainScope(testRegistrationIdentity().NamespaceUID), testFenceReadback(t, state, resourceVersion))
	require.NoError(t, err)
	identity := NamespaceDrainIdentity(committed, testRegistrationIdentity().NamespaceUID)
	require.NotEmpty(t, identity.FenceUIDDigest)
	require.NotEmpty(t, identity.FenceResourceVersionDigest)
	return committed, identity
}

func commitInstallationDrain(t *testing.T, state FenceState, resourceVersion string) (FenceState, DrainIdentity) {
	t.Helper()
	committed, err := BindDrainCommitReadback(state, InstallationDrainScope(), testFenceReadback(t, state, resourceVersion))
	require.NoError(t, err)
	identity := InstallationDrainIdentity(committed)
	require.NotEmpty(t, identity.FenceUIDDigest)
	require.NotEmpty(t, identity.FenceResourceVersionDigest)
	return committed, identity
}

func testWindowEffect(name string) EffectIdentity {
	return EffectIdentity{
		Kind:               EffectKindTargetFenceWindow,
		DeterministicName:  name,
		FullIdentityDigest: testDigest("window:" + name),
		Namespace:          "kb-system",
	}
}

func testUnitEffect(name string) EffectIdentity {
	return EffectIdentity{
		Kind:               EffectKindUnitExecution,
		DeterministicName:  name,
		FullIdentityDigest: testDigest("unit:" + name),
		Namespace:          "kb-system",
		PodUID:             types.UID("pod-uid-1"),
		ContainerName:      "mysql",
		FenceUID:           types.UID("fence-uid-1"),
	}
}

func effectTarget(effect EffectIdentity) EffectObjectTarget {
	kind := "GateWindow"
	if effect.Kind == EffectKindUnitExecution {
		kind = "UnitExecution"
	}
	return EffectObjectTarget{
		APIVersion: "protocol.kubeblocks.io/v1alpha1",
		Kind:       kind,
		Namespace:  effect.Namespace,
		Name:       effect.DeterministicName,
	}
}

func presentObservation(effect EffectIdentity, uid types.UID) EffectObjectObservation {
	return EffectObjectObservation{
		Result:                EffectObservationPresent,
		Target:                effectTarget(effect),
		ObjectUID:             uid,
		ObjectResourceVersion: "101",
		LookupResourceVersion: "5001",
		ObservedAt:            metav1.NewTime(time.Unix(1_752_211_200, 0)),
	}
}

func terminatingObservation(effect EffectIdentity, uid types.UID) EffectObjectObservation {
	observation := presentObservation(effect, uid)
	observation.Result = EffectObservationTerminating
	return observation
}

func notFoundObservation(effect EffectIdentity, drainIdentity DrainIdentity) EffectObjectObservation {
	return EffectObjectObservation{
		Result:                EffectObservationNotFound,
		Target:                effectTarget(effect),
		DrainIdentity:         drainIdentity,
		LookupResourceVersion: "5001",
		ObservedAt:            metav1.NewTime(time.Unix(1_752_211_200, 0)),
	}
}

func apiErrorObservation(effect EffectIdentity) EffectObjectObservation {
	return EffectObjectObservation{
		Result:     EffectObservationAPIError,
		Target:     effectTarget(effect),
		ObservedAt: metav1.NewTime(time.Unix(1_752_211_200, 0)),
		ErrorClass: "Timeout",
	}
}
