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

package service

import (
	"context"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type Service interface {
	Start() error

	Version() string
	URI() string

	// Refresh(actions *appsv1alpha1.ComponentLifecycleActions) error

	Call(ctx context.Context, action string, parameters map[string]string) ([]byte, error)
}

func NewService(actions *appsv1alpha1.ComponentLifecycleActions) Service {
	return &kbagent{
		action: &actionService{
			actions: actions,
		},
		probe: &probeService{
			actions: actions,
		},
	}
}

func dispatch(actions *appsv1alpha1.ComponentLifecycleActions) (map[string]*appsv1alpha1.Action, map[string]*appsv1alpha1.Action) {
	if actions == nil {
		return nil, nil
	}

	actions := map[string]*appsv1alpha1.Action{}
	probes := map[string]*appsv1alpha1.Action{}
	if actions.PostProvision != nil {

	}
	return actions, probes
}
