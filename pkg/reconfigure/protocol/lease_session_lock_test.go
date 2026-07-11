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
	"context"
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

func TestSessionLeaseLockAcquireAndRenewKeepEpochAndKeyAtomic(t *testing.T) {
	ctx := context.Background()
	client := fakeClientAssigningLeaseUID(types.UID("lease-uid-1"))
	publicKey, privateKey := testEd25519KeyPair(t)
	lock, err := NewSessionLeaseLock(SessionLeaseLockConfig{
		Client:         client.CoordinationV1(),
		Namespace:      "kb-system",
		Name:           "reconfigure-protocol",
		HolderIdentity: "controller-pod-uid-1/process-1",
		KeyPair: SessionKeyPair{
			Algorithm:  SessionKeyAlgorithmEd25519,
			PublicKey:  publicKey,
			PrivateKey: privateKey,
		},
		LeaseDuration: 30 * time.Second,
	})
	require.NoError(t, err)

	session, err := lock.Acquire(ctx)
	require.NoError(t, err)
	lease, err := client.CoordinationV1().Leases("kb-system").Get(ctx, "reconfigure-protocol", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, types.UID("lease-uid-1"), lease.UID)
	require.Equal(t, session.HolderIdentity, *lease.Spec.HolderIdentity)
	require.Equal(t, "1", lease.Annotations[LeaseSessionEpochAnnotation])
	require.Equal(t, SessionKeyAlgorithmEd25519, lease.Annotations[LeaseSessionKeyAlgorithmAnnotation])
	require.Equal(t, encodeSessionPublicKey(publicKey), lease.Annotations[LeaseSessionPublicKeyAnnotation])
	require.Equal(t, sessionKeyFingerprint(publicKey), lease.Annotations[LeaseSessionKeyFingerprintAnnotation])
	require.Equal(t, []string{"create"}, mutatingLeaseVerbs(client.Actions()), "acquire must not patch key metadata after leadership")

	keyAnnotations := leaseSessionKeyAnnotations(lease)
	holderBefore := *lease.Spec.HolderIdentity
	require.NoError(t, lock.Renew(ctx))
	renewed, err := client.CoordinationV1().Leases("kb-system").Get(ctx, lease.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, keyAnnotations, leaseSessionKeyAnnotations(renewed), "renew may only update lease timing fields")
	require.Equal(t, holderBefore, *renewed.Spec.HolderIdentity)
	require.Equal(t, []string{"create", "update"}, mutatingLeaseVerbs(client.Actions()))
}

