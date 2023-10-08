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

package dataprotection

import (
	"context"
	"time"

	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	ctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/internal/dataprotection/utils"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

// GCReconciler deletes expired backups.
type GCReconciler struct {
	client.Client
	Recorder record.EventRecorder
	clock    clock.WithTickerAndDelayedExecution
}

// SetupWithManager sets up the GCReconciler using the supplied manager.
// GCController only watches on CreateEvent for ensuring every new backup will be
// taken care of. Other events will be filtered to decrease the load on the controller.
func (r *GCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	s := dputils.NewPeriodicalEnqueueSource(mgr.GetClient(), &dpv1alpha1.BackupList{},
		getGCFrequency(), dputils.PeriodicalEnqueueSourceOption{})
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.Backup{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(_ event.UpdateEvent) bool {
				return false
			},
			DeleteFunc: func(_ event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				return false
			},
		})).
		WatchesRawSource(s, nil).
		Complete(r)
}

func (r *GCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrlutil.Reconciled()
}

func getGCFrequency() time.Duration {
	gcFrequencySeconds := viper.GetInt(dptypes.CfgKeyGCFrequencySeconds)
	if gcFrequencySeconds > 0 {
		return time.Duration(gcFrequencySeconds) * time.Second
	}
	return dptypes.DefaultGCFrequencySeconds
}
