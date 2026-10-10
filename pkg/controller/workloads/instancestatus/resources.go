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

package instancestatus

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

// ResourceObservation contains actual resources and policy-required cleanup, independently of runtime health.
type ResourceObservation struct {
	InstanceName    string
	InstancePresent bool
	StorageCleanup  bool
	Pod             *workloads.InstanceObjectReference
	Storage         *workloads.InstanceStorageIdentity
}

// ObjectReference preserves actual UID and location; nil Cluster explicitly means unknown provenance.
func ObjectReference(ctx context.Context, obj client.Object) *workloads.InstanceObjectReference {
	if obj == nil {
		return nil
	}
	result := &workloads.InstanceObjectReference{Name: obj.GetName(), Namespace: obj.GetNamespace(), UID: obj.GetUID()}
	if location, known := multicluster.ObjectLocation(ctx, obj); known {
		result.Cluster = ptr.To(location)
	}
	return result
}

func referenceKnown(ref *workloads.InstanceObjectReference) bool {
	return ref != nil && ref.UID != "" && ref.Cluster != nil
}

// ObserveStorage describes mounted storage while a Pod exists, and declared or retained claims while absent.
// Claims outside the supplied observations remain unknown. This function never infers data initialization.
func ObserveStorage(ctx context.Context, pod *corev1.Pod, expected map[string]string, pvcs []*corev1.PersistentVolumeClaim) *workloads.InstanceStorageIdentity {
	claims := make(map[string]string)
	ephemeral := false
	unsupported := false
	if pod != nil {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				claims[volume.Name] = volume.PersistentVolumeClaim.ClaimName
			}
			source := volume.VolumeSource
			source.PersistentVolumeClaim, source.EmptyDir = nil, nil
			source.Secret, source.ConfigMap, source.DownwardAPI, source.Projected = nil, nil, nil, nil
			if source != (corev1.VolumeSource{}) {
				unsupported = true
			}
			if volume.EmptyDir != nil || volume.Ephemeral != nil {
				ephemeral = true
			}
		}
		ephemeral = ephemeral || len(claims) == 0 && !unsupported
	} else {
		for name, volume := range expected {
			claims[volume] = name
		}
		for _, pvc := range pvcs {
			found := false
			for _, claimName := range claims {
				if claimName == pvc.Name {
					found = true
					break
				}
			}
			if !found {
				volume := pvc.Labels[constant.VolumeClaimTemplateNameLabelKey]
				if volume == "" {
					volume = pvc.Name
				}
				if existing, exists := claims[volume]; exists && existing != pvc.Name {
					volume = pvc.Name
				}
				claims[volume] = pvc.Name
			}
		}
		ephemeral = len(claims) == 0
	}
	claimContext := ctx
	podLocation, podLocationKnown := "", false
	if pod != nil {
		podLocation, podLocationKnown = multicluster.ObjectLocation(ctx, pod)
		if podLocationKnown {
			claimContext = multicluster.IntoContext(ctx, podLocation)
		}
	}
	observed := make(map[string]*corev1.PersistentVolumeClaim, len(pvcs))
	for _, pvc := range pvcs {
		if pod != nil {
			if !podLocationKnown || pvc.Namespace != pod.Namespace {
				continue
			}
			if location, known := multicluster.ObjectLocation(ctx, pvc); known && location != podLocation {
				continue
			}
		}
		if existing, duplicate := observed[pvc.Name]; duplicate && existing != nil && existing.UID != pvc.UID {
			observed[pvc.Name] = nil
			continue
		}
		observed[pvc.Name] = pvc
	}

	storage := &workloads.InstanceStorageIdentity{Complete: !unsupported}
	names := make([]string, 0, len(claims))
	for name := range claims {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		volume := workloads.InstanceVolumeIdentity{Name: name}
		if pvc := observed[claims[name]]; pvc != nil {
			volume.Claim = ObjectReference(claimContext, pvc)
			volume.VolumeName = pvc.Spec.VolumeName
			if owner := metav1.GetControllerOf(pvc); owner != nil {
				volume.OwnerUID = owner.UID
			}
		}
		if !referenceKnown(volume.Claim) || volume.VolumeName == "" {
			storage.Complete = false
		}
		storage.Volumes = append(storage.Volumes, volume)
	}
	if ephemeral {
		if pod != nil {
			storage.EphemeralPod = ObjectReference(ctx, pod)
		}
		if !referenceKnown(storage.EphemeralPod) {
			storage.Complete = false
		}
	}
	return storage
}

