/*
Copyright 2022.

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
	"errors"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

var tlog = ctrl.Log.WithName("controller_testing")

func TestRequeueWithError(t *testing.T) {
	_, err := CheckedRequeueWithError(errors.New("test error"), tlog, "test")
	if err == nil {
		t.Error("Expected error to fall through, got nil")
	}
}

func TestRequeueWithNotFoundError(t *testing.T) {
	notFoundErr := apierrors.NewNotFound(schema.GroupResource{
		Resource: "Pod",
	}, "no-body")
	_, err := CheckedRequeueWithError(notFoundErr, tlog, "test")
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}
}

func TestRequeueAfter(t *testing.T) {
	_, err := RequeueAfter(time.Millisecond, tlog, "test")
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}
}

func TestRequeue(t *testing.T) {
	res, err := Requeue(tlog, "test")
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}
	if !res.Requeue {
		t.Error("Expected requeue to be true, got false")
	}
}

func TestReconciled(t *testing.T) {
	res, err := Reconciled()
	if err != nil {
		t.Error("Expected error to be nil, got:", err)
	}
	if res.Requeue {
		t.Error("Expected requeue to be false, got true")
	}
}
