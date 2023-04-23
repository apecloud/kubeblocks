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

/*
Package graph tries to model the controller reconciliation loop in a more structured way.
It structures the reconciliation loop to 3 stage: Init, Build and Execute.

# Initialization Stage

the Init stage is for meta loading, object query etc.
Try loading infos that used in the following stages.

# Building Stage

## Validation

The first part of Building is Validation,
which Validates everything (object spec is legal, resources in K8s cluster are enough etc.)
to make sure the following Build and Execute parts can go well.

## Building
The Building part's target is to generate an execution plan.
The plan is composed by a DAG which represents the actions that should be taken on all K8s native objects owned by the controller,
a group of Transformers which transform the initial DAG to the final one,
and a WalkFunc which does the real action when the final DAG is walked through.

# Execution Stage

The plan is executed in this stage, all the object manipulations(create/update/delete) are committed.
*/
package graph
