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

/*
Package graph tries to model the controller reconciliation loop in a more structure way.
It structures the reconciliation loop to 4 stage: Init, Validate, Build and Execute.

# Initialization Stage

the Init stage is for meta loading, object query etc.
Try loading infos that used in the following stages.

# Validation Stage

Validating everything (object spec is legal, resources in K8s cluster are enough etc.) in this stage
to make sure the following Build and Execute stages can go well.

# Building Stage

The Build stage's target is to generate an execution plan.
The plan is composed by a DAG which represents the actions that should be taken on all K8s native objects owned by the controller,
a group of Transformers which transform the initial DAG to the final one,
and a WalkFunc which does the real action when the final DAG is walked through.

# Execution Stage

The plan is executed in this stage, all the object manipulations(create/update/delete) are committed.
*/
package graph
