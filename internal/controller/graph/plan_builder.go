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

// PlanBuilder builds a Plan by applying a group of Transformer to an empty DAG.
type PlanBuilder interface {
	// Init loads the primary object to be reconciled, and does meta initialization
	Init() error

	// AddTransformer adds transformers to the builder in sequence order.
	// And the transformers will be executed in the add order.
	AddTransformer(transformer ...Transformer) PlanBuilder

	// AddParallelTransformer adds transformers to the builder.
	// And the transformers will be executed in parallel.
	AddParallelTransformer(transformer ...Transformer) PlanBuilder

	// Build runs all the transformers added by AddTransformer and/or AddParallelTransformer.
	Build() (Plan, error)
}

// Plan defines the final actions should be executed.
type Plan interface {
	// Execute the plan
	Execute() error
}
