/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package consensusset

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

var _ handler.EventHandler = &EnqueueRequestForAncestor{}

var log = logf.FromContext(context.Background()).WithName("eventhandler").WithName("EnqueueRequestForAncestor")

// EnqueueRequestForAncestor enqueues Requests for the ancestor object.
// E.g. the ancestor object creates the StatefulSet/Deployment which then creates the Pod.
//
// If a ConsensusSet creates Pods, users may reconcile the ConsensusSet in response to Pod Events using:
//
// - a source.Kind Source with Type of Pod.
//
// - a EnqueueRequestForAncestor EventHandler with an OwnerType of ConsensusSet and UpToLevel set to 2.
//
// If source kind is corev1.Event, Event.InvolvedObject will be used as the source kind
type EnqueueRequestForAncestor struct {
	// Client used to get owner object of
	Client roclient.ReadonlyClient

	// OwnerType is the type of the Owner object to look for in OwnerReferences.  Only Group and Kind are compared.
	OwnerType runtime.Object

	// find event source up to UpToLevel
	UpToLevel int

	// InTypes specified the range to look for the ancestor, means all ancestors' type in the looking up tree should be in InTypes.
	// OwnerType will be included.
	// nil means only look for in OwnerType.
	InTypes []runtime.Object

	// groupKind is the cached Group and Kind from OwnerType
	groupKind *schema.GroupKind

	// ancestorGroupKinds is the cached Group and Kind from InTypes
	ancestorGroupKinds []schema.GroupKind

	// mapper maps GroupVersionKinds to Resources
	mapper meta.RESTMapper
}

type empty struct{}

// Create implements EventHandler.
func (e *EnqueueRequestForAncestor) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]empty{}
	e.getOwnerReconcileRequest(evt.Object, reqs)
	for req := range reqs {
		q.Add(req)
	}
}

// Update implements EventHandler.
func (e *EnqueueRequestForAncestor) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]empty{}
	e.getOwnerReconcileRequest(evt.ObjectOld, reqs)
	e.getOwnerReconcileRequest(evt.ObjectNew, reqs)
	for req := range reqs {
		q.Add(req)
	}
}

// Delete implements EventHandler.
func (e *EnqueueRequestForAncestor) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]empty{}
	e.getOwnerReconcileRequest(evt.Object, reqs)
	for req := range reqs {
		q.Add(req)
	}
}

// Generic implements EventHandler.
func (e *EnqueueRequestForAncestor) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]empty{}
	e.getOwnerReconcileRequest(evt.Object, reqs)
	for req := range reqs {
		q.Add(req)
	}
}

// parseOwnerTypeGroupKind parses the OwnerType into a Group and Kind and caches the result.  Returns false
// if the OwnerType could not be parsed using the scheme.
func (e *EnqueueRequestForAncestor) parseOwnerTypeGroupKind(scheme *runtime.Scheme) error {
	gk, err := e.parseTypeGroupKind(e.OwnerType, scheme)
	if err != nil {
		return err
	}
	// Cache the Group and Kind for the OwnerType
	e.groupKind = gk
	return nil
}

// parseInTypesGroupKind parses the InTypes into a Group and Kind and caches the result.  Returns false
// if the InTypes could not be parsed using the scheme.
func (e *EnqueueRequestForAncestor) parseInTypesGroupKind(scheme *runtime.Scheme) error {
	if e.groupKind != nil {
		e.ancestorGroupKinds = append(e.ancestorGroupKinds, *e.groupKind)
	}
	for _, inType := range e.InTypes {
		gk, err := e.parseTypeGroupKind(inType, scheme)
		if err != nil {
			return err
		}
		// Cache the Group and Kind for the inType
		e.ancestorGroupKinds = append(e.ancestorGroupKinds, *gk)
	}
	return nil
}

func (e *EnqueueRequestForAncestor) parseTypeGroupKind(object runtime.Object, scheme *runtime.Scheme) (*schema.GroupKind, error) {
	// Get the kinds of the type
	kinds, _, err := scheme.ObjectKinds(object)
	if err != nil {
		log.Error(err, "Could not get ObjectKinds", "object", fmt.Sprintf("%T", object))
		return nil, err
	}
	// Expect only 1 kind.  If there is more than one kind this is probably an edge case such as ListOptions.
	if len(kinds) != 1 {
		err := fmt.Errorf("expected exactly 1 kind for object %T, but found %s kinds", object, kinds)
		log.Error(nil, "expected exactly 1 kind for object", "object", fmt.Sprintf("%T", object), "kinds", kinds)
		return nil, err
	}
	return &schema.GroupKind{Group: kinds[0].Group, Kind: kinds[0].Kind}, nil
}

