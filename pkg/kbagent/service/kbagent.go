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
	"fmt"
)

const (
	version = "v1.0"
	uri     = "action"
)

type kbagent struct {
	action *actionService
	probe  *probeService
}

var _ Service = &kbagent{}

func (s *kbagent) Version() string {
	return version
}

func (s *kbagent) URI() string {
	return uri
}

func (s *kbagent) Start() error {
	if s.probe != nil {
		return s.probe.start()
	}
	return nil
}

func (s *kbagent) Call(ctx context.Context, action string, parameters map[string]string) ([]byte, error) {
	if s.action != nil {
		return s.action.call(ctx, action, parameters)
	}
	return nil, fmt.Errorf("%s is not supported", action)
}
