/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	"github.com/apecloud/kubeblocks/internal/constant"
)

// Reconciled returns an empty result with nil error to signal a successful reconcile
// to the controller manager
func Reconciled() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

// CheckedRequeueWithError is a convenience wrapper around logging an error message
// separate from the stacktrace and then passing the error through to the controller
// manager, this will ignore not-found errors.
func CheckedRequeueWithError(err error, logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if apierrors.IsNotFound(err) {
		return Reconciled()
	}
	return RequeueWithError(err, logger, msg, keysAndValues...)
}

// RequeueWithErrorAndRecordEvent requeue when an error occurs. if it is a not found error, send an event
func RequeueWithErrorAndRecordEvent(obj client.Object, recorder record.EventRecorder, err error, logger logr.Logger) (reconcile.Result, error) {
	if apierrors.IsNotFound(err) {
		recorder.Eventf(obj, corev1.EventTypeWarning, constant.ReasonNotFoundCR, err.Error())
	}
	return RequeueWithError(err, logger, "")
}

// RequeueWithError requeue when an error occurs
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

// HandleCRDeletion Handled CR deletion flow, will add finalizer if discovered a non-deleting object and remove finalizer during
// deletion process. Pass optional 'deletionHandler' func for external dependency deletion. Return Result pointer
// if required to return out of outer 'Reconcile' reconciliation loop.
func HandleCRDeletion(reqCtx RequestCtx,
	r client.Writer,
	cr client.Object,
	finalizer string,
	deletionHandler func() (*ctrl.Result, error)) (*ctrl.Result, error) {
	// examine DeletionTimestamp to determine if object is under deletion
	if cr.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(cr, finalizer) {
			controllerutil.AddFinalizer(cr, finalizer)
			if err := r.Update(reqCtx.Ctx, cr); err != nil {
				res, err := CheckedRequeueWithError(err, reqCtx.Log, "")
				return &res, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(cr, finalizer) {
			// We need to record the deletion event first.
			// Because if the resource has dependencies, it will not be automatically deleted.
			// so it can prevent users from manually deleting it without event records
			if reqCtx.Recorder != nil {
				cluster, ok := cr.(*v1alpha1.Cluster)
				// throw warning event if terminationPolicy set to DoNotTerminate
				if ok && cluster.Spec.TerminationPolicy == v1alpha1.DoNotTerminate {
					reqCtx.Recorder.Eventf(cr, corev1.EventTypeWarning, constant.ReasonDeleteFailed,
						"Deleting %s: %s failed due to terminationPolicy set to DoNotTerminate",
						strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
				} else {
					reqCtx.Recorder.Eventf(cr, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s: %s",
						strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
				}
			}

			// our finalizer is present, so lets handle any external dependency
			if deletionHandler != nil {
				if res, err := deletionHandler(); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
					if res == nil {
						res, err := CheckedRequeueWithError(err, reqCtx.Log, "")
						return &res, err
					}
					return res, err
				} else if res != nil {
					return res, nil
				}
			}
			// remove our finalizer from the list and update it.
			if controllerutil.RemoveFinalizer(cr, finalizer) {
				if err := r.Update(reqCtx.Ctx, cr); err != nil {
					res, err := CheckedRequeueWithError(err, reqCtx.Log, "")
					return &res, err
				}
				// record resources deleted event
				if reqCtx.Recorder != nil {
					reqCtx.Recorder.Eventf(cr, corev1.EventTypeNormal, constant.ReasonDeletedCR, "Deleted %s: %s",
						strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
				}
			}
		}

		// Stop reconciliation as the item is being deleted
		res, err := Reconciled()
		return &res, err
	}
	return nil, nil
}

// ValidateReferenceCR validate is exist referencing CRs. if exists, requeue reconcile after 30 seconds
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
			res, err := RequeueAfter(30*time.Second, reqCtx.Log, "")
			return &res, err
		}
	}
	return nil, nil
}

// RecordCreatedEvent record an event when CR created successfully
func RecordCreatedEvent(r record.EventRecorder, cr client.Object) {
	if r != nil && cr.GetGeneration() == 1 {
		r.Eventf(cr, corev1.EventTypeNormal, constant.ReasonCreatedCR, "Created %s: %s", strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
	}
}

// WorkloadFilterPredicate provide filter predicate for workload objects, i.e., deployment/statefulset/pod/pvc.
func WorkloadFilterPredicate(object client.Object) bool {
	objLabels := object.GetLabels()
	if objLabels == nil {
		return false
	}
	return objLabels[constant.AppManagedByLabelKey] == constant.AppName
}

// IgnoreIsAlreadyExists return errors that is not AlreadyExists
func IgnoreIsAlreadyExists(err error) error {
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// BackgroundDeleteObject delete the object in the background, usually used in the Reconcile method
func BackgroundDeleteObject(cli client.Client, ctx context.Context, obj client.Object) error {
	deletePropagation := metav1.DeletePropagationBackground
	deleteOptions := &client.DeleteOptions{
		PropagationPolicy: &deletePropagation,
	}

	if err := cli.Delete(ctx, obj, deleteOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// SetOwnership set owner reference and add finalizer if not exists
func SetOwnership(owner, obj client.Object, scheme *runtime.Scheme, finalizer string) error {
	if err := controllerutil.SetOwnerReference(owner, obj, scheme); err != nil {
		return err
	}
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		// pvc objects do not need to add finalizer
		_, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			controllerutil.AddFinalizer(obj, finalizer)
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
