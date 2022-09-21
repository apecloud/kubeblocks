/*
Copyright 2022 The Kubeblocks Authors

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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/conversion"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	if msg == "" {
		logger.Info(err.Error())
	} else {
		// Info log the error message and then let the reconciler dump the stacktrace
		logger.Info(msg, keysAndValues...)
	}
	return reconcile.Result{}, err
}

func RequeueAfter(duration time.Duration, logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if msg != "" {
		logger.Info(msg, keysAndValues...)
	} else {
		logger.V(1).Info("retry-after", "duration", duration)
	}
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: duration,
	}, nil
}

func Requeue(logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if msg != "" {
		logger.Info(msg, keysAndValues...)
	} else {
		logger.V(1).Info("requeue")
	}
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
	labelKey string, statusHandler func() error, objLists ...client.ObjectList) (*ctrl.Result, error) {
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
			if statusHandler != nil {
				if err = statusHandler(); err != nil {
					return nil, err
				}
			}
			res, err := RequeueAfter(30*time.Second, reqCtx.Log, "")
			return &res, err
		}
	}
	return nil, nil
}
