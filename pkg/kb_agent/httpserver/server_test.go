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

package httpserver

import (
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

func TestNewServer(t *testing.T) {
	s := NewServer()
	assert.NotNil(t, s)
}

func TestStartNonBlocking(t *testing.T) {

	t.Run("StartNonBlocking unix domain socket", func(t *testing.T) {
		config := Config{
			Port:             KBAgentDefaultPort,
			Address:          "0.0.0.0",
			ConCurrency:      0,
			UnixDomainSocket: ".",
			APILogging:       true,
		}
		s := &server{
			config:    config,
			endpoints: Endpoints(),
		}
		_ = os.Remove(config.UnixDomainSocket + "/kb_agent.socket")
		err := s.StartNonBlocking()
		assert.Nil(t, err)
		err = s.Close()
		assert.Nil(t, err)
	})

	t.Run("StartNonBlocking tcp socket", func(t *testing.T) {
		config := Config{
			Port:             KBAgentDefaultPort,
			Address:          "0.0.0.0",
			ConCurrency:      KBAgentDefaultConcurrency,
			UnixDomainSocket: "",
			APILogging:       true,
		}
		s := &server{
			config: config,
			endpoints: []Endpoint{
				{
					Route:       util.Path,
					Method:      fasthttp.MethodPost,
					Version:     util.Version,
					Handler:     actionHandler,
					LegacyRoute: "test_legacy_route",
				},
			},
		}
		err := s.StartNonBlocking()
		assert.Nil(t, err)
		err = s.Close()
		assert.Nil(t, err)
	})

	t.Run("StartNonBlocking zero listeners", func(t *testing.T) {
		config := Config{
			Port:             -1,
			Address:          "0.0.0.0",
			ConCurrency:      KBAgentDefaultConcurrency,
			UnixDomainSocket: "",
			APILogging:       false,
		}
		s := &server{
			config:    config,
			endpoints: Endpoints(),
		}
		err := s.StartNonBlocking()
		assert.Error(t, err, errors.New("no endpoint to listen on"))
	})
}
