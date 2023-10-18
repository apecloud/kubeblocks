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

package model

import (
	"fmt"
	"sync"

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

// ParallelTransformer executes a group of transformers in parallel.
// TODO: make DAG thread-safe if ParallelTransformer called.
type ParallelTransformer struct {
	Transformers []graph.Transformer
}

func (t *ParallelTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	var group sync.WaitGroup
	var errs error
	for _, transformer := range t.Transformers {
		transformer := transformer
		group.Add(1)
		go func() {
			err := transformer.Transform(ctx, dag)
			if err != nil {
				// TODO: sync.Mutex errs
				errs = fmt.Errorf("%v; %v", errs, err)
			}
			group.Done()
		}()
	}
	group.Wait()
	return errs
}

var _ graph.Transformer = &ParallelTransformer{}
