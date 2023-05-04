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
