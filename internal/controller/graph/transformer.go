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

package graph

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"

	"github.com/apecloud/kubeblocks/internal/controller/client"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// TransformContext is used by Transformer.Transform
type TransformContext interface {
	GetContext() context.Context
	GetClient() client.ReadonlyClient
	GetRecorder() record.EventRecorder
	GetLogger() logr.Logger
}

// Transformer transforms a DAG to a new version
type Transformer interface {
	Transform(ctx TransformContext, dag *DAG) error
}

// TransformerChain chains a group Transformer together
type TransformerChain []Transformer

// ErrPrematureStop is used to stop the Transformer chain for some purpose.
// Use it in Transformer.Transform when all jobs have done and no need to run following transformers
var ErrPrematureStop = errors.New("Premature-Stop")

// ApplyTo applies TransformerChain t to dag
func (r TransformerChain) ApplyTo(ctx TransformContext, dag *DAG) error {
	var delayedError error
	for _, transformer := range r {
		if err := transformer.Transform(ctx, dag); err != nil {
			if intctrlutil.IsDelayedRequeueError(err) {
				if delayedError == nil {
					delayedError = err
				}
				continue
			}
			return ignoredIfPrematureStop(err)
		}
	}
	return delayedError
}

func ignoredIfPrematureStop(err error) error {
	if err == ErrPrematureStop {
		return nil
	}
	return err
}