func TestSessionLeaseLockRecoversCommittedUpdateAfterResponseLoss(t *testing.T) {
	ctx := context.Background()
	oldPublicKey, _ := testEd25519KeyPair(t)
	oldHolder := "controller-pod-uid-1/process-1"
	expired := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kb-system",
			Name:      "reconfigure-protocol",
			UID:       types.UID("lease-uid-1"),
			Annotations: map[string]string{
				LeaseSessionEpochAnnotation:          "7",
				LeaseSessionKeyAlgorithmAnnotation:   SessionKeyAlgorithmEd25519,
				LeaseSessionPublicKeyAnnotation:      encodeSessionPublicKey(oldPublicKey),
				LeaseSessionKeyFingerprintAnnotation: sessionKeyFingerprint(oldPublicKey),
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &oldHolder,
			LeaseDurationSeconds: ptr(int32(1)),
			RenewTime:            &metav1.MicroTime{Time: time.Now().Add(-time.Minute)},
		},
	}
	client := fake.NewSimpleClientset(expired)
	updateLost := false
	client.PrependReactor("update", "leases", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if updateLost {
			return false, nil, nil
		}
		updateLost = true
		update := action.(k8stesting.UpdateAction)
		lease := update.GetObject().(*coordinationv1.Lease).DeepCopy()
		require.Equal(t, "8", lease.Annotations[LeaseSessionEpochAnnotation])
		require.Equal(t, SessionKeyAlgorithmEd25519, lease.Annotations[LeaseSessionKeyAlgorithmAnnotation])
		require.NotEmpty(t, lease.Annotations[LeaseSessionPublicKeyAnnotation])
		require.NotEmpty(t, lease.Annotations[LeaseSessionKeyFingerprintAnnotation])
		err := client.Tracker().Update(leaseGVR(), lease, lease.Namespace)
		require.NoError(t, err)
		return true, nil, errors.New("simulated response loss after committed update")
	})

	publicKey, privateKey := testEd25519KeyPair(t)
	keyPair := SessionKeyPair{
		Algorithm:  SessionKeyAlgorithmEd25519,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}
	lock, err := NewSessionLeaseLock(SessionLeaseLockConfig{
		Client:         client.CoordinationV1(),
		Namespace:      "kb-system",
		Name:           "reconfigure-protocol",
		HolderIdentity: "controller-pod-uid-1/process-2",
		KeyPair:        keyPair,
		LeaseDuration:  30 * time.Second,
	})
	require.NoError(t, err)
	_, err = lock.Acquire(ctx)
	require.Error(t, err)

	recovered, err := lock.RecoverCommittedSession(ctx, keyPair)
	require.NoError(t, err)
	require.Equal(t, int64(8), recovered.Epoch)
	require.Equal(t, sessionKeyFingerprint(publicKey), recovered.SessionKeyFingerprint)
	require.Equal(t, types.UID("lease-uid-1"), recovered.LeaseUID)
	require.Equal(t, []string{"update"}, mutatingLeaseVerbs(client.Actions()), "recovery must only read the committed Lease")

	_, wrongPrivateKey := testEd25519KeyPair(t)
	wrongKeyPair := keyPair
	wrongKeyPair.PrivateKey = wrongPrivateKey
	_, err = lock.RecoverCommittedSession(ctx, wrongKeyPair)
	require.ErrorIs(t, err, ErrLeaseSessionKeyUnavailable)
	missingPrivateKey := keyPair
	missingPrivateKey.PrivateKey = nil
	_, err = lock.RecoverCommittedSession(ctx, missingPrivateKey)
	require.ErrorIs(t, err, ErrLeaseSessionKeyUnavailable)
	require.Equal(t, []string{"update"}, mutatingLeaseVerbs(client.Actions()))
}

func TestSessionLeaseLockRecoversCommittedCreateAfterResponseLoss(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "leases", func(action k8stesting.Action) (bool, runtime.Object, error) {
		create := action.(k8stesting.CreateAction)
		lease := create.GetObject().(*coordinationv1.Lease).DeepCopy()
		lease.UID = types.UID("lease-uid-1")
		require.Equal(t, "1", lease.Annotations[LeaseSessionEpochAnnotation])
		require.NotEmpty(t, lease.Annotations[LeaseSessionPublicKeyAnnotation])
		require.NoError(t, client.Tracker().Create(leaseGVR(), lease, lease.Namespace))
		return true, nil, errors.New("simulated response loss after committed create")
	})

	publicKey, privateKey := testEd25519KeyPair(t)
	keyPair := SessionKeyPair{
		Algorithm:  SessionKeyAlgorithmEd25519,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}
	lock, err := NewSessionLeaseLock(SessionLeaseLockConfig{
		Client:         client.CoordinationV1(),
		Namespace:      "kb-system",
		Name:           "reconfigure-protocol",
		HolderIdentity: "controller-pod-uid-1/process-1",
		KeyPair:        keyPair,
		LeaseDuration:  30 * time.Second,
	})
	require.NoError(t, err)
	_, err = lock.Acquire(ctx)
	require.Error(t, err)
	require.Equal(t, []string{"create"}, mutatingLeaseVerbs(client.Actions()))

	recovered, err := lock.RecoverCommittedSession(ctx, keyPair)
	require.NoError(t, err)
	require.Equal(t, types.UID("lease-uid-1"), recovered.LeaseUID)
	require.Equal(t, int64(1), recovered.Epoch)
	require.Equal(t, sessionKeyFingerprint(publicKey), recovered.SessionKeyFingerprint)
	require.Equal(t, []string{"create"}, mutatingLeaseVerbs(client.Actions()), "recovery must not rewrite the Lease")

	_, wrongPrivateKey := testEd25519KeyPair(t)
	wrongKeyPair := keyPair
	wrongKeyPair.PrivateKey = wrongPrivateKey
	_, err = lock.RecoverCommittedSession(ctx, wrongKeyPair)
	require.ErrorIs(t, err, ErrLeaseSessionKeyUnavailable)
	missingPrivateKey := keyPair
	missingPrivateKey.PrivateKey = nil
	_, err = lock.RecoverCommittedSession(ctx, missingPrivateKey)
	require.ErrorIs(t, err, ErrLeaseSessionKeyUnavailable)

	lease, err := client.CoordinationV1().Leases("kb-system").Get(ctx, "reconfigure-protocol", metav1.GetOptions{})
	require.NoError(t, err)
	lease.UID = types.UID("lease-uid-2")
	require.NoError(t, client.Tracker().Update(leaseGVR(), lease, lease.Namespace))
	_, err = lock.RecoverCommittedSession(ctx, keyPair)
	require.ErrorIs(t, err, ErrLeaseUIDChanged)
	require.Equal(t, []string{"create"}, mutatingLeaseVerbs(client.Actions()))
}

