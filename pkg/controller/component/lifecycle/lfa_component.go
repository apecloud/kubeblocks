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

package lifecycle

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type postProvision struct {
	namespace   string
	clusterName string
	compName    string
}

var _ lifecycleAction = &postProvision{}

func (a *postProvision) name() string {
	return "postProvision"
}

func (a *postProvision) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	return nil, nil
}

type preTerminate struct {
	namespace   string
	clusterName string
	compName    string
}

var _ lifecycleAction = &preTerminate{}

func (a *preTerminate) name() string {
	return "preTerminate"
}

func (a *preTerminate) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	return nil, nil
}
