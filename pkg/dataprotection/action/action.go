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

package action

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type Action interface {
	// Execute executes the action.
	Execute(ctx Context) (*dpv1alpha1.ActionStatus, error)

	// GetName returns the Name of the action.
	GetName() string

	// Type returns the type of the action.
	Type() dpv1alpha1.ActionType
}

type Context struct {
	Ctx      context.Context
	Client   client.Client
	Recorder record.EventRecorder

	Scheme           *runtime.Scheme
	RestClientConfig *rest.Config
}