func TestSessionLeaseLockRejectsUnsupportedAndConflictingPaths(t *testing.T) {
	require.ErrorIs(t, ValidateSessionKeyBindingLock(&resourcelock.LeaseLock{}), ErrSessionKeyBindingUnsupported)

	t.Run("second contender cannot acquire current lease", func(t *testing.T) {
		ctx := context.Background()
		client := fakeClientAssigningLeaseUID(types.UID("lease-uid-1"))
		first := newTestSessionLeaseLock(t, client, "controller-pod-uid-1/process-1")
		second := newTestSessionLeaseLock(t, client, "controller-pod-uid-2/process-1")
		_, err := first.Acquire(ctx)
		require.NoError(t, err)
		_, err = second.Acquire(ctx)
		require.ErrorIs(t, err, ErrLeaseSessionHeld)
	})

	t.Run("same epoch key metadata patch fails closed", func(t *testing.T) {
		ctx := context.Background()
		client := fakeClientAssigningLeaseUID(types.UID("lease-uid-1"))
		lock := newTestSessionLeaseLock(t, client, "controller-pod-uid-1/process-1")
		_, err := lock.Acquire(ctx)
		require.NoError(t, err)
		lease, err := client.CoordinationV1().Leases("kb-system").Get(ctx, "reconfigure-protocol", metav1.GetOptions{})
		require.NoError(t, err)
		lease.Annotations[LeaseSessionKeyFingerprintAnnotation] = testDigest("forged-key")
		_, err = client.CoordinationV1().Leases("kb-system").Update(ctx, lease, metav1.UpdateOptions{})
		require.NoError(t, err)
		require.ErrorIs(t, lock.Renew(ctx), ErrLeaseSessionKeyChanged)
	})

	t.Run("same name new lease UID fails closed", func(t *testing.T) {
		ctx := context.Background()
		client := fakeClientAssigningLeaseUID(types.UID("lease-uid-1"))
		lock := newTestSessionLeaseLock(t, client, "controller-pod-uid-1/process-1")
		_, err := lock.Acquire(ctx)
		require.NoError(t, err)
		lease, err := client.CoordinationV1().Leases("kb-system").Get(ctx, "reconfigure-protocol", metav1.GetOptions{})
		require.NoError(t, err)
		lease.UID = types.UID("lease-uid-2")
		require.NoError(t, client.Tracker().Update(leaseGVR(), lease, lease.Namespace))
		require.ErrorIs(t, lock.Renew(ctx), ErrLeaseUIDChanged)
	})
}

