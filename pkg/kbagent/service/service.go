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
	"time"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type Service interface {
	Kind() string
	URI() string

	Start() error

	HandleRequest(ctx context.Context, payload []byte) ([]byte, error)
}

func New(logger logr.Logger, actions []proto.Action, probes []proto.Probe) ([]Service, error) {
	sa, err := newActionService(logger, actions)
	if err != nil {
		return nil, err
	}
	sp, err := newProbeService(logger, sa, probes)
	if err != nil {
		return nil, err
	}
	sdp, err := newDataPipeService(logger, sa)
	if err != nil {
		return nil, err
	}
	return []Service{sa, sp, sdp}, nil
}

func RunInPipeMode(services []Service, payload string) error {
	var sdp *dataPipeService
	for _, s := range services {
		if s.Kind() == proto.ServiceDataPipe.Kind {
			sdp = s.(*dataPipeService)
		}
	}
	if sdp == nil {
		return fmt.Errorf("service %s not found", proto.ServiceDataPipe.Kind)
	}

	var err error
	if err = sdp.Start(); err != nil {
		return err
	}
	_, err = sdp.HandleRequest(context.Background(), []byte(payload))
	if err != nil {
		time.Sleep(3600 * time.Second)
	}
	return err
}
