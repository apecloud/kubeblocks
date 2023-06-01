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

//go:generate go run github.com/ahmetb/gen-crd-api-reference-docs -api-dir . -config ../../../hack/docgen/api/gen-api-doc-config.json -template-dir ../../../hack/docgen/api/template -out-file ../../../docs/user_docs/api-reference/add-on.md

// +k8s:deepcopy-gen=package,register
// +k8s:openapi-gen=true
// +groupName=extensions.kubeblocks.io
package v1alpha1