func TestSessionLeaseLockOwnsPublicBindingAndDoesNotRetainPrivateKey(t *testing.T) {
	ctx := context.Background()
	client := fakeClientAssigningLeaseUID(types.UID("lease-uid-1"))
	publicKey, privateKey := testEd25519KeyPair(t)
	expectedPublic := append(ed25519.PublicKey(nil), publicKey...)
	expectedPrivate := append(ed25519.PrivateKey(nil), privateKey...)
	lock, err := NewSessionLeaseLock(SessionLeaseLockConfig{
		Client:         client.CoordinationV1(),
		Namespace:      "kb-system",
		Name:           "reconfigure-protocol",
		HolderIdentity: "controller-pod-uid-1/process-1",
		KeyPair: SessionKeyPair{
			Algorithm:  SessionKeyAlgorithmEd25519,
			PublicKey:  publicKey,
			PrivateKey: privateKey,
		},
		LeaseDuration: 30 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, expectedPublic, lock.config.KeyPair.PublicKey)
	require.Nil(t, lock.config.KeyPair.PrivateKey, "private key ownership remains with the external keystore/caller")

	publicKey[0] ^= 0xff
	privateKey[0] ^= 0xff
	session, err := lock.Acquire(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedPublic, ed25519.PublicKey(session.SessionPublicKey))
	lease, err := client.CoordinationV1().Leases("kb-system").Get(ctx, "reconfigure-protocol", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, encodeSessionPublicKey(expectedPublic), lease.Annotations[LeaseSessionPublicKeyAnnotation])
	require.Equal(t, sessionKeyFingerprint(expectedPublic), lease.Annotations[LeaseSessionKeyFingerprintAnnotation])

	session.SessionPublicKey[0] ^= 0xff
	require.NoError(t, lock.Renew(ctx))
	_, err = lock.RecoverCommittedSession(ctx, SessionKeyPair{Algorithm: SessionKeyAlgorithmEd25519, PublicKey: expectedPublic, PrivateKey: expectedPrivate})
	require.NoError(t, err)
}

func fakeClientAssigningLeaseUID(uid types.UID) *fake.Clientset {
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "leases", func(action k8stesting.Action) (bool, runtime.Object, error) {
		create := action.(k8stesting.CreateAction)
		lease := create.GetObject().(*coordinationv1.Lease).DeepCopy()
		lease.UID = uid
		if err := client.Tracker().Create(leaseGVR(), lease, lease.Namespace); err != nil {
			return true, nil, err
		}
		return true, lease, nil
	})
	return client
}

func newTestSessionLeaseLock(t *testing.T, client *fake.Clientset, holder string) *SessionLeaseLock {
	t.Helper()
	publicKey, privateKey := testEd25519KeyPair(t)
	lock, err := NewSessionLeaseLock(SessionLeaseLockConfig{
		Client:         client.CoordinationV1(),
		Namespace:      "kb-system",
		Name:           "reconfigure-protocol",
		HolderIdentity: holder,
		KeyPair: SessionKeyPair{
			Algorithm:  SessionKeyAlgorithmEd25519,
			PublicKey:  publicKey,
			PrivateKey: privateKey,
		},
		LeaseDuration: 30 * time.Second,
	})
	require.NoError(t, err)
	return lock
}

func leaseSessionKeyAnnotations(lease *coordinationv1.Lease) map[string]string {
	return map[string]string{
		LeaseSessionEpochAnnotation:          lease.Annotations[LeaseSessionEpochAnnotation],
		LeaseSessionKeyAlgorithmAnnotation:   lease.Annotations[LeaseSessionKeyAlgorithmAnnotation],
		LeaseSessionPublicKeyAnnotation:      lease.Annotations[LeaseSessionPublicKeyAnnotation],
		LeaseSessionKeyFingerprintAnnotation: lease.Annotations[LeaseSessionKeyFingerprintAnnotation],
	}
}

func leaseGVR() schema.GroupVersionResource {
	return coordinationv1.SchemeGroupVersion.WithResource("leases")
}

func mutatingLeaseVerbs(actions []k8stesting.Action) []string {
	var verbs []string
	for _, action := range actions {
		if action.GetResource().Resource != "leases" {
			continue
		}
		switch action.GetVerb() {
		case "create", "update", "patch":
			verbs = append(verbs, action.GetVerb())
		}
	}
	return verbs
}

func ptr[T any](value T) *T {
	return &value
}
