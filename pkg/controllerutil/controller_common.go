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

package controllerutil

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// ResultToP converts a Result object to a pointer.
func ResultToP(res reconcile.Result, err error) (*reconcile.Result, error) {
	return &res, err
}

// Reconciled returns an empty result with nil error to signal a successful reconcile
// to the controller manager
func Reconciled() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

// CheckedRequeueWithError passes the error through to the controller
// manager, it ignores unknown errors.
func CheckedRequeueWithError(err error, logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if apierrors.IsNotFound(err) {
		return Reconciled()
	}
	return RequeueWithError(err, logger, msg, keysAndValues...)
}

// RequeueWithErrorAndRecordEvent requeues when an error occurs. if it is an unknown error, triggers an event
func RequeueWithErrorAndRecordEvent(obj client.Object, recorder record.EventRecorder, err error, logger logr.Logger) (reconcile.Result, error) {
	if apierrors.IsNotFound(err) && recorder != nil {
		recorder.Eventf(obj, corev1.EventTypeWarning, constant.ReasonNotFoundCR, err.Error())
	}
	return RequeueWithError(err, logger, "")
}

// RequeueWithError requeues when an error occurs
func RequeueWithError(err error, logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if msg == "" {
		logger.Info(err.Error())
	} else {
		// Info log the error message and then let the reconciler dump the stacktrace
		logger.Info(msg, keysAndValues...)
	}
	return reconcile.Result{}, err
}

func RequeueAfter(duration time.Duration, logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	keysAndValues = append(keysAndValues, "duration")
	keysAndValues = append(keysAndValues, duration)
	if msg != "" {
		msg = fmt.Sprintf("reason: %s; retry-after", msg)
	} else {
		msg = "retry-after"
	}
	logger.V(1).Info(msg, keysAndValues...)
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: duration,
	}, nil
}

func Requeue(logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if msg == "" {
		msg = "requeue"
	}
	logger.V(1).Info(msg, keysAndValues...)
	return reconcile.Result{Requeue: true}, nil
}

