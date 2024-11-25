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

package kbagent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/server"
	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

const (
	ContainerName        = "kbagent"
	ContainerName4Worker = "kbagent-worker"
	InitContainerName    = "init-kbagent"

	DefaultHTTPPortName      = "http"
	DefaultStreamingPortName = "streaming"

	DefaultHTTPPort      = 3501
	DefaultStreamingPort = 3502

	actionEnvName    = "KB_AGENT_ACTION"
	probeEnvName     = "KB_AGENT_PROBE"
	streamingEnvName = "KB_AGENT_STREAMING"
	taskEnvName      = "KB_AGENT_TASK"
)

func BuildEnv4Server(actions []proto.Action, probes []proto.Probe, streaming []string) ([]corev1.EnvVar, error) {
	da, dp, err := serializeActionNProbe(actions, probes)
	if err != nil {
		return nil, err
	}
	envVars := make([]corev1.EnvVar, 0)
	if len(da) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  actionEnvName,
			Value: da,
		})
	}
	if len(dp) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  probeEnvName,
			Value: dp,
		})
	}
	if len(streaming) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  streamingEnvName,
			Value: strings.Join(streaming, ","),
		})
	}
	return append(util.DefaultEnvVars(), envVars...), nil
}

func BuildEnv4Worker(tasks []proto.Task) (*corev1.EnvVar, error) {
	dt, err := serializeTask(tasks)
	if err != nil {
		return nil, err
	}
	return &corev1.EnvVar{
		Name:  taskEnvName,
		Value: dt,
	}, nil
}

func UpdateEnv4Worker(envVars map[string]string, f func(proto.Task) *proto.Task) (*corev1.EnvVar, error) {
	if envVars == nil {
		return nil, nil
	}
	dt, ok := envVars[taskEnvName]
	if !ok || len(dt) == 0 {
		return nil, nil // has no task
	}

	tasks, err := deserializeTask(dt)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(tasks); i++ {
		if f != nil {
			task := f(tasks[i])
			if task != nil {
				tasks[i] = *task
			} else {
				tasks = append(tasks[:i], tasks[i+1:]...)
				i--
			}
		}
	}

	dt, err = serializeTask(tasks)
	if err != nil {
		return nil, err
	}
	return &corev1.EnvVar{
		Name:  taskEnvName,
		Value: dt,
	}, nil
}

func Launch(logger logr.Logger, config server.Config) (bool, error) {
	envVars := util.EnvL2M(os.Environ())

	// initialize kb-agent
	services, err := initialize(logger, envVars)
	if err != nil {
		return false, errors.Wrap(err, "init action handlers failed")
	}
	if config.Server {
		return true, runAsServer(logger, config, services)
	}
	return false, runAsWorker(logger, services, envVars)
}

func initialize(logger logr.Logger, envVars map[string]string) ([]service.Service, error) {
	da, dp, ds := getActionProbeNStreamingEnvValues(envVars)
	if len(da) == 0 {
		return nil, nil
	}

	actions, probes, err := deserializeActionNProbe(da, dp)
	if err != nil {
		return nil, err
	}

	var streaming []string
	if len(ds) > 0 {
		streaming = strings.Split(ds, ",")
	}
	return service.New(logger, actions, probes, streaming)
}

func getActionProbeNStreamingEnvValues(envVars map[string]string) (string, string, string) {
	da, ok := envVars[actionEnvName]
	if !ok {
		return "", "", ""
	}
	dp, ok := envVars[probeEnvName]
	if !ok {
		return da, "", ""
	}
	ds, ok := envVars[streamingEnvName]
	if !ok {
		return da, dp, ""
	}
	return da, dp, ds
}

func serializeActionNProbe(actions []proto.Action, probes []proto.Probe) (string, string, error) {
	da, err := json.Marshal(actions)
	if err != nil {
		return "", "", nil
	}
	dp, err := json.Marshal(probes)
	if err != nil {
		return "", "", nil
	}
	return string(da), string(dp), nil
}

func deserializeActionNProbe(da, dp string) ([]proto.Action, []proto.Probe, error) {
	actions := make([]proto.Action, 0)
	if err := json.Unmarshal([]byte(da), &actions); err != nil {
		return nil, nil, err
	}
	probes := make([]proto.Probe, 0)
	if len(dp) > 0 {
		if err := json.Unmarshal([]byte(dp), &probes); err != nil {
			return nil, nil, err
		}
	}
	return actions, probes, nil
}

func runAsServer(logger logr.Logger, config server.Config, services []service.Service) error {
	if config.Port == config.StreamingPort {
		return errors.New("HTTP port and streaming port are the same")
	}

	// start all services first
	for i := range services {
		if err := services[i].Start(); err != nil {
			logger.Error(err, fmt.Sprintf("start service %s failed", services[i].Kind()))
			return err
		}
		logger.Info(fmt.Sprintf("service %s started...", services[i].Kind()))
	}

	// start the HTTP server
	httpServer := server.NewHTTPServer(logger, config, services)
	err := httpServer.StartNonBlocking()
	if err != nil {
		return errors.Wrap(err, "failed to start the HTTP server")
	}

	// start the streaming server
	streamingServer := server.NewStreamingServer(logger, config, streamingService(services))
	err = streamingServer.StartNonBlocking()
	if err != nil {
		return errors.Wrap(err, "failed to start the streaming server")
	}
	return nil
}

func runAsWorker(logger logr.Logger, services []service.Service, envVars map[string]string) error {
	dt, ok := envVars[taskEnvName]
	if !ok || len(dt) == 0 {
		return nil // has no task
	}

	logger.Info(fmt.Sprintf("running as worker, tasks: %s", dt))

	tasks, err := deserializeTask(dt)
	if err != nil {
		return err
	}

	if err := service.RunTasks(logger, actionService(services), tasks); err != nil {
		return errors.Wrap(err, "failed to run as worker")
	}
	return nil
}

func actionService(services []service.Service) service.Service {
	for i, s := range services {
		if s.Kind() == proto.ServiceAction.Kind {
			return services[i]
		}
	}
	return nil
}

func streamingService(services []service.Service) service.Service {
	for i, s := range services {
		if s.Kind() == proto.ServiceStreaming.Kind {
			return services[i]
		}
	}
	return nil
}

func serializeTask(tasks []proto.Task) (string, error) {
	dt, err := json.Marshal(tasks)
	if err != nil {
		return "", nil
	}
	return string(dt), nil
}

func deserializeTask(dt string) ([]proto.Task, error) {
	tasks := make([]proto.Task, 0)
	if err := json.Unmarshal([]byte(dt), &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}
