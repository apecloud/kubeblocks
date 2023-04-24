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

// ErrFastReturn is used to stop the Transformer chain for some purpose.
// Use it in Transformer.Transform when all jobs have done and no need to run following transformers
var ErrFastReturn = errors.New("fast return")

// ApplyTo applies TransformerChain t to dag
func (t *TransformerChain) ApplyTo(ctx TransformContext, dag *DAG) error {
	if t == nil {
		return nil
	}
	for _, transformer := range *t {
		if err := transformer.Transform(ctx, dag); err != nil {
			return fastReturnErrorToNil(err)
		}
	}
	return nil
}

func fastReturnErrorToNil(err error) error {
	if err == ErrFastReturn {
		return nil
	}
	return err
}
