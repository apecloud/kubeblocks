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

package types

import (
	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OTeldParams struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// ReconcileCtx wrapper for reconcile procedure context parameters
type ReconcileCtx struct {
	intctrlutil.RequestCtx

	OTeld *v1alpha1.OTeld

	// Config    *Config
	// Namespace string
	OteldCfgRef OteldCfgRef
}

type ReconcileTask interface {
	Do(reqCtx ReconcileCtx) error
}

type ReconcileFunc func(reqCtx ReconcileCtx) error

func (f ReconcileFunc) Do(reqCtx ReconcileCtx) error {
	return f(reqCtx)
}

type baseTask struct {
	ReconcileFunc
}

var errNilFunc = cfgcore.MakeError("nil reconcile func")

func NewReconcileTask(name string, task ReconcileFunc) ReconcileTask {
	if task == nil {
		// not walk here
		panic(errNilFunc)
	}
	newTask := func(reqCtx ReconcileCtx) error {
		reqCtx = ReconcileCtx{
			RequestCtx: intctrlutil.RequestCtx{
				Ctx: reqCtx.Ctx,
				Req: reqCtx.Req,
				Log: reqCtx.Log.WithValues("subTask", name),
			},
			// Config:      reqCtx.Config,
			// Namespace:   reqCtx.Namespace,
			OteldCfgRef: reqCtx.OteldCfgRef,
		}
		return task(reqCtx)
	}
	return baseTask{ReconcileFunc: newTask}
}
