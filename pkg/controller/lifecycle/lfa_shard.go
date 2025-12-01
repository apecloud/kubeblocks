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

const (
	shardAddShardNameVar    = "KB_SHARD_ADD_SHARD_NAME"
	shardRemoveShardNameVar = "KB_SHARD_REMOVE_SHARD_NAME"
)

type shardAdd struct {
	shardName string
	action    *appsv1.Action
}

var _ lifecycleAction = &shardAdd{}

func (a *shardAdd) name() string {
	return "shardAdd"
}

func (a *shardAdd) parameters(context.Context, client.Reader) (map[string]string, error) {
	m := make(map[string]string)
	m[shardAddShardNameVar] = a.shardName
	return m, nil
}

type shardRemove struct {
	shardName string
	action    *appsv1.Action
}

var _ lifecycleAction = &shardRemove{}

func (a *shardRemove) name() string {
	return "shardRemove"
}

func (a *shardRemove) parameters(context.Context, client.Reader) (map[string]string, error) {
	m := make(map[string]string)
	m[shardRemoveShardNameVar] = a.shardName
	return m, nil
}

type shardPostProvision struct {
	action *appsv1.Action
}

var _ lifecycleAction = &shardPostProvision{}

func (a *shardPostProvision) name() string {
	return "shardPostProvision"
}

func (a *shardPostProvision) parameters(context.Context, client.Reader) (map[string]string, error) {
	return nil, nil
}

type shardPreTerminate struct {
	action *appsv1.Action
}

var _ lifecycleAction = &shardPreTerminate{}

func (a *shardPreTerminate) name() string {
	return "shardPreTerminate"
}

func (a *shardPreTerminate) parameters(context.Context, client.Reader) (map[string]string, error) {
	return nil, nil
}