// ReadMountedClaims reads only missing mounted PVCs at the known Pod location. Returned claims are observation-only;
// callers must never add them to their modifiable object tree. Read errors leave identity unknown and do not prove absence.
func ReadMountedClaims(ctx context.Context, reader client.Reader, pod *corev1.Pod, supplied []*corev1.PersistentVolumeClaim) ([]*corev1.PersistentVolumeClaim, bool) {
	if reader == nil || pod == nil {
		return supplied, false
	}
	location, known := multicluster.ObjectLocation(ctx, pod)
	if !known {
		return supplied, false
	}
	observed := make(map[string]bool, len(supplied))
	result := make([]*corev1.PersistentVolumeClaim, 0, len(supplied))
	for _, pvc := range supplied {
		claimLocation, claimLocationKnown := multicluster.ObjectLocation(ctx, pvc)
		if pvc.Namespace == pod.Namespace && claimLocationKnown && claimLocation == location {
			observed[pvc.Name] = true
			result = append(result, pvc)
		}
	}

	pending := false
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		name := volume.PersistentVolumeClaim.ClaimName
		if observed[name] {
			continue
		}
		observed[name] = true
		pvc := &corev1.PersistentVolumeClaim{}
		if err := reader.Get(multicluster.IntoContext(ctx, location), types.NamespacedName{Namespace: pod.Namespace, Name: name}, pvc, multicluster.InDataContext()); err != nil {
			pending = true
			continue
		}
		result = append(result, pvc)
	}
	return result, pending
}

// ReadMissingOwnedClaims verifies previously observed policy-required claims missing from a non-atomic list.
// NotFound settles that resource; unavailable or unknown location preserves the actual cleanup obligation.
func ReadMissingOwnedClaims(ctx context.Context, reader client.Reader, ownerUID types.UID, previous *workloads.InstanceStorageIdentity, supplied []*corev1.PersistentVolumeClaim) ([]*corev1.PersistentVolumeClaim, bool, map[types.UID]string) {
	result := append([]*corev1.PersistentVolumeClaim(nil), supplied...)
	pending := false
	locations := make(map[types.UID]string)
	if previous == nil || ownerUID == "" {
		return result, false, locations
	}
	for _, volume := range previous.Volumes {
		if volume.OwnerUID != ownerUID || volume.Claim == nil {
			continue
		}
		ref := volume.Claim
		seen := false
		for _, claim := range supplied {
			if location, known := multicluster.ObjectLocation(ctx, claim); known && ref.Cluster != nil && location == *ref.Cluster && claim.UID == ref.UID && claim.Namespace == ref.Namespace && claim.Name == ref.Name {
				seen = true
				break
			}
		}
		if seen {
			continue
		}
		if reader == nil || ref.Cluster == nil {
			pending = true
			continue
		}
		claim := &corev1.PersistentVolumeClaim{}
		if err := reader.Get(multicluster.IntoContext(ctx, *ref.Cluster), types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, claim, multicluster.InDataContext()); err != nil {
			if !apierrors.IsNotFound(err) {
				pending = true
			}
			continue
		}
		if owner := metav1.GetControllerOf(claim); owner != nil && owner.UID == ownerUID {
			result = append(result, claim)
			locations[claim.UID] = *ref.Cluster
		}
	}
	return result, pending, locations
}