// HandleCRDeletion handles CR deletion, adds finalizer if found a non-deleting object and removes finalizer during
// deletion process. Passes optional 'deletionHandler' func for external dependency deletion. Returns Result pointer
// if required to return out of outer 'Reconcile' reconciliation loop.
func HandleCRDeletion(reqCtx RequestCtx,
	r client.Writer,
	cr client.Object,
	finalizer string,
	deletionHandler func() (*ctrl.Result, error)) (*ctrl.Result, error) {
	// examine DeletionTimestamp to determine if object is under deletion
	if cr.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then add the finalizer and update the object. This is equivalent to
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(cr, finalizer) {
			controllerutil.AddFinalizer(cr, finalizer)
			if err := r.Update(reqCtx.Ctx, cr); err != nil {
				return ResultToP(CheckedRequeueWithError(err, reqCtx.Log, ""))
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(cr, finalizer) {
			// We need to record the deletion event first.
			// If the resource has dependencies, it will not be automatically deleted.
			// It can also prevent users from manually deleting it without event records
			if reqCtx.Recorder != nil {
				cluster, ok := cr.(*v1alpha1.Cluster)
				// throw warning event if terminationPolicy set to DoNotTerminate
				if ok && cluster.Spec.TerminationPolicy == v1alpha1.DoNotTerminate {
					reqCtx.Eventf(cr, corev1.EventTypeWarning, constant.ReasonDeleteFailed,
						"Deleting %s: %s failed due to terminationPolicy set to DoNotTerminate",
						strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
				} else {
					reqCtx.Eventf(cr, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s: %s",
						strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
				}
			}

			// our finalizer is present, so handle any external dependency
			if deletionHandler != nil {
				if res, err := deletionHandler(); err != nil {
					// if failed to delete the external dependencies here, return with error
					// so that it can be retried
					if res == nil {
						return ResultToP(CheckedRequeueWithError(err, reqCtx.Log, ""))
					}
					return res, err
				} else if res != nil {
					return res, nil
				}
			}
			// remove our finalizer from the list and update it.
			if controllerutil.RemoveFinalizer(cr, finalizer) {
				if err := r.Update(reqCtx.Ctx, cr); err != nil {
					return ResultToP(CheckedRequeueWithError(err, reqCtx.Log, ""))
				}
				// record resources deleted event
				reqCtx.Eventf(cr, corev1.EventTypeNormal, constant.ReasonDeletedCR, "Deleted %s: %s",
					strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
			}
		}

		// Stop reconciliation as the item is being deleted
		res, err := Reconciled()
		return &res, err
	}
	return nil, nil
}

// ValidateReferenceCR validates existing referencing CRs, if exists, requeue reconcile after 30 seconds
func ValidateReferenceCR(reqCtx RequestCtx, cli client.Client, obj client.Object,
	labelKey string, recordEvent func(), objLists ...client.ObjectList) (*ctrl.Result, error) {
	for _, objList := range objLists {
		// get referencing cr list
		if err := cli.List(reqCtx.Ctx, objList,
			client.MatchingLabels{labelKey: obj.GetName()}, client.Limit(1),
		); err != nil {
			return nil, err
		}
		if v, err := conversion.EnforcePtr(objList); err != nil {
			return nil, err
		} else {
			// check list items
			items := v.FieldByName("Items")
			if !items.IsValid() || items.Kind() != reflect.Slice || items.Len() == 0 {
				continue
			}
			if recordEvent != nil {
				recordEvent()
			}
			return ResultToP(RequeueAfter(time.Second, reqCtx.Log, ""))
		}
	}
	return nil, nil
}

// RecordCreatedEvent records an event when a CR created successfully
func RecordCreatedEvent(r record.EventRecorder, cr client.Object) {
	if r != nil && cr.GetGeneration() == 1 {
		r.Eventf(cr, corev1.EventTypeNormal, constant.ReasonCreatedCR, "Created %s: %s", strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
	}
}

// WorkloadFilterPredicate provides filter predicate for workload objects, i.e., deployment/statefulset/pod/pvc.
func WorkloadFilterPredicate(object client.Object) bool {
	_, containCompNameLabelKey := object.GetLabels()[constant.KBAppComponentLabelKey]
	return ManagedByKubeBlocksFilterPredicate(object) && containCompNameLabelKey
}

// ManagedByKubeBlocksFilterPredicate provides filter predicate for objects managed by kubeBlocks.
func ManagedByKubeBlocksFilterPredicate(object client.Object) bool {
	return object.GetLabels()[constant.AppManagedByLabelKey] == constant.AppName
}

// IgnoreIsAlreadyExists returns errors if 'err' is not type of AlreadyExists
func IgnoreIsAlreadyExists(err error) error {
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// BackgroundDeleteObject deletes the object in the background, usually used in the Reconcile method
func BackgroundDeleteObject(cli client.Client, ctx context.Context, obj client.Object) error {
	deletePropagation := metav1.DeletePropagationBackground
	deleteOptions := &client.DeleteOptions{
		PropagationPolicy: &deletePropagation,
	}

	if err := cli.Delete(ctx, obj, deleteOptions); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

// SetOwnership provides helper function controllerutil.SetControllerReference/controllerutil.SetOwnerReference
// and controllerutil.AddFinalizer if not exists.
func SetOwnership(owner, obj client.Object, scheme *runtime.Scheme, finalizer string, useOwnerReference ...bool) error {
	if len(useOwnerReference) > 0 && useOwnerReference[0] {
		if err := controllerutil.SetOwnerReference(owner, obj, scheme); err != nil {
			return err
		}
	} else {
		if err := controllerutil.SetControllerReference(owner, obj, scheme); err != nil {
			return err
		}
	}
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		// pvc objects do not need to add finalizer
		_, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			if !controllerutil.AddFinalizer(obj, finalizer) {
				return ErrFailedToAddFinalizer
			}
		}
	}
	return nil
}

// CheckResourceExists checks whether resource exist or not.
func CheckResourceExists(
	ctx context.Context,
	cli client.Client,
	key client.ObjectKey,
	obj client.Object) (bool, error) {
	if err := cli.Get(ctx, key, obj); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	// if found, return true
	return true, nil
}
