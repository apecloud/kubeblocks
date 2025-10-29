/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type shardAdd struct {
	namespace   string
	clusterName string
	compName    string
	action      *appsv1.Action
}

var _ lifecycleAction = &shardAdd{}

func (a *shardAdd) name() string {
	return "shardAdd"
}

func (a *shardAdd) parameters(context.Context, client.Reader) (map[string]string, error) {
	return nil, nil
}

type shardRemove struct {
	namespace   string
	clusterName string
	compName    string
	action      *appsv1.Action
}

var _ lifecycleAction = &shardRemove{}

func (a *shardRemove) name() string {
	return "shardRemove"
}

func (a *shardRemove) parameters(context.Context, client.Reader) (map[string]string, error) {
	return nil, nil
}
