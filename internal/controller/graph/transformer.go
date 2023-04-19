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
