/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

// Package kubebuilderx is a new framework builds upon the original DAG framework,
// which abstracts the reconciliation process into two stages.
// The first stage is the pure computation phase, where the goal is to generate an execution plan.
// The second stage is the plan execution phase, responsible for applying the changes computed in the first stage to the K8s API server.
// The design choice of making the first stage purely computational serves two purposes.
// Firstly, it allows leveraging the experience and patterns from functional programming to make the code more robust.
// Secondly, it enables breaking down complex business logic into smaller units, facilitating testing.
// The new framework retains this concept while attempting to address the following issues of the original approach:
// 1. The low-level exposure of the DAG data structure, which should be abstracted away.
// 2. The execution of business logic code being deferred, making step-by-step tracing and debugging challenging.
// Additionally, the new framework further abstracts the concept of object snapshots into an ObjectTree,
// making it easier to apply the experience of editing a group of related objects using kubectl.
//
// KubeBuilderX is in its very early stage.
package kubebuilderx
