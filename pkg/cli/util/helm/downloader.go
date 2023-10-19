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

package helm

import (
	"io"
	"strings"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
)

func NewDownloader(cfg *Config) (*downloader.ChartDownloader, error) {
	var err error
	var out strings.Builder

	settings := cli.New()
	settings.SetNamespace(cfg.namespace)
	settings.KubeConfig = cfg.kubeConfig
	if cfg.kubeContext != "" {
		settings.KubeContext = cfg.kubeContext
	}
	settings.Debug = cfg.debug
	client, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(io.Discard),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, err
	}
	chartsDownloaders := &downloader.ChartDownloader{
		Out:            &out,
		Verify:         downloader.VerifyNever,
		Getters:        getter.All(settings),
		Options:        []getter.Option{},
		RegistryClient: client,
	}
	return chartsDownloaders, nil
}
