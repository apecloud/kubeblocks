/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"context"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/container"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
)

type reconfigureProxy struct {
	cfgproto.ReconfigureServer
	updater cfgcm.DynamicUpdater

	ctx    context.Context
	opt    ReconfigureServiceOptions
	killer cfgutil.ContainerKiller

	logger *zap.SugaredLogger
}

var stopContainerSignal = viper.GetString(cfgutil.KillContainerSignalEnvName)

func (r *reconfigureProxy) Init(opt *VolumeWatcherOpts) error {
	if err := r.initOnlineUpdater(opt); err != nil {
		r.logger.Errorf("init online updater failed: %+v", err)
		return err
	}
	if err := r.initContainerKiller(); err != nil {
		r.logger.Errorf("init container killer failed: %+v", err)
		return err
	}
	return nil
}

func (r *reconfigureProxy) initContainerKiller() error {
	if !r.opt.ContainerRuntimeEnable {
		r.logger.Info("container killer is disabled.")
		return nil
	}

	killer, err := cfgutil.NewContainerKiller(r.opt.ContainerRuntime, r.opt.RuntimeEndpoint, r.logger)
	if err != nil {
		return cfgcore.WrapError(err, "failed to create container killer")
	}
	if err := killer.Init(r.ctx); err != nil {
		return cfgcore.WrapError(err, "failed to init killer")
	}
	r.killer = killer
	return nil
}

func (r *reconfigureProxy) StopContainer(ctx context.Context, request *cfgproto.StopContainerRequest) (*cfgproto.StopContainerResponse, error) {
	if r.killer == nil {
		return nil, cfgcore.MakeError("container killer is not initialized.")
	}
	ds := request.GetContainerIDs()
	if len(ds) == 0 {
		return &cfgproto.StopContainerResponse{ErrMessage: "not any containerId."}, nil
	}
	if err := r.killer.Kill(ctx, ds, stopContainerSignal, nil); err != nil {
		return nil, err
	}
	return &cfgproto.StopContainerResponse{}, nil
}

func (r *reconfigureProxy) OnlineUpgradeParams(_ context.Context, request *cfgproto.OnlineUpgradeParamsRequest) (*cfgproto.OnlineUpgradeParamsResponse, error) {
	if r.updater == nil {
		return nil, cfgcore.MakeError("online updater is not initialized.")
	}
	params := request.GetParams()
	if len(params) == 0 {
		return nil, cfgcore.MakeError("update params not empty.")
	}
	if err := r.updater(params); err != nil {
		return nil, err
	}
	return &cfgproto.OnlineUpgradeParamsResponse{}, nil
}

func (r *reconfigureProxy) initOnlineUpdater(opt *VolumeWatcherOpts) error {
	if opt.NotifyHandType != TPLScript || !r.opt.RemoteOnlineUpdateEnable {
		return nil
	}

	updater, err := cfgcm.OnlineUpdateParamsHandle(opt.TPLScriptPath, opt.FormatterConfig)
	if err != nil {
		return cfgcore.WrapError(err, "failed to create online updater")
	}
	r.updater = updater
	return nil
}
