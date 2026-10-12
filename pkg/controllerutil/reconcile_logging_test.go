/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestLogReconcile(t *testing.T) {
	tests := []struct {
		name      string
		verbosity int
		err       error
		want      string
	}{
		{"success", 1, nil, "Reconciliation completed"},
		{"failure", 0, errors.New("API unavailable"), "Reconciliation failed"},
		{"conflict", 1, apierrors.NewConflict(schema.GroupResource{Resource: "clusters"}, "test", errors.New("stale")), "Reconciliation will retry"},
		{"waiting", 1, NewRequeueError(time.Second, "waiting for pods"), "Reconciliation will retry"},
		{"quiet success", 0, nil, ""},
		{"quiet waiting", 0, NewRequeueError(time.Second, "waiting for pods"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entries []string
			logger := funcr.New(func(_, entry string) { entries = append(entries, entry) }, funcr.Options{Verbosity: tt.verbosity})
			finish := LogReconcile(log.IntoContext(context.Background(), logger), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test"}}, "ClusterReconciler")
			finish(ctrl.Result{RequeueAfter: time.Second}, tt.err)
			output := strings.Join(entries, "\n")
			if tt.want == "" {
				if output != "" {
					t.Fatalf("expected quiet polling, got %s", output)
				}
				return
			}
			for _, field := range []string{tt.want, "ClusterReconciler", "default", "test", "duration", "requeueAfter"} {
				if !strings.Contains(output, field) {
					t.Errorf("missing %q in %s", field, output)
				}
			}
		})
	}
}
