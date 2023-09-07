package helm

import (
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"io"
	"strings"
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
