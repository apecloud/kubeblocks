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

package multicluster

import (
	"fmt"

	"golang.org/x/exp/maps"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type Manager interface {
	GetClient() client.Client

	GetContexts() []string

	Bind(mgr ctrl.Manager) error

	Watch(b *builder.Builder, obj client.Object, eventHandler handler.EventHandler) Manager
}

type manager struct {
	cli    client.Client
	caches map[string]cache.Cache
}

var _ Manager = &manager{}

func (m *manager) GetClient() client.Client {
	return m.cli
}

func (m *manager) GetContexts() []string {
	return maps.Keys(m.caches)
}

func (m *manager) Bind(mgr ctrl.Manager) error {
	for k := range m.caches {
		if err := mgr.Add(m.caches[k]); err != nil {
			return fmt.Errorf("failed to bind cache to Manager: %s", err.Error())
		}
	}
	return nil
}

func (m *manager) Watch(b *builder.Builder, obj client.Object, eventHandler handler.EventHandler) Manager {
	for k := range m.caches {
		b.WatchesRawSource(source.Kind(m.caches[k], obj), eventHandler)
	}
	return m
}
