package httpserver

import (
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"os"
	"testing"
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
		// 如果path 文件存在，删除
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
