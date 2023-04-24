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

package lifecycle

import (
	"fmt"
	"sync"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type ParallelTransformers struct {
	transformers []graph.Transformer
}

var _ graph.Transformer = &ParallelTransformers{}

func (t *ParallelTransformers) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	var group sync.WaitGroup
	var errs error
	for _, transformer := range t.transformers {
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