// getOwnerReconcileRequest looks at object and builds a map of reconcile.Request to reconcile
// owners of object that match e.OwnerType.
func (e *EnqueueRequestForAncestor) getOwnerReconcileRequest(obj client.Object, result map[reconcile.Request]empty) {
	// get the object by the ownerRef
	object, err := e.getSourceObject(obj)
	if err != nil {
		log.Info("could not find source object", "gvk", obj.GetObjectKind().GroupVersionKind(), "name", obj.GetName(), "error", err.Error())
		return
	}

	// find the root object up to UpToLevel
	scheme := *model.GetScheme()
	ctx := context.Background()
	ref, err := e.getOwnerUpTo(ctx, object, e.UpToLevel, scheme)
	if err != nil {
		log.Info("cloud not find ancestor object",
			"source object gvk", object.GetObjectKind().GroupVersionKind(),
			"name", object.GetName(),
			"up to level", e.UpToLevel,
			"error", err.Error())
		return
	}
	if ref == nil {
		log.Info("cloud not find ancestor object",
			"source object gvk", object.GetObjectKind().GroupVersionKind(),
			"name", object.GetName(),
			"up to level", e.UpToLevel)
		return
	}

	// Parse the Group out of the OwnerReference to compare it to what was parsed out of the requested OwnerType
	refGV, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		log.Error(err, "Could not parse OwnerReference APIVersion",
			"api version", ref.APIVersion)
		return
	}

	// Compare the OwnerReference Group and Kind against the OwnerType Group and Kind specified by the user.
	// If the two match, create a Request for the objected referred to by
	// the OwnerReference.  Use the Name from the OwnerReference and the Namespace from the
	// object in the event.
	if ref.Kind == e.groupKind.Kind && refGV.Group == e.groupKind.Group {
		// Match found - add a Request for the object referred to in the OwnerReference
		request := reconcile.Request{NamespacedName: types.NamespacedName{
			Name: ref.Name,
		}}

		// if owner is not namespaced then we should set the namespace to the empty
		mapping, err := e.mapper.RESTMapping(*e.groupKind, refGV.Version)
		if err != nil {
			log.Error(err, "Could not retrieve rest mapping", "kind", e.groupKind)
			return
		}
		if mapping.Scope.Name() != meta.RESTScopeNameRoot {
			request.Namespace = object.GetNamespace()
		}

		result[request] = empty{}
	}
}

func (e *EnqueueRequestForAncestor) getSourceObject(object client.Object) (client.Object, error) {
	eventObject, ok := object.(*corev1.Event)
	// return the object directly if it's not corev1.Event kind
	if !ok {
		return object, nil
	}

	objectRef := eventObject.InvolvedObject
	scheme := *model.GetScheme()
	// convert ObjectReference to OwnerReference
	ownerRef := metav1.OwnerReference{
		APIVersion: objectRef.APIVersion,
		Kind:       objectRef.Kind,
		Name:       objectRef.Name,
		UID:        objectRef.UID,
	}

	ctx := context.Background()
	// get the object by the ownerRef
	sourceObject, err := e.getObjectByOwnerRef(ctx, objectRef.Namespace, ownerRef, scheme)
	if err != nil {
		return nil, err
	}
	return sourceObject, nil
}

// getOwnerUpTo gets the owner of object up to upToLevel.
// E.g. If ConsensusSet creates the StatefulSet which then creates the Pod,
// if the object is the Pod, then set upToLevel to 2 if you want to find the ConsensusSet.
// Each level of ownership should be a controller-relationship (i.e. controller=true in ownerReferences).
// nil return if no owner find in any level.
func (e *EnqueueRequestForAncestor) getOwnerUpTo(ctx context.Context, object client.Object, upToLevel int, scheme runtime.Scheme) (*metav1.OwnerReference, error) {
	if upToLevel <= 0 {
		return nil, nil
	}
	if object == nil {
		return nil, nil
	}
	ownerRef := metav1.GetControllerOf(object)
	if ownerRef == nil {
		return nil, nil
	}
	if upToLevel == 1 {
		return ownerRef, nil
	}
	objectNew, err := e.getObjectByOwnerRef(ctx, object.GetNamespace(), *ownerRef, scheme)
	if err != nil {
		return nil, err
	}
	return e.getOwnerUpTo(ctx, objectNew, upToLevel-1, scheme)
}

func (e *EnqueueRequestForAncestor) getObjectByOwnerRef(ctx context.Context, ownerNameSpace string, ownerRef metav1.OwnerReference, scheme runtime.Scheme) (client.Object, error) {
	gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
	if err != nil {
		return nil, err
	}
	gvk := schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    ownerRef.Kind,
	}
	if !e.inAncestorRange(gvk) {
		return nil, nil
	}
	objectRT, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	object, ok := objectRT.(client.Object)
	if !ok {
		return nil, errors.New("runtime object can't be converted to client object")
	}
	request := reconcile.Request{NamespacedName: types.NamespacedName{
		Name: ownerRef.Name,
	}}
	// if owner is not namespaced then we should set the namespace to the empty
	groupKind := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := e.mapper.RESTMapping(groupKind, gvk.Version)
	if err != nil {
		return nil, err
	}
	if mapping.Scope.Name() != meta.RESTScopeNameRoot {
		request.Namespace = ownerNameSpace
	}
	if err := e.Client.Get(ctx, request.NamespacedName, object); err != nil {
		return nil, err
	}
	return object, nil
}

func (e *EnqueueRequestForAncestor) inAncestorRange(gvk schema.GroupVersionKind) bool {
	for _, groupKind := range e.ancestorGroupKinds {
		if gvk.Group == groupKind.Group && gvk.Kind == groupKind.Kind {
			return true
		}
	}
	return false
}

var _ inject.Scheme = &EnqueueRequestForAncestor{}

// InjectScheme is called by the Controller to provide a singleton scheme to the EnqueueRequestForAncestor.
func (e *EnqueueRequestForAncestor) InjectScheme(s *runtime.Scheme) error {
	if err := e.parseOwnerTypeGroupKind(s); err != nil {
		return err
	}
	return e.parseInTypesGroupKind(s)
}

var _ inject.Mapper = &EnqueueRequestForAncestor{}

// InjectMapper  is called by the Controller to provide the rest mapper used by the manager.
func (e *EnqueueRequestForAncestor) InjectMapper(m meta.RESTMapper) error {
	e.mapper = m
	return nil
}
